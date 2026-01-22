package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ArchiveOptions holds configuration for the archive command.
type ArchiveOptions struct {
	// SpecsDir is the directory containing specs (default: "specs")
	SpecsDir string

	// DryRun if true, prints what would be moved without moving
	DryRun bool

	// Verbose enables verbose output
	Verbose bool
}

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
	// Determine source and destination
	srcDir := opts.SpecsDir
	if srcDir == "" {
		srcDir = "specs"
	}
	dstDir := filepath.Join(srcDir, "completed")

	// Ensure source directory exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("specs directory does not exist: %s", srcDir)
	}

	// Ensure completed directory exists
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create completed directory: %w", err)
	}

	// Find spec files to archive
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read specs directory: %w", err)
	}

	var archived []string
	for _, entry := range entries {
		// Skip directories and non-markdown files
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		// Skip README.md
		if strings.EqualFold(entry.Name(), "readme.md") {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		if shouldArchive(srcPath) {
			dstPath := filepath.Join(dstDir, entry.Name())

			if opts.DryRun {
				if opts.Verbose {
					log.Printf("[dry-run] Would move %s -> %s", srcPath, dstPath)
				}
				archived = append(archived, entry.Name())
				continue
			}

			if err := os.Rename(srcPath, dstPath); err != nil {
				return archived, fmt.Errorf("failed to move %s: %w", entry.Name(), err)
			}

			if opts.Verbose {
				log.Printf("Archived: %s", entry.Name())
			}
			archived = append(archived, entry.Name())
		}
	}

	if len(archived) > 0 && !opts.DryRun {
		log.Printf("Archived %d specs: %v", len(archived), archived)
	} else if len(archived) == 0 {
		log.Printf("No specs to archive")
	}

	return archived, nil
}

// shouldArchive checks if a spec file should be archived.
// A spec is archived if its frontmatter contains "status: complete".
func shouldArchive(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	content := string(data)

	// Check for YAML frontmatter
	if !strings.HasPrefix(content, "---") {
		return false
	}

	// Find end of frontmatter
	endIdx := strings.Index(content[3:], "---")
	if endIdx == -1 {
		return false
	}

	frontmatter := content[3 : 3+endIdx]

	// Check for status: complete (with various whitespace patterns)
	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "status:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "status:"))
			return value == "complete"
		}
	}

	return false
}

// ArchiveTasksDir archives completed task directories.
// This moves entire unit directories when all tasks are complete.
func ArchiveTasksDir(specsDir string, dryRun bool) ([]string, error) {
	tasksDir := filepath.Join(specsDir, "tasks")
	completedDir := filepath.Join(specsDir, "completed", "tasks")

	if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
		return nil, nil // No tasks directory, nothing to archive
	}

	// Ensure completed/tasks directory exists
	if err := os.MkdirAll(completedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create completed tasks directory: %w", err)
	}

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	var archived []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		unitDir := filepath.Join(tasksDir, entry.Name())
		if isUnitComplete(unitDir) {
			dstDir := filepath.Join(completedDir, entry.Name())

			if dryRun {
				log.Printf("[dry-run] Would move %s -> %s", unitDir, dstDir)
				archived = append(archived, entry.Name())
				continue
			}

			if err := os.Rename(unitDir, dstDir); err != nil {
				return archived, fmt.Errorf("failed to move %s: %w", entry.Name(), err)
			}

			log.Printf("Archived task unit: %s", entry.Name())
			archived = append(archived, entry.Name())
		}
	}

	return archived, nil
}

// isUnitComplete checks if all tasks in a unit directory are complete.
func isUnitComplete(unitDir string) bool {
	entries, err := os.ReadDir(unitDir)
	if err != nil {
		return false
	}

	taskCount := 0
	completeCount := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		// Skip IMPLEMENTATION_PLAN.md for task counting
		if entry.Name() == "IMPLEMENTATION_PLAN.md" {
			continue
		}

		taskCount++
		path := filepath.Join(unitDir, entry.Name())
		if shouldArchive(path) {
			completeCount++
		}
	}

	// Unit is complete if there are tasks and all are complete
	return taskCount > 0 && taskCount == completeCount
}
