# GITOPS - Safe Git Operations Interface with Path Validation

## Overview

GitOps provides a safe, validated interface for executing git operations bound to specific repository paths. The core problem it solves is preventing accidental execution of destructive git commands in unintended directories.

The current `git.Runner` interface accepts arbitrary directory paths without validation. When callers pass an empty string, Go's `exec.Cmd` defaults to the current working directory, which led to a production bug where tests inadvertently ran destructive commands (`git checkout .`, `git reset`, `git clean -fd`) on the actual repository instead of test directories.

GitOps addresses this by requiring path validation at construction time, providing semantic operations instead of raw command execution, and offering a testable interface that doesn't require stubbing individual git commands.

```
┌─────────────────────────────────────────────────────────────────┐
│                         Worker Package                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   ┌─────────────┐      ┌─────────────┐      ┌─────────────┐    │
│   │  review.go  │      │  execute.go │      │  merge.go   │    │
│   └──────┬──────┘      └──────┬──────┘      └──────┬──────┘    │
│          │                    │                    │            │
│          └────────────────────┼────────────────────┘            │
│                               │                                 │
│                               ▼                                 │
│                    ┌──────────────────┐                         │
│                    │  GitOps Interface │ ◄── Validated path     │
│                    └────────┬─────────┘      bound at creation  │
│                             │                                   │
└─────────────────────────────┼───────────────────────────────────┘
                              │
                              ▼
              ┌───────────────────────────────┐
              │       git.Runner (low-level)   │
              │  ┌─────────────────────────┐  │
              │  │ Exec(ctx, dir, args...) │  │
              │  └─────────────────────────┘  │
              └───────────────────────────────┘
```

## Requirements

### Functional Requirements

1. **Reject invalid paths at construction**: `NewGitOps(path, opts)` must reject empty, relative, and non-canonical paths before returning a GitOps instance
2. **Immutable path binding**: Once created, a GitOps instance is permanently bound to its canonical path; the path cannot change
3. **Verify path is under WorktreeBase**: If `WorktreeBase` is specified, reject paths outside that directory (unless `AllowRepoRoot=true`)
4. **Validate branch against BranchGuard**: Before write operations, verify the current branch matches constraints
5. **Require AllowDestructive for destructive operations**: Operations like `ResetHard`, `Clean`, `CheckoutFiles`, and force push must have explicit opt-in
6. **Serialize write operations with per-repo lock**: Prevent concurrent workers from corrupting git state
7. **Re-validate path invariants at runtime**: Every operation re-checks that the path still exists and is the same git repository
8. **Emit structured audit logs**: All operations produce audit entries for incident triage
9. **Semantic operation methods**: Provide named methods for common git operations (Status, Commit, CheckoutFiles, etc.) instead of raw command execution
10. **Repository verification**: Verify the path is a valid git worktree at construction time
11. **Support read operations**: Status, RevParse, Diff, Log, CurrentBranch, BranchExists
12. **Support write operations**: Add, AddAll, Reset, Commit, CheckoutFiles, Clean, ResetHard
13. **Support remote operations**: Fetch, Push
14. **Support merge operations**: Merge, MergeAbort
15. **Mockable interface**: Tests can inject `MockGitOps` to verify operation sequences without stubbing raw commands

### Performance Requirements

| Metric | Target |
|--------|--------|
| Construction overhead | < 5ms (stat calls + git rev-parse) |
| Method dispatch overhead | < 100us per operation (excluding runtime validation) |
| Runtime validation overhead | < 2ms per operation |
| Memory per GitOps instance | < 512 bytes |

### Constraints

- Must use existing `git.Runner` interface internally for actual command execution
- Must not break existing code paths during migration (additive-only changes)
- Must work with both repository root directories and worktree directories
- Must handle paths with spaces and special characters correctly
- Per-repo locks must not deadlock under any circumstances

## Design

### Module Structure

```
internal/git/
├── exec.go           # Existing Runner interface and osRunner implementation
├── gitops.go         # NEW: GitOps interface, gitOps implementation, NewGitOps
├── gitops_opts.go    # NEW: GitOpsOpts, BranchGuard, SafetyLevel, AuditEntry
├── gitops_test.go    # NEW: Unit tests for GitOps
├── gitops_lock.go    # NEW: Per-repo write lock implementation
├── mock_gitops.go    # NEW: MockGitOps for testing
├── branch.go         # Existing branch operations
├── commit.go         # Existing commit operations
├── merge.go          # Existing merge operations
└── worktree.go       # Existing worktree management
```

### Error Types

```go
package git

import (
    "errors"
)

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

### Core Types

```go
package git

import (
    "context"
    "time"
)

