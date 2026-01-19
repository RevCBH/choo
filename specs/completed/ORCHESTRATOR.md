# ORCHESTRATOR — Main Loop Coordinating Discovery, Scheduler, Workers, and Escalation

## Overview

The Orchestrator is the central coordination component that wires together all subsystems to execute units in parallel. It transforms the `choo run` command from a stub with TODO placeholders into a functioning orchestration engine.

The orchestrator implements a verify-then-continue pattern: it invokes workers to execute units, verifies outcomes through the scheduler's state machine, and either continues to the next unit or escalates to the user when operations cannot complete. This design delegates implementation details to Claude Code while the orchestrator focuses on lifecycle management and coordination.

```
┌─────────────────────────────────────────────────────────────────┐
│                      Orchestrator Loop                          │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │   1. Discover  →  2. Schedule  →  3. Dispatch  →  4. Wait │ │
│  └───────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         ▼                    ▼                    ▼
   ┌───────────┐        ┌───────────┐        ┌───────────┐
   │ Discovery │───────▶│ Scheduler │───────▶│  Worker   │
   │           │        │           │        │   Pool    │
   └───────────┘        └───────────┘        └───────────┘
         │                    │                    │
         └────────────────────┴────────────────────┘
                              │
                              ▼
   ┌─────────────────────────────────────────────────────┐
   │                     Event Bus                        │
   └─────────────────────────────────────────────────────┘
                              │
                              ▼
   ┌─────────────────────────────────────────────────────┐
   │                    Escalator                         │
   └─────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Discover units from the specified tasks directory using the discovery module
2. Build the dependency graph and validate no cycles exist
3. Initialize the scheduler with discovered units and parallelism limit
4. Run the main dispatch loop until all units complete or fail
5. Submit ready units to the worker pool respecting parallelism limits
6. Update scheduler state when workers complete or fail
7. Propagate failures to dependent units via the scheduler
8. Emit lifecycle events for orchestrator start, completion, and failure
9. Support graceful shutdown on SIGINT/SIGTERM via context cancellation
10. Support single-unit mode for targeted execution
11. Support dry-run mode for execution plan preview

### Performance Requirements

| Metric | Target |
|--------|--------|
| Unit dispatch latency | < 10ms from ready to submitted |
| State update latency | < 5ms for scheduler transitions |
| Shutdown grace period | 30 seconds for worker cleanup |
| Event buffer capacity | 1000 events before backpressure |
| Poll interval when idle | 100ms between dispatch attempts |

### Constraints

- Must use existing scheduler, worker pool, and event bus implementations
- Must respect parallelism configuration from CLI flags and config file
- Must not block on individual worker execution
- Must handle worker panics without crashing the orchestrator
- Must preserve scheduler state for resume capability (via frontmatter updates)

## Design

### Module Structure

```
internal/orchestrator/
├── orchestrator.go   # Main orchestrator type and Run method
├── escalator.go      # Escalator interface and implementations
└── orchestrator_test.go
```

### Core Types

```go
// Orchestrator coordinates unit execution across all subsystems
type Orchestrator struct {
    cfg       *config.Config
    bus       *events.Bus
    escalator Escalator
    scheduler *scheduler.Scheduler
    pool      *worker.Pool
    git       *git.WorktreeManager
    github    *github.PRClient

    // Runtime state
    units     []*discovery.Unit
    unitMap   map[string]*discovery.Unit // unitID -> Unit for quick lookup
}

// Config holds orchestrator-specific configuration
type Config struct {
    // Parallelism is the maximum concurrent units
    Parallelism int

    // TargetBranch is the branch PRs target
    TargetBranch string

    // TasksDir is the path to the specs/tasks directory
    TasksDir string

    // NoPR skips PR creation when true
    NoPR bool

    // SkipReview auto-merges without waiting for review
    SkipReview bool

    // SingleUnit limits execution to one unit when non-empty
    SingleUnit string

    // DryRun shows execution plan without running
    DryRun bool

    // ShutdownTimeout is the grace period for worker cleanup
    ShutdownTimeout time.Duration
}

// Escalator handles situations requiring user intervention
type Escalator interface {
    // Escalate notifies the user of an issue requiring intervention
    // Returns nil if the user resolved the issue, error otherwise
    Escalate(ctx context.Context, issue EscalationIssue) error
}

// EscalationIssue describes a problem requiring user attention
type EscalationIssue struct {
    UnitID      string
    Type        EscalationType
    Description string
    Error       error
    Suggestions []string
}

