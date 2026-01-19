package escalate

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

type mockEscalator struct {
	name  string
	err   error
	calls int32
}

func (m *mockEscalator) Escalate(ctx context.Context, e Escalation) error {
	atomic.AddInt32(&m.calls, 1)
	return m.err
}

func (m *mockEscalator) Name() string {
	return m.name
}

func TestMulti_Escalate(t *testing.T) {
	mock1 := &mockEscalator{name: "mock1"}
	mock2 := &mockEscalator{name: "mock2"}
	mock3 := &mockEscalator{name: "mock3"}

	multi := NewMulti(mock1, mock2, mock3)
	err := multi.Escalate(context.Background(), Escalation{
		Severity: SeverityInfo,
		Unit:     "test",
		Title:    "Test",
		Message:  "Test message",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if mock1.calls != 1 || mock2.calls != 1 || mock3.calls != 1 {
		t.Error("expected all escalators to be called once")
	}
}

func TestMulti_EscalateContinuesOnError(t *testing.T) {
	mock1 := &mockEscalator{name: "mock1"}
	mock2 := &mockEscalator{name: "mock2", err: errors.New("failed")}
	mock3 := &mockEscalator{name: "mock3"}

	multi := NewMulti(mock1, mock2, mock3)
	err := multi.Escalate(context.Background(), Escalation{
		Severity: SeverityInfo,
		Unit:     "test",
		Title:    "Test",
		Message:  "Test message",
	})

	// Should return an error
	if err == nil {
		t.Error("expected error from failing escalator")
	}

	// But all escalators should still be called
	if mock1.calls != 1 || mock2.calls != 1 || mock3.calls != 1 {
		t.Error("expected all escalators to be called despite errors")
	}
}

func TestMulti_Empty(t *testing.T) {
	multi := NewMulti()
	err := multi.Escalate(context.Background(), Escalation{
		Severity: SeverityInfo,
		Unit:     "test",
		Title:    "Test",
		Message:  "Test message",
	})

	if err != nil {
		t.Errorf("unexpected error for empty multi: %v", err)
	}
}

func TestMulti_Name(t *testing.T) {
	multi := NewMulti()
	if multi.Name() != "multi" {
		t.Errorf("expected 'multi', got %q", multi.Name())
	}
}
