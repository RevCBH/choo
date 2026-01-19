package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WorktreeManager handles creation and removal of git worktrees
type WorktreeManager struct {
	// RepoRoot is the absolute path to the main repository
	RepoRoot string

	// WorktreeBase is the base directory for worktrees (default: .ralph/worktrees/)
	WorktreeBase string

	// SetupCommands are conditional commands to run after worktree creation
	SetupCommands []ConditionalCommand

	// Claude client for branch name generation (may be nil for testing)
	Claude interface{} // placeholder for *claude.Client
}

// Worktree represents an active git worktree
type Worktree struct {
	// Path is the absolute path to the worktree directory
	Path string

	// Branch is the branch name checked out in this worktree
	Branch string

	// UnitID is the unit this worktree is associated with
	UnitID string

	// CreatedAt is when this worktree was created
	CreatedAt time.Time
}

// ConditionalCommand runs a command only if a condition file exists
type ConditionalCommand struct {
	// ConditionFile is the file that must exist for the command to run
	ConditionFile string

	// Command is the command to execute
	Command string

	// Args are the command arguments
	Args []string

	// Description is a human-readable description for logging
	Description string
}

// NewWorktreeManager creates a new worktree manager
func NewWorktreeManager(repoRoot string, claude interface{}) *WorktreeManager {
	worktreeBase := filepath.Join(repoRoot, ".ralph", "worktrees")
	return &WorktreeManager{
		RepoRoot:      repoRoot,
		WorktreeBase:  worktreeBase,
		SetupCommands: DefaultSetupCommands(),
		Claude:        claude,
	}
}

// CreateWorktree creates a new worktree for a unit
func (m *WorktreeManager) CreateWorktree(ctx context.Context, unitID, targetBranch string) (*Worktree, error) {
	// Create worktree path
	worktreePath := filepath.Join(m.WorktreeBase, unitID)

	// Ensure base directory exists
	if err := os.MkdirAll(m.WorktreeBase, 0755); err != nil {
		return nil, fmt.Errorf("failed to create worktree base directory: %w", err)
	}

	// Create branch name (for now, simple naming; Task #3 will handle proper branch naming)
	branchName := fmt.Sprintf("ralph/%s", unitID)

	// Create the worktree
	_, err := gitExec(ctx, m.RepoRoot, "worktree", "add", "-b", branchName, worktreePath, targetBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	// Run setup commands
	if err := m.runSetupCommands(ctx, worktreePath); err != nil {
		// Clean up worktree on setup failure
		_ = m.RemoveWorktree(ctx, &Worktree{Path: worktreePath, Branch: branchName, UnitID: unitID})
		return nil, fmt.Errorf("setup commands failed: %w", err)
	}

	return &Worktree{
		Path:      worktreePath,
		Branch:    branchName,
		UnitID:    unitID,
		CreatedAt: time.Now(),
	}, nil
}

// RemoveWorktree removes a worktree and its directory
func (m *WorktreeManager) RemoveWorktree(ctx context.Context, wt *Worktree) error {
	// Remove the worktree
	_, err := gitExec(ctx, m.RepoRoot, "worktree", "remove", wt.Path, "--force")
	if err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	// Clean up the directory if it still exists
	if err := os.RemoveAll(wt.Path); err != nil {
		return fmt.Errorf("failed to remove worktree directory: %w", err)
	}

	return nil
}

// ListWorktrees returns all active worktrees managed by ralph
func (m *WorktreeManager) ListWorktrees(ctx context.Context) ([]*Worktree, error) {
	output, err := gitExec(ctx, m.RepoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Resolve WorktreeBase to handle symlinks (e.g., /var vs /private/var on macOS)
	resolvedBase, err := filepath.EvalSymlinks(m.WorktreeBase)
	if err != nil {
		// If WorktreeBase doesn't exist yet, that's okay
		resolvedBase = m.WorktreeBase
	}

	var worktrees []*Worktree
	lines := strings.Split(output, "\n")

	var currentPath, currentBranch string

	processEntry := func() {
		if currentPath == "" {
			return
		}

		// Resolve current path to handle symlinks
		resolvedPath, err := filepath.EvalSymlinks(currentPath)
		if err != nil {
			resolvedPath = currentPath
		}

		// Check if this worktree is under our base directory
		if strings.HasPrefix(resolvedPath, resolvedBase) {
			// Extract unitID from path
			relPath, err := filepath.Rel(resolvedBase, resolvedPath)
			if err == nil && relPath != "." && relPath != "" {
				worktrees = append(worktrees, &Worktree{
					Path:      currentPath,
					Branch:    currentBranch,
					UnitID:    relPath,
					CreatedAt: time.Time{}, // Unknown from git worktree list
				})
			}
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			// End of worktree entry
			processEntry()
			currentPath = ""
			currentBranch = ""
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			branchRef := strings.TrimPrefix(line, "branch ")
			// Extract branch name from refs/heads/
			currentBranch = strings.TrimPrefix(branchRef, "refs/heads/")
		}
	}

	// Handle last entry if file doesn't end with blank line
	processEntry()

	return worktrees, nil
}

// CleanupOrphans removes worktrees that no longer have associated units
func (m *WorktreeManager) CleanupOrphans(ctx context.Context) error {
	worktrees, err := m.ListWorktrees(ctx)
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	for _, wt := range worktrees {
		// Check if worktree directory exists
		if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
			// Directory doesn't exist, prune the worktree reference
			_, _ = gitExec(ctx, m.RepoRoot, "worktree", "prune")
		}
	}

	return nil
}

// DefaultSetupCommands returns the default conditional setup commands
func DefaultSetupCommands() []ConditionalCommand {
	return []ConditionalCommand{
		{ConditionFile: "package.json", Command: "npm", Args: []string{"install"}, Description: "Installing npm dependencies"},
		{ConditionFile: "pnpm-lock.yaml", Command: "pnpm", Args: []string{"install"}, Description: "Installing pnpm dependencies"},
		{ConditionFile: "yarn.lock", Command: "yarn", Args: []string{"install"}, Description: "Installing yarn dependencies"},
		{ConditionFile: "Cargo.toml", Command: "cargo", Args: []string{"fetch"}, Description: "Fetching Cargo dependencies"},
		{ConditionFile: "go.mod", Command: "go", Args: []string{"mod", "download"}, Description: "Downloading Go modules"},
	}
}

// runSetupCommands runs conditional setup commands in the worktree
func (m *WorktreeManager) runSetupCommands(ctx context.Context, worktreePath string) error {
	for _, cmd := range m.SetupCommands {
		// Check if condition file exists
		conditionPath := filepath.Join(worktreePath, cmd.ConditionFile)
		if _, err := os.Stat(conditionPath); err == nil {
			// Condition file exists, run the command
			execCmd := exec.CommandContext(ctx, cmd.Command, cmd.Args...)
			execCmd.Dir = worktreePath

			if err := execCmd.Run(); err != nil {
				return fmt.Errorf("%s failed: %w", cmd.Description, err)
			}

			// Only run the first matching command
			return nil
		}
	}

	// No matching commands is not an error
	return nil
}
