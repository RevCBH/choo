package events

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// mockUnit implements the Unit interface for testing
type mockUnit struct {
	status        string
	prNumber      int
	persistErr    error
	persistCalled bool
}

func (m *mockUnit) SetStatus(s string) { m.status = s }
func (m *mockUnit) SetPRNumber(pr int) { m.prNumber = pr }
func (m *mockUnit) Persist() error {
	m.persistCalled = true
	return m.persistErr
}

func TestLogHandler_Format(t *testing.T) {
	var buf bytes.Buffer
	handler := LogHandler(LogConfig{Writer: &buf})

	task := 1
	event := Event{
		Type: TaskCompleted,
		Unit: "app-shell",
		Task: &task,
	}
	handler(event)

	output := buf.String()
	if !strings.Contains(output, "[task.completed]") {
		t.Errorf("expected output to contain [task.completed], got: %s", output)
	}
	if !strings.Contains(output, "app-shell") {
		t.Errorf("expected output to contain app-shell, got: %s", output)
	}
	if !strings.Contains(output, "task=#1") {
		t.Errorf("expected output to contain task=#1, got: %s", output)
	}
}

func TestLogHandler_DefaultWriter(t *testing.T) {
	// When Writer is nil, it should default to os.Stderr
	// We can't easily test os.Stderr output, but we can verify no panic
	handler := LogHandler(LogConfig{})
	event := Event{Type: OrchStarted}

	// Should not panic
	handler(event)
}

func TestLogHandler_IncludePayload(t *testing.T) {
	var buf bytes.Buffer
	handler := LogHandler(LogConfig{
		Writer:         &buf,
		IncludePayload: true,
	})

	event := Event{
		Type:    TaskStarted,
		Unit:    "app-shell",
		Payload: map[string]string{"key": "value"},
	}
	handler(event)

	output := buf.String()
	if !strings.Contains(output, "payload=") {
		t.Errorf("expected output to contain payload=, got: %s", output)
	}
}

func TestLogHandler_TimeFormat(t *testing.T) {
	// Custom TimeFormat should be stored (though not directly visible in output)
	// Just verify it can be set without panic
	handler := LogHandler(LogConfig{
		Writer:     &bytes.Buffer{},
		TimeFormat: time.RFC822,
	})

	event := Event{Type: OrchStarted}
	handler(event)
}

func TestLogHandler_OrchEvent(t *testing.T) {
	var buf bytes.Buffer
	handler := LogHandler(LogConfig{Writer: &buf})

	event := Event{Type: OrchStarted}
	handler(event)

	output := buf.String()
	if !strings.Contains(output, "[orch.started]") {
		t.Errorf("expected output to contain [orch.started], got: %s", output)
	}
	// Should not contain unit info
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 0 && strings.Contains(lines[0], "task=") {
		t.Errorf("orch event should not contain task info, got: %s", output)
	}
}

func TestStateHandler_UnitStarted(t *testing.T) {
	unit := &mockUnit{}
	handler := StateHandler(StateConfig{
		Units: map[string]Unit{"app-shell": unit},
	})

	event := Event{
		Type: UnitStarted,
		Unit: "app-shell",
	}
	handler(event)

	if unit.status != "in_progress" {
		t.Errorf("expected status in_progress, got: %s", unit.status)
	}
	if !unit.persistCalled {
		t.Error("expected Persist to be called")
	}
}

func TestStateHandler_UnitCompleted(t *testing.T) {
	unit := &mockUnit{}
	handler := StateHandler(StateConfig{
		Units: map[string]Unit{"app-shell": unit},
	})

	event := Event{
		Type: UnitCompleted,
		Unit: "app-shell",
	}
	handler(event)

	if unit.status != "complete" {
		t.Errorf("expected status complete, got: %s", unit.status)
	}
	if !unit.persistCalled {
		t.Error("expected Persist to be called")
	}
}

func TestStateHandler_UnitFailed(t *testing.T) {
	unit := &mockUnit{}
	handler := StateHandler(StateConfig{
		Units: map[string]Unit{"app-shell": unit},
	})

	event := Event{
		Type: UnitFailed,
		Unit: "app-shell",
	}
	handler(event)

	if unit.status != "failed" {
		t.Errorf("expected status failed, got: %s", unit.status)
	}
	if !unit.persistCalled {
		t.Error("expected Persist to be called")
	}
}

func TestStateHandler_PRCreated(t *testing.T) {
	unit := &mockUnit{}
	handler := StateHandler(StateConfig{
		Units: map[string]Unit{"app-shell": unit},
	})

	pr := 42
	event := Event{
		Type: PRCreated,
		Unit: "app-shell",
		PR:   &pr,
	}
	handler(event)

	if unit.prNumber != 42 {
		t.Errorf("expected PRNumber 42, got: %d", unit.prNumber)
	}
	if unit.status != "pr_open" {
		t.Errorf("expected status pr_open, got: %s", unit.status)
	}
	if !unit.persistCalled {
		t.Error("expected Persist to be called")
	}
}

func TestStateHandler_UnknownUnit(t *testing.T) {
	handler := StateHandler(StateConfig{
		Units: map[string]Unit{},
	})

	event := Event{
		Type: UnitStarted,
		Unit: "unknown-unit",
	}

	// Should not panic and should be ignored
	handler(event)
}

func TestStateHandler_OnError(t *testing.T) {
	persistErr := errors.New("persist failed")
	unit := &mockUnit{persistErr: persistErr}

	var capturedErr error
	handler := StateHandler(StateConfig{
		Units: map[string]Unit{"app-shell": unit},
		OnError: func(err error) {
			capturedErr = err
		},
	})

	event := Event{
		Type: UnitStarted,
		Unit: "app-shell",
	}
	handler(event)

	if capturedErr != persistErr {
		t.Errorf("expected OnError to be called with persist error, got: %v", capturedErr)
	}
}

func TestStateHandler_PRReviewInProgress(t *testing.T) {
	unit := &mockUnit{}
	handler := StateHandler(StateConfig{
		Units: map[string]Unit{"app-shell": unit},
	})

	event := Event{
		Type: PRReviewInProgress,
		Unit: "app-shell",
	}
	handler(event)

	if unit.status != "in_review" {
		t.Errorf("expected status in_review, got: %s", unit.status)
	}
	if !unit.persistCalled {
		t.Error("expected Persist to be called")
	}
}

func TestStateHandler_PRMergeQueued(t *testing.T) {
	unit := &mockUnit{}
	handler := StateHandler(StateConfig{
		Units: map[string]Unit{"app-shell": unit},
	})

	event := Event{
		Type: PRMergeQueued,
		Unit: "app-shell",
	}
	handler(event)

	if unit.status != "merging" {
		t.Errorf("expected status merging, got: %s", unit.status)
	}
	if !unit.persistCalled {
		t.Error("expected Persist to be called")
	}
}

func TestLogHandler_WithPR(t *testing.T) {
	var buf bytes.Buffer
	handler := LogHandler(LogConfig{Writer: &buf})

	pr := 42
	event := Event{
		Type: PRCreated,
		Unit: "app-shell",
		PR:   &pr,
	}
	handler(event)

	output := buf.String()
	if !strings.Contains(output, "pr=#42") {
		t.Errorf("expected output to contain pr=#42, got: %s", output)
	}
}
