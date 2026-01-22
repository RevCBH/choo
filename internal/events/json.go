package events

import (
	"encoding/json"
	"io"
	"log"
	"sync"
	"time"
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
