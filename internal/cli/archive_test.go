package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveCompleted_MovesUnitAndSpec(t *testing.T) {
	tmpDir := t.TempDir()

	specsDir := filepath.Join(tmpDir, "specs")
	tasksDir := filepath.Join(specsDir, "tasks")
	unitDir := filepath.Join(tasksDir, "unit-one")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("failed to create unit dir: %v", err)
	}

	implPlan := filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md")
	if err := os.WriteFile(implPlan, []byte(`---
unit: unit-one
orch_status: complete
---
# Plan
`), 0644); err != nil {
		t.Fatalf("failed to write implementation plan: %v", err)
	}

	taskFile := filepath.Join(unitDir, "01-task.md")
	if err := os.WriteFile(taskFile, []byte(`---
task: 1
status: complete
backpressure: "true"
---
# Task
`), 0644); err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}

	specFile := filepath.Join(specsDir, "UNIT-ONE.md")
	if err := os.WriteFile(specFile, []byte("# Spec"), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	app := New()
	err := app.ArchiveCompleted(ArchiveOptions{TasksDir: tasksDir})
	if err != nil {
		t.Fatalf("ArchiveCompleted failed: %v", err)
	}

	archivedUnitDir := filepath.Join(specsDir, "completed", "tasks", "unit-one")
	if _, err := os.Stat(archivedUnitDir); err != nil {
		t.Fatalf("expected archived unit dir to exist: %v", err)
	}

	archivedSpec := filepath.Join(specsDir, "completed", "UNIT-ONE.md")
	if _, err := os.Stat(archivedSpec); err != nil {
		t.Fatalf("expected archived spec file to exist: %v", err)
	}

	if _, err := os.Stat(unitDir); !os.IsNotExist(err) {
		t.Fatalf("expected unit dir to be moved, got err=%v", err)
	}

	if _, err := os.Stat(specFile); !os.IsNotExist(err) {
		t.Fatalf("expected spec file to be moved, got err=%v", err)
	}
}

func TestArchiveCompleted_SkipsIncompleteUnits(t *testing.T) {
	tmpDir := t.TempDir()

	specsDir := filepath.Join(tmpDir, "specs")
	tasksDir := filepath.Join(specsDir, "tasks")
	unitDir := filepath.Join(tasksDir, "unit-two")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("failed to create unit dir: %v", err)
	}

	implPlan := filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md")
	if err := os.WriteFile(implPlan, []byte(`---
unit: unit-two
orch_status: in_progress
---
# Plan
`), 0644); err != nil {
		t.Fatalf("failed to write implementation plan: %v", err)
	}

	taskFile := filepath.Join(unitDir, "01-task.md")
	if err := os.WriteFile(taskFile, []byte(`---
task: 1
status: complete
backpressure: "true"
---
# Task
`), 0644); err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}

	app := New()
	err := app.ArchiveCompleted(ArchiveOptions{TasksDir: tasksDir})
	if err != nil {
		t.Fatalf("ArchiveCompleted failed: %v", err)
	}

	if _, err := os.Stat(unitDir); err != nil {
		t.Fatalf("expected unit dir to remain, got err=%v", err)
	}
}

func TestArchiveCompleted_AllTasksComplete_NoUnitStatus(t *testing.T) {
	tmpDir := t.TempDir()

	specsDir := filepath.Join(tmpDir, "specs")
	tasksDir := filepath.Join(specsDir, "tasks")
	unitDir := filepath.Join(tasksDir, "unit-three")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("failed to create unit dir: %v", err)
	}

	implPlan := filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md")
	if err := os.WriteFile(implPlan, []byte(`---
unit: unit-three
---
# Plan
`), 0644); err != nil {
		t.Fatalf("failed to write implementation plan: %v", err)
	}

	taskFile := filepath.Join(unitDir, "01-task.md")
	if err := os.WriteFile(taskFile, []byte(`---
task: 1
status: complete
backpressure: "true"
---
# Task
`), 0644); err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}

	app := New()
	err := app.ArchiveCompleted(ArchiveOptions{TasksDir: tasksDir})
	if err != nil {
		t.Fatalf("ArchiveCompleted failed: %v", err)
	}

	archivedUnitDir := filepath.Join(specsDir, "completed", "tasks", "unit-three")
	if _, err := os.Stat(archivedUnitDir); err != nil {
		t.Fatalf("expected archived unit dir to exist: %v", err)
	}
}
