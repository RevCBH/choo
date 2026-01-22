package cli

import (
	"fmt"

	"github.com/RevCBH/choo/internal/archive"
	"github.com/spf13/cobra"
)

// ArchiveOptions holds configuration for the archive command.
type ArchiveOptions = archive.ArchiveOptions

// NewArchiveCmd creates the archive command.
func NewArchiveCmd(_ *App) *cobra.Command {
	opts := ArchiveOptions{}

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Move completed specs to specs/completed/",
		Long: `Archive moves spec files with "status: complete" in their
frontmatter to the specs/completed/ directory.

This command is typically run automatically after all units in a
feature have completed, but can be run manually to clean up specs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if verbose, err := cmd.Flags().GetBool("verbose"); err == nil {
				opts.Verbose = verbose
			}

			archived, err := Archive(opts)
			if err != nil {
				return err
			}

			if len(archived) == 0 {
				fmt.Println("No specs to archive")
			} else {
				fmt.Printf("Archived %d specs\n", len(archived))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.SpecsDir, "specs", "specs", "Path to specs directory")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show what would be archived without moving files")

	return cmd
}

// Archive moves completed specs to specs/completed/.
// Returns the list of archived file names.
func Archive(opts ArchiveOptions) ([]string, error) {
	return archive.Archive(opts)
}

// ArchiveTasksDir archives completed task directories.
// This moves entire unit directories when all tasks are complete.
func ArchiveTasksDir(specsDir string, dryRun bool) ([]string, error) {
	return archive.ArchiveTasksDir(specsDir, dryRun)
}

// shouldArchive checks if a spec file should be archived.
func shouldArchive(path string) bool {
	return archive.ShouldArchive(path)
}

// isUnitComplete checks if all tasks in a unit directory are complete.
func isUnitComplete(unitDir string) bool {
	return archive.IsUnitComplete(unitDir)
}
