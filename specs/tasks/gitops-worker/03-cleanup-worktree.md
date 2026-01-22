---
task: 3
status: pending
backpressure: "go test ./internal/worker/... -run TestCleanupWorktree -v"
depends_on: [2]
---

# cleanupWorktree Migration

**Parent spec**: `/specs/GITOPS-WORKER.md`
**Task**: #3 of 5 in implementation plan

## Objective

Migrate cleanupWorktree() to use GitOps methods (Reset, Clean, CheckoutFiles) instead of raw Runner.Exec.

## Dependencies

### External Specs (must be implemented)
- GITOPS — provides GitOps.Reset, GitOps.Clean, GitOps.CheckoutFiles
- GITOPS-MOCK — provides MockGitOps for testing

### Task Dependencies (within this unit)
- Task #2 must be complete (provides: Worker.gitOps initialized)

### Package Dependencies
- Standard library (`context`, `errors`, `fmt`, `os`)
- Internal: `internal/git` (for CleanOpts, error types)

## Deliverables

### Files to Modify

```
internal/worker/
├── review.go      # MODIFY: Update cleanupWorktree to use GitOps
└── review_test.go # MODIFY: Add cleanupWorktree tests with MockGitOps
```

### Functions to Modify

```go
// cleanupWorktree resets any uncommitted changes left by failed fix attempts.
// Uses GitOps for safe operations with path validation.
func (w *Worker) cleanupWorktree(ctx context.Context) {
    // Phase 1-2: Check if GitOps is available
    if w.gitOps == nil {
        // Fall back to old behavior during migration
        w.cleanupWorktreeLegacy(ctx)
        return
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
        // Handle specific safety errors
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
        if errors.Is(err, git.ErrDestructiveNotAllowed) {
            fmt.Fprintf(os.Stderr, "BUG: destructive operations not allowed on worktree\n")
        }
        if w.reviewConfig != nil && w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Warning: git checkout failed: %v\n", err)
        }
    }
}

// cleanupWorktreeLegacy is the old implementation using raw Runner.
// Retained for Phase 1-2 backward compatibility.
func (w *Worker) cleanupWorktreeLegacy(ctx context.Context) {
    if w.worktreePath == "" {
        return
    }

    w.runner().Exec(ctx, w.worktreePath, "reset", "HEAD")
    w.runner().Exec(ctx, w.worktreePath, "clean", "-fd")
    w.runner().Exec(ctx, w.worktreePath, "checkout", ".")
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run TestCleanupWorktree -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestCleanupWorktree_UsesGitOps` | Reset, Clean, CheckoutFiles all called |
| `TestCleanupWorktree_NilGitOps_NoOp` | Returns without error when gitOps is nil |
| `TestCleanupWorktree_ResetError_Continues` | Continues to Clean after Reset error |
| `TestCleanupWorktree_CleanError_Continues` | Continues to CheckoutFiles after Clean error |
| `TestCleanupWorktree_CleanOpts` | Clean called with Force=true, Directories=true |
| `TestCleanupWorktree_CheckoutFiles_Dot` | CheckoutFiles called with "." |

### Test Implementation

```go
func TestCleanupWorktree_UsesGitOps(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    w := &Worker{
        gitOps:       mockOps,
        reviewConfig: &config.CodeReviewConfig{Verbose: true},
    }

    w.cleanupWorktree(context.Background())

    mockOps.AssertCalled(t, "Reset")
    mockOps.AssertCalled(t, "Clean")
    mockOps.AssertCalled(t, "CheckoutFiles")

    // Verify Clean was called with correct options
    cleanCalls := mockOps.GetCallsFor("Clean")
    if len(cleanCalls) != 1 {
        t.Fatalf("expected 1 Clean call, got %d", len(cleanCalls))
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
        gitOps:       nil,
        worktreePath: "", // Prevents legacy fallback from doing anything
    }

    // Should not panic
    w.cleanupWorktree(context.Background())
}

func TestCleanupWorktree_ResetError_Continues(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    mockOps.ResetErr = errors.New("reset failed")
    w := &Worker{
        gitOps:       mockOps,
        reviewConfig: &config.CodeReviewConfig{Verbose: false},
    }

    w.cleanupWorktree(context.Background())

    // Should still call Clean and CheckoutFiles despite Reset error
    mockOps.AssertCalled(t, "Clean")
    mockOps.AssertCalled(t, "CheckoutFiles")
}

func TestCleanupWorktree_CleanError_Continues(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    mockOps.CleanErr = errors.New("clean failed")
    w := &Worker{
        gitOps:       mockOps,
        reviewConfig: &config.CodeReviewConfig{Verbose: false},
    }

    w.cleanupWorktree(context.Background())

    // Should still call CheckoutFiles despite Clean error
    mockOps.AssertCalled(t, "CheckoutFiles")
}

func TestCleanupWorktree_CheckoutFiles_Dot(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    w := &Worker{gitOps: mockOps}

    w.cleanupWorktree(context.Background())

    checkoutCalls := mockOps.GetCallsFor("CheckoutFiles")
    if len(checkoutCalls) != 1 {
        t.Fatalf("expected 1 CheckoutFiles call, got %d", len(checkoutCalls))
    }
    paths := checkoutCalls[0].Args[0].([]string)
    if len(paths) != 1 || paths[0] != "." {
        t.Errorf("expected CheckoutFiles to be called with [\".\"], got %v", paths)
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Cleanup is best-effort: errors are logged but don't stop the process
- ErrDestructiveNotAllowed indicates a bug (should never happen with NewWorktreeGitOps)
- Legacy fallback retained for Phase 1-2 backward compatibility
- All three operations run regardless of individual failures

## NOT In Scope

- commitReviewFixes migration (Task #4)
- Full test migration (Task #5)
