---
task: 2
status: pending
backpressure: "go test ./internal/worker/... -run TestRunCodeReview"
depends_on: [1]
---

# Pre-Merge Review Integration

**Parent spec**: `/docs/prd/CODE-REVIEW.md`
**Task**: #2 of 2 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Wire the `runCodeReview()` method into the Worker's `mergeToFeatureBranch()` flow, replacing the `logReviewPlaceholder()` call with the actual advisory code review.

## Dependencies

### External Specs (must be implemented)
- **review-worker** - provides `runCodeReview()` method on Worker
- **codex-reviewer** - provides CodexReviewer implementation
- **claude-reviewer** - provides ClaudeReviewer implementation

### Task Dependencies (within this unit)
- Task #1 - Reviewer is resolved and injected into WorkerDeps

### Package Dependencies
- `github.com/RevCBH/choo/internal/worker`
- `github.com/RevCBH/choo/internal/provider`

## Deliverables

### Files to Create/Modify

```
internal/worker/
├── worker.go       # MODIFY: Replace logReviewPlaceholder() with runCodeReview()
└── worker_test.go  # MODIFY: Add integration tests for review in merge flow
```

### Code Changes

#### 1. Delete `logReviewPlaceholder()` function

Remove the placeholder function (approximately lines 546-557):

```go
// DELETE THIS FUNCTION:
func (w *Worker) logReviewPlaceholder(ctx context.Context) {
    // Get the diff between target branch and current HEAD
    diff, err := w.runner().Exec(ctx, w.worktreePath, "diff", fmt.Sprintf("origin/%s...HEAD", w.config.TargetBranch), "--stat")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Review placeholder - could not get diff: %v\n", err)
        return
    }

    fmt.Fprintf(os.Stderr, "Review placeholder - changes to review for unit %s:\n%s\n", w.unit.ID, diff)
}
```

#### 2. Update `mergeToFeatureBranch()` to call `runCodeReview()`

Replace the placeholder call with the actual review:

```go
// In mergeToFeatureBranch(), around line 331-332
// BEFORE:
//     // 1. Review placeholder (log what would be reviewed)
//     w.logReviewPlaceholder(ctx)

// AFTER:
    // 1. Run advisory code review (never blocks merge)
    w.runCodeReview(ctx)
```

#### 3. Ensure Worker struct has reviewer field

The `review-worker` unit should have already added this, but verify:

```go
type Worker struct {
    unit         *discovery.Unit
    config       WorkerConfig
    events       *events.Bus
    git          *git.WorktreeManager
    gitRunner    git.Runner
    github       *github.PRClient
    provider     provider.Provider
    escalator    escalate.Escalator
    reviewer     provider.Reviewer  // Must exist from review-worker unit
    mergeMu      *sync.Mutex
    worktreePath string
    branch       string
    currentTask  *discovery.Task
    // ...
}
```

#### 4. Update NewWorker to accept reviewer

Verify that `NewWorker` properly sets the reviewer field from WorkerDeps:

```go
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) *Worker {
    gitRunner := deps.GitRunner
    if gitRunner == nil {
        gitRunner = git.DefaultRunner()
    }
    return &Worker{
        unit:      unit,
        config:    cfg,
        events:    deps.Events,
        git:       deps.Git,
        gitRunner: gitRunner,
        github:    deps.GitHub,
        provider:  deps.Provider,
        escalator: deps.Escalator,
        reviewer:  deps.Reviewer,  // Must be set from deps
        mergeMu:   deps.MergeMu,
    }
}
```

### Tests to Implement

