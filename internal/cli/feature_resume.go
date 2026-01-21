package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/RevCBH/choo/internal/feature"
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

The resume command continues a blocked feature workflow. It can be used to:
- Skip remaining review iterations and proceed to validation
- Resume from spec validation (after manual spec edits)
- Resume from task generation (after manual spec edits)

Only features in "review_blocked" state can be resumed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.PRDID = args[0]
			return app.RunFeatureResume(cmd.Context(), *opts)
		},
	}

	// Add flags
	cmd.Flags().BoolVar(&opts.SkipReview, "skip-review", false, "Skip remaining review iterations and proceed to validation")
	cmd.Flags().BoolVar(&opts.FromValidation, "from-validation", false, "Resume from spec validation (after manual spec edits)")
	cmd.Flags().BoolVar(&opts.FromTasks, "from-tasks", false, "Resume from task generation (after manual spec edits)")

	return cmd
}

// RunFeatureResume continues a blocked feature workflow
func (a *App) RunFeatureResume(ctx context.Context, opts FeatureResumeOptions) error {
	// Validate flag combinations
	if err := validateResumeOptions(opts); err != nil {
		return err
	}

	// Validate PRD exists
	prdDir := "docs/prd"
	store := feature.NewPRDStore(prdDir)

	metadata, _, err := store.Load(opts.PRDID)
	if err != nil {
		return fmt.Errorf("PRD not found: %s", opts.PRDID)
	}

	// Load current state
	state := feature.FeatureState{
		PRDID:            opts.PRDID,
		Status:           feature.FeatureStatus(metadata.FeatureStatus),
		Branch:           metadata.Branch,
		ReviewIterations: metadata.ReviewIterations,
		MaxReviewIter:    metadata.MaxReviewIter,
		LastFeedback:     metadata.LastFeedback,
		SpecCount:        metadata.SpecCount,
		TaskCount:        metadata.TaskCount,
	}

	if metadata.StartedAt != nil {
		state.StartedAt = *metadata.StartedAt
	}

	// Validate state is resumable
	if err := validateFeatureResumeState(state); err != nil {
		return err
	}

	// TODO: Initialize workflow from FEATURE-WORKFLOW
	// TODO: Execute workflow.Resume() with options
	// TODO: Handle blocked states (print next steps)
	// TODO: Handle success (print completion message)

	return fmt.Errorf("workflow execution not yet implemented (requires FEATURE-WORKFLOW spec)")
}

// validateFeatureResumeState checks if feature can be resumed
func validateFeatureResumeState(state feature.FeatureState) error {
	// Only StatusReviewBlocked can be resumed
	if state.Status != feature.StatusReviewBlocked {
		// Generate descriptive error based on current state
		switch state.Status {
		case feature.StatusSpecsCommitted:
			return fmt.Errorf(`cannot resume feature "%s"
  Current status: %s
  This feature has already completed spec generation.
  Use "choo run --feature %s" to execute units.`, state.PRDID, state.Status, state.PRDID)

		case feature.StatusGeneratingSpecs, feature.StatusReviewingSpecs,
			feature.StatusValidatingSpecs, feature.StatusGeneratingTasks:
			return fmt.Errorf(`cannot resume feature "%s"
  Current status: %s
  Only features in "review_blocked" state can be resumed.`, state.PRDID, state.Status)

		default:
			return fmt.Errorf(`cannot resume feature "%s"
  Current status: %s
  Only features in "review_blocked" state can be resumed.`, state.PRDID, state.Status)
		}
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

	return nil
}
