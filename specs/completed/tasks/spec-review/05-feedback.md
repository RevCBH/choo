---
task: 5
status: complete
backpressure: "go test ./internal/review/... -run Feedback"
depends_on: [1, 4]
---

# Feedback Application

**Parent spec**: `/specs/SPEC-REVIEW.md`
**Task**: #5 of 6 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement the FeedbackApplier that applies review feedback to specs using the Task tool interface.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: ReviewFeedback type)
- Task #4 must be complete (provides: event types for context)

### Package Dependencies
- Standard library (`context`, `encoding/json`, `fmt`)

## Deliverables

### Files to Create/Modify

```
internal/
└── review/
    └── feedback.go    # CREATE: Feedback application logic
```

### Types to Implement

```go
// TaskInvoker abstracts Task tool invocation for testing
type TaskInvoker interface {
    InvokeTask(ctx context.Context, prompt string, subagentType string) (string, error)
}

// FeedbackApplier applies review feedback to specs
type FeedbackApplier struct {
    taskTool TaskInvoker
}
```

### Functions to Implement

```go
// NewFeedbackApplier creates a new FeedbackApplier
func NewFeedbackApplier(taskTool TaskInvoker) *FeedbackApplier

// ApplyFeedback applies the given feedback to specs at the path
func (f *FeedbackApplier) ApplyFeedback(ctx context.Context, specsPath string, feedback []ReviewFeedback) error
```

### Prompt Template

The ApplyFeedback method constructs a prompt for the Task tool:

```go
prompt := fmt.Sprintf(`Apply the following review feedback to the specs.

Specs directory: %s

Feedback to apply:
%s

For each feedback item:
1. Locate the specified section in the specs
2. Address the issue according to the suggestion
3. Maintain consistency with the rest of the spec

Make the minimal changes necessary to address each issue.`, specsPath, feedbackJSON)
```

## Backpressure

### Validation Command

```bash
go test ./internal/review/... -run Feedback -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestNewFeedbackApplier` | Returns non-nil applier with task tool set |
| `TestApplyFeedback_EmptyFeedback` | Returns nil error when feedback slice is empty |
| `TestApplyFeedback_CallsTaskTool` | Invokes task tool with correct prompt and "general-purpose" subagent |
| `TestApplyFeedback_PromptContainsPath` | Generated prompt contains specs path |
| `TestApplyFeedback_PromptContainsFeedback` | Generated prompt contains JSON-serialized feedback |
| `TestApplyFeedback_TaskToolError` | Propagates error from task tool |

### Test Fixtures

No external fixtures required. Use mock TaskInvoker in tests.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- FeedbackApplier uses TaskInvoker interface for testability
- Empty feedback array is a no-op (returns nil immediately)
- Feedback is serialized to JSON for inclusion in prompt
- subagentType is always "general-purpose" per spec requirement
- Error from taskTool.InvokeTask is wrapped with context

## NOT In Scope

- Schema validation (Task #2)
- Criteria evaluation (Task #3)
- Review loop orchestration (Task #6)
- Actual Task tool implementation (external dependency)
