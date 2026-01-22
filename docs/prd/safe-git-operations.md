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

## Safety Invariants & Guardrails

### 1. Repo/Worktree Invariants (Hard-Fail)

These checks run at construction time and cause `NewGitOps` to fail if violated:

| Invariant | Check | Error |
|-----------|-------|-------|
| Non-empty path | `path != ""` | `ErrEmptyPath` |
| Absolute path | `filepath.IsAbs(path)` | `ErrRelativePath` |
| Canonical path | `filepath.EvalSymlinks` + `filepath.Clean` | `ErrNonCanonicalPath` |
| Path exists | `os.Stat(path)` | `ErrPathNotFound` |
| Path is directory | `info.IsDir()` | `ErrNotDirectory` |
| Valid git worktree | `git rev-parse --is-inside-work-tree` | `ErrNotGitRepo` |
| Path matches toplevel | `git rev-parse --show-toplevel == path` | `ErrPathMismatch` |
| Worktree not repo root | `--absolute-git-dir` resolves to worktree git dir | `ErrRepoRootNotAllowed` (unless `AllowRepoRoot=true`) |
| Under worktree base | Path under `RALPH_WORKTREE_BASE` | `ErrOutsideWorktreeBase` (unless `AllowRepoRoot=true`) |

```go
// Path validation at construction
func NewGitOps(path string, opts GitOpsOpts) (GitOps, error) {
    // 1. Non-empty
    if path == "" {
        return nil, ErrEmptyPath
    }

    // 2. Absolute
    if !filepath.IsAbs(path) {
        return nil, fmt.Errorf("%w: %s", ErrRelativePath, path)
    }

    // 3. Canonical (resolve symlinks, clean)
    canonical, err := filepath.EvalSymlinks(path)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrNonCanonicalPath, err)
    }
    canonical = filepath.Clean(canonical)

    // 4-5. Exists and is directory
    info, err := os.Stat(canonical)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrPathNotFound, err)
    }
    if !info.IsDir() {
        return nil, fmt.Errorf("%w: %s", ErrNotDirectory, canonical)
    }

    // 6. Valid git worktree
    toplevel, err := runGit(canonical, "rev-parse", "--show-toplevel")
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrNotGitRepo, err)
    }
    toplevel = filepath.Clean(strings.TrimSpace(toplevel))

    // 7. Path matches toplevel
    if toplevel != canonical {
        return nil, fmt.Errorf("%w: toplevel=%s, path=%s", ErrPathMismatch, toplevel, canonical)
    }

    // 8. Worktree vs repo root check
    if !opts.AllowRepoRoot {
        gitDir, _ := runGit(canonical, "rev-parse", "--absolute-git-dir")
        if !strings.Contains(gitDir, "worktrees") {
            return nil, ErrRepoRootNotAllowed
        }
    }

    // 9. Under worktree base
    if !opts.AllowRepoRoot && opts.WorktreeBase != "" {
        base := filepath.Clean(opts.WorktreeBase)
        rel, err := filepath.Rel(base, canonical)
        if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
            return nil, fmt.Errorf("%w: path=%s, base=%s", ErrOutsideWorktreeBase, canonical, base)
        }
    }

    return &gitOps{path: canonical, opts: opts, runner: DefaultRunner()}, nil
}
```

### 2. Branch/Remote Invariants (Hard-Fail for Destructive Ops)

Before any write or destructive operation, assert:

| Check | Operations Affected | Error |
|-------|---------------------|-------|
| HEAD matches expected branch | All write ops | `ErrUnexpectedBranch` |
| Remote URL matches expected | Push, Fetch | `ErrUnexpectedRemote` |
| Not targeting protected branch | Push, Merge, ResetHard | `ErrProtectedBranch` |

