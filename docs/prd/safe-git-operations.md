---
prd_id: safe-git-operations
title: "Safe Git Operations Interface"
status: draft
depends_on:
  - code-review
---

# PRD: Safe Git Operations Interface

## Document Info

| Field   | Value      |
| ------- | ---------- |
| Status  | Draft      |
| Author  | Claude     |
| Created | 2026-01-21 |
| Target  | v0.7       |

---

## Problem Statement

The current git operation pattern in the worker package has several issues that led to a production bug where tests inadvertently ran git commands (`git checkout .`, `git reset`, `git clean -fd`) on the actual repository instead of test directories.

### Root Cause

1. The `git.Runner` interface accepts any directory path, including empty strings
2. When `dir=""`, Go's `exec.Cmd` defaults to the current working directory
3. Tests that don't set `worktreePath` trigger code paths that run git commands
4. No validation exists at the interface level to prevent this

### Impact

- Test runs could reset uncommitted changes in the actual repository
- Potential for more severe corruption if destructive commands (`reset --hard`, `push -f`) were involved
- Difficult to debug - changes appear to "magically" revert

## Current Architecture

```go
// Low-level interface - no validation
type Runner interface {
    Exec(ctx context.Context, dir string, args ...string) (string, error)
    ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error)
}
```

**Usage pattern:**
```go
// Callers pass paths directly - no validation
w.runner().Exec(ctx, w.worktreePath, "checkout", ".")
w.runner().Exec(ctx, w.config.RepoRoot, "merge", branch)
```

**Problems:**
1. `dir` can be empty → runs in cwd (dangerous)
2. No type safety - easy to pass wrong path
3. Hard to mock at the right abstraction level
4. No audit trail or operation-level logging
5. Tests must stub raw git commands, not semantic operations

## Proposed Solution

### 1. GitOps Interface

Create a higher-level interface that encapsulates git operations with built-in safety:

```go
package git

import (
    "context"
    "errors"
    "fmt"
    "os"
)

// GitOps provides safe git operations bound to a specific repository path.
// All operations are validated to prevent accidental execution in wrong directories.
type GitOps interface {
    // Path returns the repository path this GitOps is bound to
    Path() string

    // Read operations
    Status(ctx context.Context) (StatusResult, error)
    RevParse(ctx context.Context, ref string) (string, error)
    Diff(ctx context.Context, base, head string) (string, error)
    Log(ctx context.Context, opts LogOpts) ([]Commit, error)

    // Branch operations
    CurrentBranch(ctx context.Context) (string, error)
    CheckoutBranch(ctx context.Context, branch string, create bool) error
    BranchExists(ctx context.Context, branch string) (bool, error)

    // Staging operations
    Add(ctx context.Context, paths ...string) error
    AddAll(ctx context.Context) error
    Reset(ctx context.Context, paths ...string) error

    // Commit operations
    Commit(ctx context.Context, msg string, opts CommitOpts) error

    // Working tree operations
    CheckoutFiles(ctx context.Context, paths ...string) error
    Clean(ctx context.Context, opts CleanOpts) error
    ResetHard(ctx context.Context, ref string) error

    // Remote operations
    Fetch(ctx context.Context, remote, ref string) error
    Push(ctx context.Context, remote, branch string, opts PushOpts) error

    // Merge operations
    Merge(ctx context.Context, branch string, opts MergeOpts) error
    MergeAbort(ctx context.Context) error
}

// NewGitOps creates a GitOps bound to a specific path.
// Returns error if path is empty, doesn't exist, or isn't a git repository.
func NewGitOps(path string) (GitOps, error) {
    if path == "" {
        return nil, errors.New("git: path cannot be empty")
    }

    info, err := os.Stat(path)
    if err != nil {
        return nil, fmt.Errorf("git: invalid path: %w", err)
    }
    if !info.IsDir() {
        return nil, fmt.Errorf("git: path is not a directory: %s", path)
    }

    // Optionally verify it's a git repo
    // _, err = os.Stat(filepath.Join(path, ".git"))

    return &gitOps{
        path:   path,
        runner: DefaultRunner(),
    }, nil
}
```

### 2. Option Types for Complex Operations

