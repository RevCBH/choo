---
task: 3
status: complete
backpressure: "go test ./internal/events/... -run TestJSONLineReader"
depends_on: [1]
---

# JSON Line Reader

**Parent spec**: `/specs/JSON-EVENTS.md`
**Task**: #3 of 4 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement the JSONLineReader struct for reading JSON lines from a stream and parsing them into Event structs. Includes large payload handling with 64KB buffer.

## Dependencies

### External Specs (must be implemented)
- `/specs/completed/EVENTS.md` - Existing Event type

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `JSONEvent`, `ToEvent`)

### Package Dependencies
- Standard library only (`bufio`, `bytes`, `encoding/json`, `io`)

## Deliverables

### Files to Create/Modify

```
internal/
└── events/
    └── json.go    # MODIFY: Add JSONLineReader and ParseJSONEvent
```

### Types to Implement

```go
// JSONLineReader reads events from a JSON lines stream.
// Not thread-safe; use from a single goroutine.
type JSONLineReader struct {
    r       *bufio.Reader
    maxLine int // Maximum line length (default 64KB)
}
```

### Functions to Implement

```go
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
```

## Backpressure

### Validation Command

```bash
go test ./internal/events/... -run TestJSONLineReader -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestJSONLineReader_Read` | Parses multiple JSON lines sequentially |
| `TestJSONLineReader_EOF` | Returns io.EOF when stream exhausted |
| `TestJSONLineReader_MalformedJSON` | Returns error for invalid JSON, continues on next Read |
| `TestJSONLineReader_LargePayload` | Handles payloads up to 50KB without truncation |
| `TestJSONLineReader_EmptyLine` | Skips empty lines gracefully |
| `TestParseJSONEvent` | Standalone parser works correctly |
| `TestParseJSONEvent_AllFields` | Parses events with all optional fields |
| `TestParseJSONEvent_MinimalEvent` | Parses events with only required fields |

### Test Fixtures

No external fixtures required.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### Buffer Size

The 64KB buffer handles large payloads like error messages with stack traces or commit metadata with file lists:

```go
const defaultMaxLineSize = 64 * 1024 // 64KB

func NewJSONLineReader(r io.Reader) *JSONLineReader {
    return &JSONLineReader{
        r:       bufio.NewReaderSize(r, defaultMaxLineSize),
        maxLine: defaultMaxLineSize,
    }
}
```

### Read Implementation

```go
func (jr *JSONLineReader) Read() (Event, error) {
    for {
        line, err := jr.r.ReadBytes('\n')
        if err != nil && err != io.EOF {
            return Event{}, err
        }
        if len(line) == 0 && err == io.EOF {
            return Event{}, io.EOF
        }

        // Trim trailing newline and whitespace
        line = bytes.TrimSpace(line)

        // Skip empty lines
        if len(line) == 0 {
            if err == io.EOF {
                return Event{}, io.EOF
            }
            continue
        }

        event, parseErr := ParseJSONEvent(line)
        if parseErr != nil {
            return Event{}, parseErr
        }

        return event, nil
    }
}
```

### ParseJSONEvent Implementation

```go
func ParseJSONEvent(line []byte) (Event, error) {
    var je JSONEvent
    if err := json.Unmarshal(line, &je); err != nil {
        return Event{}, fmt.Errorf("invalid JSON: %w", err)
    }

    return je.ToEvent(), nil
}
```

### Example Tests

```go
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
    input := `{"type":"unit.started","unit":"web-api","timestamp":"2024-01-15T10:30:00Z"}
not valid json
{"type":"unit.completed","unit":"web-api","timestamp":"2024-01-15T10:35:00Z"}
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

    // Third event OK (caller can continue reading)
    event3, err := reader.Read()
    if err != nil {
        t.Fatalf("expected third read to succeed: %v", err)
    }
    if event3.Type != UnitCompleted {
        t.Errorf("expected UnitCompleted, got %s", event3.Type)
    }
}

func TestJSONLineReader_LargePayload(t *testing.T) {
    // Create a 50KB payload
    largeData := strings.Repeat("x", 50*1024)
    input := fmt.Sprintf(`{"type":"task.completed","timestamp":"2024-01-15T10:30:00Z","unit":"api","task":1,"payload":{"data":"%s"}}`, largeData) + "\n"

    reader := NewJSONLineReader(strings.NewReader(input))

    event, err := reader.Read()
    if err != nil {
        t.Fatalf("Read failed: %v", err)
    }

    payload := event.Payload.(map[string]interface{})
    if len(payload["data"].(string)) != 50*1024 {
        t.Errorf("expected 50KB payload, got %d bytes", len(payload["data"].(string)))
    }
}
```

### Error Recovery Pattern

Callers should handle malformed JSON gracefully:

```go
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

    // Process event
    hostBus.EmitRaw(event)
}
```

## NOT In Scope

- Wire format types (Task #1)
- JSON emitter implementation (Task #2)
- TTY detection (Task #4)
- EmitRaw method on Bus (Task #4)
