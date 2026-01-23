# GITOPS-WORKER - Worker Integration to Use GitOps Instead of Raw Runner

## Overview

The GITOPS-WORKER spec defines the migration of the Worker package from raw `git.Runner` usage to the safe `git.GitOps` interface. This migration addresses a production bug where tests inadvertently ran destructive git commands (`git checkout .`, `git reset`, `git clean -fd`) on the actual repository instead of test directories when `worktreePath` was empty.

The current pattern passes both a path and a runner separately, creating a dangerous gap where an empty path causes commands to execute in the current working directory. The GitOps interface validates paths at construction time, making invalid states unrepresentable and eliminating this class of bugs.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    Current Pattern (Dangerous)                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   Worker                                                                 │
│   ├── worktreePath: string    ◄── Can be empty!                         │
│   └── gitRunner: git.Runner   ◄── Executes in cwd when path empty       │
│                                                                          │
│   Usage:                                                                 │
│     w.runner().Exec(ctx, w.worktreePath, "reset", "HEAD")               │
│     // If worktreePath == "", runs in current directory!                │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Proposed Pattern (Safe)                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   Worker                                                                 │
│   └── gitOps: git.GitOps      ◄── Path validated at construction        │
│                                                                          │
│   Usage:                                                                 │
│     w.gitOps.Reset(ctx)       ◄── Path embedded in GitOps instance      │
│     // Cannot operate on wrong directory                                 │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Replace raw `gitRunner` + `worktreePath` pattern with `GitOps` interface in Worker struct
2. Validate worktree path at Worker construction time, rejecting empty or invalid paths
3. Migrate `cleanupWorktree()` to use GitOps methods (Reset, Clean, CheckoutFiles)
4. Migrate `commitReviewFixes()` to use GitOps methods (Add, Commit)
5. Migrate `hasUncommittedChanges()` to use GitOps method (Status)
6. Update all Worker tests to use MockGitOps instead of stubbed Runner
7. Preserve existing behavior while improving safety guarantees
8. Maintain backward compatibility during phased migration (Phase 1-2)
9. **NEW**: Use convenience constructors (`NewWorktreeGitOps`, `NewRepoRootGitOps`) for appropriate safety defaults
10. **NEW**: Configure `BranchGuard` for repo-level operations to protect main/master
11. **NEW**: Handle new typed errors (`ErrDestructiveNotAllowed`, `ErrEmptyPath`, `ErrRelativePath`, etc.)
12. **NEW**: Optionally enable audit logging for debugging git operations

### Performance Requirements

| Metric | Target |
|--------|--------|
| Worker construction overhead | <1ms additional validation |
| GitOps method latency | Same as raw Runner (passthrough) |
| Test execution time | No regression from mock switch |

### Constraints

- Depends on: GITOPS (interface definition), GITOPS-MOCK (testing infrastructure)
- Must maintain backward compatibility during migration
- Cannot change public Worker API signatures in Phase 1-2
- Must preserve worktreePath field for provider invocation (providers need raw path)

## Design

### Module Structure

```
internal/worker/
├── worker.go           # Worker struct with new gitOps field
├── review.go           # cleanupWorktree, commitReviewFixes migration
├── git_delegate.go     # Transitional: runner() helper during migration
├── loop.go             # Task loop (future migration)
└── *_test.go           # Tests updated to use MockGitOps
```

### Core Types

```go
// internal/worker/worker.go

// Worker executes a single unit in an isolated worktree
type Worker struct {
    unit         *discovery.Unit
    config       WorkerConfig
    events       *events.Bus
    git          *git.WorktreeManager

    // Phase 1: GitOps added alongside gitRunner
    // Phase 3: gitRunner removed, only gitOps remains
    gitOps       git.GitOps    // Safe git operations interface
    gitRunner    git.Runner    // Deprecated: raw runner for unmigrated code

    github       *github.PRClient
    provider     provider.Provider
    escalator    escalate.Escalator
    mergeMu      *sync.Mutex

    // Keep raw path for provider invocation (providers need filesystem path)
    worktreePath string
    branch       string
    currentTask  *discovery.Task

    reviewer     provider.Reviewer
    reviewConfig *config.CodeReviewConfig
    prNumber     int
}

// WorkerConfig bundles worker configuration options
type WorkerConfig struct {
    // ... existing fields
    WorktreeBase string // Base directory for worktrees
    RepoRoot     string // Repository root path

    // NEW: Audit logging
    AuditLogger git.AuditLogger // Optional: log all git operations
}

// WorkerDeps bundles worker dependencies for injection
type WorkerDeps struct {
    Events       *events.Bus
    Git          *git.WorktreeManager

    // Phase 1: Both GitOps and GitRunner supported
    // Phase 3: Only GitOps required
    GitOps       git.GitOps    // Preferred: safe git interface
    GitRunner    git.Runner    // Deprecated: raw runner

    GitHub       *github.PRClient
    Provider     provider.Provider
    Escalator    escalate.Escalator
    MergeMu      *sync.Mutex
    Reviewer     provider.Reviewer
    ReviewConfig *config.CodeReviewConfig
}
```