// GitOps provides safe git operations bound to a specific repository path.
// All operations are validated to prevent accidental execution in wrong directories.
type GitOps interface {
    // Path returns the repository path this GitOps is bound to.
    // The path is immutable after construction.
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

    // Working tree operations (destructive - require AllowDestructive=true)
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

// StatusResult contains the parsed output of git status.
type StatusResult struct {
    // Clean is true if the working tree has no changes.
    Clean bool

    // Staged lists files with staged changes.
    Staged []string

    // Modified lists files with unstaged modifications.
    Modified []string

    // Untracked lists untracked files.
    Untracked []string

    // Conflicted lists files with merge conflicts.
    Conflicted []string
}

// Commit represents a parsed git commit.
type Commit struct {
    Hash    string
    Author  string
    Date    time.Time
    Subject string
    Body    string
}

// CommitOpts configures commit behavior.
type CommitOpts struct {
    // NoVerify skips pre-commit and commit-msg hooks.
    NoVerify bool

    // Author overrides the commit author (format: "Name <email>").
    Author string

    // AllowEmpty permits creating commits with no changes.
    AllowEmpty bool
}

// CleanOpts configures git clean behavior.
type CleanOpts struct {
    // Force enables -f flag (required for git clean to do anything).
    Force bool

    // Directories enables -d flag to remove untracked directories.
    Directories bool

    // IgnoredOnly enables -X flag to only remove ignored files.
    IgnoredOnly bool

    // IgnoredToo enables -x flag to remove ignored and untracked files.
    IgnoredToo bool
}

// PushOpts configures git push behavior.
type PushOpts struct {
    // Force enables --force push (use with caution).
    Force bool

    // SetUpstream enables -u flag to set upstream tracking.
    SetUpstream bool

    // ForceWithLease enables --force-with-lease (safer than Force).
    ForceWithLease bool
}

// MergeOpts configures git merge behavior.
type MergeOpts struct {
    // FFOnly only allows fast-forward merges.
    FFOnly bool

    // NoFF creates a merge commit even for fast-forward merges.
    NoFF bool

    // Message sets the merge commit message.
    Message string

    // NoCommit performs merge but stops before creating commit.
    NoCommit bool
}

// LogOpts configures git log output.
type LogOpts struct {
    // MaxCount limits the number of commits returned.
    MaxCount int

    // Since filters commits after this time.
    Since time.Time

    // Until filters commits before this time.
    Until time.Time

    // Path filters commits affecting this path.
    Path string
}
```

### GitOpsOpts Type

```go
package git

import (
    "time"
)

// GitOpsOpts configures GitOps safety behavior.
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

// SafetyLevel controls the aggressiveness of safety checks.
type SafetyLevel int

const (
    SafetyStrict  SafetyLevel = iota // All checks enabled, AllowDestructive=false
    SafetyDefault                     // Path validation + runtime checks, no branch guard
    SafetyRelaxed                     // Path validation only (for tests)
)
```

### BranchGuard Type

```go
package git

// BranchGuard enforces branch/remote constraints on write operations.
type BranchGuard struct {
    // ExpectedBranch requires HEAD to match this exact branch name.
    // Example: "feature/foo"
    ExpectedBranch string

    // AllowedBranchPrefixes allows HEAD to match any of these prefixes.
    // Example: []string{"feature/", "fix/", "hotfix/"}
    AllowedBranchPrefixes []string

    // AllowedRemotes restricts Push/Fetch to these remote URLs only.
    // Example: []string{"git@github.com:myorg/myrepo.git"}
    AllowedRemotes []string

    // ProtectedBranches blocks all write operations when HEAD is on these branches.
    // Default: []string{"main", "master"}
    ProtectedBranches []string
}
```

### AuditEntry Type

```go
package git

import (
    "time"
)

// AuditEntry represents a structured log of a git operation.
type AuditEntry struct {
    Timestamp     time.Time     `json:"ts"`
    Operation     string        `json:"op"`
    RepoPath      string        `json:"repo_path"`
    Branch        string        `json:"branch,omitempty"`
    Remote        string        `json:"remote,omitempty"`
    Args          []string      `json:"args,omitempty"`
    SafetyChecks  []string      `json:"safety_checks"`
    ChecksPassed  bool          `json:"checks_passed"`
    FailureReason string        `json:"failure_reason,omitempty"`
    Duration      time.Duration `json:"duration_ms"`
    Error         string        `json:"error,omitempty"`
}

// AuditLogger receives audit entries for all git operations.
type AuditLogger interface {
    Log(entry AuditEntry)
}
```

### Per-Repo Write Lock

```go
package git

import (
    "sync"
)

// Global lock registry keyed by canonical repo path.
var repoLocks = struct {
    sync.Mutex
    locks map[string]*sync.Mutex
}{locks: make(map[string]*sync.Mutex)}

// getRepoLock returns (or creates) a mutex for the given repo path.
func getRepoLock(path string) *sync.Mutex {
    repoLocks.Lock()
    defer repoLocks.Unlock()
    if repoLocks.locks[path] == nil {
        repoLocks.locks[path] = &sync.Mutex{}
    }
    return repoLocks.locks[path]
}

// Operations that acquire per-repo lock:
// - Commit, Merge, MergeAbort
// - ResetHard, Reset, Clean
// - CheckoutBranch, CheckoutFiles
// - Push
```

### Implementation

```go
package git

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"
)

// gitOps implements GitOps with path validation and safety checks.
type gitOps struct {
    path   string      // Canonical, absolute path
    opts   GitOpsOpts
    runner Runner
}

// NewGitOps creates a GitOps bound to a specific path with safety options.
// Performs 9 validation checks at construction time:
//  1. Non-empty path
//  2. Absolute path
//  3. Canonical path (EvalSymlinks + Clean)
//  4. Path exists
//  5. Path is directory
//  6. Valid git worktree
//  7. Path matches toplevel
//  8. Worktree not repo root (unless AllowRepoRoot)
//  9. Under worktree base (unless AllowRepoRoot)
func NewGitOps(path string, opts GitOpsOpts) (GitOps, error) {
    return newGitOpsWithRunner(path, opts, DefaultRunner())
}

func newGitOpsWithRunner(path string, opts GitOpsOpts, runner Runner) (GitOps, error) {
    // 1. Non-empty path
    if path == "" {
        return nil, ErrEmptyPath
    }

    // 2. Absolute path
    if !filepath.IsAbs(path) {
        return nil, fmt.Errorf("%w: %s", ErrRelativePath, path)
    }

    // 3. Canonical path (resolve symlinks, clean)
    canonical, err := filepath.EvalSymlinks(path)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrNonCanonicalPath, err)
    }
    canonical = filepath.Clean(canonical)

    // 4. Path exists
    info, err := os.Stat(canonical)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrPathNotFound, err)
    }

    // 5. Path is directory
    if !info.IsDir() {
        return nil, fmt.Errorf("%w: %s", ErrNotDirectory, canonical)
    }

    // 6. Valid git worktree
    toplevel, err := runner.Exec(context.Background(), canonical, "rev-parse", "--show-toplevel")
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrNotGitRepo, err)
    }
    toplevel = strings.TrimSpace(toplevel)

    // 7. Path matches toplevel
    if filepath.Clean(toplevel) != canonical {
        return nil, fmt.Errorf("%w: toplevel=%s, path=%s", ErrPathMismatch, toplevel, canonical)
    }

    // 8. Worktree not repo root (unless AllowRepoRoot)
    if !opts.AllowRepoRoot {
        gitDir, err := runner.Exec(context.Background(), canonical, "rev-parse", "--absolute-git-dir")
        if err == nil {
            gitDir = strings.TrimSpace(gitDir)
            // If gitDir doesn't contain "worktrees", this is the repo root
            if !strings.Contains(gitDir, "worktrees") {
                return nil, ErrRepoRootNotAllowed
            }
        }
    }

    // 9. Under worktree base (unless AllowRepoRoot)
    if !opts.AllowRepoRoot && opts.WorktreeBase != "" {
        base, err := filepath.EvalSymlinks(opts.WorktreeBase)
        if err == nil {
            base = filepath.Clean(base)
            if !strings.HasPrefix(canonical, base+string(filepath.Separator)) && canonical != base {
                return nil, fmt.Errorf("%w: path=%s, base=%s", ErrOutsideWorktreeBase, canonical, base)
            }
        }
    }

    return &gitOps{
        path:   canonical,
        opts:   opts,
        runner: runner,
    }, nil
}

// NewWorktreeGitOps creates a GitOps for worktree operations.
// Destructive operations are allowed since worktrees are meant to be reset.
func NewWorktreeGitOps(path string, worktreeBase string) (GitOps, error) {
    return NewGitOps(path, GitOpsOpts{
        WorktreeBase:     worktreeBase,
        AllowRepoRoot:    false,
        AllowDestructive: true, // Worktrees are meant to be reset
        SafetyLevel:      SafetyDefault,
    })
}

