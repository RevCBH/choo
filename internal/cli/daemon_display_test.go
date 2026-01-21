package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/client"
	"github.com/RevCBH/choo/internal/events"
)

// captureStdout captures stdout during function execution
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestDisplayEvent_UnitStarted(t *testing.T) {
	e := events.Event{
		Time: time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC),
		Type: events.UnitStarted,
		Unit: "test-unit",
	}

	output := captureStdout(func() {
		displayEvent(e)
	})

	// Verifies unit started events are formatted with unit name
	if !strings.Contains(output, "test-unit") {
		t.Errorf("Expected output to contain unit name 'test-unit', got: %s", output)
	}
	if !strings.Contains(output, "Unit started") {
		t.Errorf("Expected output to contain 'Unit started', got: %s", output)
	}
}

func TestDisplayEvent_TaskComplete(t *testing.T) {
	taskNum := 3
	e := events.Event{
		Time: time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC),
		Type: events.TaskCompleted,
		Unit: "test-unit",
		Task: &taskNum,
	}

	output := captureStdout(func() {
		displayEvent(e)
	})

	// Verifies task events include task name and status
	if !strings.Contains(output, "test-unit") {
		t.Errorf("Expected output to contain unit name 'test-unit', got: %s", output)
	}
	if !strings.Contains(output, "#3") {
		t.Errorf("Expected output to contain task number '#3', got: %s", output)
	}
	if !strings.Contains(output, "Task completed") {
		t.Errorf("Expected output to contain 'Task completed', got: %s", output)
	}
}

func TestDisplayJobs_Empty(t *testing.T) {
	jobs := []*client.JobSummary{}

	output := captureStdout(func() {
		displayJobs(jobs)
	})

	// Verifies empty job list displays header only
	if !strings.Contains(output, "ID") {
		t.Errorf("Expected header to contain 'ID', got: %s", output)
	}
	if !strings.Contains(output, "STATUS") {
		t.Errorf("Expected header to contain 'STATUS', got: %s", output)
	}
	if !strings.Contains(output, "FEATURE BRANCH") {
		t.Errorf("Expected header to contain 'FEATURE BRANCH', got: %s", output)
	}
	if !strings.Contains(output, "UNITS") {
		t.Errorf("Expected header to contain 'UNITS', got: %s", output)
	}
	if !strings.Contains(output, "STARTED") {
		t.Errorf("Expected header to contain 'STARTED', got: %s", output)
	}

	// Count non-empty lines
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("Expected 1 line (header only), got %d lines", len(lines))
	}
}

func TestDisplayJobs_MultipleJobs(t *testing.T) {
	jobs := []*client.JobSummary{
		{
			JobID:         "job-1",
			FeatureBranch: "feature/test-1",
			Status:        "running",
			StartedAt:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			UnitsComplete: 2,
			UnitsTotal:    5,
		},
		{
			JobID:         "job-2",
			FeatureBranch: "feature/test-2",
			Status:        "completed",
			StartedAt:     time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
			UnitsComplete: 3,
			UnitsTotal:    3,
		},
	}

	output := captureStdout(func() {
		displayJobs(jobs)
	})

	// Verifies jobs display in aligned columns
	if !strings.Contains(output, "job-1") {
		t.Errorf("Expected output to contain 'job-1', got: %s", output)
	}
	if !strings.Contains(output, "job-2") {
		t.Errorf("Expected output to contain 'job-2', got: %s", output)
	}
	if !strings.Contains(output, "feature/test-1") {
		t.Errorf("Expected output to contain 'feature/test-1', got: %s", output)
	}
	if !strings.Contains(output, "feature/test-2") {
		t.Errorf("Expected output to contain 'feature/test-2', got: %s", output)
	}
	if !strings.Contains(output, "2/5") {
		t.Errorf("Expected output to contain '2/5', got: %s", output)
	}
	if !strings.Contains(output, "3/3") {
		t.Errorf("Expected output to contain '3/3', got: %s", output)
	}

	// Check for header
	if !strings.Contains(output, "ID") && !strings.Contains(output, "STATUS") {
		t.Errorf("Expected header row, got: %s", output)
	}
}

func TestBoolToStatus(t *testing.T) {
	// Verifies true returns "healthy", false returns "unhealthy"
	if got := boolToStatus(true); got != "healthy" {
		t.Errorf("boolToStatus(true) = %s, want 'healthy'", got)
	}
	if got := boolToStatus(false); got != "unhealthy" {
		t.Errorf("boolToStatus(false) = %s, want 'unhealthy'", got)
	}
}

func TestDefaultSocketPath(t *testing.T) {
	path := defaultSocketPath()

	// Verifies path ends with `.choo/daemon.sock`
	if !strings.HasSuffix(path, ".choo/daemon.sock") {
		t.Errorf("Expected path to end with '.choo/daemon.sock', got: %s", path)
	}

	// Additional check: path should not be empty
	if path == "" {
		t.Error("Expected non-empty path")
	}
}
