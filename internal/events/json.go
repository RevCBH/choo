package events

import "time"

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
