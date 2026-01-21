package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/provider"
	"github.com/RevCBH/choo/internal/scheduler"
	"github.com/RevCBH/choo/internal/worker"
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

func TestOrchestrator_HandleEvent_UnitCompleted(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	sched := scheduler.New(bus, 2)

	// Create a minimal unit for scheduling
	units := []*discovery.Unit{
		{ID: "unit-a", DependsOn: []string{}},
	}
	sched.Schedule(units)

	// Dispatch the unit
	result := sched.Dispatch()
	if result.Unit != "unit-a" {
		t.Fatalf("expected unit-a to be dispatched")
	}

	orch := &Orchestrator{
		bus:       bus,
		scheduler: sched,
		unitMap:   buildUnitMap(units),
	}

	// Subscribe to events
	bus.Subscribe(orch.handleEvent)

	// Emit completion event
	bus.Emit(events.NewEvent(events.UnitCompleted, "unit-a"))

	// Give event time to process
	time.Sleep(50 * time.Millisecond)

	// Check scheduler state
	state, ok := sched.GetState("unit-a")
	if !ok {
		t.Fatal("unit-a state not found")
	}
	if state.Status != scheduler.StatusComplete {
		t.Errorf("expected StatusComplete, got %v", state.Status)
	}
}

func TestOrchestrator_HandleEvent_UnitFailed(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	sched := scheduler.New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit-a", DependsOn: []string{}},
		{ID: "unit-b", DependsOn: []string{"unit-a"}},
	}
	sched.Schedule(units)

	// Dispatch unit-a
	sched.Dispatch()

	orch := &Orchestrator{
		bus:       bus,
		scheduler: sched,
		unitMap:   buildUnitMap(units),
	}

	bus.Subscribe(orch.handleEvent)

	// Emit failure event
	bus.Emit(events.NewEvent(events.UnitFailed, "unit-a").WithError(fmt.Errorf("test error")))

	time.Sleep(50 * time.Millisecond)

	// Check unit-a is failed
	stateA, _ := sched.GetState("unit-a")
	if stateA.Status != scheduler.StatusFailed {
		t.Errorf("expected unit-a StatusFailed, got %v", stateA.Status)
	}

	// Check unit-b is blocked
	stateB, _ := sched.GetState("unit-b")
	if stateB.Status != scheduler.StatusBlocked {
		t.Errorf("expected unit-b StatusBlocked, got %v", stateB.Status)
	}
}

func TestOrchestrator_HandleEvent_WithEscalator(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	sched := scheduler.New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit-a", DependsOn: []string{}},
	}
	sched.Schedule(units)
	sched.Dispatch()

	// Track escalations
	escalated := make(chan escalate.Escalation, 1)
	mockEscalator := &mockEscalator{
		escalateFn: func(ctx context.Context, e escalate.Escalation) error {
			escalated <- e
			return nil
		},
	}

	orch := &Orchestrator{
		bus:       bus,
		scheduler: sched,
		escalator: mockEscalator,
		unitMap:   buildUnitMap(units),
	}

	bus.Subscribe(orch.handleEvent)

	// Emit failure with specific error type
	bus.Emit(events.NewEvent(events.UnitFailed, "unit-a").
		WithError(fmt.Errorf("merge conflict detected")))

	// Wait for escalation
	select {
	case e := <-escalated:
		if e.Unit != "unit-a" {
			t.Errorf("expected unit-a, got %s", e.Unit)
		}
		if e.Severity != escalate.SeverityBlocking {
			t.Errorf("expected blocking severity for merge conflict")
		}
		if e.Context["error_type"] != "merge_conflict" {
			t.Errorf("expected merge_conflict error type")
		}
	case <-time.After(time.Second):
		t.Error("escalation not received")
	}
}

func TestCategorizeErrorSeverity(t *testing.T) {
	tests := []struct {
		err      error
		expected escalate.Severity
	}{
		{nil, escalate.SeverityInfo},
		{fmt.Errorf("merge conflict"), escalate.SeverityBlocking},
		{fmt.Errorf("review timeout"), escalate.SeverityWarning},
		{fmt.Errorf("baseline checks failed"), escalate.SeverityCritical},
		{fmt.Errorf("unknown error"), escalate.SeverityCritical},
	}

	for _, tc := range tests {
		got := categorizeErrorSeverity(tc.err)
		if got != tc.expected {
			t.Errorf("categorizeErrorSeverity(%v) = %v, want %v", tc.err, got, tc.expected)
		}
	}
}

// mockEscalator for testing
type mockEscalator struct {
	escalateFn func(ctx context.Context, e escalate.Escalation) error
}

func (m *mockEscalator) Escalate(ctx context.Context, e escalate.Escalation) error {
	if m.escalateFn != nil {
		return m.escalateFn(ctx, e)
	}
	return nil
}

