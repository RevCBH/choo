# JSON-EVENTS - JSON Event Emitter and Parser for Container Stdout Communication

## Overview

The JSON-EVENTS module enables structured event communication between `choo run` processes and the daemon when running inside containers. When `choo run` executes non-interactively (no TTY attached), it emits events as JSON lines to stdout. The daemon reads container stdout, parses these JSON lines, and bridges the events to the web UI via the existing event bus.

This design solves the container isolation problem: processes inside containers cannot directly access the host's event bus. By serializing events to stdout as newline-delimited JSON, the daemon can reconstruct the event stream and propagate it to subscribers (web UI, logging, state persistence) without requiring IPC or network communication between container and host.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Container                                      │
│  ┌─────────────┐                                                            │
│  │  choo run   │                                                            │
│  │             │                                                            │
│  │  Event Bus  │──▶ JSONEmitter ──▶ stdout                                  │
│  │  (local)    │         │                                                  │
│  └─────────────┘         │ {"type":"unit.started",...}                      │
│                          │ {"type":"task.completed",...}                    │
│                          ▼                                                  │
└─────────────────────────────────────────────────────────────────────────────┘
                           │
                           │ Container stdout (JSON lines)
                           ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Daemon (Host)                                  │
│                                                                             │
│  Container ──▶ JSONLineReader ──▶ ParseJSONEvent ──▶ Event Bus ──▶ Web UI  │
│  stdout              │                                    │                 │
│                      │                                    ▼                 │
│                      │                              StateHandler            │
│                      │                              LogHandler              │
│                      ▼                                                      │
│              Reconstructed Events                                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. `choo run` must emit events as JSON lines to stdout when running non-interactively (no TTY)
2. Auto-detect non-TTY environment to enable JSON mode automatically
3. Support explicit `--json-events` flag to force JSON mode regardless of TTY
4. Emit all event types: unit lifecycle (started, completed, failed), unit dependency warnings, task lifecycle, orchestrator events, spec validation/normalization/repair events
5. Daemon must parse JSON lines from container stdout in real-time
6. Parsed events must be re-emitted to the host event bus for web UI bridging
7. Handle malformed JSON lines gracefully (log warning, continue processing)
8. Preserve event timestamps from container (do not overwrite on re-emit)

### Performance Requirements

| Metric | Target |
|--------|--------|
| Event serialization overhead | < 1ms per event |
| Event parsing overhead | < 10ms per event |
| Memory allocation per event | < 2KB |
| Line buffer size | 64KB (handles large payloads) |

### Constraints

1. Must use existing `Event` type from `internal/events/types.go` for compatibility
2. JSON format must be stable and backward-compatible for daemon/container version skew
3. No external dependencies beyond standard library (`encoding/json`, `bufio`)
4. Must handle concurrent writes to stdout from multiple goroutines safely
5. Must not interfere with regular log output (use structured JSON, not mixed output)

## Design

### Module Structure

```
internal/events/
├── bus.go       # Existing event bus (no changes needed)
├── types.go     # Existing event types (no changes needed)
├── handlers.go  # Existing handlers (add JSON emitter handler)
└── json.go      # NEW: JSON emitter, parser, and line reader
```

### Core Types

```go
// internal/events/json.go

// JSONEmitter writes events as JSON lines to a writer.
// Thread-safe for concurrent Emit calls.
type JSONEmitter struct {
    w   io.Writer
    mu  sync.Mutex
    enc *json.Encoder
}

// JSONLineReader reads events from a JSON lines stream.
// Not thread-safe; use from a single goroutine.
type JSONLineReader struct {
    r       *bufio.Reader
    maxLine int // Maximum line length (default 64KB)
}

// JSONEvent is the wire format for serialized events over container stdout.
// This matches the PRD-defined format with "timestamp" field.
type JSONEvent struct {
    Type      string                 `json:"type"`
    Timestamp time.Time              `json:"timestamp"`
    Unit      string                 `json:"unit,omitempty"`
    Task      *int                   `json:"task,omitempty"`
    PR        *int                   `json:"pr,omitempty"`
    Payload   map[string]interface{} `json:"payload,omitempty"`
    Error     string                 `json:"error,omitempty"`
}
```