```go
// Add to internal/worker/worker_test.go

func TestMergeToFeatureBranch_WithReview(t *testing.T) {
    // Setup mock reviewer that passes
    mockReviewer := &MockReviewer{
        ReviewResult: &provider.ReviewResult{
            Passed:  true,
            Summary: "No issues found",
        },
    }

    worker := &Worker{
        unit:     &discovery.Unit{ID: "test-unit"},
        reviewer: mockReviewer,
        config: WorkerConfig{
            TargetBranch: "main",
            NoPR:         true, // Skip actual merge for unit test
        },
        // ... other required fields with mocks
    }

    // The test verifies that runCodeReview is called (via mock)
    // and that merge proceeds regardless of review outcome
}

func TestMergeToFeatureBranch_ReviewDisabled(t *testing.T) {
    // Reviewer is nil when disabled
    worker := &Worker{
        unit:     &discovery.Unit{ID: "test-unit"},
        reviewer: nil, // Review disabled
        config: WorkerConfig{
            TargetBranch: "main",
            NoPR:         true,
        },
        // ... other required fields with mocks
    }

    // Verify merge proceeds without calling review
}

func TestMergeToFeatureBranch_ReviewFailsButMergeProceeds(t *testing.T) {
    // Mock reviewer that returns an error
    mockReviewer := &MockReviewer{
        ReviewErr: fmt.Errorf("review failed to execute"),
    }

    worker := &Worker{
        unit:     &discovery.Unit{ID: "test-unit"},
        reviewer: mockReviewer,
        config: WorkerConfig{
            TargetBranch: "main",
            NoPR:         true,
        },
        // ... other required fields with mocks
    }

    // Verify merge proceeds despite review failure (advisory)
}

func TestMergeToFeatureBranch_ReviewIssuesButMergeProceeds(t *testing.T) {
    // Mock reviewer that finds issues
    mockReviewer := &MockReviewer{
        ReviewResult: &provider.ReviewResult{
            Passed: false,
            Issues: []provider.ReviewIssue{
                {File: "test.go", Line: 10, Severity: "warning", Message: "test issue"},
            },
        },
    }

    worker := &Worker{
        unit:     &discovery.Unit{ID: "test-unit"},
        reviewer: mockReviewer,
        config: WorkerConfig{
            TargetBranch: "main",
            NoPR:         true,
        },
        // ... other required fields with mocks
    }

    // Verify merge proceeds despite issues found (advisory)
}

// MockReviewer implements provider.Reviewer for testing
type MockReviewer struct {
    ReviewResult *provider.ReviewResult
    ReviewErr    error
    ReviewCalled bool
}

func (m *MockReviewer) Review(ctx context.Context, workdir, baseBranch string) (*provider.ReviewResult, error) {
    m.ReviewCalled = true
    if m.ReviewErr != nil {
        return nil, m.ReviewErr
    }
    return m.ReviewResult, nil
}

func (m *MockReviewer) Name() provider.ProviderType {
    return "mock"
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run TestRunCodeReview -v
go test ./internal/worker/... -run TestMergeToFeatureBranch -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestMergeToFeatureBranch_WithReview` | Review is called and merge proceeds on pass |
| `TestMergeToFeatureBranch_ReviewDisabled` | Merge proceeds when reviewer is nil |
| `TestMergeToFeatureBranch_ReviewFailsButMergeProceeds` | Merge proceeds despite review error (advisory) |
| `TestMergeToFeatureBranch_ReviewIssuesButMergeProceeds` | Merge proceeds despite issues found (advisory) |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- The `runCodeReview()` method is provided by the `review-worker` unit - this task only wires it into the merge flow
- Review is **advisory only** - the merge MUST proceed regardless of review outcome (pass, fail, error, or issues found)
- When `w.reviewer` is nil, `runCodeReview()` should return immediately (no-op)
- Events are emitted by `runCodeReview()` for observability (codereview.started, codereview.passed, etc.)
- The fix attempt (if issues found) uses the Worker's task provider, not the reviewer

## NOT In Scope

- The `runCodeReview()` implementation itself (provided by review-worker unit)
- Reviewer implementations (provided by codex-reviewer, claude-reviewer units)
- Reviewer resolution (task #1)
- Event type definitions (provided by review-config unit)
- Prompt builders for fixes (provided by review-worker unit)
