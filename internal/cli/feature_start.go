package cli

import (
	"context"
	"fmt"

	"github.com/RevCBH/choo/internal/feature"
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
	opts := &FeatureStartOptions{
		PRDDir:        "docs/prds",
		SpecsDir:      "specs/tasks",
		MaxReviewIter: 3,
	}

	cmd := &cobra.Command{
		Use:   "start <prd-id>",
		Short: "Create feature branch, generate specs with review, generate tasks",
		Long: `Create feature branch, generate specs with review, generate tasks, commit to branch.

The start command initiates the feature workflow from a PRD, creating a feature
branch and orchestrating spec generation, review, validation, and task generation.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.PRDID = args[0]
			return app.RunFeatureStart(cmd.Context(), *opts)
		},
	}

	cmd.Flags().StringVar(&opts.PRDDir, "prd-dir", opts.PRDDir,
		"PRDs directory")
	cmd.Flags().StringVar(&opts.SpecsDir, "specs-dir", opts.SpecsDir,
		"Output specs directory")
	cmd.Flags().BoolVar(&opts.SkipSpecReview, "skip-spec-review", false,
		"Skip automated spec review loop")
	cmd.Flags().IntVar(&opts.MaxReviewIter, "max-review-iter", opts.MaxReviewIter,
		"Max spec review iterations")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false,
		"Show plan without executing")

	return cmd
}

// RunFeatureStart executes the feature start workflow
func (a *App) RunFeatureStart(ctx context.Context, opts FeatureStartOptions) error {
	// If dry-run, print plan and exit
	if opts.DryRun {
		return a.runDryRun(opts)
	}

	// Validate PRD exists
	prdStore := feature.NewPRDStore(opts.PRDDir)
	if !prdStore.Exists(opts.PRDID) {
		return fmt.Errorf("PRD not found: %s.md in %s", opts.PRDID, opts.PRDDir)
	}

	// TODO: Initialize workflow from FEATURE-WORKFLOW
	// workflow := feature.NewWorkflow(git, agents, prdStore, opts.SpecsDir)
	// result, err := workflow.Start(ctx, opts.PRDID, feature.StartOptions{
	//     SkipSpecReview: opts.SkipSpecReview,
	//     MaxReviewIter:  opts.MaxReviewIter,
	//     DryRun:         opts.DryRun,
	// })
	// if err != nil {
	//     return err
	// }

	// TODO: Handle blocked states (print resume instructions)
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

	return fmt.Errorf("workflow execution not yet implemented (FEATURE-WORKFLOW spec)")
}

// runDryRun prints planned actions without executing
func (a *App) runDryRun(opts FeatureStartOptions) error {
	fmt.Println("Dry run - showing planned actions:")
	fmt.Println()
	fmt.Printf("1. Read PRD from %s/%s.md\n", opts.PRDDir, opts.PRDID)
	fmt.Printf("2. Create branch feature/%s from main\n", opts.PRDID)
	fmt.Println("3. Generate specs using spec-generator agent")

	if !opts.SkipSpecReview {
		fmt.Printf("4. Review specs (max %d iterations)\n", opts.MaxReviewIter)
		fmt.Println("5. Validate specs using spec-validator agent")
	} else {
		fmt.Println("4. Skip spec review (--skip-spec-review)")
		fmt.Println("5. Validate specs using spec-validator agent")
	}

	fmt.Println("6. Generate tasks using task-generator agent")
	fmt.Printf("7. Commit specs and tasks to feature/%s\n", opts.PRDID)
	fmt.Println()
	fmt.Println("Run without --dry-run to execute.")

	return nil
}
