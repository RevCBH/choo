package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/specs"
	"github.com/spf13/cobra"
)

// NewSpecsCmd creates the specs command group.
func NewSpecsCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "specs",
		Short: "Validate and normalize spec metadata",
	}

	cmd.AddCommand(
		newSpecsValidateCmd(),
		newSpecsNormalizeCmd(),
		newSpecsRepairCmd(app),
	)

	return cmd
}

func newSpecsValidateCmd() *cobra.Command {
	opts := specs.NormalizeOptions{
		TasksDir: "specs/tasks",
	}

	cmd := &cobra.Command{
		Use:   "validate [tasks-dir]",
		Short: "Validate spec metadata without modifying files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.TasksDir = args[0]
			}
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			opts.RepoRoot = wd
			opts.Apply = false

			report, err := specs.Normalize(opts)
			if err != nil {
				return err
			}
			if report.HasErrors() {
				return &specs.ValidationError{Report: report}
			}

			units, err := discovery.Discover(opts.TasksDir)
			if err != nil {
				return err
			}
			depWarnings := discovery.ValidateUnitDependencies(units)
			if depWarnings.HasWarnings() {
				fmt.Fprintf(cmd.ErrOrStderr(), "Spec dependency warnings (%d):\n", len(depWarnings.Warnings))
				for _, warn := range depWarnings.Warnings {
					msg := warn.Message
					if warn.Unit != "" {
						msg = fmt.Sprintf("unit %q: %s", warn.Unit, warn.Message)
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "  - %s\n", msg)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Spec validation OK (tasks=%d, units=%d, warnings=%d)\n",
				report.Tasks, report.Units, len(report.Warnings))
			return nil
		},
	}

	return cmd
}

func newSpecsNormalizeCmd() *cobra.Command {
	opts := specs.NormalizeOptions{
		TasksDir: "specs/tasks",
	}

	cmd := &cobra.Command{
		Use:   "normalize [tasks-dir]",
		Short: "Normalize spec metadata into canonical frontmatter",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.TasksDir = args[0]
			}
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			opts.RepoRoot = wd
			opts.Apply = true

			report, err := specs.Normalize(opts)
			if err != nil {
				return err
			}
			if report.HasErrors() {
				return &specs.ValidationError{Report: report}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Spec normalization complete (normalized=%d)\n", len(report.Normalized))
			return nil
		},
	}

	return cmd
}

func newSpecsRepairCmd(app *App) *cobra.Command {
	opts := specs.NormalizeOptions{
		TasksDir: "specs/tasks",
	}
	parallelism := 4

	cmd := &cobra.Command{
		Use:   "repair [tasks-dir]",
		Short: "Repair spec metadata with LLM and normalize to frontmatter",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.TasksDir = args[0]
			}
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			cfg, err := config.LoadConfig(wd)
			if err != nil {
				return err
			}
			opts.RepoRoot = wd
			opts.Apply = false

			report, err := specs.Normalize(opts)
			if err != nil {
				return err
			}

			var repaired []specs.RepairResult
			if report.HasErrors() {
				ctx := context.Background()
				if parallelism < 1 {
					parallelism = 1
				}
				repaired, err = specs.RepairIssues(ctx, specs.RepairBatchOptions{
					RepoRoot:    wd,
					Config:      cfg.SpecRepair,
					Parallelism: parallelism,
					Verbose:     app != nil && app.verbose,
					Output:      cmd.ErrOrStderr(),
				}, report.Errors)
				if err != nil {
					return err
				}
			}

			opts.Apply = true
			report, err = specs.Normalize(opts)
			if err != nil {
				return err
			}
			if report.HasErrors() {
				return &specs.ValidationError{Report: report}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Spec repair complete (repaired=%d, normalized=%d, warnings=%d)\n",
				len(repaired), len(report.Normalized), len(report.Warnings))
			return nil
		},
	}

	cmd.Flags().IntVarP(&parallelism, "parallelism", "p", parallelism, "Max concurrent spec repairs")

	return cmd
}
