---
task: 2
status: pending
backpressure: "go test ./internal/daemon/... -run TestLogStreamer"
depends_on: [1]
---

# Log Streamer

**Parent spec**: `/specs/CONTAINER-DAEMON.md`
**Task**: #2 of 6 in implementation plan

## Objective

Implement LogStreamer to read container logs in real-time, parse JSON event lines, and publish events to the daemon's event bus.

## Dependencies

### External Specs (must be implemented)
- CONTAINER-MANAGER - provides `container.Manager` with `Logs()` method
- JSON-EVENTS - provides `events.JSONEvent` type and parsing

### Task Dependencies (within this unit)
- Task #1 must be complete (provides container types)

### Package Dependencies
- `bufio` (standard library)
- `context` (standard library)
- `encoding/json` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/daemon/
├── log_streamer.go      # CREATE: Log streaming implementation
└── log_streamer_test.go # CREATE: Log streamer tests
```

### Types to Implement

```go
// internal/daemon/log_streamer.go

package daemon

import (
    "bufio"
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "sync"

    "github.com/anthropics/choo/internal/container"
    "github.com/anthropics/choo/internal/events"
)

// LogStreamer reads container logs and parses JSON events.
type LogStreamer struct {
    containerID string
    manager     container.Manager
    eventBus    *events.Bus

    mu     sync.Mutex
    cancel context.CancelFunc
    done   chan struct{}
}

// NewLogStreamer creates a log streamer for a container.
func NewLogStreamer(containerID string, manager container.Manager, eventBus *events.Bus) *LogStreamer {
    return &LogStreamer{
        containerID: containerID,
        manager:     manager,
        eventBus:    eventBus,
        done:        make(chan struct{}),
    }
}
```

### Functions to Implement

```go
// Start begins streaming and parsing logs.
// It blocks until the context is cancelled or the log stream ends.
func (s *LogStreamer) Start(ctx context.Context) error {
    ctx, s.cancel = context.WithCancel(ctx)
    defer close(s.done)

    // Get log reader from container manager
    reader, err := s.manager.Logs(ctx, container.ContainerID(s.containerID))
    if err != nil {
        return fmt.Errorf("failed to get container logs: %w", err)
    }
    defer reader.Close()

    // Parse line by line
    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        line := scanner.Bytes()
        if err := s.parseLine(line); err != nil {
            // Log parse error but continue - may be non-JSON output
            log.Printf("Failed to parse log line: %v", err)
        }
    }

    return scanner.Err()
}

// Stop halts log streaming.
func (s *LogStreamer) Stop() {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.cancel != nil {
        s.cancel()
    }
}

// Done returns a channel that closes when streaming completes.
func (s *LogStreamer) Done() <-chan struct{} {
    return s.done
}

// parseLine attempts to parse a log line as a JSON event.
func (s *LogStreamer) parseLine(line []byte) error {
    // Skip empty lines
    if len(bytes.TrimSpace(line)) == 0 {
        return nil
    }

    // Attempt to parse as JSON event
    var jsonEvt events.JSONEvent
    if err := json.Unmarshal(line, &jsonEvt); err != nil {
        // Not a JSON line - likely stderr or debug output
        // This is not an error - just skip non-JSON lines
        return nil
    }

    // Validate this looks like a choo event
    if jsonEvt.Type == "" {
        return nil
    }

    // Convert to internal event and publish
    evt, err := s.convertJSONEvent(jsonEvt)
    if err != nil {
        return fmt.Errorf("failed to convert event: %w", err)
    }

    s.eventBus.Emit(evt)
    return nil
}

