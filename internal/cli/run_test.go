package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
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
	app := New()

	opts := RunOptions{
		Parallelism:  4,
		TargetBranch: "main",
		DryRun:       true,
		NoPR:         false,
		Unit:         "",
		SkipReview:   false,
		TasksDir:     "specs/tasks",
	}

	ctx := context.Background()

	// Capture stdout
	oldStdout := redirectStdout()
	defer restoreStdout(oldStdout)

	err := app.RunOrchestrator(ctx, opts)
	if err != nil {
		t.Fatalf("RunOrchestrator failed: %v", err)
	}

	// Note: In a real scenario, we'd capture and verify the output
	// For now, we just verify it doesn't error
}

func TestRunOrchestrator_SingleUnit(t *testing.T) {
	app := New()

	opts := RunOptions{
		Parallelism:  4,
		TargetBranch: "main",
		DryRun:       true, // Use dry-run to avoid actual execution
		NoPR:         false,
		Unit:         "testunit",
		SkipReview:   false,
		TasksDir:     "specs/tasks",
	}

	ctx := context.Background()

	// Capture stdout
	oldStdout := redirectStdout()
	defer restoreStdout(oldStdout)

	err := app.RunOrchestrator(ctx, opts)
	if err != nil {
		t.Fatalf("RunOrchestrator failed: %v", err)
	}

	// The --unit flag should be reflected in the dry-run output
	// Actual verification of single-unit mode execution is in task #9
}

// Helper to redirect stdout for testing
func redirectStdout() *bytes.Buffer {
	return new(bytes.Buffer)
}

// Helper to restore stdout
func restoreStdout(buf *bytes.Buffer) {
	// No-op for now, as we're not actually redirecting in these tests
}