### API Surface

```go
// internal/events/json.go

// NewJSONEmitter creates a new JSON emitter that writes to w.
// Each event is written as a single JSON line (newline-delimited).
func NewJSONEmitter(w io.Writer) *JSONEmitter

// Emit converts the internal Event to JSONEvent wire format and writes it.
// Appends a newline after each JSON object.
// Returns an error if JSON marshaling fails or the write fails.
func (e *JSONEmitter) Emit(event Event) error

// JSONEmitterHandler returns a Handler that emits events as JSON lines.
// Use this to subscribe the emitter to an event bus.
func JSONEmitterHandler(emitter *JSONEmitter) Handler

// NewJSONLineReader creates a new JSON line reader from r.
// Uses a 64KB buffer by default for line reading.
func NewJSONLineReader(r io.Reader) *JSONLineReader

// Read reads the next JSON line and parses it into an internal Event.
// Converts from JSONEvent wire format to internal Event.
// Returns io.EOF when the stream is exhausted.
// Returns an error for malformed JSON (caller should log and continue).
func (jr *JSONLineReader) Read() (Event, error)

// ParseJSONEvent parses a JSON line (in JSONEvent wire format) into an internal Event.
// Standalone function for single-line parsing.
func ParseJSONEvent(line []byte) (Event, error)

// IsJSONMode returns true if JSON event output should be enabled.
// Checks: (1) explicit --json-events flag, (2) non-TTY stdout.
func IsJSONMode(forceJSON bool) bool
```

### Event Wire Format

Events are serialized using the `JSONEvent` struct with the PRD-defined wire format:

```json
{"type":"unit.started","timestamp":"2024-01-15T10:30:00Z","unit":"web-api"}
{"type":"task.started","timestamp":"2024-01-15T10:31:00Z","unit":"web-api","task":1}
{"type":"task.claude.invoke","timestamp":"2024-01-15T10:32:00Z","unit":"web-api","task":1}
{"type":"task.completed","timestamp":"2024-01-15T10:33:00Z","unit":"web-api","task":1}
{"type":"unit.completed","timestamp":"2024-01-15T10:35:00Z","unit":"web-api"}
{"type":"orch.completed","timestamp":"2024-01-15T10:40:00Z"}
```

With payload:

```json
{"type":"task.committed","timestamp":"2024-01-15T10:30:00Z","unit":"web-api","task":1,"payload":{"commit":"abc123","files":["api.go"]}}
{"type":"unit.failed","timestamp":"2024-01-15T10:30:00Z","unit":"web-api","error":"validation failed: tests not passing"}
```

### TTY Detection Logic

```go
// internal/events/json.go

// IsJSONMode determines whether to use JSON event output.
func IsJSONMode(forceJSON bool) bool {
    // Explicit flag takes precedence
    if forceJSON {
        return true
    }

    // Auto-detect: JSON mode when stdout is not a TTY
    if f, ok := os.Stdout.(*os.File); ok {
        return !isatty.IsTerminal(f.Fd())
    }

    // Default to JSON mode if we can't determine TTY status
    return true
}
```

Note: Use `golang.org/x/term` or inline syscall for `isatty` check to avoid external dependencies.

### Integration with Event Bus

```go
// In cmd/run.go or internal/cli/run.go

func setupEventHandlers(bus *events.Bus, jsonMode bool) {
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

### Daemon-Side Parsing

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

Note: `EmitRaw` is a new method that emits without overwriting `event.Time`.

## Implementation Notes

### Thread Safety

`JSONEmitter` must be thread-safe because multiple goroutines may emit events concurrently:

```go
func (e *JSONEmitter) Emit(event Event) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    return e.enc.Encode(event)
}
```

The `sync.Mutex` ensures JSON lines are not interleaved.

### Handling Large Payloads

Some events carry large payloads (e.g., commit diffs, error messages). The line reader uses a 64KB buffer:

```go
func NewJSONLineReader(r io.Reader) *JSONLineReader {
    return &JSONLineReader{
        r:       bufio.NewReaderSize(r, 64*1024),
        maxLine: 64 * 1024,
    }
}

