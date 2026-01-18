---
task: 3
status: complete
backpressure: "go test ./internal/events/..."
depends_on: [1, 2]
---

# Built-in Handlers

**Parent spec**: `/specs/EVENTS.md`
**Task**: #3 of 3 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement LogHandler for structured logging output and StateHandler for frontmatter persistence.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY spec (provides: `Unit`, status constants) - if not available, use interface/mock

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `Event`, `EventType`, all event constants)
- Task #2 must be complete (provides: `Handler` type, `Bus`)

### Package Dependencies
- Standard library (`io`, `os`, `fmt`, `time`)

## Deliverables

### Files to Create/Modify

```
internal/
└── events/
    └── handlers.go    # CREATE: LogHandler and StateHandler
```

### Types to Implement

```go
// LogConfig configures the logging handler
type LogConfig struct {
    // Writer is where logs are written (default: os.Stderr)
    Writer io.Writer

    // IncludePayload includes event payload in log output
    IncludePayload bool

    // TimeFormat is the timestamp format (default: RFC3339)
    TimeFormat string
}

// StateConfig configures the state persistence handler
type StateConfig struct {
    // Units is the map of unit ID to Unit pointer for state updates
    Units map[string]*Unit

    // OnError is called when state persistence fails
    OnError func(error)
}

// Unit interface for state updates (matches discovery.Unit)
// Define locally to avoid circular imports
type Unit interface {
    SetStatus(status string)
    SetPRNumber(pr int)
    Persist() error
}
```

### Functions to Implement

```go
// LogHandler returns a handler that logs events to the configured writer
// Format: [event.type] unit task=#N pr=#M
func LogHandler(cfg LogConfig) Handler

// StateHandler returns a handler that persists unit state changes to frontmatter
// Maps events to unit/task status updates
func StateHandler(cfg StateConfig) Handler
```

## Backpressure

### Validation Command

```bash
go test ./internal/events/... -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestLogHandler_Format` | Output contains `[task.completed]` and `app-shell` and `task=#1` |
| `TestLogHandler_DefaultWriter` | Uses os.Stderr when Writer is nil |
| `TestLogHandler_IncludePayload` | Payload appears in output when IncludePayload is true |
| `TestLogHandler_TimeFormat` | Uses custom TimeFormat when provided |
| `TestLogHandler_OrchEvent` | Handles events without Unit (orch.started) |
| `TestStateHandler_UnitStarted` | Sets unit status to InProgress |
| `TestStateHandler_UnitCompleted` | Sets unit status to Complete |
| `TestStateHandler_UnitFailed` | Sets unit status to Failed |
| `TestStateHandler_PRCreated` | Sets unit PRNumber and status to PROpen |
| `TestStateHandler_UnknownUnit` | Ignores events for unknown units without error |
| `TestStateHandler_OnError` | Calls OnError callback when Persist fails |

### Test Fixtures

No external fixtures required. Use mock Unit implementation for StateHandler tests.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### LogHandler Output Format

The log handler produces structured log output matching this format:

```
[orch.started]
[unit.started] app-shell
[task.started] app-shell task=#1
[task.claude.invoke] app-shell task=#1
[task.claude.done] app-shell task=#1
[task.validation.ok] app-shell task=#1
[task.committed] app-shell task=#1
[task.completed] app-shell task=#1
[pr.created] app-shell pr=#42
[pr.review.approved] app-shell pr=#42
[pr.merged] app-shell pr=#42
[unit.completed] app-shell
```

Implementation:

```go
func LogHandler(cfg LogConfig) Handler {
    if cfg.Writer == nil {
        cfg.Writer = os.Stderr
    }
    if cfg.TimeFormat == "" {
        cfg.TimeFormat = time.RFC3339
    }

    return func(e Event) {
        var buf strings.Builder
        buf.WriteString("[")
        buf.WriteString(string(e.Type))
        buf.WriteString("]")

        if e.Unit != "" {
            buf.WriteString(" ")
            buf.WriteString(e.Unit)
        }
        if e.Task != nil {
            fmt.Fprintf(&buf, " task=#%d", *e.Task)
        }
        if e.PR != nil {
            fmt.Fprintf(&buf, " pr=#%d", *e.PR)
        }
        if cfg.IncludePayload && e.Payload != nil {
            fmt.Fprintf(&buf, " payload=%v", e.Payload)
        }
        buf.WriteString("\n")

        fmt.Fprint(cfg.Writer, buf.String())
    }
}
```

### StateHandler State Mapping

The state handler maps events to unit/task status updates:

| Event | State Change |
|-------|--------------|
| UnitStarted | unit.Status = InProgress |
| UnitCompleted | unit.Status = Complete |
| UnitFailed | unit.Status = Failed |
| TaskStarted | task.Status = InProgress |
| TaskCompleted | task.Status = Complete |
| TaskFailed | task.Status = Failed |
| PRCreated | unit.Status = PROpen, unit.PRNumber = pr |
| PRReviewInProgress | unit.Status = InReview |
| PRMergeQueued | unit.Status = Merging |

### Mock Unit for Testing

Create a mock Unit implementation for testing StateHandler:

```go
type mockUnit struct {
    status   string
    prNumber int
    persistErr error
    persistCalled bool
}

func (m *mockUnit) SetStatus(s string) { m.status = s }
func (m *mockUnit) SetPRNumber(pr int) { m.prNumber = pr }
func (m *mockUnit) Persist() error {
    m.persistCalled = true
    return m.persistErr
}
```

### Circular Import Avoidance

If `internal/discovery` imports `internal/events`, define a Unit interface locally in the events package rather than importing discovery.Unit.

## NOT In Scope

- Event type definitions (Task #1)
- Bus implementation (Task #2)
- Actual frontmatter parsing/writing (use Unit interface)
- Task status tracking (unit-level only for MVP)