// convertJSONEvent converts a wire-format JSON event to an internal event.
func (s *LogStreamer) convertJSONEvent(jsonEvt events.JSONEvent) (events.Event, error) {
    // Create base event from JSON event type
    evt := events.Event{
        Time: jsonEvt.Timestamp,
        Type: events.EventType(jsonEvt.Type),
    }

    // Extract unit and task from payload if present
    if jsonEvt.Payload != nil {
        // Try to extract common fields from payload
        var payload map[string]interface{}
        if err := json.Unmarshal(jsonEvt.Payload, &payload); err == nil {
            if unit, ok := payload["unit"].(string); ok {
                evt.Unit = unit
            }
            if task, ok := payload["task"].(float64); ok {
                taskInt := int(task)
                evt.Task = &taskInt
            }
        }
        evt.Payload = jsonEvt.Payload
    }

    if jsonEvt.Error != "" {
        evt.Error = jsonEvt.Error
    }

    return evt, nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestLogStreamer
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestLogStreamer_ParseJSONEvent` | Valid JSON event is parsed and published |
| `TestLogStreamer_NonJSONIgnored` | Non-JSON lines are silently skipped |
| `TestLogStreamer_EmptyLineIgnored` | Empty lines are skipped |
| `TestLogStreamer_MalformedJSONIgnored` | Malformed JSON is skipped |
| `TestLogStreamer_ExtractsUnitAndTask` | Unit and task extracted from payload |
| `TestLogStreamer_EventParsingOverhead` | Parsing overhead < 10ms per event |
| `TestLogStreamer_Stop` | Stop() cancels the streaming goroutine |
| `TestLogStreamer_Done` | Done() channel closes when streaming ends |

### Test Implementations

```go
// internal/daemon/log_streamer_test.go

package daemon

import (
    "bytes"
    "context"
    "io"
    "testing"
    "time"

    "github.com/anthropics/choo/internal/events"
)

// mockManager implements container.Manager for testing
type mockManager struct {
    logsReader io.ReadCloser
}

func (m *mockManager) Logs(ctx context.Context, id container.ContainerID) (io.ReadCloser, error) {
    return m.logsReader, nil
}

// ... other interface methods return nil/empty

func TestLogStreamer_ParseJSONEvent(t *testing.T) {
    jsonLine := `{"type":"unit.started","timestamp":"2024-01-15T10:00:00Z","payload":{"unit":"test-unit"}}`
    reader := io.NopCloser(bytes.NewBufferString(jsonLine + "\n"))

    bus := events.NewBus()
    var received events.Event
    bus.Subscribe(func(evt events.Event) {
        received = evt
    })

    manager := &mockManager{logsReader: reader}
    streamer := NewLogStreamer("test-container", manager, bus)

    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()

    streamer.Start(ctx)

    if received.Type != "unit.started" {
        t.Errorf("expected type unit.started, got %s", received.Type)
    }
    if received.Unit != "test-unit" {
        t.Errorf("expected unit test-unit, got %s", received.Unit)
    }
}

func TestLogStreamer_NonJSONIgnored(t *testing.T) {
    reader := io.NopCloser(bytes.NewBufferString("Starting orchestrator...\n"))

    bus := events.NewBus()
    eventCount := 0
    bus.Subscribe(func(evt events.Event) {
        eventCount++
    })

    manager := &mockManager{logsReader: reader}
    streamer := NewLogStreamer("test-container", manager, bus)

    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()

    streamer.Start(ctx)

    if eventCount != 0 {
        t.Errorf("expected 0 events for non-JSON line, got %d", eventCount)
    }
}

func TestLogStreamer_EmptyLineIgnored(t *testing.T) {
    reader := io.NopCloser(bytes.NewBufferString("\n\n   \n"))

    bus := events.NewBus()
    eventCount := 0
    bus.Subscribe(func(evt events.Event) {
        eventCount++
    })

    manager := &mockManager{logsReader: reader}
    streamer := NewLogStreamer("test-container", manager, bus)

    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()

    streamer.Start(ctx)

    if eventCount != 0 {
        t.Errorf("expected 0 events for empty lines, got %d", eventCount)
    }
}

func TestLogStreamer_MalformedJSONIgnored(t *testing.T) {
    reader := io.NopCloser(bytes.NewBufferString(`{"type":"broken` + "\n"))

    bus := events.NewBus()
    eventCount := 0
    bus.Subscribe(func(evt events.Event) {
        eventCount++
    })

    manager := &mockManager{logsReader: reader}
    streamer := NewLogStreamer("test-container", manager, bus)

    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()

    streamer.Start(ctx)

    if eventCount != 0 {
        t.Errorf("expected 0 events for malformed JSON, got %d", eventCount)
    }
}

func TestLogStreamer_EventParsingOverhead(t *testing.T) {
    bus := events.NewBus()
    streamer := &LogStreamer{eventBus: bus}

    eventJSON := []byte(`{"type":"task.completed","timestamp":"2024-01-15T10:00:00Z","payload":{"task":1,"unit":"test"}}`)

    iterations := 1000
    start := time.Now()
    for i := 0; i < iterations; i++ {
        streamer.parseLine(eventJSON)
    }
    elapsed := time.Since(start)

    avgLatency := elapsed / time.Duration(iterations)
    if avgLatency > 10*time.Millisecond {
        t.Errorf("Event parsing overhead %v exceeds 10ms target", avgLatency)
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Non-JSON lines are silently ignored (container may emit debug output)
- JSON events without a "type" field are skipped
- The streamer uses a single goroutine for reading and parsing
- parseLine is called synchronously in the read loop for simplicity
- Events are published to the bus as they are parsed (no buffering)

## NOT In Scope

- Container creation (Task #3)
- Container lifecycle management (Task #3)
- Event type validation (relies on JSON-EVENTS spec)
- Reconnection on log stream errors (container manager handles this)
