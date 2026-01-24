package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/git"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// CleanupOptions holds flags for the cleanup command
type CleanupOptions struct {
	TasksDir   string // Path to specs/tasks/ directory
	ResetState bool   // Also reset frontmatter status
	Nuke       bool   // Force removal of in-progress worktrees/branches
}

// NewCleanupCmd creates the cleanup command
func NewCleanupCmd(app *App) *cobra.Command {
	opts := CleanupOptions{
		TasksDir:   "specs/tasks",
		ResetState: false,
		Nuke:       false,
	}

	cmd := &cobra.Command{
		Use:   "cleanup [tasks-dir]",
		Short: "Remove worktrees and optionally reset orchestration state",
		Long: `Cleanup removes all orchestrator worktrees and optionally resets
frontmatter state to pending.

By default, removes clean orchestrator worktrees and ralph/* branches.
Use --reset-state to also reset all task and unit statuses to pending.
Use --nuke to force removal of dirty worktrees and their branches.`,
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
	cmd.Flags().BoolVar(&opts.Nuke, "nuke", false, "Discard in-progress work (force remove dirty worktrees and branches)")

	return cmd
}

// Cleanup removes worktrees and optionally resets state
func (a *App) Cleanup(opts CleanupOptions) error {
	ctx := context.Background()
	var errors []error
	removedWorktrees := []string{}
	skippedWorktrees := []string{}
	removedBranches := []string{}
	var unitCount, taskCount int

	// Load config to get worktree base path
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	worktreeBase := ".ralph/worktrees" // fallback default
	cfg, cfgErr := config.LoadConfig(repoRoot)
	if cfgErr == nil {
		worktreeBase = cfg.Worktree.BasePath
	} else {
		worktreeBase = filepath.Join(repoRoot, worktreeBase)
	}

	manager := git.NewWorktreeManager(repoRoot, nil)
	manager.WorktreeBase = worktreeBase
	gitClient := git.NewClient(repoRoot)

	worktrees, err := manager.ListWorktrees(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to list worktrees: %w", err))
	} else {
		for _, wt := range worktrees {
			if !opts.Nuke {
				dirty, err := git.HasUncommittedChanges(ctx, wt.Path)
				if err != nil {
					errors = append(errors, fmt.Errorf("failed to check worktree status %s: %w", wt.Path, err))
					continue
				}
				if dirty {
					skippedWorktrees = append(skippedWorktrees, wt.Path)
					continue
				}
			}

			if err := manager.RemoveWorktree(ctx, wt); err != nil {
				errors = append(errors, fmt.Errorf("failed to remove worktree %s: %w", wt.Path, err))
				continue
			}
			removedWorktrees = append(removedWorktrees, wt.Path)
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
	if len(skippedWorktrees) > 0 {
		fmt.Printf("Skipped %d dirty worktrees (use --nuke to force):\n", len(skippedWorktrees))
		for _, wt := range skippedWorktrees {
			fmt.Printf("  - %s\n", wt)
		}
	}

	branchPrefix := "ralph/"
	currentBranch, _ := git.GetCurrentBranch(ctx, repoRoot)
	branches, err := gitClient.ListLocalBranchesWithPrefix(ctx, branchPrefix)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to list branches: %w", err))
	} else {
		preserve := map[string]bool{}
		if currentBranch != "" {
			preserve[currentBranch] = true
		}
		if len(skippedWorktrees) > 0 {
			for _, wt := range worktrees {
				for _, skipped := range skippedWorktrees {
					if wt.Path == skipped && wt.Branch != "" {
						preserve[wt.Branch] = true
					}
				}
			}
		}

		for _, branch := range branches {
			if preserve[branch] {
				continue
			}
			if err := gitClient.DeleteBranch(ctx, branch); err != nil {
				errors = append(errors, fmt.Errorf("failed to delete branch %s: %w", branch, err))
				continue
			}
			removedBranches = append(removedBranches, branch)
		}
	}

	if len(removedBranches) > 0 {
		fmt.Printf("Deleted %d branches:\n", len(removedBranches))
		for _, branch := range removedBranches {
			fmt.Printf("  - %s\n", branch)
		}
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