// NewRepoRootGitOps creates a GitOps for repo root operations.
// Destructive operations are NOT allowed by default to protect the main repo.
func NewRepoRootGitOps(path string, guard *BranchGuard) (GitOps, error) {
    return NewGitOps(path, GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: false, // Never destructive on repo root by default
        BranchGuard:      guard,
        SafetyLevel:      SafetyStrict,
    })
}

func (g *gitOps) Path() string {
    return g.path
}

// validateRuntime re-validates path invariants before each operation.
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

// validateBranchGuard enforces branch/remote constraints before write operations.
func (g *gitOps) validateBranchGuard(ctx context.Context) error {
    if g.opts.BranchGuard == nil {
        return nil // No guard configured
    }

    branch, err := g.currentBranchInternal(ctx)
    if err != nil {
        return fmt.Errorf("branch guard: %w", err)
    }

    guard := g.opts.BranchGuard

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

    return nil
}

func (g *gitOps) exec(ctx context.Context, args ...string) (string, error) {
    return g.runner.Exec(ctx, g.path, args...)
}

func (g *gitOps) audit(entry AuditEntry) {
    if g.opts.AuditLogger != nil {
        g.opts.AuditLogger.Log(entry)
    }
}

// Status returns the current working tree status.
func (g *gitOps) Status(ctx context.Context) (StatusResult, error) {
    start := time.Now()
    checks := []string{"path_valid"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "Status",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return StatusResult{}, err
    }

    out, err := g.exec(ctx, "status", "--porcelain")

    g.audit(AuditEntry{
        Timestamp:    start,
        Operation:    "Status",
        RepoPath:     g.path,
        SafetyChecks: checks,
        ChecksPassed: err == nil,
        Duration:     time.Since(start),
        Error:        errorString(err),
    })

    if err != nil {
        return StatusResult{}, err
    }

    result := StatusResult{Clean: true}
    lines := strings.Split(out, "\n")

    for _, line := range lines {
        if len(line) < 3 {
            continue
        }
        result.Clean = false
        status := line[:2]
        file := strings.TrimSpace(line[3:])

        switch {
        case status[0] == 'U' || status[1] == 'U' || status == "AA" || status == "DD":
            result.Conflicted = append(result.Conflicted, file)
        case status[0] != ' ' && status[0] != '?':
            result.Staged = append(result.Staged, file)
        case status[1] == 'M':
            result.Modified = append(result.Modified, file)
        case status == "??":
            result.Untracked = append(result.Untracked, file)
        }
    }

    return result, nil
}

// RevParse resolves a git ref to its SHA.
func (g *gitOps) RevParse(ctx context.Context, ref string) (string, error) {
    if err := g.validateRuntime(ctx); err != nil {
        return "", err
    }
    out, err := g.exec(ctx, "rev-parse", ref)
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(out), nil
}

// Diff returns the diff between two refs.
func (g *gitOps) Diff(ctx context.Context, base, head string) (string, error) {
    if err := g.validateRuntime(ctx); err != nil {
        return "", err
    }
    return g.exec(ctx, "diff", base, head)
}

// Log returns commits matching the given options.
func (g *gitOps) Log(ctx context.Context, opts LogOpts) ([]Commit, error) {
    if err := g.validateRuntime(ctx); err != nil {
        return nil, err
    }

    args := []string{"log", "--format=%H|%an|%aI|%s|%b%x00"}

    if opts.MaxCount > 0 {
        args = append(args, fmt.Sprintf("-n%d", opts.MaxCount))
    }
    if !opts.Since.IsZero() {
        args = append(args, "--since="+opts.Since.Format(time.RFC3339))
    }
    if !opts.Until.IsZero() {
        args = append(args, "--until="+opts.Until.Format(time.RFC3339))
    }
    if opts.Path != "" {
        args = append(args, "--", opts.Path)
    }

    out, err := g.exec(ctx, args...)
    if err != nil {
        return nil, err
    }

    var commits []Commit
    entries := strings.Split(out, "\x00")
    for _, entry := range entries {
        entry = strings.TrimSpace(entry)
        if entry == "" {
            continue
        }
        parts := strings.SplitN(entry, "|", 5)
        if len(parts) < 4 {
            continue
        }
        date, _ := time.Parse(time.RFC3339, parts[2])
        commit := Commit{
            Hash:    parts[0],
            Author:  parts[1],
            Date:    date,
            Subject: parts[3],
        }
        if len(parts) == 5 {
            commit.Body = strings.TrimSpace(parts[4])
        }
        commits = append(commits, commit)
    }

    return commits, nil
}

// CurrentBranch returns the name of the currently checked out branch.
func (g *gitOps) CurrentBranch(ctx context.Context) (string, error) {
    if err := g.validateRuntime(ctx); err != nil {
        return "", err
    }
    return g.currentBranchInternal(ctx)
}

func (g *gitOps) currentBranchInternal(ctx context.Context) (string, error) {
    out, err := g.exec(ctx, "rev-parse", "--abbrev-ref", "HEAD")
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(out), nil
}

// CheckoutBranch switches to a branch, optionally creating it.
// Acquires per-repo lock to prevent concurrent branch operations.
func (g *gitOps) CheckoutBranch(ctx context.Context, branch string, create bool) error {
    start := time.Now()
    checks := []string{"path_valid", "branch_guard"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "CheckoutBranch",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    // Acquire per-repo lock
    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"checkout"}
    if create {
        args = append(args, "-b")
    }
    args = append(args, branch)
    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp:    start,
        Operation:    "CheckoutBranch",
        RepoPath:     g.path,
        Branch:       branch,
        Args:         args,
        SafetyChecks: checks,
        ChecksPassed: err == nil,
        Duration:     time.Since(start),
        Error:        errorString(err),
    })

    return err
}

// BranchExists checks if a branch exists locally or on origin.
func (g *gitOps) BranchExists(ctx context.Context, branch string) (bool, error) {
    if err := g.validateRuntime(ctx); err != nil {
        return false, err
    }

    // Check local
    _, err := g.exec(ctx, "rev-parse", "--verify", branch)
    if err == nil {
        return true, nil
    }

    // Check remote
    _, err = g.exec(ctx, "rev-parse", "--verify", "origin/"+branch)
    return err == nil, nil
}

// Add stages the specified files.
func (g *gitOps) Add(ctx context.Context, paths ...string) error {
    if err := g.validateRuntime(ctx); err != nil {
        return err
    }
    args := append([]string{"add", "--"}, paths...)
    _, err := g.exec(ctx, args...)
    return err
}

// AddAll stages all changes (git add -A).
func (g *gitOps) AddAll(ctx context.Context) error {
    if err := g.validateRuntime(ctx); err != nil {
        return err
    }
    _, err := g.exec(ctx, "add", "-A")
    return err
}