// EscalationType categorizes escalation issues
type EscalationType string

const (
    EscalationMergeConflict   EscalationType = "merge_conflict"
    EscalationReviewTimeout   EscalationType = "review_timeout"
    EscalationBaselineFailure EscalationType = "baseline_failure"
    EscalationClaudeFailure   EscalationType = "claude_failure"
)

// Result represents the outcome of an orchestration run
type Result struct {
    TotalUnits     int
    CompletedUnits int
    FailedUnits    int
    BlockedUnits   int
    Duration       time.Duration
    Error          error
}
```

### API Surface

```go
// New creates an orchestrator with the given configuration and dependencies
func New(cfg Config, deps Dependencies) *Orchestrator

// Dependencies bundles external dependencies for injection
type Dependencies struct {
    Bus       *events.Bus
    Escalator Escalator
    Git       *git.WorktreeManager
    GitHub    *github.PRClient
}

// Run executes the orchestration loop until completion or cancellation
// Returns Result with execution statistics
func (o *Orchestrator) Run(ctx context.Context) (*Result, error)

// Close releases all resources held by the orchestrator
func (o *Orchestrator) Close() error
```

### State Machine Integration

The orchestrator drives the scheduler's state machine through these transitions:

```
┌─────────┐
│ Pending │
└────┬────┘
     │ deps satisfied
     ▼
┌─────────┐
│  Ready  │
└────┬────┘
     │ dispatched
     ▼
┌─────────────┐     ┌──────────┐     ┌───────────┐     ┌──────────┐
│ In Progress │────▶│ PR Open  │────▶│ In Review │────▶│ Merging  │
└──────┬──────┘     └────┬─────┘     └─────┬─────┘     └────┬─────┘
       │                 │                 │                │
       └────────────────┴─────────────────┴────────────────┘
                                │
              ┌─────────────────┼─────────────────┐
              ▼                 ▼                 ▼
        ┌──────────┐     ┌──────────┐     ┌──────────┐
        │ Complete │     │  Failed  │     │ Blocked  │
        └──────────┘     └──────────┘     └──────────┘
```

## Implementation

### Main Loop

```go
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

    // Emit orchestrator started event
    o.bus.Emit(events.NewEvent(events.OrchStarted, "").WithPayload(map[string]any{
        "unit_count":  len(units),
        "parallelism": o.cfg.Parallelism,
    }))

    // 2. Build schedule
    schedule, err := o.scheduler.Schedule(units)
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
```

### Event Handling

```go
func (o *Orchestrator) handleEvent(e events.Event) {
    switch e.Type {
    case events.UnitCompleted:
        o.scheduler.Complete(e.Unit)

    case events.UnitFailed:
        var err error
        if e.Error != "" {
            err = fmt.Errorf("%s", e.Error)
        }
        o.scheduler.Fail(e.Unit, err)

        // Check if escalation is needed
        if o.escalator != nil {
            issue := EscalationIssue{
                UnitID:      e.Unit,
                Type:        categorizeError(err),
                Description: e.Error,
                Error:       err,
            }
            // Escalate asynchronously to avoid blocking event dispatch
            go o.escalator.Escalate(context.Background(), issue)
        }
    }
}

