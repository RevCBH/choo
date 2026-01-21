package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// FeatureStartOptions holds flags for the feature start command
type FeatureStartOptions struct {
	PRDID          string
	PRDDir         string
	SpecsDir       string
	SkipSpecReview bool
	MaxReviewIter  int
	DryRun         bool
}

// NewFeatureStartCmd creates the feature start command
func NewFeatureStartCmd(app *App) *cobra.Command {
	opts := &FeatureStartOptions{}

	cmd := &cobra.Command{
		Use:   "start <prd-id>",
		Short: "Create feature branch, generate specs with review, generate tasks",
		Long: `Create feature branch, generate specs with review, generate tasks, commit to branch.

The start command initiates the feature workflow from a PRD. It will:
1. Read the PRD from the configured directory
2. Create a feature branch
3. Generate specifications using the spec-generator agent
4. Review specs with the spec-reviewer agent (unless --skip-spec-review)
5. Validate specs using the spec-validator agent
6. Generate tasks using the task-generator agent
7. Commit specs and tasks to the feature branch`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.PRDID = args[0]
			return app.RunFeatureStart(cmd, *opts)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&opts.PRDDir, "prd-dir", "docs/prds", "PRDs directory")
	cmd.Flags().StringVar(&opts.SpecsDir, "specs-dir", "specs/tasks", "Output specs directory")
	cmd.Flags().BoolVar(&opts.SkipSpecReview, "skip-spec-review", false, "Skip automated spec review loop")
	cmd.Flags().IntVar(&opts.MaxReviewIter, "max-review-iter", 3, "Max spec review iterations")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show plan without executing")

	return cmd
}

// RunFeatureStart executes the feature start workflow
func (a *App) RunFeatureStart(cmd *cobra.Command, opts FeatureStartOptions) error {
	// If dry-run, call runDryRun and return
	if opts.DryRun {
		return a.runDryRun(cmd, opts)
	}

	// Validate PRD exists
	// NOTE: Actual workflow execution depends on FEATURE-WORKFLOW spec
	// For now, this is a stub that validates input
	if opts.PRDID == "" {
		return fmt.Errorf("PRD ID cannot be empty")
	}

	// TODO: Initialize workflow from FEATURE-WORKFLOW
	// TODO: Execute workflow.Start()
	// TODO: Handle blocked states (print resume instructions)
	// TODO: Handle success (print completion message)

	return fmt.Errorf("workflow execution not yet implemented (requires FEATURE-WORKFLOW spec)")
}

// runDryRun prints planned actions without executing
func (a *App) runDryRun(cmd *cobra.Command, opts FeatureStartOptions) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "Dry run - showing planned actions:")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "1. Read PRD from %s/%s.md\n", opts.PRDDir, opts.PRDID)
	fmt.Fprintf(out, "2. Create branch feature/%s from main\n", opts.PRDID)
	fmt.Fprintln(out, "3. Generate specs using spec-generator agent")

	if !opts.SkipSpecReview {
		fmt.Fprintf(out, "4. Review specs (max %d iterations)\n", opts.MaxReviewIter)
		fmt.Fprintln(out, "5. Validate specs using spec-validator agent")
	} else {
		fmt.Fprintln(out, "4. Skip spec review (--skip-spec-review)")
		fmt.Fprintln(out, "5. Validate specs using spec-validator agent")
	}

	fmt.Fprintln(out, "6. Generate tasks using task-generator agent")
	fmt.Fprintf(out, "7. Commit specs and tasks to feature/%s\n", opts.PRDID)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run without --dry-run to execute.")

	return nil
}
