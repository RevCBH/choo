---
task: 3
status: complete
backpressure: "go test ./internal/worker/... -run GitHelper"
depends_on: []
---

# Git Verification Helpers

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/las-vegas/specs/CLAUDE-GIT.md`
**Task**: #3 of 6 in implementation plan

## Objective

Implement git helper methods on Worker for verifying operation outcomes (HEAD ref, commit existence, branch on remote, changed files).

## Dependencies

### External Specs (must be implemented)
- WORKER - provides Worker struct with worktreePath field

### Task Dependencies (within this unit)
- None

## Deliverables

### Files to Create/Modify

```
internal/worker/
├── git_delegate.go      # CREATE: Git helper methods (partial - helpers only)
└── git_delegate_test.go # CREATE: Helper tests (partial - helper tests only)
```

### Functions to Implement

```go
// internal/worker/git_delegate.go

package worker

import (
    "context"
    "os/exec"
    "strings"
)

// getHeadRef returns the current HEAD commit SHA
func (w *Worker) getHeadRef(ctx context.Context) (string, error) {
    cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
    cmd.Dir = w.worktreePath
    out, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(out)), nil
}

// hasNewCommit checks if HEAD has moved since the given ref
func (w *Worker) hasNewCommit(ctx context.Context, sinceRef string) (bool, error) {
    currentHead, err := w.getHeadRef(ctx)
    if err != nil {
        return false, err
    }
    return currentHead != sinceRef, nil
}

// branchExistsOnRemote checks if a branch exists on the remote
func (w *Worker) branchExistsOnRemote(ctx context.Context, branch string) (bool, error) {
    cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", "origin", branch)
    cmd.Dir = w.worktreePath
    out, err := cmd.Output()
    if err != nil {
        return false, err
    }
    return strings.TrimSpace(string(out)) != "", nil
}

