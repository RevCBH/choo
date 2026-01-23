---
task: 4
status: complete
backpressure: "go test ./internal/git/... -run TestNewGitOps -v"
depends_on: [1, 2]
---

# GitOps Interface and Constructor

**Parent spec**: `/specs/GITOPS.md`
**Task**: #4 of 7 in implementation plan

## Objective

Define the GitOps interface and implement NewGitOps constructor with 9-point path validation.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: error types, GitOpsOpts)
- Task #2 must be complete (provides: StatusResult, Commit, option structs)

### Package Dependencies
- Standard library (`context`, `os`, `path/filepath`, `strings`)
- Internal: `git.Runner` (existing in exec.go)

## Deliverables

### Files to Create/Modify

```
internal/git/
├── gitops.go      # CREATE: GitOps interface, gitOps struct, NewGitOps
└── gitops_test.go # CREATE: Constructor tests
```

### Types to Implement

```go
package git

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// GitOps provides safe git operations bound to a specific repository path.
// All operations are validated to prevent accidental execution in wrong directories.
type GitOps interface {
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

    // Working tree operations (destructive)
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

// gitOps implements GitOps with path validation and safety checks.
type gitOps struct {
    path   string     // Canonical, absolute path
    opts   GitOpsOpts
    runner Runner
}

// NewGitOps creates a GitOps bound to a specific path with safety options.
// Performs 9 validation checks at construction time.
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
func NewWorktreeGitOps(path string, worktreeBase string) (GitOps, error) {
    return NewGitOps(path, GitOpsOpts{
        WorktreeBase:     worktreeBase,
        AllowRepoRoot:    false,
        AllowDestructive: true,
        SafetyLevel:      SafetyDefault,
    })
}

// NewRepoRootGitOps creates a GitOps for repo root operations.
func NewRepoRootGitOps(path string, guard *BranchGuard) (GitOps, error) {
    return NewGitOps(path, GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: false,
        BranchGuard:      guard,
        SafetyLevel:      SafetyStrict,
    })
}

func (g *gitOps) Path() string {
    return g.path
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -run TestNewGitOps -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestNewGitOps_EmptyPath` | Returns `ErrEmptyPath` |
| `TestNewGitOps_RelativePath` | Returns `ErrRelativePath` |
| `TestNewGitOps_NonExistentPath` | Returns `ErrPathNotFound` |
| `TestNewGitOps_PathIsFile` | Returns `ErrNotDirectory` |
| `TestNewGitOps_NotGitRepo` | Returns `ErrNotGitRepo` |
| `TestNewGitOps_ValidPath` | Returns valid GitOps instance |
| `TestNewGitOps_RepoRootNotAllowed` | Returns `ErrRepoRootNotAllowed` when AllowRepoRoot=false |
| `TestGitOps_PathIsImmutable` | `ops.Path()` returns same value |

### Test Implementation

```go
// internal/git/gitops_test.go
package git

import (
    "errors"
    "os"
    "os/exec"
    "testing"
)

func TestNewGitOps_EmptyPath(t *testing.T) {
    _, err := NewGitOps("", GitOpsOpts{})
    if !errors.Is(err, ErrEmptyPath) {
        t.Errorf("expected ErrEmptyPath, got %v", err)
    }
}

func TestNewGitOps_RelativePath(t *testing.T) {
    _, err := NewGitOps("./relative/path", GitOpsOpts{})
    if !errors.Is(err, ErrRelativePath) {
        t.Errorf("expected ErrRelativePath, got %v", err)
    }
}

func TestNewGitOps_NonExistentPath(t *testing.T) {
    _, err := NewGitOps("/nonexistent/path/that/does/not/exist", GitOpsOpts{})
    if !errors.Is(err, ErrPathNotFound) {
        t.Errorf("expected ErrPathNotFound, got %v", err)
    }
}

func TestNewGitOps_PathIsFile(t *testing.T) {
    f, _ := os.CreateTemp("", "gitops-test")
    f.Close()
    defer os.Remove(f.Name())

    _, err := NewGitOps(f.Name(), GitOpsOpts{})
    if !errors.Is(err, ErrNotDirectory) {
        t.Errorf("expected ErrNotDirectory, got %v", err)
    }
}

func TestNewGitOps_NotGitRepo(t *testing.T) {
    dir := t.TempDir()
    _, err := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
    if !errors.Is(err, ErrNotGitRepo) {
        t.Errorf("expected ErrNotGitRepo, got %v", err)
    }
}

func TestNewGitOps_ValidPath(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    ops, err := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if ops.Path() != dir {
        t.Errorf("expected path %s, got %s", dir, ops.Path())
    }
}

func TestNewGitOps_RepoRootNotAllowed(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    _, err := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: false})
    if !errors.Is(err, ErrRepoRootNotAllowed) {
        t.Errorf("expected ErrRepoRootNotAllowed, got %v", err)
    }
}

func TestGitOps_PathIsImmutable(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()
    ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})

    path1 := ops.Path()
    path2 := ops.Path()
    if path1 != path2 {
        t.Error("Path() returned different values")
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Uses `filepath.EvalSymlinks` to handle macOS `/var` → `/private/var` symlinks
- The existing `git.Runner` interface is used for git command execution
- `newGitOpsWithRunner` is exported for testing with fake runners

## NOT In Scope

- Read operation implementations (Task #5)
- Write operation implementations (Task #6, #7)
- Runtime validation helper (implemented with operations)
