---
task: 5
status: pending
backpressure: "go test ./internal/worker/... -v"
depends_on: [3, 4]
---

# Test Migration

**Parent spec**: `/specs/GITOPS-WORKER.md`
**Task**: #5 of 5 in implementation plan

## Objective

Update existing Worker tests to use MockGitOps instead of stubbed Runner, ensuring consistent test behavior.

## Dependencies

### External Specs (must be implemented)
- GITOPS-MOCK — provides MockGitOps with all assertion helpers

### Task Dependencies (within this unit)
- Task #3 must be complete (cleanupWorktree migrated)
- Task #4 must be complete (commitReviewFixes migrated)

### Package Dependencies
- Internal: `internal/git` (for MockGitOps, all types)

## Deliverables

### Files to Modify

```
internal/worker/
├── worker_test.go # MODIFY: Update test helpers to use MockGitOps
├── review_test.go # MODIFY: Complete migration of review tests
└── loop_test.go   # MODIFY: Update loop tests if they use git operations
```

### Test Patterns to Migrate

#### Old Pattern (stubbed Runner)

```go
// Old pattern - test stubs raw commands
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

#### New Pattern (MockGitOps)

```go
// New pattern - test uses MockGitOps
func TestCleanupWorktree(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    w := &Worker{
        gitOps: mockOps,
    }
    w.cleanupWorktree(ctx) // Safe - path embedded in mockOps

    // Verify calls
    mockOps.AssertCalled(t, "Reset")
    mockOps.AssertCalled(t, "Clean")
    mockOps.AssertCalled(t, "CheckoutFiles")
}
```

### Test Helper Updates

```go
// internal/worker/worker_test.go

// newTestWorker creates a Worker configured for testing with MockGitOps.
func newTestWorker(t *testing.T) *Worker {
    t.Helper()
    mockOps := git.NewMockGitOps("/test/worktree")
    return &Worker{
        gitOps:       mockOps,
        worktreePath: "/test/worktree",
        reviewConfig: &config.CodeReviewConfig{Verbose: false},
    }
}

// newTestWorkerWithConfig creates a Worker with custom configuration.
func newTestWorkerWithConfig(t *testing.T, cfg func(*Worker, *git.MockGitOps)) *Worker {
    t.Helper()
    mockOps := git.NewMockGitOps("/test/worktree")
    w := &Worker{
        gitOps:       mockOps,
        worktreePath: "/test/worktree",
        reviewConfig: &config.CodeReviewConfig{Verbose: false},
    }
    if cfg != nil {
        cfg(w, mockOps)
    }
    return w
}

// getMockGitOps extracts the MockGitOps from a Worker for assertions.
func getMockGitOps(t *testing.T, w *Worker) *git.MockGitOps {
    t.Helper()
    mock, ok := w.gitOps.(*git.MockGitOps)
    if !ok {
        t.Fatal("expected Worker to have MockGitOps")
    }
    return mock
}
```

### Safety Tests to Add

```go
func TestWorker_SafetyInvariants(t *testing.T) {
    t.Run("gitOps path matches worktreePath", func(t *testing.T) {
        w := newTestWorker(t)
        mock := getMockGitOps(t, w)

        if mock.Path() != w.worktreePath {
            t.Errorf("gitOps path %s doesn't match worktreePath %s",
                mock.Path(), w.worktreePath)
        }
    })

    t.Run("cleanupWorktree uses gitOps not runner", func(t *testing.T) {
        // Track if legacy runner was called
        runnerCalled := false
        fakeRunner := &fakeGitRunner{
            execFunc: func(ctx context.Context, dir string, args ...string) (string, error) {
                runnerCalled = true
                return "", nil
            },
        }

        mockOps := git.NewMockGitOps("/test/worktree")
        w := &Worker{
            gitOps:       mockOps,
            gitRunner:    fakeRunner,
            worktreePath: "/test/worktree",
        }

        w.cleanupWorktree(context.Background())

        if runnerCalled {
            t.Error("cleanupWorktree should use gitOps, not runner")
        }
        mockOps.AssertCalled(t, "Reset")
    })

    t.Run("commitReviewFixes uses gitOps not runner", func(t *testing.T) {
        runnerCalled := false
        fakeRunner := &fakeGitRunner{
            execFunc: func(ctx context.Context, dir string, args ...string) (string, error) {
                runnerCalled = true
                return "", nil
            },
        }

        mockOps := git.NewMockGitOps("/test/worktree")
        mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}
        w := &Worker{
            gitOps:       mockOps,
            gitRunner:    fakeRunner,
            worktreePath: "/test/worktree",
        }

        w.commitReviewFixes(context.Background())

        if runnerCalled {
            t.Error("commitReviewFixes should use gitOps, not runner")
        }
        mockOps.AssertCalled(t, "Commit")
    })
}

func TestWorker_ReviewLoopWithGitOps(t *testing.T) {
    t.Run("full fix loop uses correct git operations", func(t *testing.T) {
        mockOps := git.NewMockGitOps("/test/worktree")
        mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"fixed.go"}}

        w := newTestWorkerWithConfig(t, func(w *Worker, mock *git.MockGitOps) {
            mock.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}
        })

        // Simulate fix loop
        w.cleanupWorktree(context.Background())
        // ... provider makes fixes ...
        mockOps := getMockGitOps(t, w)
        mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"fixed.go"}}
        committed, _ := w.commitReviewFixes(context.Background())

        if !committed {
            t.Error("expected changes to be committed")
        }

        // Verify call order
        mockOps.AssertCallOrder(t, "Reset", "Clean", "CheckoutFiles", "Status", "AddAll", "Commit")
    })
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| All existing tests pass | No regressions |
| `TestWorker_SafetyInvariants` | GitOps used instead of Runner |
| `TestWorker_ReviewLoopWithGitOps` | Full loop works with MockGitOps |
| Test coverage maintained | No decrease in coverage |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Tests should use MockGitOps, not real git repos (faster, safer)
- Safety tests verify GitOps is used instead of Runner when both are set
- Test helpers make it easy to configure MockGitOps for specific scenarios
- getMockGitOps allows tests to access mock for assertions

## Common Migration Issues

1. **Empty worktreePath in tests**: No longer causes issues since path is embedded in MockGitOps
2. **Runner stubbing complexity**: Replaced by simple stub fields on MockGitOps
3. **Race conditions**: MockGitOps is thread-safe by design
4. **Assertion difficulty**: MockGitOps provides rich assertion helpers

## NOT In Scope

- Phase 3 changes (removing runner field)
- New feature tests
- Integration tests with real git repos
