package git

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	// If no worktree path provided, we can only fetch
	if branch.Worktree == "" {
		result.Success = true
		return result, nil
	}

	// Rebase onto origin/<target_branch>
	targetRef := fmt.Sprintf("origin/%s", branch.TargetBranch)
	hasConflicts, err := Rebase(ctx, branch.Worktree, targetRef)
	if err != nil {
		result.Error = fmt.Errorf("rebase failed: %w", err)
		return result, result.Error
	}

	// If conflicts, delegate to conflict resolution
	if hasConflicts {
		if m.Claude == nil {
			result.Error = fmt.Errorf("conflicts detected but no Claude client for resolution")
			// Abort the rebase to leave repo in clean state
			_ = AbortRebase(ctx, branch.Worktree)
			return result, result.Error
		}

		if err := m.ResolveConflicts(ctx, branch.Worktree); err != nil {
			result.Error = fmt.Errorf("conflict resolution failed: %w", err)
			// Abort the rebase to leave repo in clean state
			_ = AbortRebase(ctx, branch.Worktree)
			return result, result.Error
		}
		result.ConflictsResolved = 1
		result.Attempts = 1
	}

	// Push (force-with-lease since we rebased)
	if err := ForcePushWithLease(ctx, branch.Worktree); err != nil {
		result.Error = fmt.Errorf("push failed: %w", err)
		return result, result.Error
	}

	// Schedule branch for deletion after batch completes
	m.ScheduleBranchDelete(branch.Name)

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
			log.Printf("warning: failed to delete remote branch %s: %v", branchName, err)
		}

		// Delete local branch
		if err := deleteBranch(ctx, m.RepoRoot, branchName, false); err != nil {
			log.Printf("warning: failed to delete local branch %s: %v", branchName, err)
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

// IsRebaseInProgress checks if a rebase is currently in progress
func IsRebaseInProgress(ctx context.Context, worktreePath string) (bool, error) {
	// Check for .git/rebase-merge or .git/rebase-apply directories
	gitDir := filepath.Join(worktreePath, ".git")

	// For worktrees, .git is a file pointing to the actual git dir
	gitDirContent, err := os.ReadFile(gitDir)
	if err == nil && strings.HasPrefix(string(gitDirContent), "gitdir:") {
		// This is a worktree, extract the actual git dir
		gitDir = strings.TrimSpace(strings.TrimPrefix(string(gitDirContent), "gitdir:"))
	}

	// Check for rebase-merge (interactive rebase)
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-merge")); err == nil {
		return true, nil
	}

	// Check for rebase-apply (non-interactive rebase)
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-apply")); err == nil {
		return true, nil
	}

	return false, nil
}

// IsMergeInProgress checks if a merge is currently in progress
func IsMergeInProgress(ctx context.Context, repoPath string) (bool, error) {
	// Check for MERGE_HEAD file which indicates merge in progress
	gitDir := filepath.Join(repoPath, ".git")

	// For worktrees, .git is a file pointing to the actual git dir
	gitDirContent, err := os.ReadFile(gitDir)
	if err == nil && strings.HasPrefix(string(gitDirContent), "gitdir:") {
		gitDir = strings.TrimSpace(strings.TrimPrefix(string(gitDirContent), "gitdir:"))
	}

	// Check for MERGE_HEAD (indicates merge in progress)
	if _, err := os.Stat(filepath.Join(gitDir, "MERGE_HEAD")); err == nil {
		return true, nil
	}

	return false, nil
}

// AbortRebase aborts an in-progress rebase
func AbortRebase(ctx context.Context, worktreePath string) error {
	_, err := gitExec(ctx, worktreePath, "rebase", "--abort")
	return err
}

// GetConflictedFiles returns the list of files with merge conflicts
func GetConflictedFiles(ctx context.Context, worktreePath string) ([]string, error) {
	out, err := gitExec(ctx, worktreePath, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return []string{}, nil
	}

	return strings.Split(out, "\n"), nil
}

// getConflictedFiles is kept for backward compatibility (internal use)
func getConflictedFiles(ctx context.Context, worktreePath string) ([]string, error) {
	return GetConflictedFiles(ctx, worktreePath)
}

// continueRebase continues a rebase after conflict resolution
func continueRebase(ctx context.Context, worktreePath string) error {
	_, err := gitExec(ctx, worktreePath, "rebase", "--continue")
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
//
//nolint:unused // WIP: will be used for conflict resolution
func readConflictFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
