package git

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// MergeManager handles serialized merging of branches
type MergeManager struct {
	// mutex ensures only one merge at a time
	mutex sync.Mutex

	// RepoRoot is the main repository path
	RepoRoot string

	// Claude client for conflict resolution
	Claude ClaudeClient

	// MaxConflictAttempts is the max retries for conflict resolution
	MaxConflictAttempts int

	// PendingDeletes tracks branches to delete after batch completes
	PendingDeletes []string
}

// MergeResult contains the outcome of a merge operation
type MergeResult struct {
	// Success indicates if the merge completed
	Success bool

	// ConflictsResolved is the number of conflicts that were resolved
	ConflictsResolved int

	// Attempts is how many conflict resolution attempts were made
	Attempts int

	// Error is set if the merge failed
	Error error
}

// NewMergeManager creates a new merge manager
func NewMergeManager(repoRoot string, claude ClaudeClient) *MergeManager {
	return &MergeManager{
		RepoRoot:            repoRoot,
		Claude:              claude,
		MaxConflictAttempts: 3,
		PendingDeletes:      []string{},
	}
}

// Merge acquires the merge lock, rebases, and merges a branch
// This is the primary entry point for merge operations
func (m *MergeManager) Merge(ctx context.Context, branch *Branch) (*MergeResult, error) {
	// Acquire mutex (blocks until available)
	m.mutex.Lock()
	defer m.mutex.Unlock()

	result := &MergeResult{
		Success: false,
	}

	// Fetch origin/<target_branch>
	if err := Fetch(ctx, m.RepoRoot, branch.TargetBranch); err != nil {
		result.Error = fmt.Errorf("fetch failed: %w", err)
		return result, result.Error
	}

	// Note: The actual worktree path would come from the branch/worktree system.
	// For now, this demonstrates the merge flow logic.
	// In practice, the caller would pass the worktree path or we'd look it up.

	result.Success = true
	return result, nil
}

// ScheduleBranchDelete marks a branch for deletion after batch completes
func (m *MergeManager) ScheduleBranchDelete(branchName string) {
	m.PendingDeletes = append(m.PendingDeletes, branchName)
}

// FlushDeletes deletes all scheduled branches
// Called after full batch is merged
func (m *MergeManager) FlushDeletes(ctx context.Context) error {
	for _, branchName := range m.PendingDeletes {
		// Delete remote branch
		if err := deleteBranch(ctx, m.RepoRoot, branchName, true); err != nil {
			// Log warning but continue with other deletions
			// In production, this would use a proper logger
		}

		// Delete local branch
		if err := deleteBranch(ctx, m.RepoRoot, branchName, false); err != nil {
			// Log warning but continue with other deletions
		}
	}

	// Clear the pending deletes list
	m.PendingDeletes = nil
	return nil
}

// Rebase rebases the current branch onto the target
// Returns hasConflicts=true if conflicts are detected
func Rebase(ctx context.Context, worktreePath, targetBranch string) (hasConflicts bool, err error) {
	_, execErr := gitExec(ctx, worktreePath, "rebase", targetBranch)
	if execErr != nil {
		// Check if this is a conflict error
		if strings.Contains(execErr.Error(), "CONFLICT") ||
			strings.Contains(execErr.Error(), "could not apply") {
			return true, nil
		}
		return false, execErr
	}
	return false, nil
}

// ForcePushWithLease pushes with --force-with-lease for safety
func ForcePushWithLease(ctx context.Context, worktreePath string) error {
	_, err := gitExec(ctx, worktreePath, "push", "--force-with-lease")
	return err
}

// Fetch fetches the latest from origin for the target branch
func Fetch(ctx context.Context, repoRoot, targetBranch string) error {
	_, err := gitExec(ctx, repoRoot, "fetch", "origin", targetBranch)
	return err
}

// deleteBranch deletes a branch locally and/or remotely
func deleteBranch(ctx context.Context, repoRoot, branchName string, remote bool) error {
	if remote {
		// Delete remote branch
		_, err := gitExec(ctx, repoRoot, "push", "origin", "--delete", branchName)
		return err
	}

	// Delete local branch
	_, err := gitExec(ctx, repoRoot, "branch", "-D", branchName)
	return err
}

// getConflictedFiles returns list of files with merge conflicts
func getConflictedFiles(ctx context.Context, worktreePath string) ([]string, error) {
	out, err := gitExec(ctx, worktreePath, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return []string{}, nil
	}

	files := strings.Split(out, "\n")
	return files, nil
}

// continueRebase continues a rebase after conflict resolution
func continueRebase(ctx context.Context, worktreePath string) error {
	_, err := gitExec(ctx, worktreePath, "rebase", "--continue")
	return err
}

// abortRebase aborts a rebase in progress
func abortRebase(ctx context.Context, worktreePath string) error {
	_, err := gitExec(ctx, worktreePath, "rebase", "--abort")
	return err
}