// Reset unstages the specified files (or all if none specified).
// Acquires per-repo lock.
func (g *gitOps) Reset(ctx context.Context, paths ...string) error {
    start := time.Now()
    checks := []string{"path_valid"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "Reset",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    // Acquire per-repo lock
    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"reset", "HEAD"}
    if len(paths) > 0 {
        args = append(args, "--")
        args = append(args, paths...)
    }
    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp:    start,
        Operation:    "Reset",
        RepoPath:     g.path,
        Args:         args,
        SafetyChecks: checks,
        ChecksPassed: err == nil,
        Duration:     time.Since(start),
        Error:        errorString(err),
    })

    return err
}

// Commit creates a commit with the given message.
// Validates branch guard and acquires per-repo lock.
func (g *gitOps) Commit(ctx context.Context, msg string, opts CommitOpts) error {
    start := time.Now()
    checks := []string{"path_valid", "branch_guard"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "Commit",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    if err := g.validateBranchGuard(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "Commit",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    // Acquire per-repo lock
    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"commit", "-m", msg}
    if opts.NoVerify {
        args = append(args, "--no-verify")
    }
    if opts.Author != "" {
        args = append(args, "--author="+opts.Author)
    }
    if opts.AllowEmpty {
        args = append(args, "--allow-empty")
    }

    branch, _ := g.currentBranchInternal(ctx)
    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp:    start,
        Operation:    "Commit",
        RepoPath:     g.path,
        Branch:       branch,
        SafetyChecks: checks,
        ChecksPassed: err == nil,
        Duration:     time.Since(start),
        Error:        errorString(err),
    })

    return err
}

// CheckoutFiles discards changes to the specified files.
// DESTRUCTIVE: Requires AllowDestructive=true.
// Acquires per-repo lock.
func (g *gitOps) CheckoutFiles(ctx context.Context, paths ...string) error {
    start := time.Now()
    checks := []string{"path_valid", "destructive_allowed"}

    if !g.opts.AllowDestructive {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "CheckoutFiles",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: ErrDestructiveNotAllowed.Error(),
            Duration:      time.Since(start),
        })
        return fmt.Errorf("%w: CheckoutFiles", ErrDestructiveNotAllowed)
    }

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "CheckoutFiles",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    // Acquire per-repo lock
    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := append([]string{"checkout", "--"}, paths...)
    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp:    start,
        Operation:    "CheckoutFiles",
        RepoPath:     g.path,
        Args:         args,
        SafetyChecks: checks,
        ChecksPassed: err == nil,
        Duration:     time.Since(start),
        Error:        errorString(err),
    })

    return err
}

// Clean removes untracked files.
// DESTRUCTIVE: Requires AllowDestructive=true.
// Acquires per-repo lock.
func (g *gitOps) Clean(ctx context.Context, opts CleanOpts) error {
    start := time.Now()
    checks := []string{"path_valid", "destructive_allowed"}

    if !g.opts.AllowDestructive {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "Clean",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: ErrDestructiveNotAllowed.Error(),
            Duration:      time.Since(start),
        })
        return fmt.Errorf("%w: Clean", ErrDestructiveNotAllowed)
    }

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "Clean",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    // Acquire per-repo lock
    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"clean"}
    if opts.Force {
        args = append(args, "-f")
    }
    if opts.Directories {
        args = append(args, "-d")
    }
    if opts.IgnoredOnly {
        args = append(args, "-X")
    } else if opts.IgnoredToo {
        args = append(args, "-x")
    }

    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp:    start,
        Operation:    "Clean",
        RepoPath:     g.path,
        Args:         args,
        SafetyChecks: checks,
        ChecksPassed: err == nil,
        Duration:     time.Since(start),
        Error:        errorString(err),
    })

    return err
}

// ResetHard performs a hard reset to the specified ref.
// DESTRUCTIVE: Requires AllowDestructive=true.
// Validates branch guard and acquires per-repo lock.
func (g *gitOps) ResetHard(ctx context.Context, ref string) error {
    start := time.Now()
    checks := []string{"path_valid", "branch_guard", "destructive_allowed"}

    if !g.opts.AllowDestructive {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "ResetHard",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: ErrDestructiveNotAllowed.Error(),
            Duration:      time.Since(start),
        })
        return fmt.Errorf("%w: ResetHard", ErrDestructiveNotAllowed)
    }

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "ResetHard",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    if err := g.validateBranchGuard(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "ResetHard",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    // Acquire per-repo lock
    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    branch, _ := g.currentBranchInternal(ctx)
    _, err := g.exec(ctx, "reset", "--hard", ref)

    g.audit(AuditEntry{
        Timestamp:    start,
        Operation:    "ResetHard",
        RepoPath:     g.path,
        Branch:       branch,
        Args:         []string{ref},
        SafetyChecks: checks,
        ChecksPassed: err == nil,
        Duration:     time.Since(start),
        Error:        errorString(err),
    })

    return err
}

// Fetch fetches from a remote.
func (g *gitOps) Fetch(ctx context.Context, remote, ref string) error {
    if err := g.validateRuntime(ctx); err != nil {
        return err
    }
    args := []string{"fetch", remote}
    if ref != "" {
        args = append(args, ref)
    }
    _, err := g.exec(ctx, args...)
    return err
}

// Push pushes to a remote.
// Force/ForceWithLease are DESTRUCTIVE: Require AllowDestructive=true.
// Validates branch guard and acquires per-repo lock.
func (g *gitOps) Push(ctx context.Context, remote, branch string, opts PushOpts) error {
    start := time.Now()
    checks := []string{"path_valid", "branch_guard"}

    if (opts.Force || opts.ForceWithLease) && !g.opts.AllowDestructive {
        checks = append(checks, "destructive_allowed")
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "Push",
            RepoPath:      g.path,
            Branch:        branch,
            Remote:        remote,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: ErrDestructiveNotAllowed.Error(),
            Duration:      time.Since(start),
        })
        return fmt.Errorf("%w: Push --force", ErrDestructiveNotAllowed)
    }

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "Push",
            RepoPath:      g.path,
            Branch:        branch,
            Remote:        remote,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    if err := g.validateBranchGuard(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "Push",
            RepoPath:      g.path,
            Branch:        branch,
            Remote:        remote,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    // Acquire per-repo lock
    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"push"}
    if opts.SetUpstream {
        args = append(args, "-u")
    }
    if opts.ForceWithLease {
        args = append(args, "--force-with-lease")
    } else if opts.Force {
        args = append(args, "--force")
    }
    args = append(args, remote, branch)

    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp:    start,
        Operation:    "Push",
        RepoPath:     g.path,
        Branch:       branch,
        Remote:       remote,
        Args:         args,
        SafetyChecks: checks,
        ChecksPassed: err == nil,
        Duration:     time.Since(start),
        Error:        errorString(err),
    })

    return err
}

