package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/RevCBH/choo/internal/cli/tui"
	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/feature"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
	"github.com/RevCBH/choo/internal/orchestrator"
	"github.com/RevCBH/choo/internal/web"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// RunOptions holds flags for the run command
type RunOptions struct {
	Parallelism  int    // Max concurrent units (default: 4)
	TargetBranch string // Branch PRs target (default: main)
	DryRun       bool   // Show execution plan without running
	NoPR         bool   // Skip PR creation
	Unit         string // Run only specified unit (single-unit mode)
	SkipReview   bool   // Auto-merge without waiting for review
	TasksDir     string // Path to specs/tasks/ directory
	Web          bool   // Enable web UI event forwarding
	WebSocket    string // Custom Unix socket path (optional)
	NoTUI        bool   // Disable TUI even when stdout is a TTY
	Feature      string // PRD ID to work on in feature mode

	// Provider is the default provider for task execution
	// Units without frontmatter override use this provider
	Provider string

	// ForceTaskProvider overrides all provider settings for task inner loops
	// When set, ignores per-unit frontmatter provider field
	ForceTaskProvider string
}

// Validate checks RunOptions for validity
func (opts RunOptions) Validate() error {
	if opts.Parallelism <= 0 {
		return fmt.Errorf("parallelism must be greater than 0, got %d", opts.Parallelism)
	}
	if opts.TasksDir == "" {
		return fmt.Errorf("tasks directory must not be empty")
	}

	// Validate provider flags
	if opts.Provider != "" {
		if err := config.ValidateProviderType(opts.Provider); err != nil {
			return fmt.Errorf("invalid --provider: %w", err)
		}
	}
	if opts.ForceTaskProvider != "" {
		if err := config.ValidateProviderType(opts.ForceTaskProvider); err != nil {
			return fmt.Errorf("invalid --force-task-provider: %w", err)
		}
	}

	return nil
}

