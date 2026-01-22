---
task: 4
status: complete
backpressure: "go test ./internal/events/... -run TestIsJSONMode"
depends_on: [1, 2, 3]
---

# TTY Detection and Bus Integration

**Parent spec**: `/specs/JSON-EVENTS.md`
**Task**: #4 of 4 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement TTY detection for automatic JSON mode selection and add EmitRaw method to Bus for re-emitting events with preserved timestamps.

## Dependencies

### External Specs (must be implemented)
- `/specs/completed/EVENTS.md` - Existing Bus type

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `JSONEvent`)
- Task #2 must be complete (provides: `JSONEmitter`, `JSONEmitterHandler`)
- Task #3 must be complete (provides: `JSONLineReader`, `ParseJSONEvent`)

### Package Dependencies
- Standard library (`os`)
- `golang.org/x/term` for TTY detection (or inline syscall)

## Deliverables

### Files to Create/Modify

```
internal/
└── events/
    ├── json.go    # MODIFY: Add IsJSONMode function
    └── bus.go     # MODIFY: Add EmitRaw method
```

### Functions to Implement

```go
// IsJSONMode returns true if JSON event output should be enabled.
// Checks: (1) explicit forceJSON flag, (2) non-TTY stdout.
func IsJSONMode(forceJSON bool) bool

// EmitRaw publishes an event without modifying the timestamp.
// Use for re-emitting events from external sources (containers, replays).
func (b *Bus) EmitRaw(e Event)
```

## Backpressure

### Validation Command

```bash
go test ./internal/events/... -run TestIsJSONMode -v
go test ./internal/events/... -run TestBus_EmitRaw -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestIsJSONMode_ForceTrue` | Returns true when forceJSON=true |
| `TestIsJSONMode_ForceFalse_TTY` | Returns false when forceJSON=false and stdout is TTY |
| `TestIsJSONMode_ForceFalse_NonTTY` | Returns true when forceJSON=false and stdout is not TTY |
| `TestBus_EmitRaw` | Event timestamp is preserved (not overwritten) |
| `TestBus_EmitRaw_DropsWhenFull` | Behaves like Emit when buffer is full |

### Test Fixtures

No external fixtures required.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds
- [x] TTY tests may need mocking in CI (stdout usually not a TTY)

## Implementation Notes

### IsJSONMode Implementation

Use `golang.org/x/term` for cross-platform TTY detection:

```go
import "golang.org/x/term"

func IsJSONMode(forceJSON bool) bool {
    // Explicit flag takes precedence
    if forceJSON {
        return true
    }

    // Auto-detect: JSON mode when stdout is not a TTY
    if f, ok := os.Stdout.(*os.File); ok {
        return !term.IsTerminal(int(f.Fd()))
    }

    // Default to JSON mode if we can't determine TTY status
    return true
}
```

Alternative without external dependency (Unix-only):

```go
import (
    "os"
    "golang.org/x/sys/unix"
)

func IsJSONMode(forceJSON bool) bool {
    if forceJSON {
        return true
    }

    if f, ok := os.Stdout.(*os.File); ok {
        _, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
        return err != nil // Not a TTY if ioctl fails
    }

    return true
}
```

Prefer `golang.org/x/term` for cross-platform support (Windows compatibility).

### EmitRaw Implementation

Add to `internal/events/bus.go`:

```go
// EmitRaw publishes an event without modifying the timestamp.
// Use for re-emitting events from external sources (containers, replays).
func (b *Bus) EmitRaw(e Event) {
    b.mu.RLock()
    closed := b.closed
    b.mu.RUnlock()

    if closed {
        return
    }

    select {
    case b.ch <- e:
        // Delivered
    default:
        log.Printf("WARN: event buffer full, dropping %s", e.Type)
    }
}
```

The key difference from `Emit` is that `EmitRaw` does NOT set `e.Time = time.Now()`.

### Integration Example

Show how to wire up JSON mode in the CLI (for reference):

```go
// In cmd/run.go or internal/cli/run.go

func setupEventHandlers(bus *events.Bus, forceJSON bool) {
    jsonMode := events.IsJSONMode(forceJSON)

    if jsonMode {
        // JSON output to stdout for daemon parsing
        emitter := events.NewJSONEmitter(os.Stdout)
        bus.Subscribe(events.JSONEmitterHandler(emitter))
    } else {
        // Human-readable output for interactive use
        bus.Subscribe(events.LogHandler(events.LogConfig{
            Writer: os.Stderr,
        }))
    }

    // State handler always runs (persists to frontmatter)
    bus.Subscribe(events.StateHandler(stateConfig))
}
```

### Daemon Bridge Example

Show how the daemon uses these components (for reference):

```go
// In daemon job execution code

func bridgeContainerEvents(containerStdout io.Reader, hostBus *events.Bus) error {
    reader := events.NewJSONLineReader(containerStdout)

    for {
        event, err := reader.Read()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            // Log malformed line but continue
            log.Printf("WARN: malformed JSON event: %v", err)
            continue
        }

        // Re-emit to host bus (preserves original timestamp)
        hostBus.EmitRaw(event)
    }
}
```

### Example Tests

```go
func TestIsJSONMode_ForceTrue(t *testing.T) {
    if !IsJSONMode(true) {
        t.Error("expected JSON mode when forceJSON=true")
    }
}

func TestBus_EmitRaw(t *testing.T) {
    bus := NewBus(10)
    defer bus.Close()

    var received Event
    bus.Subscribe(func(e Event) {
        received = e
    })

    // Create event with specific timestamp
    originalTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
    event := NewEvent(UnitStarted, "test")
    event.Time = originalTime

    bus.EmitRaw(event)

    // Wait for dispatch
    time.Sleep(10 * time.Millisecond)

    // Verify timestamp was NOT overwritten
    if !received.Time.Equal(originalTime) {
        t.Errorf("expected time %v, got %v", originalTime, received.Time)
    }
}

func TestBus_EmitRaw_vs_Emit(t *testing.T) {
    bus := NewBus(10)
    defer bus.Close()

    var events []Event
    bus.Subscribe(func(e Event) {
        events = append(events, e)
    })

    originalTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

    // EmitRaw preserves timestamp
    e1 := NewEvent(UnitStarted, "raw")
    e1.Time = originalTime
    bus.EmitRaw(e1)

    // Emit overwrites timestamp
    e2 := NewEvent(UnitStarted, "normal")
    e2.Time = originalTime
    bus.Emit(e2)

    // Wait for dispatch
    time.Sleep(10 * time.Millisecond)

    if len(events) != 2 {
        t.Fatalf("expected 2 events, got %d", len(events))
    }

    // EmitRaw event has original time
    if !events[0].Time.Equal(originalTime) {
        t.Errorf("EmitRaw should preserve timestamp")
    }

    // Emit event has current time (not original)
    if events[1].Time.Equal(originalTime) {
        t.Errorf("Emit should overwrite timestamp")
    }
}
```

### Avoiding Mixed Output

When JSON mode is enabled, ensure human-readable output goes to stderr:

```go
// Good: LogHandler writes to stderr
bus.Subscribe(events.LogHandler(events.LogConfig{
    Writer: os.Stderr,  // Not stdout!
}))

// JSON emitter writes to stdout
emitter := events.NewJSONEmitter(os.Stdout)
bus.Subscribe(events.JSONEmitterHandler(emitter))
```

This allows the daemon to parse JSON from stdout while still seeing human-readable logs on stderr.

## NOT In Scope

- Wire format types (Task #1)
- JSON emitter implementation (Task #2)
- JSON reader implementation (Task #3)
- CLI flag parsing for --json-events
- Daemon implementation
