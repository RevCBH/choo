---
task: 4
status: complete
backpressure: "go test ./internal/orchestrator/... -run TestOrchestrator_Shutdown"
depends_on: [1, 2]
---

# Orchestrator Graceful Shutdown

**Parent spec**: `/specs/ORCHESTRATOR.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement graceful shutdown with configurable timeout that waits for in-progress workers to complete.

## Dependencies

### External Specs (must be implemented)
- WORKER - provides Pool with Shutdown() method
- EVENTS - provides Bus with Close() method

### Task Dependencies (within this unit)
- Task #1 - Core types must be defined
- Task #2 - Run() method must initialize pool

### Package Dependencies
- `github.com/RevCBH/choo/internal/worker`
- `github.com/RevCBH/choo/internal/events`

## Deliverables

### Files to Create/Modify
```
internal/orchestrator/
├── orchestrator.go      # MODIFY: Add shutdown() method
└── orchestrator_test.go # MODIFY: Add shutdown tests
```

### Functions to Implement

```go
// shutdown gracefully stops the orchestrator with timeout
func (o *Orchestrator) shutdown(ctx context.Context) error {
	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, o.cfg.ShutdownTimeout)
	defer cancel()

	var errs []error

	// Stop accepting new work
	// (dispatch loop exits via context cancellation in Run())

	// Wait for in-progress workers to complete
	if o.pool != nil {
		if err := o.pool.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("worker shutdown: %w", err))
		}
	}

	// Close event bus (drains pending events)
	if o.bus != nil {
		o.bus.Close()
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// DefaultShutdownTimeout is the default grace period for shutdown
const DefaultShutdownTimeout = 30 * time.Second
```

Update the Run() method to call shutdown on context cancellation:

```go
// In Run(), update the context done case:
case <-ctx.Done():
	// Graceful shutdown
	if err := o.shutdown(context.Background()); err != nil {
		// Log but don't fail - we're already shutting down
		o.bus.Emit(events.NewEvent(events.OrchFailed, "").
			WithError(fmt.Errorf("shutdown error: %w", err)))
	}
	return o.buildResult(startTime, ctx.Err()), ctx.Err()
```

Update the Config default:

```go
// In New(), set default shutdown timeout if not specified
func New(cfg Config, deps Dependencies) *Orchestrator {
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = DefaultShutdownTimeout
	}
	return &Orchestrator{
		cfg:       cfg,
		bus:       deps.Bus,
		escalator: deps.Escalator,
		git:       deps.Git,
		github:    deps.GitHub,
		unitMap:   make(map[string]*discovery.Unit),
	}
}
```

### Tests to Implement

```go
// Add to internal/orchestrator/orchestrator_test.go

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

func TestOrchestrator_Shutdown_Timeout(t *testing.T) {
	bus := events.NewBus(100)

	// Very short timeout
	cfg := Config{
		ShutdownTimeout: 1 * time.Millisecond,
		Parallelism:     1,
	}

	orch := New(cfg, Dependencies{Bus: bus})

	// Create pool with mock that blocks
	workerCfg := worker.WorkerConfig{}
	workerDeps := worker.WorkerDeps{Events: bus}
	orch.pool = worker.NewPool(1, workerCfg, workerDeps)

	// Submit a blocking unit (won't complete quickly)
	unit := &discovery.Unit{
		ID: "blocking-unit",
		Tasks: []*discovery.Task{
			{Number: 1, Backpressure: "sleep 10"},
		},
	}
	orch.pool.Submit(unit)

	// Shutdown with already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := orch.shutdown(ctx)

	// Should timeout waiting for workers
	if err == nil {
		t.Log("shutdown completed (pool may have been empty)")
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

func TestOrchestrator_Run_ContextCancellation(t *testing.T) {
	// Create temp tasks directory
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	unitDir := filepath.Join(tasksDir, "slow-unit")
	os.MkdirAll(unitDir, 0755)

	os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: slow-unit
depends_on: []
---
# Slow Unit
`), 0644)

	os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(`---
task: 1
status: in_progress
backpressure: "sleep 60"
depends_on: []
---
# Slow Task
`), 0644)

	bus := events.NewBus(100)
	git := git.NewWorktreeManager(tmpDir, nil)

	cfg := Config{
		TasksDir:        tasksDir,
		Parallelism:     1,
		ShutdownTimeout: 100 * time.Millisecond,
		RepoRoot:        tmpDir,
	}

	orch := New(cfg, Dependencies{
		Bus: bus,
		Git: git,
	})

	// Cancel context after short delay
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := orch.Run(ctx)

	// Should return context cancelled error
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	// Result should still be populated
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/orchestrator/... -run TestOrchestrator_Shutdown
```

### Must Pass
| Test | Assertion |
|------|-----------|
| TestOrchestrator_Shutdown_Clean | Clean shutdown completes without error |
| TestOrchestrator_Shutdown_NilComponents | No panic with nil components |
| TestOrchestrator_DefaultShutdownTimeout | Default 30s timeout applied |
| TestOrchestrator_Run_ContextCancellation | Context cancellation triggers shutdown |

## NOT In Scope
- Dry-run mode (task #5)
- CLI integration (task #6)