func (jr *JSONLineReader) Read() (Event, error) {
    line, err := jr.r.ReadBytes('\n')
    if err != nil && err != io.EOF {
        return Event{}, err
    }
    if len(line) == 0 {
        return Event{}, io.EOF
    }

    // Trim trailing newline
    line = bytes.TrimSuffix(line, []byte("\n"))

    return ParseJSONEvent(line)
}
```

### Preserving Timestamps

When the daemon re-emits parsed events, it must preserve the original timestamp from the container. Add a new method to `Bus`:

```go
// internal/events/bus.go

// EmitRaw publishes an event without modifying the timestamp.
// Use for re-emitting events from external sources (containers, replays).
func (b *Bus) EmitRaw(e Event) {
    select {
    case b.ch <- e:
        // Delivered
    default:
        log.Printf("WARN: event buffer full, dropping %s", e.Type)
    }
}
```

### Error Handling

Malformed JSON should not crash the event stream. The reader logs and continues:

```go
event, err := reader.Read()
if err != nil && err != io.EOF {
    // Could be JSON syntax error or truncated line
    log.Printf("WARN: skipping malformed event line: %v", err)
    continue
}
```

### Avoiding Mixed Output

When JSON mode is enabled, all human-readable output should go to stderr, not stdout. The `LogHandler` already writes to a configurable writer (default stderr). Ensure no other code writes to stdout when `--json-events` is active.

## Testing Strategy

### Unit Tests

```go
// internal/events/json_test.go

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
        var event Event
        if err := json.Unmarshal([]byte(line), &event); err != nil {
            t.Errorf("line %d is not valid JSON: %v", i, err)
        }
    }
}

func TestJSONLineReader_Read(t *testing.T) {
    input := `{"type":"unit.started","timestamp":"2024-01-15T10:30:00Z","unit":"web-api"}
{"type":"task.started","timestamp":"2024-01-15T10:31:00Z","unit":"web-api","task":1}
`
    reader := NewJSONLineReader(strings.NewReader(input))

    // First event
    event1, err := reader.Read()
    if err != nil {
        t.Fatalf("Read failed: %v", err)
    }
    if event1.Type != UnitStarted {
        t.Errorf("expected UnitStarted, got %s", event1.Type)
    }
    if event1.Unit != "web-api" {
        t.Errorf("expected web-api, got %s", event1.Unit)
    }

    // Second event
    event2, err := reader.Read()
    if err != nil {
        t.Fatalf("Read failed: %v", err)
    }
    if event2.Type != TaskStarted {
        t.Errorf("expected TaskStarted, got %s", event2.Type)
    }
    if event2.Task == nil || *event2.Task != 1 {
        t.Errorf("expected task 1, got %v", event2.Task)
    }

    // EOF
    _, err = reader.Read()
    if err != io.EOF {
        t.Errorf("expected EOF, got %v", err)
    }
}

func TestJSONLineReader_MalformedJSON(t *testing.T) {
    input := `{"type":"unit.started","unit":"web-api"}
not valid json
{"type":"unit.completed","unit":"web-api"}
`
    reader := NewJSONLineReader(strings.NewReader(input))

    // First event OK
    _, err := reader.Read()
    if err != nil {
        t.Fatalf("expected first read to succeed: %v", err)
    }

    // Second line is malformed
    _, err = reader.Read()
    if err == nil {
        t.Fatal("expected error for malformed JSON")
    }

    // Third event OK (parser recovers)
    event3, err := reader.Read()
    if err != nil {
        t.Fatalf("expected third read to succeed: %v", err)
    }
    if event3.Type != UnitCompleted {
        t.Errorf("expected UnitCompleted, got %s", event3.Type)
    }
}

func TestParseJSONEvent(t *testing.T) {
    line := []byte(`{"type":"task.completed","timestamp":"2024-01-15T10:30:00Z","unit":"api","task":3,"payload":{"commit":"abc123"}}`)

    event, err := ParseJSONEvent(line)
    if err != nil {
        t.Fatalf("ParseJSONEvent failed: %v", err)
    }

    if event.Type != TaskCompleted {
        t.Errorf("expected TaskCompleted, got %s", event.Type)
    }
    if event.Task == nil || *event.Task != 3 {
        t.Errorf("expected task 3, got %v", event.Task)
    }
    if event.Payload == nil {
        t.Error("expected payload to be set")
    }
}

