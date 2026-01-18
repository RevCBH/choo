package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupCmd_DefaultFlags(t *testing.T) {
	app := New()
	cmd := NewCleanupCmd(app)

	// Get flag values
	resetStateFlag := cmd.Flags().Lookup("reset-state")
	if resetStateFlag == nil {
		t.Fatal("reset-state flag not found")
	}
	if resetStateFlag.DefValue != "false" {
		t.Errorf("Expected default reset-state false, got %s", resetStateFlag.DefValue)
	}
}

func TestCleanupCmd_ResetStateFlag(t *testing.T) {
	app := New()
	cmd := NewCleanupCmd(app)

	// Parse flags with --reset-state
	err := cmd.ParseFlags([]string{"--reset-state"})
	if err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	// Verify flag value
	resetStateFlag, err := cmd.Flags().GetBool("reset-state")
	if err != nil {
		t.Fatalf("failed to get reset-state flag: %v", err)
	}
	if !resetStateFlag {
		t.Error("Expected reset-state to be true")
	}
}

func TestCleanup_NoWorktrees(t *testing.T) {
	app := New()

	// Create temporary directory for test
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "specs", "tasks")
	os.MkdirAll(tasksDir, 0755)

	opts := CleanupOptions{
		TasksDir:   tasksDir,
		ResetState: false,
	}

	err := app.Cleanup(opts)
	if err != nil {
		t.Errorf("Cleanup should handle no worktrees gracefully, got error: %v", err)
	}
}

func TestCleanup_RemovesWorktrees(t *testing.T) {
	app := New()

	// Create temporary directory structure
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "specs", "tasks")
	os.MkdirAll(tasksDir, 0755)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create worktree directories
	worktreeBase := ".ralph/worktrees"
	worktree1 := filepath.Join(worktreeBase, "unit-a")
	worktree2 := filepath.Join(worktreeBase, "unit-b")

	os.MkdirAll(worktree1, 0755)
	os.MkdirAll(worktree2, 0755)

	// Create dummy files in worktrees
	os.WriteFile(filepath.Join(worktree1, "test.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(worktree2, "test.txt"), []byte("test"), 0644)

	// Verify worktrees exist
	if _, err := os.Stat(worktree1); os.IsNotExist(err) {
		t.Fatal("worktree1 should exist before cleanup")
	}
	if _, err := os.Stat(worktree2); os.IsNotExist(err) {
		t.Fatal("worktree2 should exist before cleanup")
	}

	opts := CleanupOptions{
		TasksDir:   tasksDir,
		ResetState: false,
	}

	err := app.Cleanup(opts)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify worktrees are removed
	if _, err := os.Stat(worktree1); !os.IsNotExist(err) {
		t.Error("worktree1 should be removed after cleanup")
	}
	if _, err := os.Stat(worktree2); !os.IsNotExist(err) {
		t.Error("worktree2 should be removed after cleanup")
	}
}

func TestCleanup_WithResetState(t *testing.T) {
	app := New()

	// Create temporary directory structure
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "specs", "tasks")
	unitDir := filepath.Join(tasksDir, "test-unit")
	os.MkdirAll(unitDir, 0755)

	// Create IMPLEMENTATION_PLAN.md with orchestrator state
	implPlanContent := `---
unit: test-unit
orch_status: in_progress
orch_branch: ralph/test-branch
orch_pr_number: 123
orch_started_at: "2024-01-01T00:00:00Z"
orch_completed_at: null
---

# Test Unit
`
	os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(implPlanContent), 0644)

	// Create task file with in_progress status
	taskContent := `---
task: 1
status: in_progress
backpressure: "go test"
depends_on: []
---

# Test Task
`
	os.WriteFile(filepath.Join(unitDir, "01-test-task.md"), []byte(taskContent), 0644)

	opts := CleanupOptions{
		TasksDir:   tasksDir,
		ResetState: true,
	}

	err := app.Cleanup(opts)
	if err != nil {
		t.Fatalf("Cleanup with reset-state failed: %v", err)
	}

	// Read and verify IMPLEMENTATION_PLAN.md was reset
	updatedImplPlan, err := os.ReadFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"))
	if err != nil {
		t.Fatalf("Failed to read updated IMPLEMENTATION_PLAN.md: %v", err)
	}

	implPlanStr := string(updatedImplPlan)
	if !strings.Contains(implPlanStr, "orch_status: pending") {
		t.Error("IMPLEMENTATION_PLAN.md should have orch_status: pending")
	}
	if strings.Contains(implPlanStr, "ralph/test-branch") {
		t.Error("IMPLEMENTATION_PLAN.md should not contain branch name after reset")
	}

	// Read and verify task file was reset
	updatedTask, err := os.ReadFile(filepath.Join(unitDir, "01-test-task.md"))
	if err != nil {
		t.Fatalf("Failed to read updated task file: %v", err)
	}

	taskStr := string(updatedTask)
	if !strings.Contains(taskStr, "status: pending") {
		t.Error("Task file should have status: pending")
	}
}

