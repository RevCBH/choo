package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/choo/internal/discovery"
	"github.com/spf13/cobra"
)

// NewResumeCmd creates the resume command
func NewResumeCmd(app *App) *cobra.Command {
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
		Use:   "resume [tasks-dir]",
		Short: "Continue orchestration from the last saved state",
		Long: `Resume continues the orchestration loop from the last saved state.

It reads state from YAML frontmatter in task specs and continues from
the first incomplete task. Use this to recover from interruptions or
continue work after a break.`,
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

			// Resume orchestrator
			return app.ResumeOrchestrator(ctx, opts)
		},
	}

	// Add flags (inherit from run command)
	cmd.Flags().IntVarP(&opts.Parallelism, "parallelism", "p", 4, "Max concurrent units")
	cmd.Flags().StringVarP(&opts.TargetBranch, "target", "t", "main", "Branch PRs target")
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "n", false, "Show execution plan without running")
	cmd.Flags().BoolVar(&opts.NoPR, "no-pr", false, "Skip PR creation")
	cmd.Flags().StringVar(&opts.Unit, "unit", "", "Run only specified unit (single-unit mode)")
	cmd.Flags().BoolVar(&opts.SkipReview, "skip-review", false, "Auto-merge without waiting for review")

	return cmd
}

// ResumeOrchestrator continues from the last saved state
func (a *App) ResumeOrchestrator(ctx context.Context, opts RunOptions) error {
	// Validate options
	if err := opts.Validate(); err != nil {
		return err
	}

	// TODO: Load existing state from frontmatter
	// For now, create a placeholder discovery to validate the pattern
	disc := &discovery.Discovery{
		Units: []*discovery.Unit{},
	}

	// Validate state is resumable
	if err := validateResumeState(disc); err != nil {
		return err
	}

	// Continue with RunOrchestrator from saved state
	return a.RunOrchestrator(ctx, opts)
}

// validateResumeState checks if state can be resumed
func validateResumeState(disc *discovery.Discovery) error {
	// Check if discovery has any units
	if disc == nil || len(disc.Units) == 0 {
		return fmt.Errorf("nothing to resume: no previous orchestration state found")
	}

	// Check if there are incomplete units
	hasIncomplete := false
	for _, unit := range disc.Units {
		if unit.Status != discovery.UnitStatusComplete {
			hasIncomplete = true
			break
		}
	}

	if !hasIncomplete {
		return fmt.Errorf("nothing to resume: all units complete")
	}

	// Check state consistency (completed tasks should not appear after pending tasks)
	for _, unit := range disc.Units {
		if len(unit.Tasks) == 0 {
			continue
		}

		foundPendingOrInProgress := false
		for _, task := range unit.Tasks {
			if task.Status == discovery.TaskStatusPending || task.Status == discovery.TaskStatusInProgress {
				foundPendingOrInProgress = true
			} else if task.Status == discovery.TaskStatusComplete && foundPendingOrInProgress {
				return fmt.Errorf("cannot resume: state corrupted (unit %s has completed tasks after pending tasks)", unit.ID)
			}
		}
	}

	return nil
}
