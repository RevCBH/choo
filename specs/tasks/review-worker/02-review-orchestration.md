---
task: 2
status: pending
backpressure: "go test ./internal/worker/... -run TestRunCodeReview"
depends_on: [1]
---

# Review Orchestration

**Parent spec**: `specs/REVIEW-WORKER.md`
**Task**: #2 of 4 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement `runCodeReview()` as the main entry point for advisory code review in the Worker. This function orchestrates the review lifecycle: checking if review is enabled, invoking the reviewer, emitting events, and delegating to the fix loop when issues are found.

## Dependencies

### External Specs (must be implemented)
- REVIEWER-INTERFACE - provides `Reviewer` interface and `ReviewResult`, `ReviewIssue` types
- REVIEW-CONFIG - provides `CodeReviewConfig` with `MaxFixIterations`, `Verbose`

### Task Dependencies (within this unit)
- Task #1 (Code Review Event Types) - provides event constants

### Package Dependencies
- `github.com/RevCBH/choo/internal/events` - for event emission
- `github.com/RevCBH/choo/internal/provider` - for Reviewer interface
- `github.com/RevCBH/choo/internal/config` - for CodeReviewConfig

## Deliverables

### Files to Create/Modify

```
internal/worker/
├── worker.go    # MODIFY: Add reviewer and reviewConfig fields to Worker struct
└── review.go    # CREATE: Review orchestration logic
```

### Additions to Worker Struct

Add to `Worker` struct in `worker.go`:

```go
type Worker struct {
    // ... existing fields ...

    reviewer     provider.Reviewer      // For code review (may be nil if disabled)
    reviewConfig *config.CodeReviewConfig // Review configuration
}
```

Update `WorkerDeps` to accept reviewer:

```go
type WorkerDeps struct {
    // ... existing fields ...
    Reviewer     provider.Reviewer      // Optional: for code review
    ReviewConfig *config.CodeReviewConfig // Optional: review settings
}
```

Update `NewWorker` to wire reviewer and config.

### Functions to Implement

```go
// internal/worker/review.go

package worker

import (
    "context"
    "fmt"
    "os"

    "github.com/RevCBH/choo/internal/events"
)

// runCodeReview performs advisory code review after task completion.
// This function NEVER returns an error that blocks the merge.
// All review failures are logged but do not prevent merge.
func (w *Worker) runCodeReview(ctx context.Context) {
    // 1. Check if reviewer is nil (disabled)
    if w.reviewer == nil {
        return
    }

    // 2. Emit CodeReviewStarted event
    if w.events != nil {
        evt := events.NewEvent(events.CodeReviewStarted, w.unit.ID)
        w.events.Emit(evt)
    }

    // 3. Determine base ref for comparison (local feature branch)
    baseRef := w.getBaseRef()

    // 4. Invoke reviewer
    result, err := w.reviewer.Review(ctx, w.worktreePath, baseRef)
    if err != nil {
        // Log error but don't fail
        if w.reviewConfig != nil && w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Code review failed to run: %v\n", err)
        }
        if w.events != nil {
            evt := events.NewEvent(events.CodeReviewFailed, w.unit.ID).
                WithPayload(map[string]any{"error": err.Error()})
            w.events.Emit(evt)
        }
        return // Proceed to merge anyway
    }

    // 5. Check result - no issues means passed
    if result.Passed || len(result.Issues) == 0 {
        if w.reviewConfig != nil && w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Code review passed: %s\n", result.Summary)
        }
        if w.events != nil {
            evt := events.NewEvent(events.CodeReviewPassed, w.unit.ID).
                WithPayload(map[string]any{"summary": result.Summary})
            w.events.Emit(evt)
        }
        return
    }

    // 6. Issues found - always log (actionable information)
    fmt.Fprintf(os.Stderr, "Code review found %d issues\n", len(result.Issues))
    if w.events != nil {
        evt := events.NewEvent(events.CodeReviewIssuesFound, w.unit.ID).
            WithPayload(map[string]any{
                "count":  len(result.Issues),
                "issues": result.Issues,
            })
        w.events.Emit(evt)
    }

    // 7. Attempt fix loop if configured
    if w.reviewConfig != nil && w.reviewConfig.MaxFixIterations > 0 {
        w.runReviewFixLoop(ctx, result.Issues)
    }

    // 8. Merge proceeds regardless of fix outcome (handled by caller)
}

// getBaseRef returns the base branch reference for diff comparison.
// Uses the local feature branch which may contain prior unit merges
// that haven't been pushed yet.
func (w *Worker) getBaseRef() string {
    // Primary: use target branch (the local feature branch)
    if w.config.TargetBranch != "" {
        return w.config.TargetBranch
    }
    // Fallback: main
    return "main"
}
```

