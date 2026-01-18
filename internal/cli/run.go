package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
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

	// For dry-run mode, print execution plan
	if opts.DryRun {
		fmt.Printf("Dry-run mode: execution plan\n")
		fmt.Printf("Tasks directory: %s\n", opts.TasksDir)
		fmt.Printf("Parallelism: %d\n", opts.Parallelism)
		fmt.Printf("Target branch: %s\n", opts.TargetBranch)
		if opts.Unit != "" {
			fmt.Printf("Single unit mode: %s\n", opts.Unit)
		} else {
			fmt.Printf("Mode: all units\n")
		}
		fmt.Printf("PR creation: %t\n", !opts.NoPR)
		fmt.Printf("Skip review: %t\n", opts.SkipReview)
		return nil
	}

	// TODO: Wire orchestrator components (task #9)
	// TODO: Run discovery (task #9)
	// TODO: Execute scheduler loop (task #9)

	// Placeholder: Wait for context cancellation
	<-ctx.Done()

	return nil
}