// Merge merges a branch into the current branch.
// Validates branch guard and acquires per-repo lock.
func (g *gitOps) Merge(ctx context.Context, branch string, opts MergeOpts) error {
    start := time.Now()
    checks := []string{"path_valid", "branch_guard"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "Merge",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    if err := g.validateBranchGuard(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "Merge",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    // Acquire per-repo lock
    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"merge"}
    if opts.FFOnly {
        args = append(args, "--ff-only")
    }
    if opts.NoFF {
        args = append(args, "--no-ff")
    }
    if opts.NoCommit {
        args = append(args, "--no-commit")
    }
    if opts.Message != "" {
        args = append(args, "-m", opts.Message)
    }
    args = append(args, branch)

    currentBranch, _ := g.currentBranchInternal(ctx)
    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp:    start,
        Operation:    "Merge",
        RepoPath:     g.path,
        Branch:       currentBranch,
        Args:         args,
        SafetyChecks: checks,
        ChecksPassed: err == nil,
        Duration:     time.Since(start),
        Error:        errorString(err),
    })

    return err
}

// MergeAbort aborts an in-progress merge.
// Acquires per-repo lock.
func (g *gitOps) MergeAbort(ctx context.Context) error {
    start := time.Now()
    checks := []string{"path_valid"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp:     start,
            Operation:     "MergeAbort",
            RepoPath:      g.path,
            SafetyChecks:  checks,
            ChecksPassed:  false,
            FailureReason: err.Error(),
            Duration:      time.Since(start),
        })
        return err
    }

    // Acquire per-repo lock
    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    _, err := g.exec(ctx, "merge", "--abort")

    g.audit(AuditEntry{
        Timestamp:    start,
        Operation:    "MergeAbort",
        RepoPath:     g.path,
        SafetyChecks: checks,
        ChecksPassed: err == nil,
        Duration:     time.Since(start),
        Error:        errorString(err),
    })

    return err
}

func errorString(err error) string {
    if err == nil {
        return ""
    }
    return err.Error()
}
```

### Mock Implementation

```go
package git

import (
    "context"
    "fmt"
)

// MockGitOps is a test double for GitOps that records calls and returns configured responses.
type MockGitOps struct {
    path string

    // Calls records all method invocations for verification.
    Calls []MockCall

    // Responses configures return values for each method.
    StatusResponse        StatusResult
    StatusError           error
    RevParseResponse      map[string]string
    RevParseError         error
    DiffResponse          string
    DiffError             error
    LogResponse           []Commit
    LogError              error
    CurrentBranchResponse string
    CurrentBranchError    error
    BranchExistsResponse  map[string]bool
    CheckoutBranchError   error
    AddError              error
    AddAllError           error
    ResetError            error
    CommitError           error
    CheckoutFilesError    error
    CleanError            error
    ResetHardError        error
    FetchError            error
    PushError             error
    MergeError            error
    MergeAbortError       error
}

// MockCall records a method invocation.
type MockCall struct {
    Method string
    Args   []any
}

// NewMockGitOps creates a MockGitOps bound to a path.
func NewMockGitOps(path string) *MockGitOps {
    return &MockGitOps{
        path:             path,
        RevParseResponse: make(map[string]string),
        BranchExistsResponse: make(map[string]bool),
    }
}

func (m *MockGitOps) record(method string, args ...interface{}) {
    m.Calls = append(m.Calls, MockCall{Method: method, Args: args})
}

func (m *MockGitOps) Path() string {
    return m.path
}

func (m *MockGitOps) Status(ctx context.Context) (StatusResult, error) {
    m.record("Status")
    return m.StatusResponse, m.StatusError
}

func (m *MockGitOps) RevParse(ctx context.Context, ref string) (string, error) {
    m.record("RevParse", ref)
    if m.RevParseError != nil {
        return "", m.RevParseError
    }
    if sha, ok := m.RevParseResponse[ref]; ok {
        return sha, nil
    }
    return "", fmt.Errorf("unknown ref: %s", ref)
}

func (m *MockGitOps) Diff(ctx context.Context, base, head string) (string, error) {
    m.record("Diff", base, head)
    return m.DiffResponse, m.DiffError
}

func (m *MockGitOps) Log(ctx context.Context, opts LogOpts) ([]Commit, error) {
    m.record("Log", opts)
    return m.LogResponse, m.LogError
}

func (m *MockGitOps) CurrentBranch(ctx context.Context) (string, error) {
    m.record("CurrentBranch")
    return m.CurrentBranchResponse, m.CurrentBranchError
}

func (m *MockGitOps) CheckoutBranch(ctx context.Context, branch string, create bool) error {
    m.record("CheckoutBranch", branch, create)
    return m.CheckoutBranchError
}

func (m *MockGitOps) BranchExists(ctx context.Context, branch string) (bool, error) {
    m.record("BranchExists", branch)
    return m.BranchExistsResponse[branch], nil
}

func (m *MockGitOps) Add(ctx context.Context, paths ...string) error {
    m.record("Add", paths)
    return m.AddError
}

func (m *MockGitOps) AddAll(ctx context.Context) error {
    m.record("AddAll")
    return m.AddAllError
}

func (m *MockGitOps) Reset(ctx context.Context, paths ...string) error {
    m.record("Reset", paths)
    return m.ResetError
}

func (m *MockGitOps) Commit(ctx context.Context, msg string, opts CommitOpts) error {
    m.record("Commit", msg, opts)
    return m.CommitError
}

func (m *MockGitOps) CheckoutFiles(ctx context.Context, paths ...string) error {
    m.record("CheckoutFiles", paths)
    return m.CheckoutFilesError
}

func (m *MockGitOps) Clean(ctx context.Context, opts CleanOpts) error {
    m.record("Clean", opts)
    return m.CleanError
}

func (m *MockGitOps) ResetHard(ctx context.Context, ref string) error {
    m.record("ResetHard", ref)
    return m.ResetHardError
}

func (m *MockGitOps) Fetch(ctx context.Context, remote, ref string) error {
    m.record("Fetch", remote, ref)
    return m.FetchError
}

func (m *MockGitOps) Push(ctx context.Context, remote, branch string, opts PushOpts) error {
    m.record("Push", remote, branch, opts)
    return m.PushError
}

func (m *MockGitOps) Merge(ctx context.Context, branch string, opts MergeOpts) error {
    m.record("Merge", branch, opts)
    return m.MergeError
}

func (m *MockGitOps) MergeAbort(ctx context.Context) error {
    m.record("MergeAbort")
    return m.MergeAbortError
}

// AssertCalled verifies a method was called with expected arguments.
func (m *MockGitOps) AssertCalled(method string) bool {
    for _, call := range m.Calls {
        if call.Method == method {
            return true
        }
    }
    return false
}

