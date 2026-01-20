package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/RevCBH/choo/internal/cli/tui"
	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
	"github.com/RevCBH/choo/internal/orchestrator"
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
	NoTUI        bool   // Disable TUI even when stdout is a TTY
}

// Validate checks RunOptions for validity
func (opts RunOptions) Validate() error {
	if opts.Parallelism <= 0 {
		return fmt.Errorf("parallelism must be greater than 0, got %d", opts.Parallelism)
	}
	if opts.TasksDir == "" {
		return fmt.Errorf("tasks directory must not be empty")
	}
	return nil
}

// NewRunCmd creates the run command
func NewRunCmd(app *App) *cobra.Command {
	opts := RunOptions{
		Parallelism:  4,
		TargetBranch: "main",
		DryRun:       false,
		NoPR:         false,
		Unit:         "",
		SkipReview:   false,
		TasksDir:     "specs/tasks",
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

			// Validate options
			if err := opts.Validate(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				os.Exit(2)
			}

			// Create context
			ctx := context.Background()

			// Run orchestrator
			return app.RunOrchestrator(ctx, opts)
		},
	}

	// Add flags
	cmd.Flags().IntVarP(&opts.Parallelism, "parallelism", "p", 4, "Max concurrent units")
	cmd.Flags().StringVarP(&opts.TargetBranch, "target", "t", "main", "Branch PRs target")
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "n", false, "Show execution plan without running")
	cmd.Flags().BoolVar(&opts.NoPR, "no-pr", false, "Skip PR creation")
	cmd.Flags().StringVar(&opts.Unit, "unit", "", "Run only specified unit (single-unit mode)")
	cmd.Flags().BoolVar(&opts.SkipReview, "skip-review", false, "Auto-merge without waiting for review")
	cmd.Flags().BoolVar(&opts.NoTUI, "no-tui", false, "Disable interactive TUI (use summary-only output)")

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