```go
type CommitOpts struct {
    NoVerify bool   // Skip pre-commit hooks
    Author   string // Override author
}

type CleanOpts struct {
    Force       bool // -f
    Directories bool // -d
    IgnoredOnly bool // -X
}

type PushOpts struct {
    Force      bool
    SetUpstream bool
}

type MergeOpts struct {
    FFOnly  bool
    NoFF    bool
    Message string
}

type LogOpts struct {
    MaxCount int
    Since    time.Time
    Until    time.Time
}
```

### 3. Mock Implementation for Testing

```go
// MockGitOps provides a testable implementation
type MockGitOps struct {
    path string

    // Stubbed responses
    StatusResult    StatusResult
    StatusErr       error
    CurrentBranchResult string
    // ... etc

    // Call tracking
    Calls []GitOpsCall
}

type GitOpsCall struct {
    Method string
    Args   []any
}

func NewMockGitOps(path string) *MockGitOps {
    return &MockGitOps{path: path}
}

func (m *MockGitOps) Path() string { return m.path }

func (m *MockGitOps) Status(ctx context.Context) (StatusResult, error) {
    m.Calls = append(m.Calls, GitOpsCall{Method: "Status"})
    return m.StatusResult, m.StatusErr
}
// ... etc
```

### 4. Worker Integration

```go
type Worker struct {
    // Replace raw path + runner with GitOps
    gitOps GitOps

    // Keep for provider invocation (needs raw path)
    worktreePath string

    // ... other fields
}

func NewWorker(opts WorkerOpts) (*Worker, error) {
    gitOps, err := git.NewGitOps(opts.WorktreePath)
    if err != nil {
        return nil, fmt.Errorf("invalid worktree path: %w", err)
    }

    return &Worker{
        gitOps:       gitOps,
        worktreePath: opts.WorktreePath,
        // ...
    }, nil
}

// Usage in cleanupWorktree
func (w *Worker) cleanupWorktree(ctx context.Context) {
    // GitOps validates path at construction - no empty path possible
    _ = w.gitOps.Reset(ctx)        // git reset HEAD
    _ = w.gitOps.Clean(ctx, CleanOpts{Force: true, Directories: true})
    _ = w.gitOps.CheckoutFiles(ctx, ".")
}
```

## Migration Strategy

### Phase 1: Add GitOps Interface (Non-Breaking)
1. Create `git.GitOps` interface and `git.NewGitOps` constructor
2. Create `git.MockGitOps` for testing
3. Add to Worker as optional field alongside existing `gitRunner`

### Phase 2: Migrate Critical Paths
1. Migrate `cleanupWorktree` to use GitOps
2. Migrate `commitReviewFixes` to use GitOps
3. Migrate merge operations to use GitOps
4. Update tests to use MockGitOps

### Phase 3: Full Migration
1. Migrate remaining git operations
2. Remove raw `gitRunner` field from Worker
3. Update all tests

### Phase 4: Enforce Invariants
1. Add linting rule to prevent raw `runner.Exec` calls in worker package
2. Consider removing public access to raw Runner

## Success Criteria

1. **Safety**: Empty path rejected at construction time, not runtime
2. **Testability**: Tests use `MockGitOps` instead of stubbing raw commands
3. **Auditability**: All git operations go through a single interface
4. **No regression**: All existing tests pass after migration

## Alternatives Considered

### 1. Validating Wrapper Around Runner

```go
type SafeRunner struct {
    inner Runner
}

func (s *SafeRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
    if dir == "" {
        return "", errors.New("dir cannot be empty")
    }
    return s.inner.Exec(ctx, dir, args...)
}
```

**Pros**: Minimal change
**Cons**: Still low-level, still hard to mock, doesn't prevent other misuses

### 2. Path Type Instead of String

```go
type RepoPath string

func NewRepoPath(path string) (RepoPath, error) {
    if path == "" {
        return "", errors.New("path cannot be empty")
    }
    return RepoPath(path), nil
}
```

**Pros**: Type safety
**Cons**: Doesn't help with mocking, still low-level interface

## Open Questions

1. Should GitOps verify the path is a git repository at construction?
2. Should we support both worktree and repo operations in one interface, or split them?
3. How to handle operations that need different paths (e.g., merge in repo root vs commit in worktree)?

## References

- Issue discovered in: `internal/worker/review.go:cleanupWorktree()`
- Related test files: `review_test.go`, `worker_test.go`
- Current git interface: `internal/git/exec.go`