func (m *mockEscalator) Name() string {
	return "mock"
}

func TestOrchestrator_Shutdown_Clean(t *testing.T) {
	bus := events.NewBus(100)

	cfg := Config{
		ShutdownTimeout: 5 * time.Second,
		Parallelism:     2,
	}

	orch := New(cfg, Dependencies{Bus: bus})

	// Initialize pool manually for testing
	workerCfg := worker.WorkerConfig{}
	workerDeps := worker.WorkerDeps{Events: bus}
	orch.pool = worker.NewPool(2, workerCfg, workerDeps)

	// Shutdown should complete cleanly
	ctx := context.Background()
	err := orch.shutdown(ctx)

	if err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}
}

func TestOrchestrator_Shutdown_NilComponents(t *testing.T) {
	// Test shutdown with nil pool and bus doesn't panic
	orch := &Orchestrator{
		cfg: Config{ShutdownTimeout: time.Second},
	}

	err := orch.shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error with nil components: %v", err)
	}
}

func TestOrchestrator_DefaultShutdownTimeout(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	// Don't set ShutdownTimeout
	cfg := Config{
		Parallelism: 1,
	}

	orch := New(cfg, Dependencies{Bus: bus})

	if orch.cfg.ShutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("expected default timeout %v, got %v",
			DefaultShutdownTimeout, orch.cfg.ShutdownTimeout)
	}
}

func TestOrchestrator_DryRun_Basic(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	units := []*discovery.Unit{
		{
			ID:        "unit-a",
			DependsOn: []string{},
			Tasks:     make([]*discovery.Task, 3),
		},
		{
			ID:        "unit-b",
			DependsOn: []string{"unit-a"},
			Tasks:     make([]*discovery.Task, 2),
		},
	}

	orch := &Orchestrator{
		cfg: Config{
			DryRun:      true,
			Parallelism: 4,
		},
		bus:     bus,
		unitMap: buildUnitMap(units),
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	result, err := orch.dryRun(units)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalUnits != 2 {
		t.Errorf("expected 2 total units, got %d", result.TotalUnits)
	}

	// Verify output contains expected information
	expectedStrings := []string{
		"Execution Plan",
		"Units to execute: 2",
		"Max parallelism: 4",
		"unit-a",
		"unit-b",
		"Topological order",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q", expected)
		}
	}
}

