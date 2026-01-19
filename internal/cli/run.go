package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/anthropics/choo/internal/discovery"
	"github.com/anthropics/choo/internal/events"
	"github.com/anthropics/choo/internal/scheduler"
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

	// Load configuration
	cfg, err := loadConfig(opts.TasksDir)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply CLI overrides
	cfg.Parallelism = opts.Parallelism
	cfg.TargetBranch = opts.TargetBranch

	// Wire orchestrator components
	fmt.Fprintf(os.Stderr, "Initializing orchestrator...\n")
	orch, err := WireOrchestrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to wire orchestrator: %w", err)
	}
	defer orch.Close()

	// Discover units
	fmt.Fprintf(os.Stderr, "Discovering units in %s...\n", opts.TasksDir)
	units, err := discovery.Discover(opts.TasksDir)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	if len(units) == 0 {
		fmt.Fprintf(os.Stderr, "No units found in %s\n", opts.TasksDir)
		return nil
	}

	// Filter to single unit if specified
	if opts.Unit != "" {
		filtered := []*discovery.Unit{}
		for _, unit := range units {
			if unit.ID == opts.Unit {
				filtered = append(filtered, unit)
				break
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("unit %q not found", opts.Unit)
		}
		units = filtered
		fmt.Fprintf(os.Stderr, "Running single unit: %s\n", opts.Unit)
	} else {
		fmt.Fprintf(os.Stderr, "Found %d unit(s)\n", len(units))
	}

	// Build execution schedule
	fmt.Fprintf(os.Stderr, "Building execution schedule...\n")
	schedule, err := orch.Scheduler.Schedule(units)
	if err != nil {
		return fmt.Errorf("scheduler initialization failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Schedule: %d unit(s), %d parallel worker(s)\n",
		len(units), schedule.MaxParallelism)

	// Main orchestration loop
	fmt.Fprintf(os.Stderr, "Starting orchestration loop...\n")
	return a.runOrchestrationLoop(ctx, orch, opts)
}

// runOrchestrationLoop executes the main dispatch loop
func (a *App) runOrchestrationLoop(ctx context.Context, orch *Orchestrator, opts RunOptions) error {
	// Get units and build lookup map
	units, err := discovery.Discover(opts.TasksDir)
	if err != nil {
		return fmt.Errorf("failed to discover units: %w", err)
	}

	// Build unit lookup map
	unitMap := make(map[string]*discovery.Unit)
	for _, unit := range units {
		unitMap[unit.ID] = unit
	}

	// Track dispatched units to avoid re-submission
	dispatched := make(map[string]bool)

	// Subscribe to events from worker pool to update scheduler
	eventChan := make(chan struct{}, 100)
	orch.Events.Subscribe(func(e events.Event) {
		// On unit completion or failure, update scheduler and wake up dispatch loop
		if e.Type == events.UnitCompleted {
			orch.Scheduler.Complete(e.Unit)
			select {
			case eventChan <- struct{}{}:
			default:
			}
		} else if e.Type == events.UnitFailed {
			var failErr error
			if e.Error != "" {
				failErr = fmt.Errorf("%s", e.Error)
			}
			orch.Scheduler.Fail(e.Unit, failErr)
			select {
			case eventChan <- struct{}{}:
			default:
			}
		}
	})

	// Main loop: dispatch ready units and wait for completion
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "\nShutdown requested, waiting for workers to finish...\n")
			// Wait for workers to complete gracefully
			if err := orch.Workers.Wait(); err != nil {
				return fmt.Errorf("workers did not complete cleanly: %w", err)
			}
			return ctx.Err()

		case <-eventChan:
			// Event received, continue to dispatch check

		case <-ticker.C:
			// Regular tick, check dispatch
		}

		// Check if we're done
		if orch.Scheduler.IsComplete() {
			fmt.Fprintf(os.Stderr, "\nAll units complete!\n")
			// Wait for any remaining workers
			if err := orch.Workers.Wait(); err != nil {
				return fmt.Errorf("worker execution failed: %w", err)
			}
			return nil
		}

		// Check for failures
		if orch.Scheduler.HasFailures() {
			// Wait for workers to finish
			orch.Workers.Wait()
			return fmt.Errorf("execution failed: one or more units failed or are blocked")
		}

		// Try to dispatch ready units
		for {
			result := orch.Scheduler.Dispatch()

			// Handle dispatch result
			switch result.Reason {
			case scheduler.ReasonNone:
				// Successfully dispatched
				unitID := result.Unit
				if !dispatched[unitID] {
					// Get the unit from map
					unit, ok := unitMap[unitID]
					if !ok {
						return fmt.Errorf("dispatched unit %q not found in unit map", unitID)
					}

					// Submit to worker pool
					fmt.Fprintf(os.Stderr, "[%s] Dispatching unit...\n", unitID)
					if err := orch.Workers.Submit(unit); err != nil {
						return fmt.Errorf("failed to submit unit %s: %w", unitID, err)
					}

					dispatched[unitID] = true
				}

			case scheduler.ReasonAtCapacity:
				// At parallelism limit, wait for next tick
				goto exitDispatchLoop

			case scheduler.ReasonNoReady:
				// No ready units yet, wait for completions
				goto exitDispatchLoop

			case scheduler.ReasonAllComplete:
				// All done
				goto exitDispatchLoop

			case scheduler.ReasonAllBlocked:
				// Deadlock or failure
				goto exitDispatchLoop
			}
		}
	exitDispatchLoop:
	}
}