// getChangedFiles returns list of modified/added/deleted files
func (w *Worker) getChangedFiles(ctx context.Context) ([]string, error) {
    cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
    cmd.Dir = w.worktreePath
    out, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    var files []string
    lines := strings.Split(string(out), "\n")
    for _, line := range lines {
        if len(line) >= 3 {
            // Format: "XY filename" where XY is status
            files = append(files, strings.TrimSpace(line[3:]))
        }
    }
    return files, nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run GitHelper
```

### Must Pass

| Test | Assertion |
|------|-----------|
| TestGetHeadRef_ReturnsCommitSHA | returns 40-char hex string |
| TestHasNewCommit_FalseWhenSame | returns false when HEAD unchanged |
| TestHasNewCommit_TrueAfterCommit | returns true after new commit |
| TestBranchExistsOnRemote_FalseForNonexistent | returns false for non-existent branch |
| TestGetChangedFiles_Empty | returns empty slice for clean worktree |
| TestGetChangedFiles_ParsesPorcelain | parses git status porcelain format |

### Test Implementations

```go
// internal/worker/git_delegate_test.go

package worker

import (
    "context"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

// setupTestGitRepo creates a temporary git repo for testing
func setupTestGitRepo(t *testing.T) (string, func()) {
    t.Helper()

    dir, err := os.MkdirTemp("", "git-helper-test-*")
    if err != nil {
        t.Fatalf("failed to create temp dir: %v", err)
    }

    cleanup := func() {
        os.RemoveAll(dir)
    }

    // Initialize git repo
    cmds := [][]string{
        {"git", "init"},
        {"git", "config", "user.email", "test@example.com"},
        {"git", "config", "user.name", "Test User"},
    }

    for _, args := range cmds {
        cmd := exec.Command(args[0], args[1:]...)
        cmd.Dir = dir
        if err := cmd.Run(); err != nil {
            cleanup()
            t.Fatalf("git setup failed: %v", err)
        }
    }

    // Create initial commit
    testFile := filepath.Join(dir, "test.txt")
    if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
        cleanup()
        t.Fatalf("failed to write test file: %v", err)
    }

    cmds = [][]string{
        {"git", "add", "-A"},
        {"git", "commit", "-m", "initial commit"},
    }

    for _, args := range cmds {
        cmd := exec.Command(args[0], args[1:]...)
        cmd.Dir = dir
        if err := cmd.Run(); err != nil {
            cleanup()
            t.Fatalf("git commit failed: %v", err)
        }
    }

    return dir, cleanup
}

func TestGetHeadRef_ReturnsCommitSHA(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    w := &Worker{worktreePath: dir}
    ref, err := w.getHeadRef(context.Background())
    if err != nil {
        t.Fatalf("getHeadRef failed: %v", err)
    }

    // SHA should be 40 hex characters
    if len(ref) != 40 {
        t.Errorf("expected 40-char SHA, got %d chars: %s", len(ref), ref)
    }

    for _, c := range ref {
        if !strings.ContainsRune("0123456789abcdef", c) {
            t.Errorf("SHA contains non-hex character: %c", c)
        }
    }
}

func TestHasNewCommit_FalseWhenSame(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    w := &Worker{worktreePath: dir}
    ref, _ := w.getHeadRef(context.Background())

    hasNew, err := w.hasNewCommit(context.Background(), ref)
    if err != nil {
        t.Fatalf("hasNewCommit failed: %v", err)
    }
    if hasNew {
        t.Error("expected false when HEAD unchanged")
    }
}

func TestHasNewCommit_TrueAfterCommit(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    w := &Worker{worktreePath: dir}
    refBefore, _ := w.getHeadRef(context.Background())

    // Create new commit
    testFile := filepath.Join(dir, "new.txt")
    if err := os.WriteFile(testFile, []byte("new content"), 0644); err != nil {
        t.Fatalf("failed to write file: %v", err)
    }

    cmds := [][]string{
        {"git", "add", "-A"},
        {"git", "commit", "-m", "new commit"},
    }
    for _, args := range cmds {
        cmd := exec.Command(args[0], args[1:]...)
        cmd.Dir = dir
        if err := cmd.Run(); err != nil {
            t.Fatalf("git command failed: %v", err)
        }
    }

    hasNew, err := w.hasNewCommit(context.Background(), refBefore)
    if err != nil {
        t.Fatalf("hasNewCommit failed: %v", err)
    }
    if !hasNew {
        t.Error("expected true after new commit")
    }
}

func TestBranchExistsOnRemote_FalseForNonexistent(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    w := &Worker{worktreePath: dir}

    // No remote configured, so any branch should return false
    exists, err := w.branchExistsOnRemote(context.Background(), "nonexistent-branch")
    if err != nil {
        // Error expected since no remote exists
        return
    }
    if exists {
        t.Error("expected false for non-existent branch")
    }
}

func TestGetChangedFiles_Empty(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    w := &Worker{worktreePath: dir}
    files, err := w.getChangedFiles(context.Background())
    if err != nil {
        t.Fatalf("getChangedFiles failed: %v", err)
    }
    if len(files) != 0 {
        t.Errorf("expected empty slice for clean worktree, got %v", files)
    }
}

func TestGetChangedFiles_ParsesPorcelain(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    // Create untracked file
    newFile := filepath.Join(dir, "untracked.txt")
    if err := os.WriteFile(newFile, []byte("untracked"), 0644); err != nil {
        t.Fatalf("failed to write file: %v", err)
    }

    // Modify existing file
    testFile := filepath.Join(dir, "test.txt")
    if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
        t.Fatalf("failed to modify file: %v", err)
    }

    w := &Worker{worktreePath: dir}
    files, err := w.getChangedFiles(context.Background())
    if err != nil {
        t.Fatalf("getChangedFiles failed: %v", err)
    }

    if len(files) != 2 {
        t.Errorf("expected 2 files, got %d: %v", len(files), files)
    }

    hasUntracked := false
    hasModified := false
    for _, f := range files {
        if f == "untracked.txt" {
            hasUntracked = true
        }
        if f == "test.txt" {
            hasModified = true
        }
    }

    if !hasUntracked {
        t.Error("expected untracked.txt in changed files")
    }
    if !hasModified {
        t.Error("expected test.txt in changed files")
    }
}
```

## NOT In Scope

- Claude invocation methods (handled in delegate tasks)
- Retry logic (handled in task 1)
- Escalation on failure (handled in delegate tasks)