func TestIsJSONMode(t *testing.T) {
    // Explicit flag always wins
    if !IsJSONMode(true) {
        t.Error("expected JSON mode when forceJSON=true")
    }

    // Note: TTY detection is hard to test without actual TTY
    // Integration tests cover this scenario
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Container event flow | Spawn subprocess with JSON mode, capture stdout, verify events parse correctly |
| Daemon bridge | Mock container stdout, parse events, verify re-emit to host bus |
| Large payload handling | Emit event with 50KB payload, verify round-trip |
| TTY detection | Run with/without TTY, verify correct output mode |
| Error recovery | Inject malformed lines, verify stream continues |

### Manual Testing

- [ ] Run `choo run --json-events` and verify JSON output to stdout
- [ ] Run `choo run` in a non-TTY (pipe to cat), verify JSON mode auto-enabled
- [ ] Run `choo run` interactively, verify human-readable output
- [ ] Start daemon, submit job, verify events appear in web UI
- [ ] Inject malformed JSON into container stdout, verify daemon recovers
- [ ] Run with large payloads, verify no truncation

## Design Decisions

### Why JSON Lines Instead of Other Formats?

JSON lines (newline-delimited JSON) was chosen for several reasons:

1. **Streaming-friendly**: Each line is a complete, parseable unit. No need to buffer the entire stream.
2. **Simple parsing**: `bufio.ReadBytes('\n')` + `json.Unmarshal` is trivial to implement.
3. **Human-debuggable**: Developers can `cat` container logs and read events directly.
4. **Wide tooling support**: Works with `jq`, log aggregators, and monitoring systems.
5. **No framing overhead**: Unlike length-prefixed protocols, no binary framing needed.

Alternatives considered:
- **gRPC/protobuf**: Overkill for stdout streaming, adds dependency
- **MessagePack**: Binary format, harder to debug
- **Length-prefixed JSON**: More complex, no significant benefit

### Why Reuse Existing Event Type?

The existing `Event` struct already has JSON tags and represents the full event schema. Creating a separate wire type would require:
- Mapping code between wire and internal types
- Version synchronization
- Duplicate field definitions

Since `Event` serializes cleanly and the fields are stable, direct serialization is simpler and less error-prone.

### Why Auto-Detect TTY?

Container environments typically redirect stdout to capture logs. By auto-detecting non-TTY, `choo run` does the right thing automatically:
- Interactive terminal: Human-readable output
- Container/pipe: JSON for machine parsing

The `--json-events` flag provides an escape hatch for edge cases.

### Why 64KB Line Buffer?

Most events are small (<1KB), but some payloads can be large:
- Error messages with stack traces
- Commit metadata with file lists
- Validation output

64KB handles 99.9% of cases while limiting memory. Events exceeding this are likely bugs (unbounded payloads should be truncated at the source).

### Why EmitRaw for Re-Emission?

The existing `Emit` method overwrites `event.Time` with `time.Now()`. When the daemon parses container events, the timestamp reflects when the event occurred inside the container, not when the daemon parsed it. `EmitRaw` preserves this timing information for accurate dashboards and debugging.

## Future Enhancements

1. **Event compression**: For high-volume scenarios, support optional gzip compression of the JSON stream
2. **Schema versioning**: Add `"version": 1` field to events for backward compatibility during format changes
3. **Binary mode**: Optional MessagePack serialization for bandwidth-constrained environments
4. **Event filtering**: Allow configuring which event types to emit (reduce noise for specific use cases)
5. **Checkpointing**: Support for resuming event parsing after daemon restart (sequence numbers)

## References

- [EVENTS spec](/Users/bennett/conductor/workspaces/choo/dakar/specs/completed/EVENTS.md) - Existing event system design
- [CONTAINER-ISOLATION spec](/Users/bennett/conductor/workspaces/choo/dakar/specs/CONTAINER-ISOLATION.md) - Container architecture context
- [JSON Lines specification](https://jsonlines.org/) - Wire format standard
- [Go encoding/json](https://pkg.go.dev/encoding/json) - Standard library JSON support
