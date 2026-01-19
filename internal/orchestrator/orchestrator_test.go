package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/choo/internal/events"
)

func TestOrchestrator_Run_Discovery(t *testing.T) {
	// Create temp directory with test tasks
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	unitDir := filepath.Join(tasksDir, "test-unit")
	os.MkdirAll(unitDir, 0755)

	// Create IMPLEMENTATION_PLAN.md
	implPlan := `---
unit: test-unit
depends_on: []
---
# Test Unit
`
	os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(implPlan), 0644)

	// Create a task file
	taskFile := `---
task: 1
status: in_progress
backpressure: "echo ok"
depends_on: []
---
# Task 1
`
	os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(taskFile), 0644)

	// Create orchestrator
	bus := events.NewBus(100)
	defer bus.Close()

	orch := New(Config{
		TasksDir:    tasksDir,
		Parallelism: 1,
		DryRun:      true, // Use dry-run to test discovery without full execution
	}, Dependencies{
		Bus: bus,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := orch.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalUnits != 1 {
		t.Errorf("expected 1 unit, got %d", result.TotalUnits)
	}
}

func TestOrchestrator_Run_SingleUnit(t *testing.T) {
	// Create temp directory with multiple units
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")

	// Create unit-a
	unitADir := filepath.Join(tasksDir, "unit-a")
	os.MkdirAll(unitADir, 0755)
	os.WriteFile(filepath.Join(unitADir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: unit-a
depends_on: []
---
# Unit A
`), 0644)
	os.WriteFile(filepath.Join(unitADir, "01-task.md"), []byte(`---
task: 1
status: in_progress
backpressure: "echo ok"
depends_on: []
---
# Task 1
`), 0644)

	// Create unit-b
	unitBDir := filepath.Join(tasksDir, "unit-b")
	os.MkdirAll(unitBDir, 0755)
	os.WriteFile(filepath.Join(unitBDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: unit-b
depends_on: [unit-a]
---
# Unit B
`), 0644)
	os.WriteFile(filepath.Join(unitBDir, "01-task.md"), []byte(`---
task: 1
status: in_progress
backpressure: "echo ok"
depends_on: []
---
# Task 1
`), 0644)

	bus := events.NewBus(100)
	defer bus.Close()

	// Run with single unit mode targeting unit-b
	orch := New(Config{
		TasksDir:    tasksDir,
		Parallelism: 1,
		SingleUnit:  "unit-b",
		DryRun:      true,
	}, Dependencies{
		Bus: bus,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := orch.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should include unit-b and its dependency unit-a
	if result.TotalUnits != 2 {
		t.Errorf("expected 2 units (unit-b + dependency), got %d", result.TotalUnits)
	}
}

func TestOrchestrator_Run_UnitNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	bus := events.NewBus(100)
	defer bus.Close()

	orch := New(Config{
		TasksDir:    tasksDir,
		Parallelism: 1,
		SingleUnit:  "nonexistent",
	}, Dependencies{
		Bus: bus,
	})

	ctx := context.Background()
	_, err := orch.Run(ctx)

	if err == nil {
		t.Error("expected error for nonexistent unit")
	}
}