### Worker Construction with Safety Features

```go
// internal/worker/worker.go

// NewWorker creates a worker for executing a unit.
// Uses convenience constructors for appropriate safety defaults.
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) (*Worker, error) {
    // Phase 1: Use GitOps if provided, else create from WorktreeBase
    gitOps := deps.GitOps
    if gitOps == nil && cfg.WorktreeBase != "" {
        worktreePath := filepath.Join(cfg.WorktreeBase, unit.ID)

        // NEW: Use convenience constructor with safety options
        // NewWorktreeGitOps sets AllowDestructive=true because:
        // - Worktrees are isolated from main repository
        // - cleanupWorktree() needs Reset, Clean, CheckoutFiles
        // - Worktrees are disposable and meant to be reset
        var err error
        gitOps, err = git.NewWorktreeGitOps(worktreePath, cfg.WorktreeBase)
        if err != nil {
            return nil, fmt.Errorf("invalid worktree path %q: %w", worktreePath, err)
        }
    }

    // NEW: Alternative with explicit options and audit logging
    // gitOps, err = git.NewGitOps(worktreePath, git.GitOpsOpts{
    //     WorktreeBase:     cfg.WorktreeBase,
    //     AllowDestructive: true,  // Worktrees need destructive ops
    //     AuditLogger:      cfg.AuditLogger,  // Pass through if configured
    // })

    // Phase 1-2: Fall back to raw runner for backward compatibility
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
```

### BranchGuard for Repo-Level Operations

For operations that touch the repo root (e.g., merge operations), configure BranchGuard:

```go
// internal/worker/merge.go or worker.go

// createRepoGitOps creates a GitOps for repo-root operations with branch protection
func (w *Worker) createRepoGitOps() (git.GitOps, error) {
    return git.NewRepoRootGitOps(w.config.RepoRoot, &git.BranchGuard{
        AllowedBranchPrefixes: []string{"feature/", "fix/", "chore/"},
        ProtectedBranches:     []string{"main", "master"},
    })
}

// Usage in merge operations
func (w *Worker) mergeToFeatureBranch(ctx context.Context, branch string) error {
    repoGitOps, err := w.createRepoGitOps()
    if err != nil {
        return fmt.Errorf("creating repo gitops: %w", err)
    }

    // BranchGuard automatically prevents writes to main/master
    return repoGitOps.Merge(ctx, branch, git.MergeOpts{})
}
```

### API Surface with Safety Error Handling