func TestResetFrontmatterState(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "specs", "tasks")

	// Create multiple units
	unit1Dir := filepath.Join(tasksDir, "unit-1")
	unit2Dir := filepath.Join(tasksDir, "unit-2")
	os.MkdirAll(unit1Dir, 0755)
	os.MkdirAll(unit2Dir, 0755)

	// Unit 1 - with 2 tasks
	implPlan1 := `---
unit: unit-1
orch_status: complete
---

# Unit 1
`
	os.WriteFile(filepath.Join(unit1Dir, "IMPLEMENTATION_PLAN.md"), []byte(implPlan1), 0644)

	task1_1 := `---
task: 1
status: complete
backpressure: "test"
---

# Task 1
`
	task1_2 := `---
task: 2
status: in_progress
backpressure: "test"
---

# Task 2
`
	os.WriteFile(filepath.Join(unit1Dir, "01-task.md"), []byte(task1_1), 0644)
	os.WriteFile(filepath.Join(unit1Dir, "02-task.md"), []byte(task1_2), 0644)

	// Unit 2 - with 1 task
	implPlan2 := `---
unit: unit-2
orch_status: in_progress
---

# Unit 2
`
	os.WriteFile(filepath.Join(unit2Dir, "IMPLEMENTATION_PLAN.md"), []byte(implPlan2), 0644)

	task2_1 := `---
task: 1
status: complete
backpressure: "test"
---

# Task 1
`
	os.WriteFile(filepath.Join(unit2Dir, "01-task.md"), []byte(task2_1), 0644)

	// Reset state
	unitCount, taskCount, err := resetFrontmatterState(tasksDir)
	if err != nil {
		t.Fatalf("resetFrontmatterState failed: %v", err)
	}

	// Verify counts
	if unitCount != 2 {
		t.Errorf("Expected 2 units reset, got %d", unitCount)
	}
	if taskCount != 3 {
		t.Errorf("Expected 3 tasks reset, got %d", taskCount)
	}

	// Verify all statuses are pending
	units := []string{"unit-1", "unit-2"}
	for _, unit := range units {
		implPlan, _ := os.ReadFile(filepath.Join(tasksDir, unit, "IMPLEMENTATION_PLAN.md"))
		if !strings.Contains(string(implPlan), "orch_status: pending") {
			t.Errorf("%s IMPLEMENTATION_PLAN.md should have orch_status: pending", unit)
		}
	}

	// Check task files
	task1_1Content, _ := os.ReadFile(filepath.Join(unit1Dir, "01-task.md"))
	if !strings.Contains(string(task1_1Content), "status: pending") {
		t.Error("unit-1/01-task.md should have status: pending")
	}

	task1_2Content, _ := os.ReadFile(filepath.Join(unit1Dir, "02-task.md"))
	if !strings.Contains(string(task1_2Content), "status: pending") {
		t.Error("unit-1/02-task.md should have status: pending")
	}

	task2_1Content, _ := os.ReadFile(filepath.Join(unit2Dir, "01-task.md"))
	if !strings.Contains(string(task2_1Content), "status: pending") {
		t.Error("unit-2/01-task.md should have status: pending")
	}
}
