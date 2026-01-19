---
task: 1
status: pending
backpressure: "go test ./internal/git/... -run \"Rebase|Conflict\""
depends_on: []
---

# Git Conflict Detection Helpers

**Parent spec**: `/specs/CONFLICT-RESOLUTION.md`
**Task**: #1 of 4 in implementation plan

## Objective

Export git helper functions for rebase state detection and conflict file enumeration.

## Dependencies

### External Specs (must be implemented)
- GIT spec (completed) - provides gitExec and worktree utilities

### Task Dependencies (within this unit)
- None

## Deliverables

### Files to Modify
```
internal/git/
└── merge.go    # MODIFY: Export existing helpers, add IsRebaseInProgress
```

### Functions to Implement

The existing `abortRebase` and `getConflictedFiles` functions need to be exported. Add `IsRebaseInProgress` for checking rebase state via filesystem.

```go
// internal/git/merge.go (additions/modifications)

import (
	"os"
	"path/filepath"
	"strings"
)

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

// AbortRebase aborts an in-progress rebase (export existing abortRebase)
func AbortRebase(ctx context.Context, worktreePath string) error {
	_, err := gitExec(ctx, worktreePath, "rebase", "--abort")
	return err
}

// GetConflictedFiles returns the list of files with merge conflicts (export existing getConflictedFiles)
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
```

### Tests to Add

```go
// internal/git/merge_test.go (additions)

func TestIsRebaseInProgress_NotInRebase(t *testing.T) {
	// Create a clean git repo
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user for commits
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()

	inRebase, err := IsRebaseInProgress(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inRebase {
		t.Error("expected not in rebase")
	}
}

func TestGetConflictedFiles_NoConflicts(t *testing.T) {
	// Create a clean git repo with initial commit
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()

	// Create initial commit
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("content"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()

	files, err := GetConflictedFiles(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected no conflicted files, got %v", files)
	}
}

func TestAbortRebase_NoRebaseInProgress(t *testing.T) {
	// Create a clean git repo
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()

	// Create initial commit
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("content"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()

	// AbortRebase should error when no rebase in progress
	err := AbortRebase(context.Background(), dir)
	if err == nil {
		t.Error("expected error when no rebase in progress")
	}
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/git/... -run "Rebase|Conflict"
```

## NOT In Scope
- Merge flow orchestration (Task 3)
- Prompt building (Task 2)
- Force push and PR merge (Task 4)
- Retry logic with backoff (Task 3)
