---
task: 4
status: complete
backpressure: "go build ./internal/review/..."
depends_on: [1]
---

# Review Event Types

**Parent spec**: `/specs/SPEC-REVIEW.md`
**Task**: #4 of 6 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Define event type constants and payload structs for spec review workflow state transitions.

## Dependencies

### External Specs (must be implemented)
- EVENTS - provides EventType pattern and event bus

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: ReviewFeedback, IterationHistory types)

### Package Dependencies
- `internal/events` (EventType)

## Deliverables

### Files to Create/Modify

```
internal/
└── review/
    └── events.go    # CREATE: Event type constants and payloads
```

### Event Type Constants

```go
import "choo/internal/events"

// Review event types
const (
    SpecReviewStarted   events.EventType = "spec.review.started"
    SpecReviewFeedback  events.EventType = "spec.review.feedback"
    SpecReviewPassed    events.EventType = "spec.review.passed"
    SpecReviewBlocked   events.EventType = "spec.review.blocked"
    SpecReviewIteration events.EventType = "spec.review.iteration"
    SpecReviewMalformed events.EventType = "spec.review.malformed"
)
```

### Payload Types to Implement

```go
// ReviewStartedPayload contains data for review started events
type ReviewStartedPayload struct {
    Feature   string `json:"feature"`
    PRDPath   string `json:"prd_path"`
    SpecsPath string `json:"specs_path"`
}

// ReviewFeedbackPayload contains data for feedback events
type ReviewFeedbackPayload struct {
    Feature   string           `json:"feature"`
    Iteration int              `json:"iteration"`
    Feedback  []ReviewFeedback `json:"feedback"`
    Scores    map[string]int   `json:"scores"`
}

// ReviewBlockedPayload contains data for blocked events
type ReviewBlockedPayload struct {
    Feature      string             `json:"feature"`
    Reason       string             `json:"reason"`
    Iterations   []IterationHistory `json:"iterations"`
    Recovery     []string           `json:"recovery_actions"`
    CurrentSpecs string             `json:"current_specs_path"`
}

// ReviewMalformedPayload contains data for malformed output events
type ReviewMalformedPayload struct {
    Feature     string `json:"feature"`
    RawOutput   string `json:"raw_output"`
    ParseError  string `json:"parse_error"`
    RetryNumber int    `json:"retry_number"`
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/review/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Compilation | Package compiles with events import |
| Type compatibility | Payload types can be assigned to `any` for event payload |

### Test Fixtures

No external fixtures required.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Event types follow the `spec.review.*` naming pattern
- Payloads use JSON tags for serialization consistency
- ReviewBlockedPayload.Recovery contains actionable recovery instructions
- ReviewMalformedPayload.RawOutput preserved for debugging
- All payloads are exported for use by handlers

## NOT In Scope

- Event bus implementation (from EVENTS spec)
- Event emission logic (Task #6)
- Handler implementations
- State persistence