### Integration Note

This task defines `runCodeReview()` in `review.go`. The actual wiring (replacing `logReviewPlaceholder()` call with `runCodeReview()` in `mergeToFeatureBranch()`) is done by the **review-wiring** spec task #2. This separation ensures clear ownership: this task implements the function, review-wiring integrates it.

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run TestRunCodeReview
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestRunCodeReview_NilReviewer` | Returns immediately, no events emitted |
| `TestRunCodeReview_ReviewerError` | Emits CodeReviewFailed, function returns (no panic) |
| `TestRunCodeReview_Passed` | Emits CodeReviewStarted then CodeReviewPassed |
| `TestRunCodeReview_IssuesFound` | Emits CodeReviewIssuesFound, calls fix loop |
| `TestRunCodeReview_IssuesFound_ZeroIterations` | Does not call fix loop when MaxFixIterations=0 |

### Test Fixtures

```go
func TestRunCodeReview_NilReviewer(t *testing.T) {
    w := &Worker{
        reviewer: nil, // Disabled
        events:   events.NewBus(),
    }

    // Should return immediately without error or panic
    ctx := context.Background()
    w.runCodeReview(ctx)
    // No assertions needed - just verify no panic
}

func TestRunCodeReview_ReviewerError(t *testing.T) {
    eventBus := events.NewBus()
    collected := collectEvents(eventBus)

    w := &Worker{
        unit:     &discovery.Unit{ID: "test-unit"},
        reviewer: &mockReviewer{err: errors.New("reviewer unavailable")},
        events:   eventBus,
        reviewConfig: &config.CodeReviewConfig{
            Enabled: true,
            Verbose: true,
        },
    }

    ctx := context.Background()
    w.runCodeReview(ctx)

    // Should emit started, then failed
    require.Len(t, *collected, 2)
    assert.Equal(t, events.CodeReviewStarted, (*collected)[0].Type)
    assert.Equal(t, events.CodeReviewFailed, (*collected)[1].Type)
}

func TestRunCodeReview_Passed(t *testing.T) {
    eventBus := events.NewBus()
    collected := collectEvents(eventBus)

    w := &Worker{
        unit: &discovery.Unit{ID: "test-unit"},
        reviewer: &mockReviewer{
            result: &provider.ReviewResult{
                Passed:  true,
                Summary: "All checks passed",
            },
        },
        events: eventBus,
        reviewConfig: &config.CodeReviewConfig{
            Enabled: true,
            Verbose: true,
        },
    }

    ctx := context.Background()
    w.runCodeReview(ctx)

    require.Len(t, *collected, 2)
    assert.Equal(t, events.CodeReviewStarted, (*collected)[0].Type)
    assert.Equal(t, events.CodeReviewPassed, (*collected)[1].Type)
}

// collectEvents subscribes to the event bus and collects events for testing.
// Returns a pointer to a slice that will be populated with events as they occur.
func collectEvents(bus *events.Bus) *[]events.Event {
    collected := &[]events.Event{}
    bus.Subscribe(func(e events.Event) {
        *collected = append(*collected, e)
    })
    return collected
}

// mockReviewer for testing
type mockReviewer struct {
    result *provider.ReviewResult
    err    error
}

func (m *mockReviewer) Review(ctx context.Context, workdir, baseBranch string) (*provider.ReviewResult, error) {
    return m.result, m.err
}

func (m *mockReviewer) Name() provider.ProviderType {
    return "mock"
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- **Error Handling**: This function has unusual error handling - errors are logged but never returned. This is intentional per the spec: review is advisory and must never block merge.
- **Verbose Flag**: When `Verbose` is false, only issues requiring attention are printed (not success messages).
- **Base Ref**: Uses local feature branch, not `origin/feature-branch`. This ensures we review only the current unit's changes, not all changes since the target branch diverged.
- **Nil Checks**: Both `w.events` and `w.reviewConfig` may be nil - always check before use.

### Stderr Convention

All review output goes to stderr because:
1. stdout might be piped to other tools
2. This matches existing worker logging conventions
3. Review output is diagnostic, not primary data

## NOT In Scope

- Fix loop implementation (Task #3)
- Commit and cleanup operations (Task #4)
- Pre-merge review (orchestrator-level, not in this unit)
- Fix prompt building (Task #3)
- Wiring `runCodeReview()` into `mergeToFeatureBranch()` (review-wiring spec, task #2)
