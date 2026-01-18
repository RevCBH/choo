---
task: 1
status: pending
backpressure: "go test ./internal/events/..."
depends_on: []
---

# Event Types

**Parent spec**: `/specs/EVENTS.md`
**Task**: #1 of 3 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Define the Event struct, EventType constants, and builder methods for creating events.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None (this is the foundation task)

### Package Dependencies
- Standard library only (`time`, `fmt`)

## Deliverables

### Files to Create/Modify

```
internal/
└── events/
    └── types.go    # CREATE: Event struct and EventType constants
```

### Types to Implement

```go
// Event represents a single occurrence in the orchestrator lifecycle
type Event struct {
    // Time is when the event occurred (set by bus on emit)
    Time time.Time `json:"time"`

    // Type identifies what happened
    Type EventType `json:"type"`

    // Unit is the unit ID this event relates to (empty for orchestrator events)
    Unit string `json:"unit,omitempty"`

    // Task is the task number within the unit (nil if not task-related)
    Task *int `json:"task,omitempty"`

    // PR is the pull request number (nil if not PR-related)
    PR *int `json:"pr,omitempty"`

    // Payload contains event-specific data (type varies by event)
    Payload any `json:"payload,omitempty"`

    // Error contains error message if this is a failure event
    Error string `json:"error,omitempty"`
}

// EventType is a string constant identifying the event category
type EventType string
```

### Event Type Constants

```go
// Orchestrator lifecycle events
const (
    OrchStarted   EventType = "orch.started"
    OrchCompleted EventType = "orch.completed"
    OrchFailed    EventType = "orch.failed"
)

// Unit lifecycle events
const (
    UnitQueued    EventType = "unit.queued"
    UnitStarted   EventType = "unit.started"
    UnitCompleted EventType = "unit.completed"
    UnitFailed    EventType = "unit.failed"
)

// Task lifecycle events
const (
    TaskStarted        EventType = "task.started"
    TaskClaudeInvoke   EventType = "task.claude.invoke"
    TaskClaudeDone     EventType = "task.claude.done"
    TaskBackpressure   EventType = "task.backpressure"
    TaskValidationOK   EventType = "task.validation.ok"
    TaskValidationFail EventType = "task.validation.fail"
    TaskCommitted      EventType = "task.committed"
    TaskCompleted      EventType = "task.completed"
    TaskRetry          EventType = "task.retry"
    TaskFailed         EventType = "task.failed"
)

// PR lifecycle events
const (
    PRCreated           EventType = "pr.created"
    PRReviewPending     EventType = "pr.review.pending"
    PRReviewInProgress  EventType = "pr.review.in_progress"
    PRReviewApproved    EventType = "pr.review.approved"
    PRFeedbackReceived  EventType = "pr.feedback.received"
    PRFeedbackAddressed EventType = "pr.feedback.addressed"
    PRMergeQueued       EventType = "pr.merge.queued"
    PRConflict          EventType = "pr.conflict"
    PRMerged            EventType = "pr.merged"
    PRFailed            EventType = "pr.failed"
)

// Git operation events
const (
    WorktreeCreated EventType = "worktree.created"
    WorktreeRemoved EventType = "worktree.removed"
    BranchPushed    EventType = "branch.pushed"
)
```

### Functions to Implement

```go
// NewEvent creates an event with the given type and unit
func NewEvent(eventType EventType, unit string) Event

// WithTask returns a copy of the event with the task number set
func (e Event) WithTask(task int) Event

// WithPR returns a copy of the event with the PR number set
func (e Event) WithPR(pr int) Event

// WithPayload returns a copy of the event with the payload set
func (e Event) WithPayload(payload any) Event

// WithError returns a copy of the event with the error message set
func (e Event) WithError(err error) Event

// IsFailure returns true if this is a failure event type
func (e Event) IsFailure() bool

// String returns a human-readable representation of the event
func (e Event) String() string
```

## Backpressure

### Validation Command

```bash
go test ./internal/events/... -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestNewEvent` | Creates event with correct Type and Unit |
| `TestEvent_WithTask` | Returns new event with Task pointer set |
| `TestEvent_WithPR` | Returns new event with PR pointer set |
| `TestEvent_WithPayload` | Returns new event with Payload set |
| `TestEvent_WithError` | Returns new event with Error string from err.Error() |
| `TestEvent_IsFailure` | Returns true for OrchFailed, UnitFailed, TaskFailed, PRFailed |
| `TestEvent_IsFailure_Success` | Returns false for completed/success events |
| `TestEvent_String` | Returns formatted string like "[task.completed] app-shell task=#1" |

### Test Fixtures

No external fixtures required.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Builder methods return copies (events are immutable once created)
- WithError should handle nil error gracefully (empty string)
- IsFailure checks for suffix ".failed" or exact match of failure types
- String format should match log output: `[event.type] unit task=#N pr=#M`

## NOT In Scope

- Event bus implementation (Task #2)
- Handler implementations (Task #3)
- Time field population (done by Bus.Emit)
