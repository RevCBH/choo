package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunCmd_DefaultFlags(t *testing.T) {
	app := New()
	cmd := NewRunCmd(app)

	// Get flag values
	parallelismFlag := cmd.Flags().Lookup("parallelism")
	if parallelismFlag == nil {
		t.Fatal("parallelism flag not found")
	}
	if parallelismFlag.DefValue != "4" {
		t.Errorf("Expected default parallelism 4, got %s", parallelismFlag.DefValue)
	}

	targetFlag := cmd.Flags().Lookup("target")
	if targetFlag == nil {
		t.Fatal("target flag not found")
	}
	if targetFlag.DefValue != "main" {
		t.Errorf("Expected default target 'main', got %s", targetFlag.DefValue)
	}
}

func TestRunCmd_CustomFlags(t *testing.T) {
	app := New()
	cmd := NewRunCmd(app)

	// Get flag values after setting args
	cmd.SetArgs([]string{
		"--parallelism", "8",
		"--target", "develop",
		"--dry-run",
		"--no-pr",
		"--unit", "myunit",
		"--skip-review",
	})

	// Parse flags
	err := cmd.ParseFlags([]string{
		"--parallelism", "8",
		"--target", "develop",
		"--dry-run",
		"--no-pr",
		"--unit", "myunit",
		"--skip-review",
	})
	if err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	// Verify flag values
	parallelismFlag, err := cmd.Flags().GetInt("parallelism")
	if err != nil {
		t.Fatalf("failed to get parallelism flag: %v", err)
	}
	if parallelismFlag != 8 {
		t.Errorf("Expected parallelism 8, got %d", parallelismFlag)
	}

	targetFlag, err := cmd.Flags().GetString("target")
	if err != nil {
		t.Fatalf("failed to get target flag: %v", err)
	}
	if targetFlag != "develop" {
		t.Errorf("Expected target 'develop', got %s", targetFlag)
	}

	dryRunFlag, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		t.Fatalf("failed to get dry-run flag: %v", err)
	}
	if !dryRunFlag {
		t.Error("Expected dry-run to be true")
	}

	noPRFlag, err := cmd.Flags().GetBool("no-pr")
	if err != nil {
		t.Fatalf("failed to get no-pr flag: %v", err)
	}
	if !noPRFlag {
		t.Error("Expected no-pr to be true")
	}

	unitFlag, err := cmd.Flags().GetString("unit")
	if err != nil {
		t.Fatalf("failed to get unit flag: %v", err)
	}
	if unitFlag != "myunit" {
		t.Errorf("Expected unit 'myunit', got %s", unitFlag)
	}

	skipReviewFlag, err := cmd.Flags().GetBool("skip-review")
	if err != nil {
		t.Fatalf("failed to get skip-review flag: %v", err)
	}
	if !skipReviewFlag {
		t.Error("Expected skip-review to be true")
	}
}

func TestRunOptions_Validate_Valid(t *testing.T) {
	opts := RunOptions{
		Parallelism:  4,
		TargetBranch: "main",
		TasksDir:     "specs/tasks",
	}

	err := opts.Validate()
	if err != nil {
		t.Errorf("Valid options should pass validation, got error: %v", err)
	}
}

func TestRunOptions_Validate_ZeroParallelism(t *testing.T) {
	opts := RunOptions{
		Parallelism:  0,
		TargetBranch: "main",
		TasksDir:     "specs/tasks",
	}

	err := opts.Validate()
	if err == nil {
		t.Error("Expected error for parallelism=0, got nil")
	}
	if !strings.Contains(err.Error(), "parallelism must be greater than 0") {
		t.Errorf("Expected error message about parallelism, got: %v", err)
	}
}

func TestRunOptions_Validate_EmptyTasksDir(t *testing.T) {
	opts := RunOptions{
		Parallelism:  4,
		TargetBranch: "main",
		TasksDir:     "",
	}

	err := opts.Validate()
	if err == nil {
		t.Error("Expected error for empty tasks dir, got nil")
	}
	if !strings.Contains(err.Error(), "tasks directory must not be empty") {
		t.Errorf("Expected error message about tasks directory, got: %v", err)
	}
}

func TestRunOrchestrator_DryRun(t *testing.T) {
	// Create temp directory with tasks
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	unitDir := filepath.Join(tasksDir, "test-unit")
	os.MkdirAll(unitDir, 0755)

	os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: test-unit
depends_on: []
---
# Test Unit
`), 0644)

	os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(`---
task: 1
status: in_progress
backpressure: "echo ok"
depends_on: []
---
# Task 1
`), 0644)

	// Create a minimal config to avoid auto-detect
	os.WriteFile(filepath.Join(tmpDir, ".choo.yaml"), []byte(`github:
  owner: testowner
  repo: testrepo
`), 0644)

	// Change to tmpDir for config loading
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app := New()
	opts := RunOptions{
		TasksDir:    tasksDir,
		Parallelism: 1,
		DryRun:      true,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := app.RunOrchestrator(ctx, opts)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Execution Plan") {
		t.Error("expected dry-run output")
	}
}

func TestRunOrchestrator_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	unitDir := filepath.Join(tasksDir, "test-unit")
	os.MkdirAll(unitDir, 0755)

	os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: test-unit
depends_on: []
---
# Test Unit
`), 0644)

	os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(`---
task: 1
status: in_progress
backpressure: "sleep 60"
depends_on: []
---
# Long Task
`), 0644)

	// Create a minimal config to avoid auto-detect
	os.WriteFile(filepath.Join(tmpDir, ".choo.yaml"), []byte(`github:
  owner: testowner
  repo: testrepo
`), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app := New()
	opts := RunOptions{
		TasksDir:    tasksDir,
		Parallelism: 1,
	}

	// Cancel after short delay
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := app.RunOrchestrator(ctx, opts)

	// Should return context cancellation
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRunOrchestrator_InvalidTasksDir(t *testing.T) {
	app := New()
	opts := RunOptions{
		TasksDir:    "/nonexistent/path",
		Parallelism: 1,
	}

	ctx := context.Background()
	err := app.RunOrchestrator(ctx, opts)

	if err == nil {
		t.Error("expected error for invalid tasks directory")
	}
}

func TestRunCmd_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	unitDir := filepath.Join(tasksDir, "test-unit")
	os.MkdirAll(unitDir, 0755)

	os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: test-unit
depends_on: []
---
# Test Unit
`), 0644)

	os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(`---
task: 1
status: in_progress
backpressure: "echo ok"
depends_on: []
---
# Task 1
`), 0644)

	// Create a minimal config to avoid auto-detect
	os.WriteFile(filepath.Join(tmpDir, ".choo.yaml"), []byte(`github:
  owner: testowner
  repo: testrepo
`), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app := New()
	cmd := NewRunCmd(app)

	// Set args for dry-run
	cmd.SetArgs([]string{tasksDir, "--dry-run"})

	// Capture output
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("command execution failed: %v", err)
	}
}