```go
// internal/worker/review.go - After migration with safety error handling

// cleanupWorktree resets any uncommitted changes left by failed fix attempts.
// Uses GitOps for safe operations with path validation.
func (w *Worker) cleanupWorktree(ctx context.Context) {
    if w.gitOps == nil {
        return // No GitOps configured (shouldn't happen after Phase 2)
    }

    // 1. Reset staged changes
    if err := w.gitOps.Reset(ctx); err != nil {
        // Log but continue - cleanup is best-effort
        if w.reviewConfig != nil && w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Warning: git reset failed: %v\n", err)
        }
    }

    // 2. Clean untracked files
    if err := w.gitOps.Clean(ctx, git.CleanOpts{Force: true, Directories: true}); err != nil {
        // NEW: Handle specific safety errors
        if errors.Is(err, git.ErrDestructiveNotAllowed) {
            // This shouldn't happen with NewWorktreeGitOps, but log it
            fmt.Fprintf(os.Stderr, "BUG: destructive operations not allowed on worktree\n")
        }
        if w.reviewConfig != nil && w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Warning: git clean failed: %v\n", err)
        }
    }

    // 3. Restore modified files
    if err := w.gitOps.CheckoutFiles(ctx, "."); err != nil {
        // NEW: Handle specific safety errors
        if errors.Is(err, git.ErrDestructiveNotAllowed) {
            fmt.Fprintf(os.Stderr, "BUG: destructive operations not allowed on worktree\n")
        }
        if w.reviewConfig != nil && w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Warning: git checkout failed: %v\n", err)
        }
    }
}

// commitReviewFixes commits any changes made during the fix attempt.
// Returns (true, nil) if changes were committed, (false, nil) if no changes.
func (w *Worker) commitReviewFixes(ctx context.Context) (bool, error) {
    // 1. Check for staged/unstaged changes via Status
    status, err := w.gitOps.Status(ctx)
    if err != nil {
        return false, fmt.Errorf("checking for changes: %w", err)
    }
    if status.Clean {
        return false, nil
    }

    // 2. Stage all changes
    if err := w.gitOps.AddAll(ctx); err != nil {
        return false, fmt.Errorf("staging changes: %w", err)
    }

    // 3. Commit with standardized message
    commitMsg := "fix: address code review feedback"
    if err := w.gitOps.Commit(ctx, commitMsg, git.CommitOpts{NoVerify: true}); err != nil {
        // NEW: Handle branch guard errors
        if errors.Is(err, git.ErrProtectedBranch) {
            return false, fmt.Errorf("cannot commit to protected branch: %w", err)
        }
        return false, fmt.Errorf("committing changes: %w", err)
    }

    return true, nil
}

// hasUncommittedChanges returns true if there are staged or unstaged changes.
func (w *Worker) hasUncommittedChanges(ctx context.Context) (bool, error) {
    status, err := w.gitOps.Status(ctx)
    if err != nil {
        return false, err
    }
    return !status.Clean, nil
}
```

### Migration Strategy

