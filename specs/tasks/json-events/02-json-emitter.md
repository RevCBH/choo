---
task: 2
status: pending
backpressure: "go test ./internal/events/... -run TestJSONEmitter"
depends_on: [1]
---

# JSON Emitter

**Parent spec**: `/specs/JSON-EVENTS.md`
**Task**: #2 of 4 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement the JSONEmitter struct for thread-safe JSON line writing to an io.Writer, plus the JSONEmitterHandler for event bus subscription.

## Dependencies

### External Specs (must be implemented)
- `/specs/completed/EVENTS.md` - Existing Event type and Handler type

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `JSONEvent`, `ToJSONEvent`)

### Package Dependencies
- Standard library only (`encoding/json`, `io`, `sync`)

## Deliverables

### Files to Create/Modify

```
internal/
└── events/
    └── json.go    # MODIFY: Add JSONEmitter and JSONEmitterHandler
```

### Types to Implement

```go
// JSONEmitter writes events as JSON lines to a writer.
// Thread-safe for concurrent Emit calls.
type JSONEmitter struct {
    w   io.Writer
    mu  sync.Mutex
    enc *json.Encoder
}
```

### Functions to Implement

```go
// NewJSONEmitter creates a new JSON emitter that writes to w.
// Each event is written as a single JSON line (newline-delimited).
func NewJSONEmitter(w io.Writer) *JSONEmitter

// Emit converts the internal Event to JSONEvent wire format and writes it.
// Thread-safe: uses mutex to prevent interleaved writes.
// Returns an error if JSON encoding fails or the write fails.
func (e *JSONEmitter) Emit(event Event) error

// JSONEmitterHandler returns a Handler that emits events as JSON lines.
// Use this to subscribe the emitter to an event bus.
// Errors are logged but not propagated (handler interface has no return).
func JSONEmitterHandler(emitter *JSONEmitter) Handler
```

## Backpressure

### Validation Command

```bash
go test ./internal/events/... -run TestJSONEmitter -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestJSONEmitter_Emit` | Writes valid JSON line with newline terminator |
| `TestJSONEmitter_EmitFormat` | Output matches PRD format with correct field names |
| `TestJSONEmitter_ConcurrentWrites` | 100 concurrent writes produce 100 valid JSON lines |
| `TestJSONEmitter_NoInterleaving` | Concurrent writes do not produce garbled output |
| `TestJSONEmitterHandler` | Handler wraps emitter and calls Emit |
| `TestJSONEmitterHandler_Error` | Handler logs error but does not panic |

### Test Fixtures

No external fixtures required.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### Thread Safety

The mutex ensures JSON lines are not interleaved when multiple goroutines emit concurrently:

```go
func (e *JSONEmitter) Emit(event Event) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    je := ToJSONEvent(event)
    return e.enc.Encode(je)
}
```

The json.Encoder automatically appends a newline after each Encode call.

### Constructor

```go
func NewJSONEmitter(w io.Writer) *JSONEmitter {
    return &JSONEmitter{
        w:   w,
        enc: json.NewEncoder(w),
    }
}
```

### Handler Implementation

The handler wraps the emitter for event bus subscription. Since Handler has no error return, errors are logged:

```go
func JSONEmitterHandler(emitter *JSONEmitter) Handler {
    return func(e Event) {
        if err := emitter.Emit(e); err != nil {
            log.Printf("WARN: failed to emit JSON event: %v", err)
        }
    }
}
```

### Example Test

```go
func TestJSONEmitter_Emit(t *testing.T) {
    var buf bytes.Buffer
    emitter := NewJSONEmitter(&buf)

    event := NewEvent(UnitStarted, "web-api")
    event.Time = time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

    err := emitter.Emit(event)
    if err != nil {
        t.Fatalf("Emit failed: %v", err)
    }

    expected := `{"type":"unit.started","timestamp":"2024-01-15T10:30:00Z","unit":"web-api"}` + "\n"
    if buf.String() != expected {
        t.Errorf("expected %q, got %q", expected, buf.String())
    }
}

func TestJSONEmitter_ConcurrentWrites(t *testing.T) {
    var buf bytes.Buffer
    emitter := NewJSONEmitter(&buf)

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            event := NewEvent(TaskStarted, "unit").WithTask(n)
            event.Time = time.Now()
            emitter.Emit(event)
        }(i)
    }
    wg.Wait()

    // Verify all lines are complete JSON
    lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
    if len(lines) != 100 {
        t.Errorf("expected 100 lines, got %d", len(lines))
    }

    for i, line := range lines {
        var je JSONEvent
        if err := json.Unmarshal([]byte(line), &je); err != nil {
            t.Errorf("line %d is not valid JSON: %v", i, err)
        }
    }
}
```

### Integration with Event Bus

Usage example (for reference, not part of this task):

```go
if jsonMode {
    emitter := events.NewJSONEmitter(os.Stdout)
    bus.Subscribe(events.JSONEmitterHandler(emitter))
}
```

## NOT In Scope

- Wire format types (Task #1)
- JSON reader implementation (Task #3)
- TTY detection (Task #4)
- EmitRaw method on Bus (Task #4)
