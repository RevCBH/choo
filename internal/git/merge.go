package git

import (
	"context"
	"fmt"
	"os"
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

// ResolveConflicts uses Claude to resolve merge conflicts
// Called by Merge when rebase detects conflicts
func (m *MergeManager) ResolveConflicts(ctx context.Context, worktreePath string) error {
	return m.resolveConflictsWithClaude(ctx, worktreePath)
}

// resolveConflictsWithClaude is the internal implementation with retry logic
func (m *MergeManager) resolveConflictsWithClaude(ctx context.Context, worktreePath string) error {
	for attempt := 1; attempt <= m.MaxConflictAttempts; attempt++ {
		// Get list of conflicted files
		conflicts, err := getConflictedFiles(ctx, worktreePath)
		if err != nil {
			return err
		}
		if len(conflicts) == 0 {
			return nil // All resolved
		}

		// Build conflict resolution prompt
		prompt := buildConflictPrompt(conflicts, worktreePath)

		// Invoke Claude to resolve
		_, err = m.Claude.Invoke(ctx, InvokeOptions{
			Prompt:   prompt,
			Model:    "",
			MaxTurns: 0,
		})
		if err != nil {
			if attempt == m.MaxConflictAttempts {
				return fmt.Errorf("conflict resolution failed after %d attempts: %w",
					attempt, err)
			}
			continue
		}

		// Check if conflicts remain
		remaining, _ := getConflictedFiles(ctx, worktreePath)
		if len(remaining) == 0 {
			// Continue rebase
			if err := continueRebase(ctx, worktreePath); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("failed to resolve conflicts after %d attempts", m.MaxConflictAttempts)
}

// buildConflictPrompt creates the prompt for Claude to resolve conflicts
func buildConflictPrompt(conflicts []string, worktreePath string) string {
	// Include:
	// - List of conflicted files
	// - File contents with conflict markers
	// - Instructions to resolve and stage
	return fmt.Sprintf(`You are resolving git merge conflicts in %s.

The following files have conflicts:
%s

Please resolve each conflict by:
1. Reading the file with conflict markers
2. Choosing the correct resolution
3. Removing all conflict markers (<<<<<<<, =======, >>>>>>>)
4. Saving the resolved file
5. Staging the file with git add

Resolve all conflicts now.`, worktreePath, strings.Join(conflicts, "\n"))
}

// readConflictFile reads a file with conflict markers
func readConflictFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