func TestOrchestrator_DryRun_TaskCounts(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	units := []*discovery.Unit{
		{
			ID:        "unit-a",
			DependsOn: []string{},
			Tasks: []*discovery.Task{
				{Number: 1},
				{Number: 2},
				{Number: 3},
			},
		},
	}

	orch := &Orchestrator{
		cfg: Config{
			DryRun:      true,
			Parallelism: 2,
		},
		bus:     bus,
		unitMap: buildUnitMap(units),
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_, err := orch.dryRun(units)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show task count
	if !strings.Contains(output, "(3 tasks)") {
		t.Errorf("expected output to contain task count, got: %s", output)
	}
}

func TestOrchestrator_DryRun_Levels(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	// Create a graph that will have multiple levels
	// Level 1: unit-a, unit-b (no deps)
	// Level 2: unit-c (depends on a), unit-d (depends on b)
	// Level 3: unit-e (depends on c and d)
	units := []*discovery.Unit{
		{ID: "unit-a", DependsOn: []string{}},
		{ID: "unit-b", DependsOn: []string{}},
		{ID: "unit-c", DependsOn: []string{"unit-a"}},
		{ID: "unit-d", DependsOn: []string{"unit-b"}},
		{ID: "unit-e", DependsOn: []string{"unit-c", "unit-d"}},
	}

	orch := &Orchestrator{
		cfg: Config{
			DryRun:      true,
			Parallelism: 4,
		},
		bus:     bus,
		unitMap: buildUnitMap(units),
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_, err := orch.dryRun(units)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 execution levels
	if !strings.Contains(output, "Execution levels: 3") {
		t.Errorf("expected 3 execution levels, got: %s", output)
	}

	// Verify level structure
	if !strings.Contains(output, "Level 1") {
		t.Error("expected Level 1 in output")
	}
	if !strings.Contains(output, "Level 2") {
		t.Error("expected Level 2 in output")
	}
	if !strings.Contains(output, "Level 3") {
		t.Error("expected Level 3 in output")
	}
}

func TestOrchestrator_DryRun_CycleDetection(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	// Create a cycle: a -> b -> c -> a
	units := []*discovery.Unit{
		{ID: "unit-a", DependsOn: []string{"unit-c"}},
		{ID: "unit-b", DependsOn: []string{"unit-a"}},
		{ID: "unit-c", DependsOn: []string{"unit-b"}},
	}

	orch := &Orchestrator{
		cfg: Config{
			DryRun:      true,
			Parallelism: 2,
		},
		bus:     bus,
		unitMap: buildUnitMap(units),
	}

	_, err := orch.dryRun(units)

	if err == nil {
		t.Error("expected error for cyclic dependencies")
	}
}

func TestOrchestrator_Run_DryRunMode(t *testing.T) {
	// Integration test: Run() with DryRun=true should call dryRun()
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

	bus := events.NewBus(100)
	defer bus.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	orch := New(Config{
		TasksDir:    tasksDir,
		Parallelism: 1,
		DryRun:      true,
	}, Dependencies{
		Bus: bus,
	})

	ctx := context.Background()
	result, err := orch.Run(ctx)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalUnits != 1 {
		t.Errorf("expected 1 unit, got %d", result.TotalUnits)
	}

	// Should have printed execution plan
	if !strings.Contains(output, "Execution Plan") {
		t.Error("expected Execution Plan header in output")
	}
}

func TestFilterOutCompleteUnits(t *testing.T) {
	tests := []struct {
		name     string
		units    []*discovery.Unit
		expected []string // IDs of units that should remain
	}{
		{
			name:     "empty list",
			units:    []*discovery.Unit{},
			expected: []string{},
		},
		{
			name: "no complete units",
			units: []*discovery.Unit{
				{ID: "unit-a", Status: discovery.UnitStatusPending},
				{ID: "unit-b", Status: discovery.UnitStatusInProgress},
			},
			expected: []string{"unit-a", "unit-b"},
		},
		{
			name: "all complete units",
			units: []*discovery.Unit{
				{ID: "unit-a", Status: discovery.UnitStatusComplete},
				{ID: "unit-b", Status: discovery.UnitStatusComplete},
			},
			expected: []string{},
		},
		{
			name: "mixed - filters out complete",
			units: []*discovery.Unit{
				{ID: "unit-a", Status: discovery.UnitStatusComplete},
				{ID: "unit-b", Status: discovery.UnitStatusPending},
				{ID: "unit-c", Status: discovery.UnitStatusInProgress},
				{ID: "unit-d", Status: discovery.UnitStatusComplete},
			},
			expected: []string{"unit-b", "unit-c"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := filterOutCompleteUnits(tc.units)

			if len(result) != len(tc.expected) {
				t.Fatalf("expected %d units, got %d", len(tc.expected), len(result))
			}

			for i, unit := range result {
				if unit.ID != tc.expected[i] {
					t.Errorf("expected unit %s at position %d, got %s", tc.expected[i], i, unit.ID)
				}
			}
		})
	}
}

func TestOrchestrator_DryRun_FiltersCompleteUnits(t *testing.T) {
	// Test that dry-run excludes already-complete units
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")

	// Create complete-unit with all tasks complete
	completeDir := filepath.Join(tasksDir, "complete-unit")
	os.MkdirAll(completeDir, 0755)
	os.WriteFile(filepath.Join(completeDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: complete-unit
depends_on: []
---
# Complete Unit
`), 0644)
	os.WriteFile(filepath.Join(completeDir, "01-task.md"), []byte(`---
task: 1
status: complete
backpressure: "echo ok"
---
# Task 1
`), 0644)

	// Create pending-unit with pending tasks
	pendingDir := filepath.Join(tasksDir, "pending-unit")
	os.MkdirAll(pendingDir, 0755)
	os.WriteFile(filepath.Join(pendingDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: pending-unit
depends_on: []
---
# Pending Unit
`), 0644)
	os.WriteFile(filepath.Join(pendingDir, "01-task.md"), []byte(`---
task: 1
status: pending
backpressure: "echo ok"
---
# Task 1
`), 0644)

	bus := events.NewBus(100)
	defer bus.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	orch := New(Config{
		TasksDir:    tasksDir,
		Parallelism: 1,
		DryRun:      true,
	}, Dependencies{
		Bus: bus,
	})

	ctx := context.Background()
	result, err := orch.Run(ctx)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only include the pending unit
	if result.TotalUnits != 1 {
		t.Errorf("expected 1 unit (pending only), got %d", result.TotalUnits)
	}

	// Output should contain pending-unit but not complete-unit
	if !strings.Contains(output, "pending-unit") {
		t.Error("expected output to contain pending-unit")
	}
	if strings.Contains(output, "complete-unit") {
		t.Error("expected output NOT to contain complete-unit")
	}
}

func TestOrchestrator_DryRun_AllUnitsComplete(t *testing.T) {
	// Test that when all units are complete, dry-run returns early
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")

	// Create only complete units
	completeDir := filepath.Join(tasksDir, "complete-unit")
	os.MkdirAll(completeDir, 0755)
	os.WriteFile(filepath.Join(completeDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: complete-unit
depends_on: []
---
# Complete Unit
`), 0644)
	os.WriteFile(filepath.Join(completeDir, "01-task.md"), []byte(`---
task: 1
status: complete
backpressure: "echo ok"
---
# Task 1
`), 0644)

	bus := events.NewBus(100)
	defer bus.Close()

	orch := New(Config{
		TasksDir:    tasksDir,
		Parallelism: 1,
		DryRun:      true,
	}, Dependencies{
		Bus: bus,
	})

	ctx := context.Background()
	result, err := orch.Run(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All units complete, so TotalUnits should be 0
	if result.TotalUnits != 0 {
		t.Errorf("expected 0 units when all are complete, got %d", result.TotalUnits)
	}
}

func TestOrchestrator_SingleUnit_FiltersCompleteDependencies(t *testing.T) {
	// Test that when targeting a single unit, complete dependencies are filtered out
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")

	// Create dependency unit that is complete
	depDir := filepath.Join(tasksDir, "dep-unit")
	os.MkdirAll(depDir, 0755)
	os.WriteFile(filepath.Join(depDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: dep-unit
depends_on: []
---
# Dependency Unit
`), 0644)
	os.WriteFile(filepath.Join(depDir, "01-task.md"), []byte(`---
task: 1
status: complete
backpressure: "echo ok"
---
# Task 1
`), 0644)

	// Create target unit that depends on complete unit
	targetDir := filepath.Join(tasksDir, "target-unit")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: target-unit
depends_on: [dep-unit]
---
# Target Unit
`), 0644)
	os.WriteFile(filepath.Join(targetDir, "01-task.md"), []byte(`---
task: 1
status: pending
backpressure: "echo ok"
---
# Task 1
`), 0644)

	bus := events.NewBus(100)
	defer bus.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	orch := New(Config{
		TasksDir:    tasksDir,
		Parallelism: 1,
		SingleUnit:  "target-unit",
		DryRun:      true,
	}, Dependencies{
		Bus: bus,
	})

	ctx := context.Background()
	result, err := orch.Run(ctx)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only include target-unit (dependency is complete)
	if result.TotalUnits != 1 {
		t.Errorf("expected 1 unit (target only), got %d", result.TotalUnits)
	}

	// Output should contain target-unit but not dep-unit
	if !strings.Contains(output, "target-unit") {
		t.Error("expected output to contain target-unit")
	}
	if strings.Contains(output, "dep-unit") {
		t.Error("expected output NOT to contain dep-unit (it's complete)")
	}
}

func TestResolveProviderForUnit_Precedence(t *testing.T) {
	tests := []struct {
		name         string
		cfg          Config
		unitProvider string
		expectedType provider.ProviderType
	}{
		{
			name: "force_overrides_all",
			cfg: Config{
				ForceTaskProvider: "codex",
				DefaultProvider:   "claude",
				ProviderConfig: config.ProviderConfig{
					Type: "claude",
				},
			},
			unitProvider: "claude",
			expectedType: provider.ProviderCodex,
		},
		{
			name: "frontmatter_overrides_default",
			cfg: Config{
				DefaultProvider: "claude",
			},
			unitProvider: "codex",
			expectedType: provider.ProviderCodex,
		},
		{
			name: "default_used_when_no_frontmatter",
			cfg: Config{
				DefaultProvider: "codex",
			},
			unitProvider: "",
			expectedType: provider.ProviderCodex,
		},
		{
			name: "config_used_when_no_default",
			cfg: Config{
				ProviderConfig: config.ProviderConfig{
					Type: "codex",
				},
			},
			unitProvider: "",
			expectedType: provider.ProviderCodex,
		},
		{
			name:         "claude_is_fallback",
			cfg:          Config{},
			unitProvider: "",
			expectedType: provider.ProviderClaude,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := Dependencies{Bus: events.NewBus(100)}
			o := New(tt.cfg, deps)

			unit := &discovery.Unit{
				ID:       "test-unit",
				Provider: tt.unitProvider,
			}

			prov, err := o.resolveProviderForUnit(unit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if prov.Name() != tt.expectedType {
				t.Errorf("expected %s, got %s", tt.expectedType, prov.Name())
			}
		})
	}
}

func TestResolveProviderForUnit_CommandOverride(t *testing.T) {
	cfg := Config{
		DefaultProvider: "codex",
		ProviderConfig: config.ProviderConfig{
			Providers: map[config.ProviderType]config.ProviderSettings{
				"codex": {
					Command: "/custom/path/to/codex",
				},
			},
		},
	}
	deps := Dependencies{Bus: events.NewBus(100)}
	o := New(cfg, deps)

	unit := &discovery.Unit{ID: "test-unit"}

	prov, err := o.resolveProviderForUnit(unit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Provider should be created successfully with custom command
	if prov.Name() != provider.ProviderCodex {
		t.Errorf("expected codex, got %s", prov.Name())
	}
}