```go
// BranchGuard enforces branch/remote constraints
type BranchGuard struct {
    ExpectedBranch       string   // Exact branch name (e.g., "feature/foo")
    AllowedBranchPrefixes []string // Allowed prefixes (e.g., ["feature/", "fix/"])
    AllowedRemotes       []string // Allowed remote URLs
    ProtectedBranches    []string // Never allow writes to these (default: ["main", "master"])
}

func (g *gitOps) validateBranchGuard(ctx context.Context, remote string) error {
    if g.opts.BranchGuard == nil {
        return nil // No guard configured
    }

    guard := g.opts.BranchGuard
    branch, err := g.CurrentBranch(ctx)
    if err != nil {
        return fmt.Errorf("branch guard: %w", err)
    }

    // Check exact match
    if guard.ExpectedBranch != "" && branch != guard.ExpectedBranch {
        return fmt.Errorf("%w: expected=%s, actual=%s", ErrUnexpectedBranch, guard.ExpectedBranch, branch)
    }

    // Check prefix match
    if len(guard.AllowedBranchPrefixes) > 0 {
        allowed := false
        for _, prefix := range guard.AllowedBranchPrefixes {
            if strings.HasPrefix(branch, prefix) {
                allowed = true
                break
            }
        }
        if !allowed {
            return fmt.Errorf("%w: branch=%s, allowed=%v", ErrUnexpectedBranch, branch, guard.AllowedBranchPrefixes)
        }
    }

    // Check protected branches
    protected := guard.ProtectedBranches
    if len(protected) == 0 {
        protected = []string{"main", "master"}
    }
    for _, p := range protected {
        if branch == p {
            return fmt.Errorf("%w: %s", ErrProtectedBranch, branch)
        }
    }

    // Check remote URL
    if remote != "" && len(guard.AllowedRemotes) > 0 {
        url, err := g.runner.Exec(ctx, g.path, "remote", "get-url", remote)
        if err != nil {
            return fmt.Errorf("branch guard: %w", err)
        }
        url = strings.TrimSpace(url)
        allowed := false
        for _, r := range guard.AllowedRemotes {
            if url == r {
                allowed = true
                break
            }
        }
        if !allowed {
            return fmt.Errorf("%w: remote=%s, url=%s, allowed=%v", ErrUnexpectedRemote, remote, url, guard.AllowedRemotes)
        }
    }

    return nil
}
```

### 3. Destructive Ops Require Explicit Opt-In

The following operations are considered destructive and require `AllowDestructive=true`:

| Operation | Why Destructive |
|-----------|-----------------|
| `ResetHard(ref)` | Discards uncommitted changes |
| `Clean(opts)` | Deletes untracked files |
| `CheckoutFiles(paths...)` | Discards uncommitted changes to files |
| `Push(opts{Force: true})` | Rewrites remote history |
| `Push(opts{ForceWithLease: true})` | Can rewrite remote history |

```go
type GitOpsOpts struct {
    // ... other fields
    AllowDestructive bool // Must be true for destructive operations
}

var ErrDestructiveNotAllowed = errors.New("destructive operation not allowed: set AllowDestructive=true")

func (g *gitOps) ResetHard(ctx context.Context, ref string) error {
    if !g.opts.AllowDestructive {
        return fmt.Errorf("%w: ResetHard", ErrDestructiveNotAllowed)
    }
    if err := g.validateBranchGuard(ctx, ""); err != nil {
        return err
    }
    // ... execute
}

func (g *gitOps) Clean(ctx context.Context, opts CleanOpts) error {
    if !g.opts.AllowDestructive {
        return fmt.Errorf("%w: Clean", ErrDestructiveNotAllowed)
    }
    // ... execute
}

func (g *gitOps) Push(ctx context.Context, remote, branch string, opts PushOpts) error {
    if (opts.Force || opts.ForceWithLease) && !g.opts.AllowDestructive {
        return fmt.Errorf("%w: Push --force", ErrDestructiveNotAllowed)
    }
    if err := g.validateBranchGuard(ctx, remote); err != nil {
        return err
    }
    // ... execute
}
```

### 4. Per-Repo Write Lock

Serialize write operations to prevent concurrent workers from corrupting git state:

