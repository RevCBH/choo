package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/choo/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// CleanupOptions holds flags for the cleanup command
type CleanupOptions struct {
	TasksDir   string // Path to specs/tasks/ directory
	ResetState bool   // Also reset frontmatter status
}

// NewCleanupCmd creates the cleanup command
func NewCleanupCmd(app *App) *cobra.Command {
	opts := CleanupOptions{
		TasksDir:   "specs/tasks",
		ResetState: false,
	}

	cmd := &cobra.Command{
		Use:   "cleanup [tasks-dir]",
		Short: "Remove worktrees and optionally reset orchestration state",
		Long: `Cleanup removes all orchestrator worktrees and optionally resets
frontmatter state to pending.

By default, only removes worktrees from .ralph/worktrees/.
Use --reset-state to also reset all task and unit statuses to pending.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Override tasks-dir from positional arg if provided
			if len(args) > 0 {
				opts.TasksDir = args[0]
			}

			return app.Cleanup(opts)
		},
	}

	// Add flags
	cmd.Flags().BoolVar(&opts.ResetState, "reset-state", false, "Reset frontmatter status to pending")

	return cmd
}

// Cleanup removes worktrees and optionally resets state
func (a *App) Cleanup(opts CleanupOptions) error {
	var errors []error
	removedWorktrees := []string{}
	var unitCount, taskCount int

	// Load config to get worktree base path
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	worktreeBase := ".ralph/worktrees" // fallback default
	if cfg, err := config.LoadConfig(repoRoot); err == nil {
		worktreeBase = cfg.Worktree.BasePath
	}

	// Find and remove all orchestrator worktrees
	if _, err := os.Stat(worktreeBase); err == nil {
		// Worktree directory exists, list and remove worktrees
		entries, err := os.ReadDir(worktreeBase)
		if err != nil {
			return fmt.Errorf("failed to read worktree directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				worktreePath := filepath.Join(worktreeBase, entry.Name())

				// Attempt to remove worktree
				if err := os.RemoveAll(worktreePath); err != nil {
					errors = append(errors, fmt.Errorf("failed to remove worktree %s: %w", worktreePath, err))
				} else {
					removedWorktrees = append(removedWorktrees, worktreePath)
				}
			}
		}
	}

	// If reset-state is enabled, reset frontmatter
	if opts.ResetState {
		uc, tc, err := resetFrontmatterState(opts.TasksDir)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to reset frontmatter state: %w", err))
		}
		unitCount = uc
		taskCount = tc
	}

	// Print summary
	if len(removedWorktrees) > 0 {
		fmt.Printf("Removed %d worktrees:\n", len(removedWorktrees))
		for _, wt := range removedWorktrees {
			fmt.Printf("  - %s\n", wt)
		}
	} else {
		fmt.Println("No worktrees found to remove")
	}

	if opts.ResetState {
		fmt.Printf("Reset state for %d units (%d tasks)\n", unitCount, taskCount)
	}

	// Return errors if any occurred
	if len(errors) > 0 {
		var errMsgs []string
		for _, err := range errors {
			errMsgs = append(errMsgs, err.Error())
		}
		return fmt.Errorf("cleanup completed with errors:\n%s", strings.Join(errMsgs, "\n"))
	}

	return nil
}

// resetFrontmatterState resets all task/unit status to pending
func resetFrontmatterState(tasksDir string) (int, int, error) {
	unitCount := 0
	taskCount := 0
	var errors []error

	// Find all unit directories
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		unitDir := filepath.Join(tasksDir, entry.Name())

		// Reset IMPLEMENTATION_PLAN.md
		implPlanPath := filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md")
		if _, err := os.Stat(implPlanPath); err == nil {
			if err := resetUnitFrontmatter(implPlanPath); err != nil {
				errors = append(errors, fmt.Errorf("failed to reset %s: %w", implPlanPath, err))
			} else {
				unitCount++
			}
		}

		// Reset task files
		pattern := filepath.Join(unitDir, "[0-9][0-9]-*.md")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to glob task files in %s: %w", unitDir, err))
			continue
		}

		for _, taskPath := range matches {
			if err := resetTaskFrontmatter(taskPath); err != nil {
				errors = append(errors, fmt.Errorf("failed to reset %s: %w", taskPath, err))
			} else {
				taskCount++
			}
		}
	}

	if len(errors) > 0 {
		var errMsgs []string
		for _, err := range errors {
			errMsgs = append(errMsgs, err.Error())
		}
		return unitCount, taskCount, fmt.Errorf("reset frontmatter errors:\n%s", strings.Join(errMsgs, "\n"))
	}

	return unitCount, taskCount, nil
}

// resetUnitFrontmatter resets unit frontmatter fields to pending
func resetUnitFrontmatter(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Extract frontmatter
	fm, body, err := extractFrontmatter(content)
	if err != nil {
		return err
	}

	// Parse YAML
	var data map[string]interface{}
	if err := yaml.Unmarshal(fm, &data); err != nil {
		return err
	}

	// Reset orchestrator fields
	data["orch_status"] = "pending"
	data["orch_branch"] = nil
	data["orch_pr_number"] = nil
	data["orch_started_at"] = nil
	data["orch_completed_at"] = nil

	// Marshal back to YAML
	newFM, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	// Reconstruct file
	newContent := fmt.Sprintf("---\n%s---\n%s", string(newFM), string(body))
	return os.WriteFile(path, []byte(newContent), 0644)
}

// resetTaskFrontmatter resets task frontmatter status to pending
func resetTaskFrontmatter(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Extract frontmatter
	fm, body, err := extractFrontmatter(content)
	if err != nil {
		return err
	}

	// Parse YAML
	var data map[string]interface{}
	if err := yaml.Unmarshal(fm, &data); err != nil {
		return err
	}

	// Reset status to pending
	data["status"] = "pending"

	// Marshal back to YAML
	newFM, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	// Reconstruct file
	newContent := fmt.Sprintf("---\n%s---\n%s", string(newFM), string(body))
	return os.WriteFile(path, []byte(newContent), 0644)
}

// extractFrontmatter extracts YAML frontmatter from markdown content
func extractFrontmatter(content []byte) (frontmatter []byte, body []byte, err error) {
	str := string(content)

	// Check for opening delimiter
	if !strings.HasPrefix(str, "---\n") {
		return nil, content, fmt.Errorf("no frontmatter found")
	}

	// Find closing delimiter
	rest := str[4:] // Skip opening "---\n"
	endIdx := strings.Index(rest, "\n---\n")
	if endIdx == -1 {
		return nil, nil, fmt.Errorf("unclosed frontmatter")
	}

	frontmatter = []byte(rest[:endIdx+1]) // Include the newline
	body = []byte(rest[endIdx+5:])        // Skip "\n---\n"

	return frontmatter, body, nil
}