// NewRunCmd creates the run command
func NewRunCmd(app *App) *cobra.Command {
	opts := RunOptions{
		Parallelism:       4,
		TargetBranch:      "main",
		DryRun:            false,
		NoPR:              false,
		Unit:              "",
		SkipReview:        false,
		TasksDir:          "specs/tasks",
		Provider:          "", // Empty means use default from config/env
		ForceTaskProvider: "", // Empty means respect per-unit settings
	}

	cmd := &cobra.Command{
		Use:   "run [tasks-dir]",
		Short: "Execute orchestration with configured units",
		Long: `Run executes the orchestration loop, processing development units in parallel.

By default, runs all units found in specs/tasks/ with parallelism of 4.
Use --unit to run a single unit, or --dry-run to preview execution plan.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Override tasks-dir from positional arg if provided
			if len(args) > 0 {
				opts.TasksDir = args[0]
			}

			// Create context
			ctx := context.Background()

			// If --target wasn't explicitly set, use current branch
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			if !cmd.Flags().Changed("target") {
				currentBranch, err := git.GetCurrentBranch(ctx, wd)
				if err != nil {
					// Fall back to "main" if we can't detect current branch
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not detect current branch (%v), using 'main'\n", err)
					opts.TargetBranch = "main"
				} else {
					opts.TargetBranch = currentBranch
				}
			}

			// Ensure target branch exists on remote (auto-push if not)
			if !opts.DryRun {
				pushed, err := git.EnsureBranchOnRemote(ctx, wd, opts.TargetBranch)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not verify target branch on remote: %v\n", err)
				} else if pushed {
					fmt.Fprintf(cmd.OutOrStdout(), "Pushed target branch '%s' to remote\n", opts.TargetBranch)
				}
			}

			// Validate options
			if err := opts.Validate(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				os.Exit(2)
			}

			// Run orchestrator
			return app.RunOrchestrator(ctx, opts)
		},
	}

	// Add flags
	cmd.Flags().IntVarP(&opts.Parallelism, "parallelism", "p", 4, "Max concurrent units")
	cmd.Flags().StringVarP(&opts.TargetBranch, "target", "t", "main", "Branch PRs target (default: current branch)")
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "n", false, "Show execution plan without running")
	cmd.Flags().BoolVar(&opts.NoPR, "no-pr", false, "Skip PR creation")
	cmd.Flags().StringVar(&opts.Unit, "unit", "", "Run only specified unit (single-unit mode)")
	cmd.Flags().BoolVar(&opts.SkipReview, "skip-review", false, "Auto-merge without waiting for review")
	cmd.Flags().BoolVar(&opts.Web, "web", false, "Enable web UI event forwarding")
	cmd.Flags().StringVar(&opts.WebSocket, "web-socket", "", "Custom Unix socket path (default: ~/.choo/web.sock)")
	cmd.Flags().BoolVar(&opts.NoTUI, "no-tui", false, "Disable interactive TUI (use summary-only output)")
	cmd.Flags().StringVar(&opts.Feature, "feature", "", "PRD ID for feature mode (targets feature branch)")

	// Provider flags
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "Default provider for task execution (claude, codex). Units without frontmatter override use this.")
	cmd.Flags().StringVar(&opts.ForceTaskProvider, "force-task-provider", "", "Force provider for ALL task execution, ignoring per-unit frontmatter (claude, codex)")

	return cmd
}

// RunOrchestrator executes the main orchestration loop
func (a *App) RunOrchestrator(ctx context.Context, opts RunOptions) error {
	// Validate options (defensive)
	if err := opts.Validate(); err != nil {
		return err
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Setup signal handler
	handler := NewSignalHandler(cancel)
	handler.OnShutdown(func() {
		fmt.Fprintln(os.Stderr, "\nShutting down gracefully...")
	})
	handler.Start()
	defer handler.Stop()

	// Load configuration
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.LoadConfig(wd)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create event bus
	eventBus := events.NewBus(1000)
	defer eventBus.Close()

	// Create SocketPusher if --web flag is set
	var pusher *web.SocketPusher
	if opts.Web {
		pusherCfg := web.DefaultPusherConfig()
		if opts.WebSocket != "" {
			pusherCfg.SocketPath = opts.WebSocket
		}
		pusher = web.NewSocketPusher(eventBus, pusherCfg)
		defer pusher.Close()

		// Start pusher - failure is non-fatal
		if err := pusher.Start(ctx); err != nil {
			log.Printf("WARN: failed to start web pusher: %v", err)
			pusher = nil // Don't use a pusher that failed to start
		}
	}
	_ = pusher // pusher is available for future graph integration

	// Determine if we should use TUI
	useTUI := !opts.NoTUI && !opts.DryRun && term.IsTerminal(int(os.Stdout.Fd()))

	// Set up TUI if enabled
	var tuiProgram *tea.Program
	var tuiBridge *tui.Bridge
	if useTUI {
		model := tui.NewModel(0, opts.Parallelism) // totalUnits set via OrchStarted event
		tuiProgram = tea.NewProgram(model, tea.WithAltScreen())
		tuiBridge = tui.NewBridge(tuiProgram)
		eventBus.Subscribe(tuiBridge.Handler())

		// Run TUI in background
		go func() {
			if _, err := tuiProgram.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			}
		}()
		defer func() {
			tuiBridge.SendDone()
		}()
	}

	// Create Git WorktreeManager
	gitManager := git.NewWorktreeManager(wd, nil)

	// Create GitHub PRClient (only if not dry-run, as it requires GitHub config)
	var ghClient *github.PRClient
	if !opts.DryRun {
		pollInterval, err := cfg.ReviewPollIntervalDuration()
		if err != nil {
			return fmt.Errorf("invalid review poll interval: %w", err)
		}
		reviewTimeout, err := cfg.ReviewTimeoutDuration()
		if err != nil {
			return fmt.Errorf("invalid review timeout: %w", err)
		}
		ghClient, err = github.NewPRClient(github.PRClientConfig{
			Owner:         cfg.GitHub.Owner,
			Repo:          cfg.GitHub.Repo,
			PollInterval:  pollInterval,
			ReviewTimeout: reviewTimeout,
		})
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}
	}

	// Create escalator (terminal by default)
	esc := escalate.NewTerminal()

	// Build orchestrator config from CLI options and loaded config
	orchCfg := orchestrator.Config{
		Parallelism:     opts.Parallelism,
		TargetBranch:    opts.TargetBranch,
		TasksDir:        opts.TasksDir,
		RepoRoot:        wd,
		WorktreeBase:    cfg.Worktree.BasePath,
		NoPR:            opts.NoPR,
		SkipReview:      opts.SkipReview,
		SingleUnit:      opts.Unit,
		DryRun:          opts.DryRun,
		ShutdownTimeout: orchestrator.DefaultShutdownTimeout,
		SuppressOutput:  useTUI,
	}

	// Configure feature mode if --feature flag provided
	if opts.Feature != "" {
		gitClient := git.NewClient(wd)
		branchMgr := feature.NewBranchManager(gitClient, cfg.Feature.BranchPrefix)

		orchCfg.FeatureMode = true
		orchCfg.FeatureBranch = branchMgr.GetBranchName(opts.Feature)

		// Ensure feature branch exists
		exists, err := branchMgr.Exists(ctx, opts.Feature)
		if err != nil {
			return fmt.Errorf("checking feature branch: %w", err)
		}
		if !exists {
			// Create the feature branch from main
			if err := branchMgr.Create(ctx, opts.Feature, orchCfg.TargetBranch); err != nil {
				return fmt.Errorf("creating feature branch: %w", err)
			}
		}
	}

	// Create orchestrator
	orch := orchestrator.New(orchCfg, orchestrator.Dependencies{
		Bus:       eventBus,
		Escalator: esc,
		Git:       gitManager,
		GitHub:    ghClient,
	})
	defer orch.Close()

	// Run orchestrator
	result, err := orch.Run(ctx)

	// Print summary
	if result != nil {
		fmt.Printf("\nOrchestration complete:\n")
		fmt.Printf("  Total units:     %d\n", result.TotalUnits)
		fmt.Printf("  Completed:       %d\n", result.CompletedUnits)
		fmt.Printf("  Failed:          %d\n", result.FailedUnits)
		fmt.Printf("  Blocked:         %d\n", result.BlockedUnits)
		fmt.Printf("  Duration:        %s\n", result.Duration.Round(time.Millisecond))
	}

	return err
}
