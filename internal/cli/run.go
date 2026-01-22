package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/RevCBH/choo/internal/cli/tui"
	"github.com/RevCBH/choo/internal/client"
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
	CloneURL     string // URL to clone before running (used in container)
	JSONEvents   bool   // Emit events as JSON to stdout (for daemon parsing)
	Web          bool   // Enable web UI event forwarding
	WebSocket    string // Custom Unix socket path (optional)
	NoTUI        bool   // Disable TUI even when stdout is a TTY
	Feature      string // PRD ID to work on in feature mode
	UseDaemon    bool   // Use daemon mode
	Force        bool   // Force run even with uncommitted changes

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

// isContainerMode returns true if running in container mode.
func (opts RunOptions) isContainerMode() bool {
	return opts.CloneURL != "" || opts.JSONEvents
}

// runWithDaemon executes a job via the daemon and attaches to event stream.
// If the daemon is not running, it will be started automatically.
func runWithDaemon(ctx context.Context, tasksDir string, parallelism int, target, feature string) error {
	// Auto-start daemon if not running
	if !isDaemonRunning() {
		fmt.Println("Starting daemon...")
		if err := startDaemonBackground(DaemonStartOptions{}); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
		// Give daemon a moment to initialize
		time.Sleep(500 * time.Millisecond)
	}

	c, err := client.New(defaultSocketPath())
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer c.Close()

	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	jobID, err := c.StartJob(ctx, client.JobConfig{
		TasksDir:      tasksDir,
		TargetBranch:  target,
		FeatureBranch: feature,
		Parallelism:   parallelism,
		RepoPath:      repoPath,
	})
	if err != nil {
		// Check if this is a connection error and provide helpful message
		if strings.Contains(err.Error(), "connection error") || strings.Contains(err.Error(), "connect:") {
			return fmt.Errorf("failed to connect to daemon: %w (is daemon running?)", err)
		}
		return err
	}

	fmt.Printf("Started job %s\n", jobID)

	// Attach to event stream and display events
	return c.WatchJob(ctx, jobID, 0, displayEvent)
}

// runInline executes jobs directly without daemon (existing behavior)
func runInline(ctx context.Context, opts RunOptions, app *App) error {
	// This will contain the existing inline execution logic
	return app.RunOrchestrator(ctx, opts)
}

// registerRunFlags adds flags to the run command.
func registerRunFlags(cmd *cobra.Command, opts *RunOptions) {
	cmd.Flags().IntVarP(&opts.Parallelism, "parallelism", "p", opts.Parallelism, "Max concurrent units")
	cmd.Flags().StringVarP(&opts.TargetBranch, "target", "t", opts.TargetBranch, "Branch PRs target (default: current branch)")
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "n", opts.DryRun, "Show execution plan without running")
	cmd.Flags().BoolVar(&opts.NoPR, "no-pr", opts.NoPR, "Skip PR creation")
	cmd.Flags().StringVar(&opts.Unit, "unit", opts.Unit, "Run only specified unit (single-unit mode)")
	cmd.Flags().BoolVar(&opts.SkipReview, "skip-review", opts.SkipReview, "Auto-merge without waiting for review")
	cmd.Flags().StringVar(&opts.TasksDir, "tasks", opts.TasksDir, "Path to tasks directory")
	cmd.Flags().StringVar(&opts.CloneURL, "clone-url", opts.CloneURL,
		"Clone repository from URL before running (container mode)")
	cmd.Flags().BoolVar(&opts.JSONEvents, "json-events", opts.JSONEvents,
		"Emit events as JSON to stdout (for daemon parsing)")
	cmd.Flags().BoolVar(&opts.Web, "web", opts.Web, "Enable web UI event forwarding")
	cmd.Flags().StringVar(&opts.WebSocket, "web-socket", opts.WebSocket, "Custom Unix socket path (default: ~/.choo/web.sock)")
	cmd.Flags().BoolVar(&opts.NoTUI, "no-tui", opts.NoTUI, "Disable interactive TUI (use summary-only output)")
	cmd.Flags().StringVar(&opts.Feature, "feature", opts.Feature, "PRD ID for feature mode (targets feature branch)")
	cmd.Flags().BoolVar(&opts.UseDaemon, "use-daemon", opts.UseDaemon, "Use daemon mode")
	cmd.Flags().StringVar(&opts.Provider, "provider", opts.Provider, "Default provider for task execution (claude, codex). Units without frontmatter override use this.")
	cmd.Flags().StringVar(&opts.ForceTaskProvider, "force-task-provider", opts.ForceTaskProvider, "Force provider for ALL task execution, ignoring per-unit frontmatter (claude, codex)")
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
		CloneURL:          "",
		JSONEvents:        false,
		Provider:          "", // Empty means use default from config/env
		ForceTaskProvider: "", // Empty means respect per-unit settings
		UseDaemon:         true,
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

			// Get working directory
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			// Check for uncommitted changes (skip for dry-run)
			if !opts.DryRun && !opts.Force {
				status, err := git.GetWorkingDirStatus(ctx, wd, "specs/")
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not check for uncommitted changes: %v\n", err)
				} else if status.HasChanges {
					fmt.Fprintf(cmd.ErrOrStderr(), "Error: working directory has uncommitted changes\n")
					if status.PathChanges["specs/"] {
						fmt.Fprintf(cmd.ErrOrStderr(), "  Changes in specs/ must be committed before running,\n")
						fmt.Fprintf(cmd.ErrOrStderr(), "  as they need to propagate to worktrees.\n")
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "\nChanged files:\n")
					for _, f := range status.ChangedFiles {
						fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", f)
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "\nCommit your changes or use --force to bypass this check.\n")
					return fmt.Errorf("uncommitted changes in working directory")
				}
			}

			// If --target wasn't explicitly set, use current branch
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
			if !opts.DryRun && !opts.UseDaemon {
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

			// Dispatch based on mode
			if opts.UseDaemon {
				return runWithDaemon(ctx, opts.TasksDir, opts.Parallelism, opts.TargetBranch, opts.Feature)
			}
			return runInline(ctx, opts, app)
		},
	}

	registerRunFlags(cmd, &opts)

	// Safety flags
	cmd.Flags().BoolVar(&opts.Force, "force", false, "Force run even with uncommitted changes in working directory")

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
		Parallelism:       opts.Parallelism,
		TargetBranch:      opts.TargetBranch,
		TasksDir:          opts.TasksDir,
		RepoRoot:          wd,
		WorktreeBase:      cfg.Worktree.BasePath,
		NoPR:              opts.NoPR,
		SkipReview:        opts.SkipReview,
		SingleUnit:        opts.Unit,
		DryRun:            opts.DryRun,
		ShutdownTimeout:   orchestrator.DefaultShutdownTimeout,
		SuppressOutput:    useTUI,
		DefaultProvider:   opts.Provider,
		ForceTaskProvider: opts.ForceTaskProvider,
		ProviderConfig:    cfg.Provider,
		ClaudeCommand:     config.GetProviderCommand(cfg, config.ProviderClaude),
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