```
┌─────────────────────────────────────────────────────────────────────────┐
│                   Phase 1: Add GitOps (Non-Breaking)                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. Add gitOps field to Worker struct (alongside gitRunner)             │
│  2. Update NewWorker to accept GitOps in deps                           │
│  3. Create GitOps from WorktreeBase if not provided                     │
│  4. All existing code continues to use runner()                         │
│                                                                          │
│  Result: No behavioral changes, GitOps available for new code           │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                   Phase 2: Migrate Critical Paths                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. cleanupWorktree() → GitOps.Reset, Clean, CheckoutFiles              │
│  2. commitReviewFixes() → GitOps.Add, Commit                            │
│  3. hasUncommittedChanges() → GitOps.HasUncommittedChanges              │
│  4. Update tests for migrated methods to use MockGitOps                 │
│                                                                          │
│  NEW: Safety features in Phase 2:                                        │
│  - Use NewWorktreeGitOps (automatic safety defaults)                    │
│  - AllowDestructive=true for worktree operations                        │
│  - Tests verify safety errors are handled correctly                     │
│  - Handle ErrDestructiveNotAllowed, ErrEmptyPath, ErrRelativePath       │
│                                                                          │
│  Result: Bug-prone code paths now use validated GitOps                  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                   Phase 3: Full Migration                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. Migrate remaining git operations in worker package                  │
│     - setupWorktree branch operations                                   │
│     - runBaselinePhase commit operations                                │
│     - mergeToFeatureBranch git operations                               │
│  2. Remove gitRunner field from Worker                                  │
│  3. Remove runner() helper method                                       │
│  4. Update all remaining tests                                          │
│                                                                          │
│  Result: Worker package fully migrated to GitOps                        │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                   Phase 4: Enforce Invariants                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. Add linting rule to prevent raw Runner.Exec in worker package       │
│  2. Consider removing public access to raw Runner                       │
│  3. Document GitOps as the required pattern for git operations          │
│                                                                          │
│  NEW: Optional safety enhancements in Phase 4:                           │
│  - Enable BranchGuard for repo-level operations                         │
│  - Enable audit logging for debugging/incident triage                   │
│  - Configure per-repo write locks for concurrent safety                 │
│                                                                          │
│  Result: Invalid path usage prevented at compile/lint time              │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Critical Worker Methods to Migrate

| Method | Current Pattern | GitOps Migration |
|--------|-----------------|------------------|
| `cleanupWorktree()` | `w.runner().Exec(ctx, w.worktreePath, "reset", ...)` | `w.gitOps.Reset(ctx)` |
| `cleanupWorktree()` | `w.runner().Exec(ctx, w.worktreePath, "clean", ...)` | `w.gitOps.Clean(ctx, opts)` |
| `cleanupWorktree()` | `w.runner().Exec(ctx, w.worktreePath, "checkout", ...)` | `w.gitOps.CheckoutFiles(ctx, ".")` |
| `commitReviewFixes()` | `w.runner().Exec(ctx, w.worktreePath, "add", ...)` | `w.gitOps.Add(ctx, opts)` |
| `commitReviewFixes()` | `w.runner().Exec(ctx, w.worktreePath, "commit", ...)` | `w.gitOps.Commit(ctx, msg, opts)` |
| `hasUncommittedChanges()` | `w.runner().Exec(ctx, w.worktreePath, "status", ...)` | `w.gitOps.Status(ctx)` + check `!Clean` |
| `setupWorktree()` | `w.runner().Exec(ctx, w.worktreePath, "checkout", ...)` | `w.gitOps.CheckoutBranch(ctx, branch)` |
| `runBaselinePhase()` | `w.runner().Exec(ctx, w.worktreePath, "add", ...)` | `w.gitOps.Add(ctx, opts)` |
| `runBaselinePhase()` | `w.runner().Exec(ctx, w.worktreePath, "commit", ...)` | `w.gitOps.Commit(ctx, msg, opts)` |

## Implementation Notes

### Why Keep worktreePath Field?

The `worktreePath` field must be retained even after full GitOps migration because:

1. **Provider invocation** - Task providers (Claude, Codex) need a filesystem path to set as their working directory
2. **Log file paths** - Logging uses worktreePath to construct log file locations
3. **Unit status updates** - File path construction for IMPLEMENTATION_PLAN.md updates

The GitOps interface encapsulates the path for git operations, but non-git operations still need the raw path.

### Empty Path Guard in Current Code

The current `cleanupWorktree()` has a guard that checks for empty path:

```go
func (w *Worker) cleanupWorktree(ctx context.Context) {
    // Guard: skip if no worktree path configured
    if w.worktreePath == "" {
        return
    }
    // ... rest of implementation
}
```

This guard is necessary because tests may create Workers without setting worktreePath. With GitOps, this guard becomes unnecessary because:

1. GitOps validates path at construction - empty path rejected with error
2. If `w.gitOps` is nil, the migrated method returns early
3. Tests provide MockGitOps, ensuring valid mock behavior

### AllowDestructive for Worktrees

Worktrees require destructive operations because they are designed to be reset:

```go
// NewWorktreeGitOps sets AllowDestructive=true because:
// - Worktrees are isolated from the main repository
// - cleanupWorktree() needs Reset, Clean, CheckoutFiles
// - Worktrees are disposable and meant to be thrown away
// - Worker cleanup should always succeed

// In contrast, NewRepoRootGitOps sets AllowDestructive=false because:
// - Repo root contains the authoritative state
// - Destructive operations could lose important work
// - Merge operations don't need Reset/Clean/CheckoutFiles
```

### Test Migration Pattern

Tests currently stub the Runner to prevent real git commands:

```go
// Current pattern - test stubs raw commands
func TestCleanupWorktree(t *testing.T) {
    fakeRunner := &fakeGitRunner{
        execFunc: func(ctx context.Context, dir string, args ...string) (string, error) {
            // Verify correct commands called
            return "", nil
        },
    }
    w := &Worker{
        gitRunner:    fakeRunner,
        worktreePath: "", // Oops - empty path in test
    }
    w.cleanupWorktree(ctx) // Runs in cwd!
}
```

After migration, tests use MockGitOps which has path embedded:

```go
// New pattern - test uses MockGitOps
func TestCleanupWorktree(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    w := &Worker{
        gitOps: mockOps,
    }
    w.cleanupWorktree(ctx) // Safe - path embedded in mockOps

    // Verify calls
    if !mockOps.ResetCalled {
        t.Error("expected Reset to be called")
    }
}
```

### Error Handling During Migration

Phase 1-2 maintain backward compatibility by allowing nil gitOps:

```go
func (w *Worker) cleanupWorktree(ctx context.Context) {
    // Phase 1-2: Fall back to old behavior if gitOps not set
    if w.gitOps == nil {
        // Old implementation using runner()
        if w.worktreePath == "" {
            return
        }
        w.runner().Exec(ctx, w.worktreePath, "reset", "HEAD")
        // ...
        return
    }

    // New GitOps implementation
    w.gitOps.Reset(ctx)
    // ...
}
```

Phase 3 removes the fallback, making gitOps required.

## Testing Strategy

### Unit Tests

```go
// internal/worker/review_test.go

