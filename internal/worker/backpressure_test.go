package worker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
)

func TestRunBackpressure_Success(t *testing.T) {
	result := RunBackpressure(context.Background(), "exit 0", t.TempDir(), time.Minute)

	if !result.Success {
		t.Error("expected success for exit 0")
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestRunBackpressure_Failure(t *testing.T) {
	result := RunBackpressure(context.Background(), "exit 1", t.TempDir(), time.Minute)

	if result.Success {
		t.Error("expected failure for exit 1")
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
}

func TestRunBackpressure_Timeout(t *testing.T) {
	result := RunBackpressure(context.Background(), "sleep 10", t.TempDir(), 100*time.Millisecond)

	if result.Success {
		t.Error("expected failure for timeout")
	}
}

func TestRunBackpressure_CapturesOutput(t *testing.T) {
	result := RunBackpressure(context.Background(), "echo hello && echo world >&2", t.TempDir(), time.Minute)

	if !strings.Contains(result.Output, "hello") {
		t.Error("expected stdout to be captured")
	}
	if !strings.Contains(result.Output, "world") {
		t.Error("expected stderr to be captured")
	}
}

func TestRunBackpressure_TracksDuration(t *testing.T) {
	result := RunBackpressure(context.Background(), "sleep 0.1", t.TempDir(), time.Minute)

	if result.Duration < 100*time.Millisecond {
		t.Errorf("expected duration >= 100ms, got %v", result.Duration)
	}
}

func TestValidateTaskComplete_Complete(t *testing.T) {
	task := &discovery.Task{Status: discovery.TaskStatusComplete}
	if !ValidateTaskComplete(task) {
		t.Error("expected true for complete task")
	}
}

func TestValidateTaskComplete_Pending(t *testing.T) {
	task := &discovery.Task{Status: discovery.TaskStatusPending}
	if ValidateTaskComplete(task) {
		t.Error("expected false for pending task")
	}
}
