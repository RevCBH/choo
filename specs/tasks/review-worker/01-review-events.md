---
task: 1
status: complete
backpressure: "go build ./internal/events/..."
depends_on: []
---

# Code Review Event Types

**Parent spec**: `specs/REVIEW-WORKER.md`
**Task**: #1 of 4 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Add event types to the events package for the code review lifecycle, enabling observability of review start, pass, issues found, fixes applied, and failures.

## Dependencies

### External Specs (must be implemented)
- None - events package already exists

### Task Dependencies (within this unit)
- None (this is the foundation task)

### Package Dependencies
- None - modifying existing package

## Deliverables

### Files to Create/Modify

```
internal/events/
└── types.go    # MODIFY: Add code review event types
```

### Types to Implement

Add the following constants to `internal/events/types.go` in a new "Code review events" section:

```go
// Code review events (advisory, never block merge)
const (
    // CodeReviewStarted is emitted when code review begins for a unit
    CodeReviewStarted EventType = "codereview.started"

    // CodeReviewPassed is emitted when review finds no issues
    CodeReviewPassed EventType = "codereview.passed"

    // CodeReviewIssuesFound is emitted when review discovers issues
    CodeReviewIssuesFound EventType = "codereview.issues_found"

    // CodeReviewFixAttempt is emitted when a fix iteration begins
    CodeReviewFixAttempt EventType = "codereview.fix_attempt"

    // CodeReviewFixApplied is emitted when fix changes are successfully committed
    CodeReviewFixApplied EventType = "codereview.fix_applied"

    // CodeReviewFailed is emitted when review fails to run (non-blocking)
    CodeReviewFailed EventType = "codereview.failed"
)
```

### Event Payload Shapes

Document the expected payload structure for each event in comments:

| Event | Payload Fields |
|-------|----------------|
| `codereview.started` | `unit: string` |
| `codereview.passed` | `summary: string` |
| `codereview.issues_found` | `count: int`, `issues: []ReviewIssue` |
| `codereview.fix_attempt` | `iteration: int`, `max_iterations: int` |
| `codereview.fix_applied` | `iteration: int` |
| `codereview.failed` | `error: string` |

## Backpressure

### Validation Command

```bash
go build ./internal/events/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | `go build ./internal/events/...` exits 0 |
| Constants defined | All 6 event types are exported constants |
| No duplicate values | Each EventType has a unique string value |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Place the new constants in a clearly labeled section after "Feature lifecycle events" and before any existing sections
- Use lowercase dot-separated format consistent with existing events (e.g., `unit.started`, `task.completed`)
- The `codereview.` prefix groups all review events for easy filtering
- These events are informational only - they document what happened but don't change control flow

### Placement in types.go

Add after the Feature lifecycle events section (around line 103) and before PRD events:

```go
// Feature lifecycle events
const (
    ...
)

// Code review events (advisory, never block merge)
const (
    CodeReviewStarted     EventType = "codereview.started"
    CodeReviewPassed      EventType = "codereview.passed"
    CodeReviewIssuesFound EventType = "codereview.issues_found"
    CodeReviewFixAttempt  EventType = "codereview.fix_attempt"
    CodeReviewFixApplied  EventType = "codereview.fix_applied"
    CodeReviewFailed      EventType = "codereview.failed"
)

// PRD events
const (
    ...
)
```

## NOT In Scope

- Event emission logic (Task #2 - Review Orchestration)
- Event handlers or subscribers (handled by existing event bus infrastructure)
- TUI rendering of code review events (separate concern)