func TestCleanupWorktree_UsesGitOps(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    w := &Worker{
        gitOps:       mockOps,
        reviewConfig: &config.CodeReviewConfig{Verbose: true},
    }

    w.cleanupWorktree(context.Background())

    // Verify all cleanup operations were called
    mockOps.AssertCalled(t, "Reset")
    mockOps.AssertCalled(t, "Clean")
    mockOps.AssertCalled(t, "CheckoutFiles")

    // Verify Clean was called with correct options
    cleanCalls := mockOps.GetCallsFor("Clean")
    if len(cleanCalls) == 0 {
        t.Fatal("Clean was not called")
    }
    opts := cleanCalls[0].Args[0].(git.CleanOpts)
    if !opts.Force {
        t.Error("expected Force=true for Clean")
    }
    if !opts.Directories {
        t.Error("expected Directories=true for Clean")
    }
}

func TestCleanupWorktree_NilGitOps_NoOp(t *testing.T) {
    w := &Worker{
        gitOps: nil, // Not configured
    }

    // Should not panic
    w.cleanupWorktree(context.Background())
}

func TestCommitReviewFixes_NoChanges(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    mockOps.StatusResult = git.StatusResult{Clean: true}

    w := &Worker{gitOps: mockOps}

    committed, err := w.commitReviewFixes(context.Background())

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if committed {
        t.Error("expected committed=false when no changes")
    }
    if mockOps.AssertCalled("AddAll") {
        t.Error("AddAll should not be called when no changes")
    }
}

func TestCommitReviewFixes_WithChanges(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

    w := &Worker{gitOps: mockOps}

    committed, err := w.commitReviewFixes(context.Background())

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if !committed {
        t.Error("expected committed=true when changes exist")
    }
    mockOps.AssertCalled(t, "AddAll")
    mockOps.AssertCalled(t, "Commit")
}

func TestNewWorker_InvalidPath_ReturnsError(t *testing.T) {
    unit := &discovery.Unit{ID: "test-unit"}
    cfg := WorkerConfig{
        WorktreeBase: "", // Empty base - will construct empty path
    }
    deps := WorkerDeps{} // No GitOps provided

    // After Phase 2, this should return an error
    _, err := NewWorker(unit, cfg, deps)

    // During Phase 1, this succeeds (backward compat)
    // After Phase 2, this fails
    if err == nil {
        t.Skip("Phase 1: backward compatibility mode")
    }

    if !strings.Contains(err.Error(), "invalid worktree path") {
        t.Errorf("expected invalid path error, got: %v", err)
    }
}
```

### Safety Feature Tests

```go
// internal/worker/review_test.go - NEW safety tests

func TestNewWorker_RejectsEmptyPath(t *testing.T) {
    unit := &discovery.Unit{ID: "test"}
    cfg := WorkerConfig{WorktreeBase: ""} // Empty
    deps := WorkerDeps{}

    _, err := NewWorker(unit, cfg, deps)

    if err == nil {
        t.Error("expected error for empty WorktreeBase")
    }
    if !errors.Is(err, git.ErrEmptyPath) {
        t.Errorf("expected ErrEmptyPath, got %v", err)
    }
}

func TestNewWorker_RejectsRelativePath(t *testing.T) {
    unit := &discovery.Unit{ID: "test"}
    cfg := WorkerConfig{WorktreeBase: "./relative/path"}
    deps := WorkerDeps{}

    _, err := NewWorker(unit, cfg, deps)

    if !errors.Is(err, git.ErrRelativePath) {
        t.Errorf("expected ErrRelativePath, got %v", err)
    }
}

