package cli

import (
	"context"
	"fmt"

	"github.com/RevCBH/choo/internal/feature"
	"github.com/spf13/cobra"
)

// FeatureResumeOptions holds flags for the feature resume command
type FeatureResumeOptions struct {
	PRDID          string
	SkipReview     bool
	FromValidation bool
	FromTasks      bool
}

// NewFeatureResumeCmd creates the feature resume command
func NewFeatureResumeCmd(app *App) *cobra.Command {
	opts := &FeatureResumeOptions{}

	cmd := &cobra.Command{
		Use:   "resume <prd-id>",
		Short: "Resume a blocked feature workflow",
		Long: `Resume a feature workflow from a blocked state.

This command continues a feature workflow that was previously blocked
during spec review, validation, or task generation.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.PRDID = args[0]
			return app.RunFeatureResume(cmd.Context(), *opts)
		},
	}

	cmd.Flags().BoolVar(&opts.SkipReview, "skip-review", false,
		"Skip remaining review iterations and proceed to validation")
	cmd.Flags().BoolVar(&opts.FromValidation, "from-validation", false,
		"Resume from spec validation (after manual spec edits)")
	cmd.Flags().BoolVar(&opts.FromTasks, "from-tasks", false,
		"Resume from task generation (after manual spec edits)")

	return cmd
}

// RunFeatureResume continues a blocked feature workflow
func (a *App) RunFeatureResume(ctx context.Context, opts FeatureResumeOptions) error {
	// Validate flag combinations
	if err := validateResumeOptions(opts); err != nil {
		return err
	}

	// Validate PRD exists
	prdStore := feature.NewPRDStore("docs/prds")
	if !prdStore.Exists(opts.PRDID) {
		return fmt.Errorf("PRD not found: %s.md in docs/prds", opts.PRDID)
	}

	// Load current state
	meta, _, err := prdStore.Load(opts.PRDID)
	if err != nil {
		return fmt.Errorf("failed to load PRD: %w", err)
	}

	// Validate state is resumable
	currentState := feature.FeatureState{
		PRDID:  opts.PRDID,
		Status: feature.FeatureStatus(meta.FeatureStatus),
	}
	if err := validateFeatureResumeState(currentState); err != nil {
		return err
	}

	// TODO: Initialize workflow from FEATURE-WORKFLOW
	// workflow := feature.NewWorkflow(git, agents, prdStore, "specs/tasks")
	// result, err := workflow.Resume(ctx, opts.PRDID, feature.ResumeOptions{
	//     SkipReview:     opts.SkipReview,
	//     FromValidation: opts.FromValidation,
	//     FromTasks:      opts.FromTasks,
	// })
	// if err != nil {
	//     return err
	// }

	// TODO: Handle blocked states (print next steps)
	// if result.Blocked {
	//     fmt.Printf("Workflow blocked in state: %s\n", result.FinalStatus)
	//     fmt.Printf("Reason: %s\n", result.BlockReason)
	//     fmt.Printf("\nResume with: choo feature resume %s\n", opts.PRDID)
	//     return nil
	// }

	// TODO: Handle success (print completion message)
	// fmt.Printf("Feature workflow completed successfully\n")
	// fmt.Printf("Branch: feature/%s\n", opts.PRDID)
	// fmt.Printf("Specs generated: %d\n", result.SpecsGenerated)
	// fmt.Printf("Tasks generated: %d\n", result.TasksGenerated)

	return fmt.Errorf("workflow resume not yet implemented (FEATURE-WORKFLOW spec)")
}

// validateFeatureResumeState checks if feature can be resumed
func validateFeatureResumeState(state feature.FeatureState) error {
	// Only StatusReviewBlocked can be resumed
	if state.Status != feature.StatusReviewBlocked {
		// Return descriptive error for other states
		if state.Status == feature.StatusSpecsCommitted {
			return fmt.Errorf(`cannot resume feature "%s"
  Current status: %s
  This feature has already completed spec generation.
  Use "choo run --feature %s" to execute units.`, state.PRDID, state.Status, state.PRDID)
		}

		return fmt.Errorf(`cannot resume feature "%s"
  Current status: %s
  Only features in "review_blocked" state can be resumed.`, state.PRDID, state.Status)
	}

	return nil
}

// validateResumeOptions checks flag combinations
func validateResumeOptions(opts FeatureResumeOptions) error {
	// --from-validation and --from-tasks are mutually exclusive
	if opts.FromValidation && opts.FromTasks {
		return fmt.Errorf(`invalid flag combination
  --from-validation and --from-tasks are mutually exclusive`)
	}

	// --skip-review can combine with others
	return nil
}