```go
// Global lock registry keyed by canonical repo path
var repoLocks = struct {
    sync.Mutex
    locks map[string]*sync.Mutex
}{locks: make(map[string]*sync.Mutex)}

func getRepoLock(path string) *sync.Mutex {
    repoLocks.Lock()
    defer repoLocks.Unlock()
    if repoLocks.locks[path] == nil {
        repoLocks.locks[path] = &sync.Mutex{}
    }
    return repoLocks.locks[path]
}

// Write operations acquire lock
func (g *gitOps) Commit(ctx context.Context, msg string, opts CommitOpts) error {
    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    // ... execute commit
}

// Operations requiring lock:
// - Commit, Merge, MergeAbort
// - ResetHard, Reset, Clean
// - CheckoutBranch, CheckoutFiles
// - Push
```

### 5. Runtime Safety Checks

Every GitOps method re-validates path invariants before execution:

```go
func (g *gitOps) validateRuntime(ctx context.Context) error {
    // Re-check path exists
    info, err := os.Stat(g.path)
    if err != nil {
        return fmt.Errorf("runtime check failed: %w", err)
    }
    if !info.IsDir() {
        return fmt.Errorf("runtime check failed: path no longer a directory")
    }

    // Verify still a git repo with same toplevel
    toplevel, err := g.runner.Exec(ctx, g.path, "rev-parse", "--show-toplevel")
    if err != nil {
        return fmt.Errorf("runtime check failed: not a git repo: %w", err)
    }
    if filepath.Clean(strings.TrimSpace(toplevel)) != g.path {
        return fmt.Errorf("runtime check failed: toplevel changed")
    }

    return nil
}

// Called at start of every operation
func (g *gitOps) Status(ctx context.Context) (StatusResult, error) {
    if err := g.validateRuntime(ctx); err != nil {
        return StatusResult{}, err
    }
    // ... execute
}
```

### 6. Audit Logging

Every git operation emits structured logs for incident triage:

```go
type AuditEntry struct {
    Timestamp     time.Time         `json:"ts"`
    Operation     string            `json:"op"`
    RepoPath      string            `json:"repo_path"`
    Branch        string            `json:"branch,omitempty"`
    Remote        string            `json:"remote,omitempty"`
    Args          []string          `json:"args,omitempty"`
    SafetyChecks  []string          `json:"safety_checks"`
    ChecksPassed  bool              `json:"checks_passed"`
    FailureReason string            `json:"failure_reason,omitempty"`
    Duration      time.Duration     `json:"duration_ms"`
    Error         string            `json:"error,omitempty"`
}

func (g *gitOps) audit(entry AuditEntry) {
    if g.opts.AuditLogger != nil {
        g.opts.AuditLogger.Log(entry)
    }
}

// Example audit log output:
// {"ts":"2026-01-21T10:30:00Z","op":"ResetHard","repo_path":"/worktrees/feature-123",
//  "branch":"feature/foo","safety_checks":["path_valid","branch_allowed","destructive_allowed"],
//  "checks_passed":true,"duration_ms":45}
//
// {"ts":"2026-01-21T10:30:01Z","op":"Push","repo_path":"/worktrees/feature-123",
//  "branch":"main","safety_checks":["path_valid","branch_allowed"],
//  "checks_passed":false,"failure_reason":"ErrProtectedBranch: main"}
```

### 7. API Changes Summary

Updated `GitOpsOpts` with all safety options:

```go
type GitOpsOpts struct {
    // Path constraints
    WorktreeBase  string // Required path prefix (e.g., "/tmp/ralph-worktrees")
    AllowRepoRoot bool   // Allow operating on repo root (not just worktrees)

    // Branch/remote constraints
    BranchGuard *BranchGuard // Branch and remote validation rules

    // Destructive operation control
    AllowDestructive bool // Must be true for ResetHard, Clean, CheckoutFiles, force push

    // Safety level (alternative to individual flags)
    SafetyLevel SafetyLevel // Strict, Default, or Relaxed

    // Audit logging
    AuditLogger AuditLogger // Optional structured logger for all operations
}

type SafetyLevel int

const (
    SafetyStrict  SafetyLevel = iota // All checks enabled, AllowDestructive=false
    SafetyDefault                     // Path validation + runtime checks, no branch guard
    SafetyRelaxed                     // Path validation only (for tests)
)

// Convenience constructors
func NewWorktreeGitOps(path string, worktreeBase string) (GitOps, error) {
    return NewGitOps(path, GitOpsOpts{
        WorktreeBase:     worktreeBase,
        AllowRepoRoot:    false,
        AllowDestructive: true, // Worktrees are meant to be reset
        SafetyLevel:      SafetyDefault,
    })
}

func NewRepoRootGitOps(path string, guard *BranchGuard) (GitOps, error) {
    return NewGitOps(path, GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: false, // Never destructive on repo root by default
        BranchGuard:      guard,
        SafetyLevel:      SafetyStrict,
    })
}
```