func TestCleanupWorktree_DestructiveAllowed(t *testing.T) {
    // Use NewWorktreeGitOps which sets AllowDestructive=true
    mock := git.NewMockGitOpsWithOpts("/worktree", git.GitOpsOpts{
        AllowDestructive: true,
    })
    w := &Worker{gitOps: mock}

    w.cleanupWorktree(context.Background())

    // Should succeed because AllowDestructive=true
    mock.AssertCalled(t, "Reset")
    mock.AssertCalled(t, "Clean")
    mock.AssertCalled(t, "CheckoutFiles")
}

func TestCleanupWorktree_DestructiveNotAllowed_LogsBug(t *testing.T) {
    // Simulate misconfigured GitOps without AllowDestructive
    mock := git.NewMockGitOpsWithOpts("/worktree", git.GitOpsOpts{
        AllowDestructive: false,
    })
    mock.CleanErr = git.ErrDestructiveNotAllowed

    var stderr bytes.Buffer
    w := &Worker{
        gitOps:       mock,
        reviewConfig: &config.CodeReviewConfig{Verbose: true},
    }

    // Redirect stderr for test
    oldStderr := os.Stderr
    r, wr, _ := os.Pipe()
    os.Stderr = wr
    defer func() { os.Stderr = oldStderr }()

    w.cleanupWorktree(context.Background())

    wr.Close()
    io.Copy(&stderr, r)

    // Should log the BUG message
    if !strings.Contains(stderr.String(), "BUG: destructive operations not allowed") {
        t.Error("expected BUG message for ErrDestructiveNotAllowed")
    }
}

func TestCommitReviewFixes_ProtectedBranchError(t *testing.T) {
    mock := git.NewMockGitOps("/worktree")
    mock.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}
    mock.CommitErr = git.ErrProtectedBranch

    w := &Worker{gitOps: mock}

    _, err := w.commitReviewFixes(context.Background())

    if err == nil {
        t.Error("expected error for protected branch")
    }
    if !errors.Is(err, git.ErrProtectedBranch) {
        t.Errorf("expected ErrProtectedBranch, got %v", err)
    }
}

func TestRepoGitOps_BranchGuard(t *testing.T) {
    // Test that repo-level GitOps respects branch guard
    repoGitOps, err := git.NewRepoRootGitOps("/repo", &git.BranchGuard{
        AllowedBranchPrefixes: []string{"feature/", "fix/"},
        ProtectedBranches:     []string{"main", "master"},
    })
    if err != nil {
        t.Fatalf("failed to create repo GitOps: %v", err)
    }

    // Verify that merging to main would be rejected
    // (In real tests, mock would be used to simulate being on main)
    _ = repoGitOps // Use in integration tests
}
```

### Audit Logging Tests

```go
// internal/worker/worker_test.go - NEW audit logging tests

func TestWorker_AuditLogging(t *testing.T) {
    var logs []git.AuditEntry
    logger := &testAuditLogger{entries: &logs}

    unit := &discovery.Unit{ID: "test"}
    cfg := WorkerConfig{
        WorktreeBase: "/tmp/worktrees",
        AuditLogger:  logger,
    }
    deps := WorkerDeps{}

    w, err := NewWorker(unit, cfg, deps)
    if err != nil {
        t.Fatalf("failed to create worker: %v", err)
    }

    // Perform git operations
    w.cleanupWorktree(context.Background())

    // Verify audit logs contain expected entries
    if len(logs) == 0 {
        t.Error("expected audit logs to be captured")
    }

    for _, entry := range logs {
        if entry.RepoPath == "" {
            t.Error("audit entry missing RepoPath")
        }
        if entry.Operation == "" {
            t.Error("audit entry missing Operation")
        }
    }
}

type testAuditLogger struct {
    entries *[]git.AuditEntry
}

