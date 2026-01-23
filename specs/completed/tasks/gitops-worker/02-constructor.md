---
task: 2
status: complete
backpressure: "go test ./internal/worker/... -run TestNewWorker -v"
depends_on: [1]
---

# Constructor Updates

**Parent spec**: `/specs/GITOPS-WORKER.md`
**Task**: #2 of 5 in implementation plan

## Objective

Update NewWorker to create GitOps from WorktreeBase when not provided in deps, with proper error handling for invalid paths.

## Dependencies

### External Specs (must be implemented)
- GITOPS — provides NewWorktreeGitOps, error types

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: Worker.gitOps field, WorkerDeps.GitOps)

### Package Dependencies
- Standard library (`fmt`, `path/filepath`)
- Internal: `internal/git` (for NewWorktreeGitOps)

## Deliverables

### Files to Modify

```
internal/worker/
├── worker.go      # MODIFY: Update NewWorker to create GitOps
└── worker_test.go # MODIFY: Add constructor tests
```

### Functions to Modify

```go
// NewWorker creates a worker for executing a unit.
// Uses convenience constructors for appropriate safety defaults.
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) (*Worker, error) {
    // Use provided GitOps or create from WorktreeBase
    gitOps := deps.GitOps
    if gitOps == nil && cfg.WorktreeBase != "" && unit != nil {
        worktreePath := filepath.Join(cfg.WorktreeBase, unit.ID)

        // Use convenience constructor with safety options
        // NewWorktreeGitOps sets AllowDestructive=true because:
        // - Worktrees are isolated from main repository
        // - cleanupWorktree() needs Reset, Clean, CheckoutFiles
        // - Worktrees are disposable and meant to be reset
        var err error
        gitOps, err = git.NewWorktreeGitOps(worktreePath, cfg.WorktreeBase)
        if err != nil {
            // During Phase 1-2, path may not exist yet (created later)
            // Only fail on validation errors, not path-not-found
            if !errors.Is(err, git.ErrPathNotFound) && !errors.Is(err, git.ErrNotGitRepo) {
                return nil, fmt.Errorf("invalid worktree path %q: %w", worktreePath, err)
            }
            // Path doesn't exist yet - that's OK, gitOps will be nil
            gitOps = nil
        }
    }

    // Fall back to raw runner for backward compatibility
    gitRunner := deps.GitRunner
    if gitRunner == nil {
        gitRunner = git.DefaultRunner()
    }

    return &Worker{
        unit:         unit,
        config:       cfg,
        events:       deps.Events,
        git:          deps.Git,
        gitOps:       gitOps,
        gitRunner:    gitRunner,
        github:       deps.GitHub,
        provider:     deps.Provider,
        escalator:    deps.Escalator,
        mergeMu:      deps.MergeMu,
        reviewer:     deps.Reviewer,
        reviewConfig: deps.ReviewConfig,
    }, nil
}

// InitGitOps initializes GitOps after worktree is created.
// Call this after setupWorktree() to enable safe git operations.
func (w *Worker) InitGitOps() error {
    if w.gitOps != nil {
        return nil // Already initialized
    }

    if w.worktreePath == "" {
        return fmt.Errorf("worktree path not set")
    }

    var err error
    w.gitOps, err = git.NewWorktreeGitOps(w.worktreePath, w.config.WorktreeBase)
    if err != nil {
        return fmt.Errorf("initializing gitops: %w", err)
    }

    return nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run TestNewWorker -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestNewWorker_WithGitOps` | Uses provided GitOps |
| `TestNewWorker_CreatesGitOps` | Creates GitOps from WorktreeBase when path exists |
| `TestNewWorker_NoGitOps_PathNotFound` | gitOps is nil when worktree doesn't exist yet |
| `TestNewWorker_RejectsEmptyPath` | Returns error for empty WorktreeBase (when strict) |
| `TestNewWorker_RejectsRelativePath` | Returns error for relative WorktreeBase |
| `TestInitGitOps` | Initializes gitOps after worktree exists |

### Test Implementation

```go
func TestNewWorker_WithGitOps(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    unit := &discovery.Unit{ID: "test-unit"}
    cfg := WorkerConfig{WorktreeBase: "/tmp/worktrees"}
    deps := WorkerDeps{
        GitOps: mockOps,
    }

    w, err := NewWorker(unit, cfg, deps)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if w.gitOps != mockOps {
        t.Error("expected provided GitOps to be used")
    }
}

func TestNewWorker_NoGitOps_PathNotFound(t *testing.T) {
    unit := &discovery.Unit{ID: "test-unit"}
    cfg := WorkerConfig{WorktreeBase: "/tmp/nonexistent-base"}
    deps := WorkerDeps{}

    w, err := NewWorker(unit, cfg, deps)

    // Should succeed - path will be created later
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if w.gitOps != nil {
        t.Error("expected gitOps to be nil when path doesn't exist")
    }
}

func TestNewWorker_RejectsRelativePath(t *testing.T) {
    unit := &discovery.Unit{ID: "test-unit"}
    cfg := WorkerConfig{WorktreeBase: "./relative/path"}
    deps := WorkerDeps{}

    _, err := NewWorker(unit, cfg, deps)

    // Relative path should be rejected
    if err == nil {
        t.Error("expected error for relative WorktreeBase")
    }
    if !errors.Is(err, git.ErrRelativePath) {
        t.Errorf("expected ErrRelativePath, got %v", err)
    }
}

func TestInitGitOps(t *testing.T) {
    // Create a real worktree for this test
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    // Create worker with no initial gitOps
    w := &Worker{
        worktreePath: dir,
        config:       WorkerConfig{WorktreeBase: filepath.Dir(dir)},
    }

    err := w.InitGitOps()

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if w.gitOps == nil {
        t.Error("expected gitOps to be initialized")
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- During Phase 1-2, worktree may not exist at Worker creation time
- Path-not-found errors are tolerated; other validation errors are not
- InitGitOps() can be called after setupWorktree() creates the worktree
- Relative path errors are always fatal (safety requirement)

## NOT In Scope

- Method migrations (Tasks #3, #4)
- Test updates (Task #5)