### 8. Error Types

```go
var (
    // Path errors
    ErrEmptyPath          = errors.New("git: path cannot be empty")
    ErrRelativePath       = errors.New("git: path must be absolute")
    ErrNonCanonicalPath   = errors.New("git: path must be canonical")
    ErrPathNotFound       = errors.New("git: path not found")
    ErrNotDirectory       = errors.New("git: path is not a directory")
    ErrNotGitRepo         = errors.New("git: path is not a git repository")
    ErrPathMismatch       = errors.New("git: path does not match git toplevel")
    ErrRepoRootNotAllowed = errors.New("git: repo root not allowed (use AllowRepoRoot)")
    ErrOutsideWorktreeBase = errors.New("git: path outside worktree base")

    // Branch/remote errors
    ErrUnexpectedBranch   = errors.New("git: unexpected branch")
    ErrUnexpectedRemote   = errors.New("git: unexpected remote URL")
    ErrProtectedBranch    = errors.New("git: cannot write to protected branch")

    // Operation errors
    ErrDestructiveNotAllowed = errors.New("git: destructive operation not allowed")
    ErrConcurrentWrite       = errors.New("git: concurrent write operation in progress")
)
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

### Core Safety

1. **Path Safety**: Any git op with an empty, relative, or non-existent path fails before executing
2. **Worktree Isolation**: Operations bound to worktrees cannot accidentally run on repo root (unless explicitly allowed)
3. **Branch Safety**: Wrong-branch or wrong-remote attempts are blocked and logged
4. **Destructive Guard**: Destructive commands (`ResetHard`, `Clean`, `CheckoutFiles`, `Push --force`) cannot run unless explicitly enabled
5. **Concurrency Safety**: Per-repo locking prevents concurrent write operations from corrupting state

### Testability

6. **Mock Interface**: Tests use `MockGitOps` instead of stubbing raw commands
7. **Error Types**: All safety violations return typed errors for easy assertion

### Auditability

8. **Structured Logging**: All git operations emit structured audit logs with operation, path, branch, and safety check results
9. **Failure Logging**: Safety check failures include detailed reason for incident triage

### Compatibility

10. **No Regression**: All existing tests pass after migration
11. **Incremental Adoption**: GitOps can coexist with raw Runner during migration

## Required Tests

### Construction Tests

| Test Case | Expected Result |
|-----------|-----------------|
| Empty path | `ErrEmptyPath` |
| Relative path (`./foo`) | `ErrRelativePath` |
| Non-existent path | `ErrPathNotFound` |
| Path is a file, not directory | `ErrNotDirectory` |
| Path is not a git repo | `ErrNotGitRepo` |
| Path doesn't match `--show-toplevel` | `ErrPathMismatch` |
| Repo root when worktree required | `ErrRepoRootNotAllowed` |
| Path outside `WorktreeBase` | `ErrOutsideWorktreeBase` |
| Valid worktree path | Success |
| Repo root with `AllowRepoRoot=true` | Success |

### Branch Guard Tests

| Test Case | Expected Result |
|-----------|-----------------|
| HEAD not matching `ExpectedBranch` | `ErrUnexpectedBranch` |
| HEAD not in `AllowedBranchPrefixes` | `ErrUnexpectedBranch` |
| Attempt to write to `main` | `ErrProtectedBranch` |
| Attempt to write to `master` | `ErrProtectedBranch` |
| Remote URL mismatch on Push | `ErrUnexpectedRemote` |
| Valid branch in allowed prefix | Success |

### Destructive Operation Tests

| Test Case | Expected Result |
|-----------|-----------------|
| `ResetHard` without `AllowDestructive` | `ErrDestructiveNotAllowed` |
| `Clean` without `AllowDestructive` | `ErrDestructiveNotAllowed` |
| `CheckoutFiles` without `AllowDestructive` | `ErrDestructiveNotAllowed` |
| `Push --force` without `AllowDestructive` | `ErrDestructiveNotAllowed` |
| `ResetHard` with `AllowDestructive=true` | Success |

### Concurrency Tests

```go
func TestConcurrentWritesAreSerialized(t *testing.T) {
    gitOps, _ := NewGitOps(worktreePath, GitOpsOpts{AllowDestructive: true})

    var wg sync.WaitGroup
    var order []int
    var mu sync.Mutex

    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            gitOps.Commit(ctx, fmt.Sprintf("commit %d", n), CommitOpts{})
            mu.Lock()
            order = append(order, n)
            mu.Unlock()
        }(i)
    }

    wg.Wait()
    // Commits should complete without corruption
    // (actual order may vary, but no concurrent execution)
}
```

### Audit Logging Tests

```go
func TestAuditLogContainsRequiredFields(t *testing.T) {
    var logs []AuditEntry
    logger := &testLogger{entries: &logs}

    gitOps, _ := NewGitOps(path, GitOpsOpts{AuditLogger: logger})
    gitOps.Status(ctx)

    require.Len(t, logs, 1)
    entry := logs[0]
    assert.Equal(t, "Status", entry.Operation)
    assert.Equal(t, path, entry.RepoPath)
    assert.True(t, entry.ChecksPassed)
    assert.Contains(t, entry.SafetyChecks, "path_valid")
}