func (l *testAuditLogger) Log(entry git.AuditEntry) {
    *l.entries = append(*l.entries, entry)
}
```

### Integration Tests

| Scenario | Description |
|----------|-------------|
| Full review fix loop | Create worker with MockGitOps, run fix loop, verify all git ops called in order |
| Worktree cleanup after provider failure | Simulate provider error, verify cleanup resets state |
| Commit flow with changes | Mock HasUncommittedChanges=true, verify Add+Commit sequence |
| Commit flow without changes | Mock HasUncommittedChanges=false, verify no Add/Commit calls |
| Worker construction with invalid path | Verify error returned for empty/invalid WorktreeBase |
| **NEW**: Safety error handling | Verify ErrDestructiveNotAllowed is logged, not propagated |
| **NEW**: BranchGuard validation | Verify ErrProtectedBranch prevents commits to main |
| **NEW**: Audit log capture | Verify all operations emit audit entries |

### Manual Testing

- [ ] Create Worker with valid WorktreeBase, verify GitOps created
- [ ] Run review fix loop, verify worktree is properly cleaned after failure
- [ ] Verify commit message format matches expected pattern
- [ ] Run full unit execution with GitOps, verify no regressions
- [ ] Attempt to create Worker with empty WorktreeBase in Phase 2+, verify error
- [ ] **NEW**: Verify NewWorktreeGitOps allows destructive operations
- [ ] **NEW**: Verify NewRepoRootGitOps blocks destructive operations
- [ ] **NEW**: Enable audit logging and verify entries are captured

## Design Decisions

### Why Migrate to GitOps Instead of Adding Path Validation to Runner?

Adding path validation to Runner would require:
1. Changing the Runner interface (breaking change)
2. Validating at every call site (error-prone)
3. Runtime validation instead of construction-time (late failure)

GitOps provides:
1. Construction-time validation (fail fast)
2. Path encapsulated in instance (impossible to forget)
3. Higher-level API (Reset vs Exec + args)
4. Easier testing (mock entire interface vs stub individual calls)
5. **NEW**: Built-in safety invariants (branch guards, destructive protection)

### Why Keep Raw worktreePath for Providers?

Providers are external processes (Claude CLI, Codex CLI) that need:
1. A working directory for their subprocess
2. No knowledge of GitOps abstraction

Passing GitOps to providers would:
1. Couple providers to git abstraction (unnecessary)
2. Require providers to extract path (violation of encapsulation)
3. Add complexity for no safety benefit

The worktreePath is safe to pass to providers because they don't execute destructive git commands - they work on files.

### Why Phased Migration Instead of Big Bang?

Phased migration provides:
1. Lower risk - each phase can be tested independently
2. Easier rollback - Phase 1 changes are additive only
3. Incremental value - Phase 2 fixes the critical bug
4. Manageable reviews - smaller PRs are easier to review

Big bang would require:
1. Massive PR touching all worker code
2. Higher risk of regression
3. Harder to bisect if issues arise

### Why AllowDestructive=true for Worktrees?

Worktrees are designed to be disposable:
1. Created fresh for each unit execution
2. Expected to be reset between tasks
3. Isolated from main repository state
4. No valuable state that could be lost

Setting `AllowDestructive=true` via `NewWorktreeGitOps` is safe because:
1. The worktree path is validated to be under `WorktreeBase`
2. Operations cannot accidentally target the repo root
3. Cleanup operations (`Reset`, `Clean`, `CheckoutFiles`) are essential for worker functionality

## Future Enhancements

1. **Lint rule for raw Runner** - Add custom linter to prevent `runner().Exec` calls in worker package
2. **GitOps for merge operations** - Extend GitOps interface to cover merge/rebase operations
3. **GitOps for all packages** - Migrate other packages (git_delegate, merge) to GitOps
4. **Transaction support** - Add GitOps transaction wrapper for atomic multi-operation sequences
5. ~~Audit logging~~ - **DONE**: Add optional logging layer to GitOps for debugging
6. **Per-repo write locks** - Configure lock timeouts and try-lock semantics

## References

- GITOPS spec - Interface definition and constructor
- GITOPS-MOCK spec - Mock implementation for testing
- **NEW**: GITOPS-SAFETY PRD - Safety invariants and guardrails (`docs/prd/safe-git-operations.md`)
- Issue: Tests ran git commands on actual repository due to empty worktreePath
- Related files:
  - `/Users/bennett/conductor/workspaces/choo/phoenix-v1/internal/worker/review.go` - cleanupWorktree implementation
  - `/Users/bennett/conductor/workspaces/choo/phoenix-v1/internal/worker/worker.go` - Worker struct definition
  - `/Users/bennett/conductor/workspaces/choo/phoenix-v1/internal/git/exec.go` - Current Runner interface
