package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/spf13/cobra"
)

// ArchiveOptions holds flags for the archive command
type ArchiveOptions struct {
	TasksDir string // Path to specs/tasks/ directory
}

// NewArchiveCmd creates the archive command
func NewArchiveCmd(app *App) *cobra.Command {
	opts := ArchiveOptions{
		TasksDir: "specs/tasks",
	}

	cmd := &cobra.Command{
		Use:   "archive [tasks-dir]",
		Short: "Archive completed specs and tasks",
		Long: `Archive moves completed units from specs/tasks into specs/completed/tasks,
and moves matching spec files from specs/ into specs/completed.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.TasksDir = args[0]
			}
			return app.ArchiveCompleted(opts)
		},
	}

	return cmd
}

// ArchiveCompleted moves completed units/tasks and matching specs into specs/completed.
func (a *App) ArchiveCompleted(opts ArchiveOptions) error {
	units, err := discovery.Discover(opts.TasksDir)
	if err != nil {
		return fmt.Errorf("failed to load discovery: %w", err)
	}

	specsDir := filepath.Dir(opts.TasksDir)
	completedDir := filepath.Join(specsDir, "completed")
	completedTasksDir := filepath.Join(completedDir, "tasks")

	specIndex, err := indexSpecFiles(specsDir)
	if err != nil {
		return err
	}

	repoRoot := gitRepoRoot(opts.TasksDir)

	archivedUnits := 0
	archivedSpecs := 0
	skipped := make(map[string][]string)

	for _, unit := range units {
		complete, reasons := unitCompletion(unit)
		if !complete {
			skipped[unit.ID] = reasons
			continue
		}

		unitDest := filepath.Join(completedTasksDir, unit.ID)
		if err := movePath(unit.Path, unitDest, repoRoot); err != nil {
			return fmt.Errorf("failed to archive unit %s: %w", unit.ID, err)
		}
		archivedUnits++

		if specPath, ok := specIndex[strings.ToLower(unit.ID)]; ok {
			specDest := filepath.Join(completedDir, filepath.Base(specPath))
			if err := movePath(specPath, specDest, repoRoot); err != nil {
				return fmt.Errorf("failed to archive spec %s: %w", filepath.Base(specPath), err)
			}
			archivedSpecs++
		}
	}

	if archivedUnits == 0 && archivedSpecs == 0 {
		if len(skipped) > 0 {
			fmt.Printf("No completed units found to archive. Skipped %d incomplete unit(s).\n", len(skipped))
			if a.verbose {
				printSkipReasons(skipped)
			}
		} else {
			fmt.Println("No completed units found to archive.")
		}
		return nil
	}

	fmt.Printf("Archived %d unit(s) and %d spec file(s).\n", archivedUnits, archivedSpecs)
	if len(skipped) > 0 {
		fmt.Printf("Skipped %d incomplete unit(s).\n", len(skipped))
		if a.verbose {
			printSkipReasons(skipped)
		}
	}

	return nil
}

func unitCompletion(unit *discovery.Unit) (bool, []string) {
	var reasons []string
	if unit.Status != discovery.UnitStatusComplete && unit.Status != discovery.UnitStatusPending {
		reasons = append(reasons, fmt.Sprintf("unit status %s", unit.Status))
	}

	var incomplete []string
	for _, task := range unit.Tasks {
		if task.Status != discovery.TaskStatusComplete {
			incomplete = append(incomplete, fmt.Sprintf("#%d %s (%s)", task.Number, task.FilePath, task.Status))
		}
	}
	if len(incomplete) > 0 {
		reasons = append(reasons, "incomplete tasks: "+summarizeList(incomplete, 5))
	}

	return len(reasons) == 0, reasons
}

func summarizeList(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(items[:max], ", "), len(items)-max)
}

func printSkipReasons(skipped map[string][]string) {
	for unitID, reasons := range skipped {
		if len(reasons) == 0 {
			fmt.Printf("Incomplete unit %s: unknown reason\n", unitID)
			continue
		}
		fmt.Printf("Incomplete unit %s: %s\n", unitID, strings.Join(reasons, "; "))
	}
}

func indexSpecFiles(specsDir string) (map[string]string, error) {
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read specs dir: %w", err)
	}

	specs := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		key := strings.ToLower(strings.TrimSuffix(name, ".md"))
		if existing, ok := specs[key]; ok {
			return nil, fmt.Errorf("duplicate spec name for %s: %s and %s", key, filepath.Base(existing), name)
		}
		specs[key] = filepath.Join(specsDir, name)
	}
	return specs, nil
}

func gitRepoRoot(fromPath string) string {
	// Use git -C to run from the given path context, so this works
	// even when invoked from outside the repo with an absolute path
	cmd := exec.Command("git", "-C", fromPath, "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func movePath(src, dst, repoRoot string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	relSrc, relDst, ok := relPaths(repoRoot, src, dst)
	if ok {
		tracked, err := hasTrackedFiles(repoRoot, relSrc)
		if err == nil && tracked {
			return gitMove(repoRoot, relSrc, relDst)
		}
	}

	return os.Rename(src, dst)
}

func relPaths(repoRoot, src, dst string) (string, string, bool) {
	if repoRoot == "" {
		return "", "", false
	}

	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", false
	}
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return "", "", false
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return "", "", false
	}

	relSrc, err := filepath.Rel(absRoot, absSrc)
	if err != nil || relOutsideRoot(relSrc) {
		return "", "", false
	}
	relDst, err := filepath.Rel(absRoot, absDst)
	if err != nil || relOutsideRoot(relDst) {
		return "", "", false
	}

	return relSrc, relDst, true
}

func relOutsideRoot(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func hasTrackedFiles(repoRoot, path string) (bool, error) {
	cmd := exec.Command("git", "ls-files", "-z", "--", path)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

func gitMove(repoRoot, src, dst string) error {
	cmd := exec.Command("git", "mv", "--", src, dst)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git mv failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}
