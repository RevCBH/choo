---
task: 7
status: complete
backpressure: "go test ./internal/worker/... -run Pool"
depends_on: [6]
---

# Worker Pool Management

**Parent spec**: `/specs/WORKER.md`
**Task**: #7 of 8 in implementation plan

## Objective

Implement the worker pool that manages multiple workers executing units in parallel with controlled concurrency.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides `Unit`
- EVENTS - provides `Bus`

### Task Dependencies (within this unit)
- Task #6 (worker) - provides `Worker`, `NewWorker`, `Worker.Run()`

### Package Dependencies
- `sync` - for Mutex and WaitGroup
- `context` - for cancellation

## Deliverables

### Files to Modify

```
internal/worker/
└── pool.go         # Add pool methods to existing file
```

### Functions to Implement

```go
// NewPool creates a worker pool with the specified parallelism
func NewPool(maxWorkers int, cfg WorkerConfig, deps WorkerDeps) *Pool {
    return &Pool{
        maxWorkers: maxWorkers,
        config:     cfg,
        events:     deps.Events,
        git:        deps.Git,
        github:     deps.GitHub,
        claude:     deps.Claude,
        workers:    make(map[string]*Worker),
    }
}

// Submit queues a unit for execution
// Blocks if pool is at capacity until a slot opens
func (p *Pool) Submit(unit *discovery.Unit) error {
    // 1. Check for duplicate unit ID
    // 2. Block until slot available (respect maxWorkers)
    // 3. Create worker for unit
    // 4. Add to workers map
    // 5. Start worker in goroutine
    // 6. Handle completion/failure in goroutine
}

// Wait blocks until all submitted units complete
func (p *Pool) Wait() error {
    // 1. Wait on WaitGroup
    // 2. Return first error encountered (or nil)
}

// Stats returns current pool statistics
func (p *Pool) Stats() PoolStats {
    // 1. Lock mutex
    // 2. Count active, completed, failed from workers map
    // 3. Sum task counts
    // 4. Return stats
}

// Shutdown gracefully stops all workers
func (p *Pool) Shutdown(ctx context.Context) error {
    // 1. Signal workers to stop (via context cancellation)
    // 2. Wait for workers to complete (with timeout from ctx)
    // 3. Clean up resources
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run Pool
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestPool_Submit_SingleUnit` | Unit worker started, runs to completion |
| `TestPool_Submit_MultipleUnits` | All units processed |
| `TestPool_Submit_RespectsMaxWorkers` | Never exceeds maxWorkers concurrent |
| `TestPool_Submit_DuplicateUnit` | Returns error for duplicate unit ID |
| `TestPool_Wait_BlocksUntilComplete` | Returns only when all done |
| `TestPool_Stats_Accurate` | Stats reflect actual worker states |
| `TestPool_Shutdown_GracefulStop` | Workers complete in-flight work |
| `TestPool_Shutdown_Timeout` | Returns error if workers don't stop |

### Test Implementation

```go
func TestPool_Submit_SingleUnit(t *testing.T) {
    deps := mockDeps(t)
    pool := NewPool(2, WorkerConfig{}, deps)

    unit := &discovery.Unit{
        ID:    "test-unit",
        Tasks: []*discovery.Task{},
    }

    err := pool.Submit(unit)
    if err != nil {
        t.Fatalf("submit failed: %v", err)
    }

    err = pool.Wait()
    if err != nil {
        t.Errorf("wait failed: %v", err)
    }

    stats := pool.Stats()
    if stats.CompletedUnits != 1 {
        t.Errorf("expected 1 completed, got %d", stats.CompletedUnits)
    }
}

func TestPool_Submit_RespectsMaxWorkers(t *testing.T) {
    deps := mockDeps(t)
    pool := NewPool(2, WorkerConfig{}, deps)

    var concurrent int32
    var maxConcurrent int32

    // Create 5 units that track concurrency
    for i := 0; i < 5; i++ {
        unit := &discovery.Unit{
            ID:    fmt.Sprintf("unit-%d", i),
            Tasks: []*discovery.Task{},
        }
        pool.Submit(unit)
    }

    pool.Wait()

    if atomic.LoadInt32(&maxConcurrent) > 2 {
        t.Errorf("exceeded max workers: %d > 2", maxConcurrent)
    }
}

func TestPool_Submit_DuplicateUnit(t *testing.T) {
    deps := mockDeps(t)
    pool := NewPool(2, WorkerConfig{}, deps)

    unit := &discovery.Unit{ID: "same-id"}

    pool.Submit(unit)
    err := pool.Submit(unit)

    if err == nil {
        t.Error("expected error for duplicate unit")
    }
}

func TestPool_Stats_Accurate(t *testing.T) {
    deps := mockDeps(t)
    pool := NewPool(2, WorkerConfig{}, deps)

    for i := 0; i < 3; i++ {
        pool.Submit(&discovery.Unit{
            ID: fmt.Sprintf("unit-%d", i),
            Tasks: []*discovery.Task{
                {Number: 1},
                {Number: 2},
            },
        })
    }

    pool.Wait()

    stats := pool.Stats()
    if stats.CompletedUnits != 3 {
        t.Errorf("expected 3 completed units, got %d", stats.CompletedUnits)
    }
    if stats.TotalTasks != 6 {
        t.Errorf("expected 6 total tasks, got %d", stats.TotalTasks)
    }
}

func TestPool_Shutdown_GracefulStop(t *testing.T) {
    deps := mockDeps(t)
    pool := NewPool(2, WorkerConfig{}, deps)

    // Submit slow unit
    pool.Submit(&discovery.Unit{ID: "slow"})

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := pool.Shutdown(ctx)
    if err != nil {
        t.Errorf("shutdown failed: %v", err)
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (uses mocks)
- [x] Runs in <60 seconds

## Implementation Notes

- Use a semaphore pattern or buffered channel to limit concurrent workers
- Lock mutex when accessing workers map
- Track per-unit state for stats calculation
- Shutdown should propagate context cancellation to all workers
- Consider using errgroup for coordinating worker completion

## NOT In Scope

- Priority-based scheduling
- Dynamic resizing of pool
- Per-unit resource limits
- Execute() entry point (task #8)
