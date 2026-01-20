---
task: 6
status: pending
backpressure: "go test ./internal/review/... -run ReviewLoop"
depends_on: [1, 2, 3, 4, 5]
---

# Review Loop Orchestration

**Parent spec**: `/specs/SPEC-REVIEW.md`
**Task**: #6 of 6 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement the Reviewer struct and RunReviewLoop method that orchestrates the full spec review process with retry handling, iteration tracking, and blocked state transitions.

## Dependencies

### External Specs (must be implemented)
- EVENTS - provides event bus Publisher interface

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: ReviewConfig, ReviewResult, ReviewSession, IterationHistory)
- Task #2 must be complete (provides: ParseAndValidate)
- Task #3 must be complete (provides: DefaultCriteria)
- Task #4 must be complete (provides: event types and payloads)
- Task #5 must be complete (provides: FeedbackApplier, TaskInvoker)

### Package Dependencies
- Standard library (`context`, `fmt`, `path/filepath`, `time`)
- `internal/events` (Publisher interface)

## Deliverables

### Files to Create/Modify

```
internal/
└── review/
    └── review.go    # CREATE: Reviewer struct and review loop
```

### Types to Implement

```go
// Reviewer orchestrates the spec review loop
type Reviewer struct {
    config    ReviewConfig
    publisher events.Publisher
    taskTool  TaskInvoker
}
```

### Functions to Implement

```go
// NewReviewer creates a new Reviewer with the given configuration
func NewReviewer(config ReviewConfig, publisher events.Publisher, taskTool TaskInvoker) *Reviewer

// RunReviewLoop executes the full review loop for a feature
func (r *Reviewer) RunReviewLoop(ctx context.Context, feature, prdPath, specsPath string) (*ReviewSession, error)

// ReviewSpecs performs a single review invocation
func (r *Reviewer) ReviewSpecs(ctx context.Context, prdPath, specsPath string) (*ReviewResult, error)

// reviewWithRetry attempts review with configured retries on malformed output
func (r *Reviewer) reviewWithRetry(ctx context.Context, prdPath, specsPath string) (*ReviewResult, error)

// invokeReviewer calls the Task tool with the review prompt
func (r *Reviewer) invokeReviewer(ctx context.Context, prdPath, specsPath string) (string, error)

// applyFeedback applies feedback using FeedbackApplier
func (r *Reviewer) applyFeedback(ctx context.Context, specsPath string, feedback []ReviewFeedback) error

// publishBlocked emits a SpecReviewBlocked event with recovery info
func (r *Reviewer) publishBlocked(session *ReviewSession, reason string)
```

### Review Loop Logic

1. Emit `SpecReviewStarted` event
2. For each iteration up to MaxIterations:
   a. Call `reviewWithRetry` to get result
   b. On malformed output after retries: set blocked state, emit `SpecReviewBlocked`, return
   c. Record iteration in session.Iterations
   d. Emit `SpecReviewIteration` event
   e. If verdict == "pass": set final verdict, emit `SpecReviewPassed`, return
   f. If verdict == "needs_revision" and not last iteration:
      - Emit `SpecReviewFeedback` event
      - Call `applyFeedback`
      - On feedback error: set blocked state, emit `SpecReviewBlocked`, return
3. Max iterations exhausted: set blocked state, emit `SpecReviewBlocked`, return

### Reviewer Prompt Template

```go
prompt := fmt.Sprintf(`Review the generated specs for quality and completeness.

PRD: %s
Specs: %s

Review criteria:
1. COMPLETENESS: All PRD requirements have corresponding spec sections
2. CONSISTENCY: Types, interfaces, and naming are consistent throughout
3. TESTABILITY: Backpressure commands are specific and executable
4. ARCHITECTURE: Follows existing patterns in codebase

Output format (MUST be valid JSON):
{
  "verdict": "pass" | "needs_revision",
  "score": { "completeness": 0-100, "consistency": 0-100, "testability": 0-100, "architecture": 0-100 },
  "feedback": [
    { "section": "...", "issue": "...", "suggestion": "..." }
  ]
}`, prdPath, specsPath)
```

## Backpressure

### Validation Command

```bash
go test ./internal/review/... -run ReviewLoop -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestReviewer_RunReviewLoop_PassOnFirstIteration` | Returns session with FinalVerdict="pass", single iteration |
| `TestReviewer_RunReviewLoop_PassAfterRevision` | Returns pass after needs_revision then pass |
| `TestReviewer_RunReviewLoop_BlockedAfterMaxIterations` | Returns blocked after exhausting iterations |
| `TestReviewer_RunReviewLoop_BlockedOnMalformedOutput` | Returns blocked when output fails schema validation after retry |
| `TestReviewer_RetryOnMalformedThenSuccess` | Succeeds on retry after first malformed output |
| `TestReviewer_ReviewSpecs_ValidOutput` | Parses and returns valid ReviewResult |
| `TestReviewer_ReviewSpecs_MalformedOutput` | Returns error with RawOutput preserved |
| `TestReviewer_EmitsCorrectEvents_Pass` | Emits Started, Iteration, Passed events in order |
| `TestReviewer_EmitsCorrectEvents_Blocked` | Emits Started, Iteration(s), Blocked events |
| `TestReviewer_BlockedPayload_ContainsRecovery` | Blocked event payload includes recovery actions |

### Test Fixtures

No external fixtures required. Use mock TaskInvoker and Publisher in tests.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Publisher interface requires Publish(eventType, payload) method
- Brief delay (1 second) between malformed output retries
- Recovery actions include "choo feature resume <feature>" command
- Iteration history preserved for debugging blocked states
- Event payload types from Task #4 used for type safety

## NOT In Scope

- Actual Task tool implementation (external dependency)
- Event bus implementation (from EVENTS spec)
- CLI integration
- Persistence of session state
