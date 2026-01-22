package events

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

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

// IsJSONMode returns true if JSON event output should be enabled.
// Checks: (1) explicit forceJSON flag, (2) non-TTY stdout.
func IsJSONMode(forceJSON bool) bool {
	if forceJSON {
		return true
	}

	if os.Stdout != nil {
		return !term.IsTerminal(int(os.Stdout.Fd()))
	}

	return true
}

// JSONEmitter writes events as JSON lines to a writer.
// Thread-safe for concurrent Emit calls.
type JSONEmitter struct {
	w   io.Writer
	mu  sync.Mutex
	enc *json.Encoder
}

// NewJSONEmitter creates a new JSON emitter that writes to w.
// Each event is written as a single JSON line (newline-delimited).
func NewJSONEmitter(w io.Writer) *JSONEmitter {
	return &JSONEmitter{
		w:   w,
		enc: json.NewEncoder(w),
	}
}

// Emit converts the internal Event to JSONEvent wire format and writes it.
// Thread-safe: uses mutex to prevent interleaved writes.
// Returns an error if JSON encoding fails or the write fails.
func (e *JSONEmitter) Emit(event Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	je := ToJSONEvent(event)
	return e.enc.Encode(je)
}

// JSONEmitterHandler returns a Handler that emits events as JSON lines.
// Use this to subscribe the emitter to an event bus.
// Errors are logged but not propagated (handler interface has no return).
func JSONEmitterHandler(emitter *JSONEmitter) Handler {
	return func(e Event) {
		if err := emitter.Emit(e); err != nil {
			log.Printf("WARN: failed to emit JSON event: %v", err)
		}
	}
}

// ToJSONEvent converts an internal Event to the wire format JSONEvent.
// This is used by JSONEmitter when serializing events.
func ToJSONEvent(e Event) JSONEvent {
	je := JSONEvent{
		Type:      string(e.Type),
		Timestamp: e.Time,
		Unit:      e.Unit,
		Task:      e.Task,
		PR:        e.PR,
		Error:     e.Error,
	}

	if e.Payload != nil {
		switch p := e.Payload.(type) {
		case map[string]interface{}:
			je.Payload = p
		default:
			je.Payload = map[string]interface{}{"value": e.Payload}
		}
	}

	return je
}

// ToEvent converts a wire format JSONEvent back to an internal Event.
// This is used by JSONLineReader when parsing events.
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

const defaultMaxLineSize = 64 * 1024

// JSONLineReader reads events from a JSON lines stream.
// Not thread-safe; use from a single goroutine.
type JSONLineReader struct {
	r       *bufio.Reader
	maxLine int
}

// NewJSONLineReader creates a new JSON line reader from r.
// Uses a 64KB buffer by default for line reading.
func NewJSONLineReader(r io.Reader) *JSONLineReader {
	return &JSONLineReader{
		r:       bufio.NewReaderSize(r, defaultMaxLineSize),
		maxLine: defaultMaxLineSize,
	}
}

// Read reads the next JSON line and parses it into an internal Event.
// Converts from JSONEvent wire format to internal Event.
// Returns io.EOF when the stream is exhausted.
// Returns an error for malformed JSON (caller should log and continue).
func (jr *JSONLineReader) Read() (Event, error) {
	for {
		line, err := jr.r.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return Event{}, err
		}
		if len(line) == 0 && err == io.EOF {
			return Event{}, io.EOF
		}

		line = bytes.TrimSpace(line)
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

// ParseJSONEvent parses a JSON line (in JSONEvent wire format) into an internal Event.
// Standalone function for single-line parsing.
func ParseJSONEvent(line []byte) (Event, error) {
	var je JSONEvent
	if err := json.Unmarshal(line, &je); err != nil {
		return Event{}, fmt.Errorf("invalid JSON: %w", err)
	}

	return je.ToEvent(), nil
}
