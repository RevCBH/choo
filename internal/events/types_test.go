package events

import (
	"errors"
	"testing"
)

func TestNewEvent(t *testing.T) {
	event := NewEvent(TaskStarted, "app-shell")

	if event.Type != TaskStarted {
		t.Errorf("expected Type to be %q, got %q", TaskStarted, event.Type)
	}

	if event.Unit != "app-shell" {
		t.Errorf("expected Unit to be %q, got %q", "app-shell", event.Unit)
	}
}

func TestEvent_WithTask(t *testing.T) {
	event := NewEvent(TaskStarted, "app-shell")
	eventWithTask := event.WithTask(1)

	if eventWithTask.Task == nil {
		t.Fatal("expected Task pointer to be set")
	}

	if *eventWithTask.Task != 1 {
		t.Errorf("expected Task to be 1, got %d", *eventWithTask.Task)
	}

	if event.Task != nil {
		t.Error("expected original event to be unchanged")
	}
}

func TestEvent_WithPR(t *testing.T) {
	event := NewEvent(PRCreated, "app-shell")
	eventWithPR := event.WithPR(42)

	if eventWithPR.PR == nil {
		t.Fatal("expected PR pointer to be set")
	}

	if *eventWithPR.PR != 42 {
		t.Errorf("expected PR to be 42, got %d", *eventWithPR.PR)
	}

	if event.PR != nil {
		t.Error("expected original event to be unchanged")
	}
}

func TestEvent_WithPayload(t *testing.T) {
	event := NewEvent(TaskStarted, "app-shell")
	payload := map[string]string{"key": "value"}
	eventWithPayload := event.WithPayload(payload)

	if eventWithPayload.Payload == nil {
		t.Fatal("expected Payload to be set")
	}

	payloadMap, ok := eventWithPayload.Payload.(map[string]string)
	if !ok {
		t.Fatal("expected Payload to be a map[string]string")
	}

	if payloadMap["key"] != "value" {
		t.Errorf("expected Payload[key] to be %q, got %q", "value", payloadMap["key"])
	}

	if event.Payload != nil {
		t.Error("expected original event to be unchanged")
	}
}

func TestEvent_WithError(t *testing.T) {
	event := NewEvent(TaskFailed, "app-shell")
	err := errors.New("something went wrong")
	eventWithError := event.WithError(err)

	if eventWithError.Error != "something went wrong" {
		t.Errorf("expected Error to be %q, got %q", "something went wrong", eventWithError.Error)
	}

	if event.Error != "" {
		t.Error("expected original event to be unchanged")
	}
}

func TestEvent_WithError_Nil(t *testing.T) {
	event := NewEvent(TaskCompleted, "app-shell")
	eventWithError := event.WithError(nil)

	if eventWithError.Error != "" {
		t.Errorf("expected Error to be empty string for nil error, got %q", eventWithError.Error)
	}
}

func TestEvent_IsFailure(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		expected bool
	}{
		{
			name:     "OrchFailed",
			event:    NewEvent(OrchFailed, ""),
			expected: true,
		},
		{
			name:     "UnitFailed",
			event:    NewEvent(UnitFailed, "app-shell"),
			expected: true,
		},
		{
			name:     "TaskFailed",
			event:    NewEvent(TaskFailed, "app-shell"),
			expected: true,
		},
		{
			name:     "PRFailed",
			event:    NewEvent(PRFailed, "app-shell"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.IsFailure(); got != tt.expected {
				t.Errorf("IsFailure() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEvent_IsFailure_Success(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		expected bool
	}{
		{
			name:     "OrchCompleted",
			event:    NewEvent(OrchCompleted, ""),
			expected: false,
		},
		{
			name:     "UnitCompleted",
			event:    NewEvent(UnitCompleted, "app-shell"),
			expected: false,
		},
		{
			name:     "TaskCompleted",
			event:    NewEvent(TaskCompleted, "app-shell"),
			expected: false,
		},
		{
			name:     "PRMerged",
			event:    NewEvent(PRMerged, "app-shell"),
			expected: false,
		},
		{
			name:     "TaskStarted",
			event:    NewEvent(TaskStarted, "app-shell"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.IsFailure(); got != tt.expected {
				t.Errorf("IsFailure() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEvent_String(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		expected string
	}{
		{
			name:     "basic event with unit",
			event:    NewEvent(TaskCompleted, "app-shell"),
			expected: "[task.completed] app-shell",
		},
		{
			name:     "event with task",
			event:    NewEvent(TaskCompleted, "app-shell").WithTask(1),
			expected: "[task.completed] app-shell task=#1",
		},
		{
			name:     "event with PR",
			event:    NewEvent(PRCreated, "app-shell").WithPR(42),
			expected: "[pr.created] app-shell pr=#42",
		},
		{
			name:     "event with task and PR",
			event:    NewEvent(TaskCompleted, "app-shell").WithTask(1).WithPR(42),
			expected: "[task.completed] app-shell task=#1 pr=#42",
		},
		{
			name:     "orchestrator event without unit",
			event:    NewEvent(OrchStarted, ""),
			expected: "[orch.started]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}
