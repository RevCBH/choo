---
task: 5
status: pending
backpressure: "go test ./internal/orchestrator/... -run TestOrchestrator_DryRun"
depends_on: [1, 2]
---

# Orchestrator Dry-Run Mode

**Parent spec**: `/specs/ORCHESTRATOR.md`
**Task**: #5 of 6 in implementation plan

## Objective

Implement dry-run mode that shows the execution plan without actually running workers.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides Unit type with Tasks
- SCHEDULER - provides Schedule() returning Schedule with Levels and TopologicalOrder

### Task Dependencies (within this unit)
- Task #1 - Core types must be defined
- Task #2 - Run() calls dryRun() when DryRun config is true

### Package Dependencies
- `github.com/anthropics/choo/internal/discovery`
- `github.com/anthropics/choo/internal/scheduler`

## Deliverables

### Files to Create/Modify
```
internal/orchestrator/
├── orchestrator.go      # MODIFY: Add dryRun() method
└── orchestrator_test.go # MODIFY: Add dry-run tests
```

### Functions to Implement

```go
// dryRun prints the execution plan without running workers
func (o *Orchestrator) dryRun(units []*discovery.Unit) (*Result, error) {
	// Build schedule without executing
	sched := scheduler.New(o.bus, o.cfg.Parallelism)
	schedule, err := sched.Schedule(units)
	if err != nil {
		return nil, err
	}

	// Build unit map for task counts
	unitMap := buildUnitMap(units)

	// Print execution plan
	fmt.Printf("Execution Plan\n")
	fmt.Printf("==============\n\n")
	fmt.Printf("Units to execute: %d\n", len(units))
	fmt.Printf("Max parallelism: %d\n", o.cfg.Parallelism)
	fmt.Printf("Execution levels: %d\n\n", len(schedule.Levels))

	for i, level := range schedule.Levels {
		fmt.Printf("Level %d (parallel):\n", i+1)
		for _, unitID := range level {
			unit := unitMap[unitID]
			taskCount := 0
			if unit != nil {
				taskCount = len(unit.Tasks)
			}
			fmt.Printf("  - %s (%d tasks)\n", unitID, taskCount)
		}
		fmt.Println()
	}

	fmt.Printf("Topological order:\n")
	for i, unitID := range schedule.TopologicalOrder {
		fmt.Printf("  %d. %s\n", i+1, unitID)
	}

	return &Result{
		TotalUnits: len(units),
	}, nil
}
```

### Tests to Implement

```go
// Add to internal/orchestrator/orchestrator_test.go

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
status: pending
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
```

## Backpressure

### Validation Command
```bash
go test ./internal/orchestrator/... -run TestOrchestrator_DryRun
```

### Must Pass
| Test | Assertion |
|------|-----------|
| TestOrchestrator_DryRun_Basic | Execution plan printed with unit names |
| TestOrchestrator_DryRun_TaskCounts | Task counts shown for each unit |
| TestOrchestrator_DryRun_Levels | Execution levels correctly computed |
| TestOrchestrator_DryRun_CycleDetection | Cyclic dependencies return error |
| TestOrchestrator_Run_DryRunMode | Run() invokes dryRun() when flag set |

## NOT In Scope
- CLI integration (task #6)
- Actual worker execution
- Event emission in dry-run mode
