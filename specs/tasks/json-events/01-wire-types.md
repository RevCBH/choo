---
task: 1
status: pending
backpressure: "go build ./internal/events/..."
depends_on: []
---

# Wire Format Types

**Parent spec**: `/specs/JSON-EVENTS.md`
**Task**: #1 of 4 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Define the JSONEvent struct for wire format serialization with proper JSON tags matching the PRD format.

## Dependencies

### External Specs (must be implemented)
- `/specs/completed/EVENTS.md` - Existing Event type from internal/events

### Task Dependencies (within this unit)
- None (this is the foundation task)

### Package Dependencies
- Standard library only (`time`)

## Deliverables

### Files to Create/Modify

```
internal/
└── events/
    └── json.go    # CREATE: JSONEvent wire format type
```

### Types to Implement

```go
// JSONEvent is the wire format for serialized events over container stdout.
// This matches the PRD-defined format with "timestamp" field.
type JSONEvent struct {
    // Type identifies the event (e.g., "unit.started", "task.completed")
    Type string `json:"type"`

    // Timestamp is when the event occurred (RFC3339 format)
    Timestamp time.Time `json:"timestamp"`

    // Unit is the unit ID this event relates to (omitted for orchestrator events)
    Unit string `json:"unit,omitempty"`

    // Task is the task number within the unit (nil if not task-related)
    Task *int `json:"task,omitempty"`

    // PR is the pull request number (nil if not PR-related)
    PR *int `json:"pr,omitempty"`

    // Payload contains event-specific data (type varies by event)
    Payload map[string]interface{} `json:"payload,omitempty"`

    // Error contains error message if this is a failure event
    Error string `json:"error,omitempty"`
}
```

### Functions to Implement

```go
// ToJSONEvent converts an internal Event to the wire format JSONEvent.
// This is used by JSONEmitter when serializing events.
func ToJSONEvent(e Event) JSONEvent

// ToEvent converts a wire format JSONEvent back to an internal Event.
// This is used by JSONLineReader when parsing events.
func (je JSONEvent) ToEvent() Event
```

### Wire Format Examples

Unit started event:
```json
{"type":"unit.started","timestamp":"2024-01-15T10:30:00Z","unit":"web-api"}
```

Task completed event with payload:
```json
{"type":"task.completed","timestamp":"2024-01-15T10:33:00Z","unit":"web-api","task":1,"payload":{"commit":"abc123"}}
```

Unit failed event with error:
```json
{"type":"unit.failed","timestamp":"2024-01-15T10:30:00Z","unit":"web-api","error":"validation failed: tests not passing"}
```

Orchestrator event (no unit):
```json
{"type":"orch.completed","timestamp":"2024-01-15T10:40:00Z"}
```

## Backpressure

### Validation Command

```bash
go build ./internal/events/...
```

### Must Pass

| Check | Assertion |
|-------|-----------|
| Build succeeds | Package compiles without errors |
| JSON tags correct | JSONEvent fields have proper json tags with omitempty |
| Type field required | Type and Timestamp have no omitempty (always present) |
| Optional fields | Unit, Task, PR, Payload, Error have omitempty |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### Type Conversion

ToJSONEvent must handle:
- Copy Type as string (EventType -> string)
- Copy Time as Timestamp
- Copy Unit directly
- Copy Task and PR pointers
- Convert Payload from `any` to `map[string]interface{}` (may need type assertion)
- Copy Error directly

```go
func ToJSONEvent(e Event) JSONEvent {
    je := JSONEvent{
        Type:      string(e.Type),
        Timestamp: e.Time,
        Unit:      e.Unit,
        Task:      e.Task,
        PR:        e.PR,
        Error:     e.Error,
    }

    // Handle payload conversion
    if e.Payload != nil {
        switch p := e.Payload.(type) {
        case map[string]interface{}:
            je.Payload = p
        case map[string]any:
            je.Payload = p
        default:
            // For other types, wrap in a single-key map
            je.Payload = map[string]interface{}{"value": e.Payload}
        }
    }

    return je
}
```

### ToEvent Conversion

ToEvent must handle:
- Convert Type string back to EventType
- Copy Timestamp as Time
- Copy all other fields directly

```go
func (je JSONEvent) ToEvent() Event {
    var payload any
    if je.Payload != nil {
        payload = je.Payload
    }

    return Event{
        Type:    EventType(je.Type),
        Time:    je.Timestamp,
        Unit:    je.Unit,
        Task:    je.Task,
        PR:      je.PR,
        Payload: payload,
        Error:   je.Error,
    }
}
```

### Timestamp Format

Go's encoding/json marshals time.Time as RFC3339 by default, which matches the PRD format:
```
2024-01-15T10:30:00Z
```

No custom marshaler is needed.

## NOT In Scope

- JSON emitter implementation (Task #2)
- JSON reader implementation (Task #3)
- TTY detection (Task #4)
- Event Bus integration (Task #4)