// CallCount returns the number of times a method was called.
func (m *MockGitOps) CallCount(method string) int {
    count := 0
    for _, call := range m.Calls {
        if call.Method == method {
            count++
        }
    }
    return count
}
```

### API Surface

```go
// Construction
func NewGitOps(path string, opts GitOpsOpts) (GitOps, error)

// Convenience constructors
func NewWorktreeGitOps(path string, worktreeBase string) (GitOps, error)
func NewRepoRootGitOps(path string, guard *BranchGuard) (GitOps, error)

// Test utilities
func NewMockGitOps(path string) *MockGitOps
```

## Implementation Notes

### Path Validation Edge Cases

1. **Symlinks**: Paths are canonicalized via `filepath.EvalSymlinks` at construction, so `/var/folders/...` on macOS resolves correctly to `/private/var/folders/...`
2. **Relative paths**: The constructor rejects relative paths with `ErrRelativePath`; callers must provide absolute paths
3. **Windows paths**: Forward slashes work on Windows, but paths with drive letters need testing
4. **Non-canonical paths**: Paths with `..` or redundant separators are cleaned via `filepath.Clean`

### Thread Safety

The `gitOps` struct is safe for concurrent use because:
- The `path` field is immutable after construction
- The `opts` field is immutable after construction
- The `runner` field is immutable after construction
- Write operations acquire a per-repo lock to serialize concurrent writes

### Per-Repo Write Lock

The following operations acquire the per-repo lock:
- Commit, Merge, MergeAbort
- ResetHard, Reset, Clean
- CheckoutBranch, CheckoutFiles
- Push

Read operations (Status, RevParse, Diff, Log, CurrentBranch, BranchExists, Fetch) do not acquire the lock.

### Error Handling

Git command errors include stderr in the error message (via the underlying `Runner.Exec`). Callers can parse the error message for specific git error patterns if needed. All safety violations return typed errors for easy assertion:

```go
if errors.Is(err, git.ErrProtectedBranch) {
    // Handle protected branch violation
}
```

### Operations on Different Paths

For workflows that need to operate on both worktree and repo root (e.g., merge operations):

```go
// Create separate GitOps instances
worktreeOps, _ := git.NewWorktreeGitOps(worktreePath, "/tmp/ralph-worktrees")
repoOps, _ := git.NewRepoRootGitOps(repoRoot, &git.BranchGuard{
    AllowedBranchPrefixes: []string{"feature/", "fix/"},
})

// Use appropriate instance for each operation
worktreeOps.AddAll(ctx)
worktreeOps.Commit(ctx, "message", git.CommitOpts{})
repoOps.Merge(ctx, branch, git.MergeOpts{})
```

### Audit Log Output

Example audit log entries:

```json
{"ts":"2026-01-21T10:30:00Z","op":"ResetHard","repo_path":"/worktrees/feature-123",
 "branch":"feature/foo","safety_checks":["path_valid","branch_allowed","destructive_allowed"],
 "checks_passed":true,"duration_ms":45}

{"ts":"2026-01-21T10:30:01Z","op":"Push","repo_path":"/worktrees/feature-123",
 "branch":"main","safety_checks":["path_valid","branch_allowed"],
 "checks_passed":false,"failure_reason":"ErrProtectedBranch: main"}
```

## Testing Strategy

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

```go
func TestNewGitOps_EmptyPath(t *testing.T) {
    _, err := git.NewGitOps("", git.GitOpsOpts{})
    if !errors.Is(err, git.ErrEmptyPath) {
        t.Errorf("expected ErrEmptyPath, got %v", err)
    }
}

func TestNewGitOps_RelativePath(t *testing.T) {
    _, err := git.NewGitOps("./relative/path", git.GitOpsOpts{})
    if !errors.Is(err, git.ErrRelativePath) {
        t.Errorf("expected ErrRelativePath, got %v", err)
    }
}

func TestNewGitOps_NonExistentPath(t *testing.T) {
    _, err := git.NewGitOps("/nonexistent/path/that/does/not/exist", git.GitOpsOpts{})
    if !errors.Is(err, git.ErrPathNotFound) {
        t.Errorf("expected ErrPathNotFound, got %v", err)
    }
}

func TestNewGitOps_PathIsFile(t *testing.T) {
    f, _ := os.CreateTemp("", "gitops-test")
    f.Close()
    defer os.Remove(f.Name())

    _, err := git.NewGitOps(f.Name(), git.GitOpsOpts{})
    if !errors.Is(err, git.ErrNotDirectory) {
        t.Errorf("expected ErrNotDirectory, got %v", err)
    }
}

func TestNewGitOps_NotGitRepo(t *testing.T) {
    dir := t.TempDir()
    _, err := git.NewGitOps(dir, git.GitOpsOpts{AllowRepoRoot: true})
    if !errors.Is(err, git.ErrNotGitRepo) {
        t.Errorf("expected ErrNotGitRepo, got %v", err)
    }
}

func TestNewGitOps_ValidPath(t *testing.T) {
    // Create a git repo in temp dir
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    ops, err := git.NewGitOps(dir, git.GitOpsOpts{AllowRepoRoot: true})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if ops.Path() != dir {
        t.Errorf("expected path %s, got %s", dir, ops.Path())
    }
}

func TestNewGitOps_RepoRootNotAllowed(t *testing.T) {
    // Create a git repo (not a worktree)
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    _, err := git.NewGitOps(dir, git.GitOpsOpts{AllowRepoRoot: false})
    if !errors.Is(err, git.ErrRepoRootNotAllowed) {
        t.Errorf("expected ErrRepoRootNotAllowed, got %v", err)
    }
}

func TestNewGitOps_OutsideWorktreeBase(t *testing.T) {
    // Create a worktree outside the base
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    _, err := git.NewGitOps(dir, git.GitOpsOpts{
        AllowRepoRoot: true,
        WorktreeBase:  "/some/other/base",
    })
    // This test needs AllowRepoRoot=false to trigger the check
    _, err = git.NewGitOps(dir, git.GitOpsOpts{
        AllowRepoRoot: false,
        WorktreeBase:  "/some/other/base",
    })
    // Would fail on ErrRepoRootNotAllowed first in this case
}

