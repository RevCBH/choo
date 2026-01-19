---
task: 5
status: complete
backpressure: "go test ./internal/scheduler/... -run TestDispatch"
depends_on: [4]
---

# Dispatch Logic and Parallelism Control

**Parent spec**: `/specs/SCHEDULER.md`
**Task**: #5 of 6 in implementation plan

## Objective

Implement the Dispatch() method with parallelism enforcement and dispatch result types.

## Dependencies

### External Specs (must be implemented)
- EVENTS - provides event types for UnitStarted

### Task Dependencies (within this unit)
- Task #4 (provides: Scheduler struct, states map, ready queue, Transition method)

### Package Dependencies
- Standard library only (`time`)

## Deliverables

### Files to Create/Modify

```
internal/
└── scheduler/
    ├── dispatch.go       # CREATE: Dispatch types and logic
    └── dispatch_test.go  # CREATE: Dispatch unit tests
```

### Types to Implement

```go
// internal/scheduler/dispatch.go

// DispatchResult represents the outcome of a dispatch attempt
type DispatchResult struct {
    Unit       string
    Dispatched bool
    Reason     DispatchBlockReason
}

// DispatchBlockReason explains why dispatch didn't occur
type DispatchBlockReason string

const (
    ReasonNone        DispatchBlockReason = ""
    ReasonNoReady     DispatchBlockReason = "no_ready_units"
    ReasonAtCapacity  DispatchBlockReason = "at_capacity"
    ReasonAllComplete DispatchBlockReason = "all_complete"
    ReasonAllBlocked  DispatchBlockReason = "all_blocked"
)
```

### Functions to Implement

```go
// Dispatch attempts to dispatch the next ready unit
// Returns the dispatched unit ID or empty if none available
// Respects parallelism limit and emits UnitStarted event
func (s *Scheduler) Dispatch() DispatchResult

// allBlockedOrComplete checks if remaining units are all blocked/complete
// Called with lock held
func (s *Scheduler) allBlockedOrComplete() bool
```

## Backpressure

### Validation Command

```bash
go test ./internal/scheduler/... -run TestDispatch -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestDispatch_Success` | Returns Dispatched=true with unit ID |
| `TestDispatch_SetsStartedAt` | Unit state has StartedAt timestamp |
| `TestDispatch_TransitionsToInProgress` | Unit status becomes in_progress |
| `TestDispatch_EmitsEvent` | UnitStarted event emitted |
| `TestDispatch_RespectsParallelism` | Returns AtCapacity when at limit |
| `TestDispatch_NoReady` | Returns NoReady when queue empty but work remains |
| `TestDispatch_AllComplete` | Returns AllComplete when all units done |
| `TestDispatch_AllBlocked` | Returns AllBlocked when remaining units blocked |
| `TestDispatch_ParallelismIncludesPRPhase` | pr_open, in_review, merging count against limit |
| `TestDispatch_Sequential` | Multiple dispatches return different units |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| None | N/A | Tests use mock events.Bus and inline setup |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Dispatch order: check parallelism first, then try pop from ready queue
- ActiveCount includes: in_progress, pr_open, in_review, merging
- Set StartedAt timestamp when transitioning to in_progress
- Use Transition() method internally (already handles event emission)
- allBlockedOrComplete: iterate states, check if any are pending/ready

## NOT In Scope

- Complete/Fail methods (Task #6)
- Re-evaluation of pending units after completion (Task #6)
- Worker pool integration (separate package)