func TestAuditLogOnSafetyFailure(t *testing.T) {
    var logs []AuditEntry
    logger := &testLogger{entries: &logs}

    gitOps, _ := NewGitOps(path, GitOpsOpts{
        AuditLogger:  logger,
        BranchGuard:  &BranchGuard{ProtectedBranches: []string{"main"}},
    })

    // Checkout to main, then try to commit
    gitOps.CheckoutBranch(ctx, "main", false)
    err := gitOps.Commit(ctx, "bad commit", CommitOpts{})

    require.Error(t, err)
    lastLog := logs[len(logs)-1]
    assert.False(t, lastLog.ChecksPassed)
    assert.Contains(t, lastLog.FailureReason, "ErrProtectedBranch")
}
```

### Runtime Validation Tests

```go
func TestRuntimeCheckDetectsDeletedWorktree(t *testing.T) {
    gitOps, _ := NewGitOps(worktreePath, GitOpsOpts{})

    // Delete the worktree directory
    os.RemoveAll(worktreePath)

    // Next operation should fail with runtime check error
    _, err := gitOps.Status(ctx)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "runtime check failed")
}
```

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

### Resolved

1. ~~Should GitOps verify the path is a git repository at construction?~~
   **Answer**: Yes. The safety invariants require `git rev-parse --show-toplevel` verification at construction.

2. ~~Should we support both worktree and repo operations in one interface, or split them?~~
   **Answer**: Single interface with `AllowRepoRoot` option. Use `NewWorktreeGitOps()` or `NewRepoRootGitOps()` convenience constructors.

3. ~~How to handle operations that need different paths (e.g., merge in repo root vs commit in worktree)?~~
   **Answer**: Create separate `GitOps` instances for each path. Worker has `worktreeGitOps` for worktree operations and optionally `repoGitOps` for repo-level operations.

### Open

4. Should `SafetyLevel` presets be used, or require explicit configuration of each option?
5. How aggressive should runtime re-validation be? (Every call vs. periodic vs. write-ops only)
6. Should the per-repo lock be configurable (e.g., timeout, try-lock)?
7. What audit log retention/rotation policy is appropriate?

## References

- Issue discovered in: `internal/worker/review.go:cleanupWorktree()`
- Related test files: `review_test.go`, `worker_test.go`
- Current git interface: `internal/git/exec.go`
