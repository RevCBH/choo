---
task: 5
status: complete
backpressure: "go test ./internal/feature/... -run TestReview"
depends_on: [1]
---

# Review Cycle Management

**Parent spec**: `/specs/FEATURE-WORKFLOW.md`
**Task**: #5 of 6 in implementation plan

## Objective

Implement the review cycle loop that manages spec review iterations, blocking, and resume.

## Dependencies

### External Specs (must be implemented)
- SPEC-REVIEW - provides `review.Reviewer` for spec review

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `FeatureStatus`, `CanTransition`)

### Package Dependencies
- `internal/review` - spec reviewer

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── review.go    # CREATE: Review cycle management
```

### Types to Implement

```go
// ReviewCycle manages the review iteration loop
type ReviewCycle struct {
    reviewer      *review.Reviewer
    maxIterations int
    transitionFn  func(FeatureStatus) error
    escalateFn    func(string, error) error
}

// ReviewCycleConfig configures the review cycle
type ReviewCycleConfig struct {
    MaxIterations int // default 3
}

// ResumeOptions configures how to resume from blocked state
type ResumeOptions struct {
    SkipReview bool   // Skip directly to validation
    Message    string // User-provided context for resume
}
```

### Functions to Implement

```go
// NewReviewCycle creates a review cycle manager
func NewReviewCycle(reviewer *review.Reviewer, cfg ReviewCycleConfig) *ReviewCycle

// Run executes the review cycle until pass, blocked, or error
func (rc *ReviewCycle) Run(ctx context.Context, specs []Spec) error

// Resume continues from blocked state
func (rc *ReviewCycle) Resume(ctx context.Context, opts ResumeOptions) error

// isMalformedOutput checks if error indicates malformed review output
func isMalformedOutput(err error) bool
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestReview
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestReviewCycle_PassFirstIteration` | Transitions to `validating_specs` on pass |
| `TestReviewCycle_PassAfterRevisions` | Multiple iterations, then pass |
| `TestReviewCycle_MaxIterationsBlocked` | Transitions to `review_blocked` at max |
| `TestReviewCycle_MalformedOutputBlocked` | Transitions to `review_blocked` on malformed |
| `TestReviewCycle_StateTransitions` | Correct transitions: reviewing -> updating -> reviewing |
| `TestReviewCycle_ResumeFromBlocked` | Resume transitions back to `reviewing_specs` |
| `TestReviewCycle_ResumeSkipReview` | Resume with skip goes to `validating_specs` |
| `TestReviewCycle_EscalationOnBlock` | Escalate called when blocked |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| `review_pass.json` | `internal/feature/testdata/` | Review pass response |
| `review_needs_revision.json` | `internal/feature/testdata/` | Review needs revision response |
| `review_malformed.json` | `internal/feature/testdata/` | Malformed review response |

### CI Compatibility

- [x] No external API keys required (mock reviewer)
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Review cycle tracks iterations and handles blocking
- Max iterations reached or malformed output triggers `review_blocked`
- Blocked state is recoverable (unlike `failed`)
- Escalation includes context for user intervention

```go
func (w *Workflow) runReviewCycle(ctx context.Context) error {
    for iteration := 0; iteration < w.maxReviewIterations; iteration++ {
        result, err := w.reviewer.Review(ctx, w.prd.Specs)
        if err != nil {
            if isMalformedOutput(err) {
                w.transitionTo(StatusReviewBlocked)
                return w.escalate("Review produced malformed output", err)
            }
            return err
        }

        switch result.Verdict {
        case review.Pass:
            return w.transitionTo(StatusValidatingSpecs)
        case review.NeedsRevision:
            w.transitionTo(StatusUpdatingSpecs)
            if err := w.updateSpecs(ctx, result.Feedback); err != nil {
                return err
            }
            w.transitionTo(StatusReviewingSpecs)
        }
    }

    // Max iterations reached
    w.transitionTo(StatusReviewBlocked)
    return w.escalate("Max review iterations reached", nil)
}
```

## NOT In Scope

- State definitions (Task #1)
- Commit operations (Task #2)
- Drift detection (Task #3)
- Completion logic (Task #4)
- Workflow orchestration (Task #6)