func categorizeError(err error) EscalationType {
    if err == nil {
        return ""
    }
    errStr := err.Error()
    switch {
    case strings.Contains(errStr, "merge conflict"):
        return EscalationMergeConflict
    case strings.Contains(errStr, "review timeout"):
        return EscalationReviewTimeout
    case strings.Contains(errStr, "baseline"):
        return EscalationBaselineFailure
    default:
        return EscalationClaudeFailure
    }
}
```

### Graceful Shutdown

```go
func (o *Orchestrator) shutdown(ctx context.Context) error {
    // Create shutdown context with timeout
    shutdownCtx, cancel := context.WithTimeout(ctx, o.cfg.ShutdownTimeout)
    defer cancel()

    // Stop accepting new work
    // (dispatch loop exits via context cancellation)

    // Wait for in-progress workers to complete
    if err := o.pool.Shutdown(shutdownCtx); err != nil {
        return fmt.Errorf("worker shutdown failed: %w", err)
    }

    // Close event bus (drains pending events)
    o.bus.Close()

    return nil
}
```

### Dry Run Mode

```go
func (o *Orchestrator) dryRun(units []*discovery.Unit) (*Result, error) {
    // Build schedule without executing
    schedule, err := o.scheduler.Schedule(units)
    if err != nil {
        return nil, err
    }

    // Print execution plan
    fmt.Printf("Execution Plan\n")
    fmt.Printf("==============\n\n")
    fmt.Printf("Units to execute: %d\n", len(units))
    fmt.Printf("Max parallelism: %d\n", schedule.MaxParallelism)
    fmt.Printf("Execution levels: %d\n\n", len(schedule.Levels))

    for i, level := range schedule.Levels {
        fmt.Printf("Level %d (parallel):\n", i+1)
        for _, unitID := range level {
            unit := o.unitMap[unitID]
            fmt.Printf("  - %s (%d tasks)\n", unitID, len(unit.Tasks))
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

### Wire Integration

The existing `wire.go` file contains an `Orchestrator` struct that holds wired components. The new orchestrator module uses this as a foundation:

```go
// FromWired creates an Orchestrator from the wired components
func FromWired(wired *cli.Orchestrator, cfg Config) *Orchestrator {
    return &Orchestrator{
        cfg:       cfg,
        bus:       wired.Events,
        scheduler: wired.Scheduler,
        pool:      wired.Workers,
        git:       wired.Git,
        github:    wired.GitHub,
    }
}
```

## Implementation Notes

### Thread Safety

- The scheduler uses a mutex for all state access
- The worker pool uses a semaphore for concurrency control
- The event bus is thread-safe for concurrent emit/subscribe
- The orchestrator's handleEvent runs in the event bus goroutine

### Context Propagation

- The main context flows from `choo run` through SignalHandler
- Workers receive a derived context from the pool
- Cancellation propagates from orchestrator to all workers
- Workers must respect context cancellation within 5 seconds

### Error Handling

- Discovery errors are fatal (invalid specs)
- Scheduling errors are fatal (cycles, missing deps)
- Individual worker failures are recorded but don't stop other workers
- Blocked units are marked but don't cause orchestrator failure

### Memory Management

- Unit maps are built once during initialization
- Worker pool reuses goroutines via semaphore
- Event bus drops events when buffer is full (logs warning)
- Worktrees are cleaned up by workers, not orchestrator

## Testing Strategy

### Unit Tests

```go
func TestOrchestrator_Run_Success(t *testing.T) {
    // Setup mock components
    bus := events.NewBus(100)
    defer bus.Close()

    mockScheduler := &MockScheduler{
        scheduleResult: &scheduler.Schedule{
            TopologicalOrder: []string{"unit-a", "unit-b"},
            Levels:           [][]string{{"unit-a"}, {"unit-b"}},
        },
    }

    // Configure dispatch sequence
    mockScheduler.dispatchResults = []scheduler.DispatchResult{
        {Unit: "unit-a", Dispatched: true},
        {Reason: scheduler.ReasonNoReady},
        {Unit: "unit-b", Dispatched: true},
        {Reason: scheduler.ReasonAllComplete},
    }

    mockPool := &MockPool{}
    mockEscalator := &MockEscalator{}

    orch := &Orchestrator{
        cfg: Config{
            TasksDir:    "testdata/tasks",
            Parallelism: 2,
        },
        bus:       bus,
        scheduler: mockScheduler,
        pool:      mockPool,
        escalator: mockEscalator,
    }

    // Simulate worker completions
    go func() {
        time.Sleep(50 * time.Millisecond)
        bus.Emit(events.NewEvent(events.UnitCompleted, "unit-a"))
        time.Sleep(50 * time.Millisecond)
        bus.Emit(events.NewEvent(events.UnitCompleted, "unit-b"))
    }()

    ctx := context.Background()
    result, err := orch.Run(ctx)

    assert.NoError(t, err)
    assert.Equal(t, 2, result.TotalUnits)
    assert.Equal(t, 2, result.CompletedUnits)
    assert.Equal(t, 0, result.FailedUnits)
}

func TestOrchestrator_Run_FailurePropagation(t *testing.T) {
    bus := events.NewBus(100)
    defer bus.Close()

    mockScheduler := &MockScheduler{}
    mockScheduler.dispatchResults = []scheduler.DispatchResult{
        {Unit: "unit-a", Dispatched: true},
        {Reason: scheduler.ReasonNoReady},
        {Reason: scheduler.ReasonAllBlocked},
    }

    orch := &Orchestrator{
        cfg: Config{TasksDir: "testdata/tasks"},
        bus: bus,
        scheduler: mockScheduler,
    }

    // Simulate failure
    go func() {
        time.Sleep(50 * time.Millisecond)
        bus.Emit(events.NewEvent(events.UnitFailed, "unit-a").
            WithError(fmt.Errorf("baseline checks failed")))
    }()

    ctx := context.Background()
    result, err := orch.Run(ctx)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "blocked")
    assert.Equal(t, 1, result.FailedUnits)
}

func TestOrchestrator_GracefulShutdown(t *testing.T) {
    bus := events.NewBus(100)
    defer bus.Close()

    mockPool := &MockPool{
        shutdownDelay: 100 * time.Millisecond,
    }

    orch := &Orchestrator{
        cfg: Config{
            ShutdownTimeout: 5 * time.Second,
        },
        bus:  bus,
        pool: mockPool,
    }

    // Cancel context immediately
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    startTime := time.Now()
    err := orch.shutdown(ctx)
    elapsed := time.Since(startTime)

    assert.NoError(t, err)
    assert.True(t, elapsed >= 100*time.Millisecond)
    assert.True(t, mockPool.shutdownCalled)
}

func TestOrchestrator_DryRun(t *testing.T) {
    bus := events.NewBus(100)
    defer bus.Close()

    units := []*discovery.Unit{
        {ID: "unit-a", Tasks: make([]*discovery.Task, 3)},
        {ID: "unit-b", Tasks: make([]*discovery.Task, 2), DependsOn: []string{"unit-a"}},
    }

    orch := &Orchestrator{
        cfg: Config{
            DryRun:      true,
            Parallelism: 4,
        },
        bus: bus,
    }

    result, err := orch.dryRun(units)

    assert.NoError(t, err)
    assert.Equal(t, 2, result.TotalUnits)
}
```

### Integration Tests

- Test full orchestration with real scheduler and mock workers
- Test context cancellation during active worker execution
- Test dependency resolution with multi-level graphs
- Test escalator integration on simulated failures

### Manual Testing

- [ ] Run `choo run specs/tasks/` with sample task specs
- [ ] Verify parallelism flag limits concurrent workers
- [ ] Test SIGINT handling with workers in progress
- [ ] Verify dry-run mode shows correct execution plan
- [ ] Test single-unit mode with `--unit` flag
- [ ] Verify events emit for unit lifecycle transitions

## Design Decisions

### Why Event-Driven Completion?

The orchestrator subscribes to events rather than polling worker status. This approach:
- Decouples the orchestrator from worker implementation details
- Enables other subscribers (display, logging) to observe progress
- Avoids busy-waiting on worker completion
- Supports future extensions like persistence and webhooks

The trade-off is increased complexity in the event flow, but the event bus already exists and provides thread-safe delivery.

### Why Separate Escalator Interface?

The escalator is injected rather than hardcoded to support:
- CLI escalation (print error, wait for user input)
- Webhook escalation (notify external systems)
- Slack/email escalation for production deployments
- Test mocking for unit tests

### Why 100ms Poll Interval?

When no units are ready and workers are running, the orchestrator polls at 100ms intervals. This balances:
- Responsiveness (units dispatched within 100ms of becoming ready)
- CPU usage (not busy-spinning)
- Worker overhead (not checking state on every iteration)

For comparison, GitHub Actions uses 10-second intervals for status polling.

## Future Enhancements

1. **Checkpointing**: Persist orchestrator state to disk for crash recovery
2. **Progress Display**: Real-time TUI showing unit status and worker activity
3. **Metrics Export**: Prometheus/OpenTelemetry metrics for observability
4. **Remote Workers**: Distribute workers across multiple machines
5. **Partial Re-run**: Resume from specific unit after failure resolution
6. **Dependency Visualization**: Generate Mermaid diagrams of execution graph

## References

- PRD Section 4.1: Problem statement with TODO placeholders
- PRD Section 4.2: Design diagram showing component relationships
- PRD Section 4.3: Implementation code snippets
- PRD Section 4.4: Acceptance criteria
- PRD Section 2.4: Verification pattern (invoke, verify, continue/retry)
- PRD Section 1.4: Success criteria for self-hosting
- `internal/cli/run.go`: RunOptions and TODO locations
- `internal/cli/wire.go`: Existing Orchestrator struct
- `internal/scheduler/scheduler.go`: Scheduler API
- `internal/worker/pool.go`: Worker pool API
- `internal/events/bus.go`: Event bus API
