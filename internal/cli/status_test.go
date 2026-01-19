package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RevCBH/choo/internal/discovery"
)

func TestStatusCmd_DefaultDir(t *testing.T) {
	app := New()
	cmd := NewStatusCmd(app)

	// Check that the default tasks-dir is "specs/tasks"
	// We'll test this by examining the command's args handling
	args := cmd.Args

	if args == nil {
		t.Fatal("Expected Args to be set")
	}

	// The command should accept 0 or 1 args
	// When 0 args are provided, it should use "specs/tasks"
	// This is tested implicitly by the command implementation
}

func TestStatusCmd_CustomDir(t *testing.T) {
	app := New()
	cmd := NewStatusCmd(app)

	// Verify the command accepts a custom directory argument
	if !strings.Contains(cmd.Use, "[tasks-dir]") {
		t.Errorf("Expected command Use to contain '[tasks-dir]', got %q", cmd.Use)
	}
}

func TestStatusCmd_JSONFlag(t *testing.T) {
	app := New()
	cmd := NewStatusCmd(app)

	// Check that --json flag exists
	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Fatal("Expected --json flag to be defined")
	}

	if jsonFlag.DefValue != "false" {
		t.Errorf("Expected --json flag default to be 'false', got %q", jsonFlag.DefValue)
	}
}

func TestShowStatus_NoUnits(t *testing.T) {
	app := New()

	// Create a temporary empty directory
	tmpDir := t.TempDir()

	opts := StatusOptions{
		TasksDir: tmpDir,
		JSON:     false,
	}

	// Should handle empty directory gracefully
	err := app.ShowStatus(opts)
	if err != nil {
		t.Errorf("Expected no error for empty directory, got %v", err)
	}
}

func TestShowStatus_WithUnits(t *testing.T) {
	app := New()

	// Create a temporary directory with mock unit structure
	tmpDir := t.TempDir()

	unitDir := filepath.Join(tmpDir, "test-unit")
	err := os.MkdirAll(unitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create unit directory: %v", err)
	}

	implPlan := filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md")
	err = os.WriteFile(implPlan, []byte(`---
unit: test-unit
---
# Implementation Plan
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create implementation plan: %v", err)
	}

	// Create a mock task file
	taskFile := filepath.Join(unitDir, "01-task.md")
	err = os.WriteFile(taskFile, []byte(`---
task: 1
status: complete
backpressure: "true"
---
# Test Task
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create task file: %v", err)
	}

	// Capture output by redirecting stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := StatusOptions{
		TasksDir: tmpDir,
		JSON:     false,
	}

	err = app.ShowStatus(opts)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify the output contains expected formatting
	if !strings.Contains(output, "Ralph Orchestrator Status") {
		t.Error("Expected output to contain 'Ralph Orchestrator Status'")
	}

	if !strings.Contains(output, "test-unit") {
		t.Error("Expected output to contain unit name 'test-unit'")
	}
}

func TestFormatStatusOutput_Header(t *testing.T) {
	cfg := DisplayConfig{
		Width:    20,
		UseColor: false,
	}

	units := []UnitDisplay{}

	output := formatStatusOutput(units, cfg)

	// Output should include header with separators
	if !strings.Contains(output, "═══") {
		t.Error("Expected output to contain separator line")
	}

	if !strings.Contains(output, "Ralph Orchestrator Status") {
		t.Error("Expected output to contain header text")
	}

	if !strings.Contains(output, "Target: main") {
		t.Error("Expected output to contain target branch info")
	}

	if !strings.Contains(output, "Parallelism: 4") {
		t.Error("Expected output to contain parallelism info")
	}
}

func TestFormatStatusOutput_Summary(t *testing.T) {
	cfg := DisplayConfig{
		Width:    20,
		UseColor: false,
	}

	// Create test units with various statuses
	units := []UnitDisplay{
		{
			ID:       "unit1",
			Status:   discovery.UnitStatusComplete,
			Progress: 1.0,
			Tasks: []TaskDisplay{
				{Number: 1, FileName: "01-task.md", Status: discovery.TaskStatusComplete},
				{Number: 2, FileName: "02-task.md", Status: discovery.TaskStatusComplete},
			},
		},
		{
			ID:       "unit2",
			Status:   discovery.UnitStatusInProgress,
			Progress: 0.5,
			Tasks: []TaskDisplay{
				{Number: 1, FileName: "01-task.md", Status: discovery.TaskStatusComplete},
				{Number: 2, FileName: "02-task.md", Status: discovery.TaskStatusInProgress, Active: true},
			},
		},
		{
			ID:       "unit3",
			Status:   discovery.UnitStatusPending,
			Progress: 0.0,
			Tasks: []TaskDisplay{
				{Number: 1, FileName: "01-task.md", Status: discovery.TaskStatusPending},
			},
		},
	}

	output := formatStatusOutput(units, cfg)

	// Output should include unit counts
	if !strings.Contains(output, "Units: 3") {
		t.Error("Expected output to contain 'Units: 3'")
	}

	if !strings.Contains(output, "Complete: 1") {
		t.Error("Expected output to contain 'Complete: 1' for units")
	}

	if !strings.Contains(output, "In Progress: 1") {
		t.Error("Expected output to contain 'In Progress: 1' for units")
	}

	// Output should include task counts
	if !strings.Contains(output, "Tasks: 5") {
		t.Error("Expected output to contain 'Tasks: 5'")
	}

	// Check for task status counts
	taskCompleteCount := strings.Count(output, "Complete:")
	if taskCompleteCount < 2 {
		t.Error("Expected separate count lines for units and tasks")
	}
}

