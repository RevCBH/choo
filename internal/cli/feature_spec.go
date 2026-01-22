package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/RevCBH/choo/internal/spec"
	"github.com/spf13/cobra"
)

// FeatureSpecOptions holds flags for the feature spec command.
type FeatureSpecOptions struct {
	// PRDPath is the path to the PRD file (positional arg).
	PRDPath string

	// SpecsDir is the output directory for specs.
	SpecsDir string

	// Phase is the specific phase to run (spec, validate, ralph-prep, all).
	Phase string

	// Resume continues from saved state.
	Resume bool

	// DryRun shows prompts without executing.
	DryRun bool

	// Force overwrites existing state.
	Force bool

	// SkipValidation skips the validation phase.
	SkipValidation bool

	// Stream enables JSON streaming output for visibility.
	Stream bool
}

// NewFeatureSpecCmd creates the feature spec command.
func NewFeatureSpecCmd(app *App) *cobra.Command {
	opts := &FeatureSpecOptions{}

	cmd := &cobra.Command{
		Use:   "spec <prd-path>",
		Short: "Generate technical specifications from a PRD",
		Long: `Generate technical specifications from a Product Requirements Document (PRD).

This command runs a three-phase workflow:
1. Spec Generation - Creates technical specs from the PRD
2. Validation - Validates specs for consistency and completeness
3. Ralph Prep - Decomposes specs into atomic, Ralph-executable tasks

The command can be run against any repository, not just the current one.
Skills are loaded from embedded defaults or user overrides in ~/.choo/skills/.

Examples:
  # Full autonomous workflow
  choo feature spec docs/prd/MY-FEATURE.md

  # Run specific phase
  choo feature spec docs/prd/MY-FEATURE.md --phase spec
  choo feature spec docs/prd/MY-FEATURE.md --phase validate
  choo feature spec docs/prd/MY-FEATURE.md --phase ralph-prep

  # Resume interrupted workflow
  choo feature spec docs/prd/MY-FEATURE.md --resume

  # Dry run (show prompts without executing)
  choo feature spec docs/prd/MY-FEATURE.md --dry-run

  # Skip validation (faster but risky)
  choo feature spec docs/prd/MY-FEATURE.md --skip-validation`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.PRDPath = args[0]
			}
			return app.RunFeatureSpec(cmd, *opts)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&opts.SpecsDir, "specs-dir", "", "Output directory for specs (default: from config or 'specs')")
	cmd.Flags().StringVar(&opts.Phase, "phase", "all", "Phase to run: spec, validate, ralph-prep, all")
	cmd.Flags().BoolVar(&opts.Resume, "resume", false, "Resume from saved state")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show prompts without executing")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "Overwrite existing state")
	cmd.Flags().BoolVar(&opts.SkipValidation, "skip-validation", false, "Skip validation phase")
	cmd.Flags().BoolVar(&opts.Stream, "stream", true, "Stream JSON output showing Claude's progress (default: enabled)")

	return cmd
}