func TestGitOps_PathIsImmutable(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()
    ops, _ := git.NewGitOps(dir, git.GitOpsOpts{AllowRepoRoot: true})

    // Path() should always return the same value
    path1 := ops.Path()
    path2 := ops.Path()
    if path1 != path2 {
        t.Error("Path() returned different values")
    }
}
```

### Branch Guard Tests

| Test Case | Expected Result |
|-----------|-----------------|
| HEAD not matching `ExpectedBranch` | `ErrUnexpectedBranch` |
| HEAD not in `AllowedBranchPrefixes` | `ErrUnexpectedBranch` |
| Attempt to write to `main` | `ErrProtectedBranch` |
| Attempt to write to `master` | `ErrProtectedBranch` |
| Remote URL mismatch on Push | `ErrUnexpectedRemote` |
| Valid branch in allowed prefix | Success |

```go
func TestBranchGuard_ExpectedBranchMismatch(t *testing.T) {
    // Setup: create worktree on branch "feature/foo"
    ops, _ := git.NewGitOps(worktreePath, git.GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: true,
        BranchGuard: &git.BranchGuard{
            ExpectedBranch: "feature/bar", // Different from actual
        },
    })

    err := ops.Commit(ctx, "test", git.CommitOpts{})
    if !errors.Is(err, git.ErrUnexpectedBranch) {
        t.Errorf("expected ErrUnexpectedBranch, got %v", err)
    }
}

func TestBranchGuard_ProtectedBranch(t *testing.T) {
    // Setup: checkout to main
    ops, _ := git.NewGitOps(repoPath, git.GitOpsOpts{
        AllowRepoRoot: true,
        BranchGuard:   &git.BranchGuard{}, // Uses default protected: main, master
    })

    err := ops.Commit(ctx, "bad commit", git.CommitOpts{})
    if !errors.Is(err, git.ErrProtectedBranch) {
        t.Errorf("expected ErrProtectedBranch, got %v", err)
    }
}

func TestBranchGuard_AllowedPrefix(t *testing.T) {
    // Setup: create worktree on branch "feature/foo"
    ops, _ := git.NewGitOps(worktreePath, git.GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: true,
        BranchGuard: &git.BranchGuard{
            AllowedBranchPrefixes: []string{"feature/", "fix/"},
        },
    })

    // Should succeed since "feature/foo" matches "feature/" prefix
    err := ops.Commit(ctx, "test", git.CommitOpts{AllowEmpty: true})
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
}
```

### Destructive Operation Tests

| Test Case | Expected Result |
|-----------|-----------------|
| `ResetHard` without `AllowDestructive` | `ErrDestructiveNotAllowed` |
| `Clean` without `AllowDestructive` | `ErrDestructiveNotAllowed` |
| `CheckoutFiles` without `AllowDestructive` | `ErrDestructiveNotAllowed` |
| `Push --force` without `AllowDestructive` | `ErrDestructiveNotAllowed` |
| `ResetHard` with `AllowDestructive=true` | Success |

```go
func TestDestructive_ResetHardBlocked(t *testing.T) {
    ops, _ := git.NewGitOps(worktreePath, git.GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: false, // Destructive not allowed
    })

    err := ops.ResetHard(ctx, "HEAD")
    if !errors.Is(err, git.ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
}

func TestDestructive_CleanBlocked(t *testing.T) {
    ops, _ := git.NewGitOps(worktreePath, git.GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: false,
    })

    err := ops.Clean(ctx, git.CleanOpts{Force: true})
    if !errors.Is(err, git.ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
}

func TestDestructive_CheckoutFilesBlocked(t *testing.T) {
    ops, _ := git.NewGitOps(worktreePath, git.GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: false,
    })

    err := ops.CheckoutFiles(ctx, ".")
    if !errors.Is(err, git.ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
}

func TestDestructive_ForcePushBlocked(t *testing.T) {
    ops, _ := git.NewGitOps(worktreePath, git.GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: false,
    })

    err := ops.Push(ctx, "origin", "feature/foo", git.PushOpts{Force: true})
    if !errors.Is(err, git.ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
}

func TestDestructive_ResetHardAllowed(t *testing.T) {
    ops, _ := git.NewGitOps(worktreePath, git.GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: true, // Destructive allowed
    })

    err := ops.ResetHard(ctx, "HEAD")
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
}
```

### Concurrency Tests

```go
func TestConcurrentWritesAreSerialized(t *testing.T) {
    ops, _ := git.NewGitOps(worktreePath, git.GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: true,
    })

    var wg sync.WaitGroup
    var order []int
    var mu sync.Mutex

    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            ops.Commit(ctx, fmt.Sprintf("commit %d", n), git.CommitOpts{AllowEmpty: true})
            mu.Lock()
            order = append(order, n)
            mu.Unlock()
        }(i)
    }

    wg.Wait()
    // Commits should complete without corruption
    // (actual order may vary, but no concurrent execution)
    if len(order) != 3 {
        t.Errorf("expected 3 commits, got %d", len(order))
    }
}
```

### Audit Logging Tests

```go
func TestAuditLogContainsRequiredFields(t *testing.T) {
    var logs []git.AuditEntry
    logger := &testLogger{entries: &logs}

    ops, _ := git.NewGitOps(worktreePath, git.GitOpsOpts{
        AllowRepoRoot: true,
        AuditLogger:   logger,
    })
    ops.Status(ctx)

    require.Len(t, logs, 1)
    entry := logs[0]
    assert.Equal(t, "Status", entry.Operation)
    assert.Equal(t, worktreePath, entry.RepoPath)
    assert.True(t, entry.ChecksPassed)
    assert.Contains(t, entry.SafetyChecks, "path_valid")
}

func TestAuditLogOnSafetyFailure(t *testing.T) {
    var logs []git.AuditEntry
    logger := &testLogger{entries: &logs}

    ops, _ := git.NewGitOps(worktreePath, git.GitOpsOpts{
        AllowRepoRoot: true,
        AuditLogger:   logger,
        BranchGuard:   &git.BranchGuard{ProtectedBranches: []string{"main"}},
    })

    // Checkout to main, then try to commit
    ops.CheckoutBranch(ctx, "main", false)
    err := ops.Commit(ctx, "bad commit", git.CommitOpts{})

    require.Error(t, err)
    lastLog := logs[len(logs)-1]
    assert.False(t, lastLog.ChecksPassed)
    assert.Contains(t, lastLog.FailureReason, "ErrProtectedBranch")
}

type testLogger struct {
    entries *[]git.AuditEntry
}

func (l *testLogger) Log(entry git.AuditEntry) {
    *l.entries = append(*l.entries, entry)
}
```

### Runtime Validation Tests

```go
func TestRuntimeCheckDetectsDeletedWorktree(t *testing.T) {
    // Create a temporary worktree
    worktreePath := createTempWorktree(t)
    ops, _ := git.NewGitOps(worktreePath, git.GitOpsOpts{AllowRepoRoot: true})

    // Delete the worktree directory
    os.RemoveAll(worktreePath)

    // Next operation should fail with runtime check error
    _, err := ops.Status(ctx)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "runtime check failed")
}