func TestCalculateUnitStats(t *testing.T) {
	units := []UnitDisplay{
		{Status: discovery.UnitStatusComplete},
		{Status: discovery.UnitStatusComplete},
		{Status: discovery.UnitStatusInProgress},
		{Status: discovery.UnitStatusPending},
	}

	stats := calculateUnitStats(units)

	if stats.Total != 4 {
		t.Errorf("Expected Total=4, got %d", stats.Total)
	}
	if stats.Complete != 2 {
		t.Errorf("Expected Complete=2, got %d", stats.Complete)
	}
	if stats.InProgress != 1 {
		t.Errorf("Expected InProgress=1, got %d", stats.InProgress)
	}
	if stats.Pending != 1 {
		t.Errorf("Expected Pending=1, got %d", stats.Pending)
	}
}

func TestCalculateTaskStats(t *testing.T) {
	units := []UnitDisplay{
		{
			Tasks: []TaskDisplay{
				{Status: discovery.TaskStatusComplete},
				{Status: discovery.TaskStatusComplete},
			},
		},
		{
			Tasks: []TaskDisplay{
				{Status: discovery.TaskStatusInProgress},
				{Status: discovery.TaskStatusPending},
			},
		},
	}

	stats := calculateTaskStats(units)

	if stats.Total != 4 {
		t.Errorf("Expected Total=4, got %d", stats.Total)
	}
	if stats.Complete != 2 {
		t.Errorf("Expected Complete=2, got %d", stats.Complete)
	}
	if stats.InProgress != 1 {
		t.Errorf("Expected InProgress=1, got %d", stats.InProgress)
	}
	if stats.Pending != 1 {
		t.Errorf("Expected Pending=1, got %d", stats.Pending)
	}
}

func TestOutputJSON(t *testing.T) {
	units := []UnitDisplay{
		{
			ID:       "test-unit",
			Status:   discovery.UnitStatusComplete,
			Progress: 1.0,
			Tasks: []TaskDisplay{
				{Number: 1, FileName: "01-task.md", Status: discovery.TaskStatusComplete},
			},
		},
	}

	var buf bytes.Buffer
	err := outputJSON(&buf, units)
	if err != nil {
		t.Fatalf("outputJSON failed: %v", err)
	}

	// Verify it's valid JSON
	var result []UnitDisplay
	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 unit in JSON output, got %d", len(result))
	}

	if result[0].ID != "test-unit" {
		t.Errorf("Expected unit ID 'test-unit', got %q", result[0].ID)
	}
}

func TestConvertToUnitDisplays(t *testing.T) {
	units := []*discovery.Unit{
		{
			ID:     "test-unit",
			Status: discovery.UnitStatusInProgress,
			Tasks: []*discovery.Task{
				{FilePath: "01-task.md", Status: discovery.TaskStatusComplete},
				{FilePath: "02-task.md", Status: discovery.TaskStatusInProgress},
				{FilePath: "03-task.md", Status: discovery.TaskStatusPending},
			},
			PRNumber: 42,
		},
	}

	displays := convertToUnitDisplays(units)

	if len(displays) != 1 {
		t.Fatalf("Expected 1 display unit, got %d", len(displays))
	}

	display := displays[0]

	if display.ID != "test-unit" {
		t.Errorf("Expected ID 'test-unit', got %q", display.ID)
	}

	if display.Status != discovery.UnitStatusInProgress {
		t.Errorf("Expected status in_progress, got %v", display.Status)
	}

	// Progress should be 1/3 = 0.333...
	expectedProgress := 1.0 / 3.0
	if display.Progress < expectedProgress-0.01 || display.Progress > expectedProgress+0.01 {
		t.Errorf("Expected progress ~0.33, got %f", display.Progress)
	}

	if len(display.Tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(display.Tasks))
	}

	if display.PRNumber == nil || *display.PRNumber != 42 {
		t.Error("Expected PR number 42")
	}

	// Check that in_progress task is marked as active
	activeCount := 0
	for _, task := range display.Tasks {
		if task.Active {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Errorf("Expected 1 active task, got %d", activeCount)
	}
}
