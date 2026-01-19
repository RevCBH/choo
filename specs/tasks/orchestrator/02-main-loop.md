---
task: 2
status: complete
backpressure: "go test ./internal/orchestrator/... -run TestOrchestrator_Run"
depends_on: [1]
---

# Orchestrator Main Loop

**Parent spec**: `/specs/ORCHESTRATOR.md`
**Task**: #2 of 6 in implementation plan

## Objective

Implement the main Run() method that executes the orchestration loop: discovery, scheduling, and dispatch.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides Discover() function
- SCHEDULER - provides Scheduler with Schedule(), Dispatch(), Complete(), Fail()
- WORKER - provides Pool with Submit()

### Task Dependencies (within this unit)
- Task #1 - Core types must be defined

### Package Dependencies
- `github.com/anthropics/choo/internal/discovery`
- `github.com/anthropics/choo/internal/scheduler`
- `github.com/anthropics/choo/internal/worker`
- `github.com/anthropics/choo/internal/events`

## Deliverables

### Files to Create/Modify
```
internal/orchestrator/
├── orchestrator.go    # MODIFY: Add Run() method
└── orchestrator_test.go # CREATE: Tests for Run()
```

### Functions to Implement

```go
// Run executes the orchestration loop until completion or cancellation
// Returns Result with execution statistics
func (o *Orchestrator) Run(ctx context.Context) (*Result, error) {
	startTime := time.Now()

	// 1. Discovery phase
	units, err := discovery.Discover(o.cfg.TasksDir)
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	// Filter to single unit if specified
	if o.cfg.SingleUnit != "" {
		units = filterToUnit(units, o.cfg.SingleUnit)
		if len(units) == 0 {
			return nil, fmt.Errorf("unit %q not found", o.cfg.SingleUnit)
		}
	}

	o.units = units
	o.unitMap = buildUnitMap(units)

	// Handle dry-run mode
	if o.cfg.DryRun {
		return o.dryRun(units)
	}

	// Emit orchestrator started event
	o.bus.Emit(events.NewEvent(events.OrchStarted, "").WithPayload(map[string]any{
		"unit_count":  len(units),
		"parallelism": o.cfg.Parallelism,
	}))

	// 2. Build schedule
	o.scheduler = scheduler.New(o.bus, o.cfg.Parallelism)
	_, err = o.scheduler.Schedule(units)
	if err != nil {
		return nil, fmt.Errorf("scheduling failed: %w", err)
	}

	// 3. Initialize worker pool
	workerCfg := worker.WorkerConfig{
		RepoRoot:     o.cfg.RepoRoot,
		TargetBranch: o.cfg.TargetBranch,
		WorktreeBase: o.cfg.WorktreeBase,
		NoPR:         o.cfg.NoPR,
	}

	workerDeps := worker.WorkerDeps{
		Events: o.bus,
		Git:    o.git,
		GitHub: o.github,
	}

	o.pool = worker.NewPool(o.cfg.Parallelism, workerCfg, workerDeps)

	// Subscribe to worker completion events
	o.bus.Subscribe(o.handleEvent)

	// 4. Main dispatch loop
	for {
		select {
		case <-ctx.Done():
			return o.buildResult(startTime, ctx.Err()), ctx.Err()
		default:
		}

		// Attempt to dispatch next ready unit
		result := o.scheduler.Dispatch()

		switch result.Reason {
		case scheduler.ReasonNone:
			// Successfully dispatched, submit to pool
			unit := o.unitMap[result.Unit]
			if err := o.pool.Submit(unit); err != nil {
				o.scheduler.Fail(result.Unit, err)
			}

		case scheduler.ReasonAllComplete:
			// All units finished successfully
			o.bus.Emit(events.NewEvent(events.OrchCompleted, ""))
			return o.buildResult(startTime, nil), nil

		case scheduler.ReasonAllBlocked:
			// All remaining units are blocked by failures
			err := fmt.Errorf("execution blocked: all remaining units depend on failed units")
			o.bus.Emit(events.NewEvent(events.OrchFailed, "").WithError(err))
			return o.buildResult(startTime, err), err

		case scheduler.ReasonAtCapacity, scheduler.ReasonNoReady:
			// Wait for workers to complete or dependencies to resolve
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// buildResult constructs the Result from current scheduler state
func (o *Orchestrator) buildResult(startTime time.Time, err error) *Result {
	result := &Result{
		TotalUnits: len(o.units),
		Duration:   time.Since(startTime),
		Error:      err,
	}

	if o.scheduler == nil {
		return result
	}

	// Count states from scheduler
	states := o.scheduler.GetAllStates()
	for _, state := range states {
		switch state.Status {
		case scheduler.StatusComplete:
			result.CompletedUnits++
		case scheduler.StatusFailed:
			result.FailedUnits++
		case scheduler.StatusBlocked:
			result.BlockedUnits++
		}
	}

	return result
}
```

### Tests to Implement

```go
// internal/orchestrator/orchestrator_test.go

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
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
```

## Backpressure

### Validation Command
```bash
go test ./internal/orchestrator/... -run TestOrchestrator_Run
```

### Must Pass
| Test | Assertion |
|------|-----------|
| TestOrchestrator_Run_Discovery | Units discovered from directory |
| TestOrchestrator_Run_SingleUnit | Single unit mode filters correctly |
| TestOrchestrator_Run_UnitNotFound | Error returned for missing unit |

## NOT In Scope
- Event handling implementation (task #3)
- Shutdown timeout handling (task #4)
- Dry-run output formatting (task #5)
- CLI integration (task #6)