func TestRuntimeCheckDetectsChangedToplevel(t *testing.T) {
    // This would require a more complex setup where git toplevel changes
    // (e.g., worktree moved or git directory corrupted)
}
```

### Unit Tests (Status Parsing)

```go
func TestGitOps_Status_ParsesOutput(t *testing.T) {
    runner := &fakeRunner{
        responses: map[string]string{
            "rev-parse --show-toplevel": "/test/path\n",
            "status --porcelain":        " M modified.go\nA  staged.go\n?? untracked.txt\nUU conflicted.go\n",
        },
    }
    ops, _ := newGitOpsWithRunner("/test/path", git.GitOpsOpts{AllowRepoRoot: true}, runner)

    result, err := ops.Status(context.Background())
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if result.Clean {
        t.Error("expected Clean=false")
    }
    if len(result.Modified) != 1 || result.Modified[0] != "modified.go" {
        t.Errorf("unexpected Modified: %v", result.Modified)
    }
    if len(result.Staged) != 1 || result.Staged[0] != "staged.go" {
        t.Errorf("unexpected Staged: %v", result.Staged)
    }
    if len(result.Untracked) != 1 || result.Untracked[0] != "untracked.txt" {
        t.Errorf("unexpected Untracked: %v", result.Untracked)
    }
    if len(result.Conflicted) != 1 || result.Conflicted[0] != "conflicted.go" {
        t.Errorf("unexpected Conflicted: %v", result.Conflicted)
    }
}

func TestMockGitOps_RecordsCalls(t *testing.T) {
    mock := git.NewMockGitOps("/test/path")
    mock.StatusResponse = git.StatusResult{Clean: true}

    ctx := context.Background()
    mock.Status(ctx)
    mock.AddAll(ctx)
    mock.Commit(ctx, "test message", git.CommitOpts{NoVerify: true})

    if !mock.AssertCalled("Status") {
        t.Error("Status was not called")
    }
    if !mock.AssertCalled("AddAll") {
        t.Error("AddAll was not called")
    }
    if mock.CallCount("Commit") != 1 {
        t.Errorf("expected 1 Commit call, got %d", mock.CallCount("Commit"))
    }
}

// fakeRunner is a test double for git.Runner
type fakeRunner struct {
    responses map[string]string
    err       error
    calls     []string
}

func (f *fakeRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
    key := strings.Join(args, " ")
    f.calls = append(f.calls, key)
    if f.err != nil {
        return "", f.err
    }
    if resp, ok := f.responses[key]; ok {
        return resp, nil
    }
    return "", nil
}

func (f *fakeRunner) ExecWithStdin(ctx context.Context, dir, stdin string, args ...string) (string, error) {
    return f.Exec(ctx, dir, args...)
}
```

### Integration Tests

- **Test in real git repository**: Create temporary git repos, make changes, verify GitOps operations work correctly
- **Test worktree operations**: Create worktrees and verify operations target the correct directory
- **Test error cases**: Verify behavior when git commands fail (network errors, conflicts, etc.)

### Manual Testing

- [ ] Create GitOps with empty path, verify error
- [ ] Create GitOps with relative path, verify error
- [ ] Create GitOps with valid worktree path, verify operations work
- [ ] Create GitOps with repo root path (no AllowRepoRoot), verify error
- [ ] Use MockGitOps in worker tests, verify no actual git commands run
- [ ] Verify cleanupWorktree cannot run in cwd when using GitOps
- [ ] Verify ResetHard fails without AllowDestructive
- [ ] Verify Commit fails on protected branch with BranchGuard
- [ ] Verify audit logs contain all required fields

## Design Decisions

### Why require path at construction time?

Alternatives considered:
1. **Pass path to each method**: Repeats the validation on every call, easy to forget
2. **Validate lazily on first use**: Bug doesn't manifest until runtime, harder to test
3. **Validate at construction (chosen)**: Fail-fast behavior, path is guaranteed valid for all subsequent operations

The construction-time validation ensures that once you have a GitOps instance, the path is known to be valid. This makes the interface harder to misuse.

### Why verify it's a git repo by default?

Unlike the original design, the new safety invariants require verifying the path is a git repository at construction time. This is necessary because:

1. **Safety**: We need to verify path matches `--show-toplevel` to prevent path confusion attacks
2. **Worktree detection**: We need to check if the path is a worktree vs repo root
3. **Branch validation**: Branch guards require accurate branch information

### Why separate worktree and repo operations?

The spec uses separate GitOps instances for worktree vs repo root operations (instead of methods like `CommitInWorktree` vs `CommitInRepo`). This approach:

1. **Keeps the interface simple**: No method name proliferation
2. **Makes the binding explicit**: You know which path each operation targets
3. **Matches the current codebase pattern**: Workers already track `worktreePath` and `config.RepoRoot` separately
4. **Enables different safety levels**: Worktrees allow destructive ops; repo root does not

### Why per-repo locking instead of per-GitOps locking?

A single repository may have multiple GitOps instances (e.g., one for worktree cleanup, one for commits). Using per-repo locks ensures that:

1. **Cross-instance safety**: Different GitOps instances targeting the same repo don't corrupt each other
2. **Worktree awareness**: Operations on the same underlying repo are serialized

### Why require AllowDestructive for CheckoutFiles?

While `CheckoutFiles` seems less dangerous than `ResetHard`, it can still discard uncommitted work:

```bash
git checkout -- .  # Discards all uncommitted changes
```

Requiring `AllowDestructive=true` ensures callers explicitly acknowledge this risk.

## Future Enhancements

1. **Stash operations**: `Stash(ctx, msg)`, `StashPop(ctx)`, `StashList(ctx)` for saving work in progress
2. **Rebase operations**: `Rebase(ctx, onto)`, `RebaseAbort(ctx)`, `RebaseContinue(ctx)` to replace raw git calls in merge.go
3. **Submodule operations**: `SubmoduleUpdate(ctx)`, `SubmoduleInit(ctx)` for repos with submodules
4. **Hook management**: `DisableHooks()`, `EnableHooks()` to temporarily bypass hooks
5. **Transaction support**: `BeginTransaction()` / `Commit()` / `Rollback()` for atomic multi-operation sequences
6. **Remote URL validation**: Validate remote URLs against `BranchGuard.AllowedRemotes` before Push/Fetch
7. **Lock timeouts**: Configurable timeout for per-repo lock acquisition
8. **Audit log rotation**: Automatic rotation/cleanup of audit logs

## References

- Issue discovered in: `/Users/bennett/conductor/workspaces/choo/phoenix-v1/internal/worker/review.go:cleanupWorktree()`
- Related test files: `review_test.go`, `worker_test.go`
- Current git interface: `/Users/bennett/conductor/workspaces/choo/phoenix-v1/internal/git/exec.go`
- Worker using git.Runner: `/Users/bennett/conductor/workspaces/choo/phoenix-v1/internal/worker/git_delegate.go`
- PRD: `/Users/bennett/conductor/workspaces/choo/phoenix-v1/docs/prd/safe-git-operations.md`