// RunFeatureSpec executes the feature spec workflow.
func (a *App) RunFeatureSpec(cmd *cobra.Command, opts FeatureSpecOptions) error {
	// Validate options
	if !opts.Resume && opts.PRDPath == "" {
		return fmt.Errorf("PRD path required (or use --resume)")
	}

	// Get current directory as repo root
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Check for existing state
	existingState, err := spec.LoadState(repoRoot)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	if existingState != nil && !opts.Force {
		// --resume flag: crash recovery mode
		if opts.Resume {
			workflow, err := spec.NewWorkflow(spec.WorkflowOptions{
				PRDPath:  existingState.PRDPath,
				SpecsDir: existingState.SpecsDir,
				RepoRoot: repoRoot,
				DryRun:   opts.DryRun,
				Stream:   opts.Stream,
				Verbose:  a.verbose,
				Debug:    a.debug,
				Stdout:   cmd.OutOrStdout(),
				Stderr:   cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}

			// Setup context with cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle interrupt
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Fprintln(cmd.ErrOrStderr(), "\nInterrupted - saving state...")
				cancel()
			}()

			if opts.Phase != "all" {
				// --resume --phase X: restart from specific phase (recovery mode)
				targetPhase, err := spec.ParsePhase(opts.Phase)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Resuming workflow from phase: %s\n", targetPhase)
				return workflow.RunPhase(ctx, targetPhase)
			}
			// --resume alone: continue from where we left off
			return workflow.Resume(ctx)
		}

		// Smart continuation: check if requested phase is the expected next step
		if opts.Phase != "all" {
			requestedPhase, err := spec.ParsePhase(opts.Phase)
			if err != nil {
				return err
			}
			nextPhase := existingState.NextPhase()

			if requestedPhase == nextPhase {
				// Auto-continue: requested phase matches expected next phase
				fmt.Fprintf(cmd.OutOrStdout(), "Continuing workflow: %s → %s\n",
					existingState.Phase, requestedPhase)
				workflow, err := spec.NewWorkflow(spec.WorkflowOptions{
					PRDPath:  existingState.PRDPath,
					SpecsDir: existingState.SpecsDir,
					RepoRoot: repoRoot,
					DryRun:   opts.DryRun,
					Stream:   opts.Stream,
					Verbose:  a.verbose,
					Debug:    a.debug,
					Stdout:   cmd.OutOrStdout(),
					Stderr:   cmd.ErrOrStderr(),
				})
				if err != nil {
					return err
				}

				// Setup context with cancellation
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				// Handle interrupt
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
				go func() {
					<-sigCh
					fmt.Fprintln(cmd.ErrOrStderr(), "\nInterrupted - saving state...")
					cancel()
				}()

				return workflow.RunPhase(ctx, requestedPhase)
			} else if requestedPhase.Order() <= existingState.Phase.Order() {
				return fmt.Errorf("phase %s already completed (current: %s)\nuse --resume --phase %s to re-run, or --force to restart",
					requestedPhase, existingState.Phase, requestedPhase)
			} else {
				return fmt.Errorf("cannot skip to %s (current: %s, next: %s)\nrun phases in order",
					requestedPhase, existingState.Phase, nextPhase)
			}
		}

		// No specific phase requested and no --resume - show status and suggest next step
		return fmt.Errorf("workflow in progress (phase: %s, next: %s)\nrun with --phase %s to continue, or --force to restart",
			existingState.Phase, existingState.NextPhase(), existingState.NextPhase())
	}

	// Clear existing state if forcing
	if opts.Force {
		if err := spec.ClearState(repoRoot); err != nil {
			return fmt.Errorf("clear state: %w", err)
		}
	}

	// Create workflow
	workflow, err := spec.NewWorkflow(spec.WorkflowOptions{
		PRDPath:  opts.PRDPath,
		SpecsDir: opts.SpecsDir,
		RepoRoot: repoRoot,
		DryRun:   opts.DryRun,
		Stream:   opts.Stream,
		Verbose:  a.verbose,
		Debug:    a.debug,
		Stdout:   cmd.OutOrStdout(),
		Stderr:   cmd.ErrOrStderr(),
	})
	if err != nil {
		return fmt.Errorf("create workflow: %w", err)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(cmd.ErrOrStderr(), "\nInterrupted - saving state...")
		cancel()
	}()

	// Run workflow
	if opts.Phase != "all" {
		// Run single phase
		phase, err := spec.ParsePhase(opts.Phase)
		if err != nil {
			return err
		}

		// Skip validation if requested and phase is validation
		if opts.SkipValidation && phase == spec.PhaseValidation {
			fmt.Fprintln(cmd.OutOrStdout(), "Skipping validation phase (--skip-validation)")
			return nil
		}

		return workflow.RunPhase(ctx, phase)
	}

	// Run full workflow, possibly skipping validation
	if opts.SkipValidation {
		// Run spec generation
		if err := workflow.RunPhase(ctx, spec.PhaseSpecGeneration); err != nil {
			return err
		}
		// Skip validation, go straight to ralph-prep
		fmt.Fprintln(cmd.OutOrStdout(), "\nSkipping validation phase (--skip-validation)")
		return workflow.RunPhase(ctx, spec.PhaseRalphPrep)
	}

	return workflow.Run(ctx)
}
