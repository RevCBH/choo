---
task: 4
status: pending
backpressure: "go test ./internal/scheduler/... -run TestScheduler"
depends_on: [1, 2, 3]
---

# Scheduler Core and Schedule Method

**Parent spec**: `/specs/SCHEDULER.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement the main Scheduler struct, constructor, Schedule() method, and state query methods.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides `*discovery.Unit` type
- EVENTS - provides `*events.Bus` type

### Task Dependencies (within this unit)
- Task #1 (provides: Graph, NewGraph, CycleError, MissingDependencyError)
- Task #2 (provides: UnitState, UnitStatus, NewUnitState, CanTransition)
- Task #3 (provides: ReadyQueue, NewReadyQueue)

### Package Dependencies
- Standard library (`sync`, `time`)

## Deliverables

### Files to Create/Modify

```
internal/
└── scheduler/
    ├── scheduler.go       # CREATE: Scheduler type and Schedule method
    └── scheduler_test.go  # CREATE: Scheduler unit tests
```

### Types to Implement

```go
// internal/scheduler/scheduler.go

// Scheduler manages unit execution order and dispatch
type Scheduler struct {
    maxParallelism int
    graph          *Graph
    states         map[string]*UnitState
    ready          *ReadyQueue
    events         *events.Bus
    mu             sync.RWMutex
}

// Schedule represents the execution plan
type Schedule struct {
    TopologicalOrder []string
    Levels           [][]string
    MaxParallelism   int
}
```

### Functions to Implement

```go
// New creates a new Scheduler with the given event bus and parallelism limit
func New(events *events.Bus, maxParallelism int) *Scheduler

// Schedule builds the execution plan from discovered units
// Returns error if dependencies are invalid (cycles, missing refs)
// Initializes all units as pending and evaluates initial ready set
func (s *Scheduler) Schedule(units []*discovery.Unit) (*Schedule, error)

// Transition moves a unit to a new status
// Returns error if the transition is invalid
// Emits appropriate event on successful transition
func (s *Scheduler) Transition(unitID string, to UnitStatus) error

// GetState returns the current state of a unit
func (s *Scheduler) GetState(unitID string) (*UnitState, bool)

// GetAllStates returns a snapshot of all unit states
func (s *Scheduler) GetAllStates() map[string]*UnitState

// ReadyQueue returns the current list of ready unit IDs
func (s *Scheduler) ReadyQueue() []string

// ActiveCount returns the number of units consuming parallelism slots
func (s *Scheduler) ActiveCount() int

// IsComplete returns true if all units have reached terminal states
func (s *Scheduler) IsComplete() bool

// HasFailures returns true if any units failed or are blocked
func (s *Scheduler) HasFailures() bool

// evaluateReady checks if a unit's deps are satisfied and moves to ready
// Called with lock held
func (s *Scheduler) evaluateReady(unitID string)
```

## Backpressure

### Validation Command

```bash
go test ./internal/scheduler/... -run TestScheduler -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestScheduler_New` | Creates scheduler with correct parallelism |
| `TestScheduler_Schedule_Simple` | Builds schedule from units without error |
| `TestScheduler_Schedule_CycleError` | Returns CycleError for circular deps |
| `TestScheduler_Schedule_MissingDep` | Returns MissingDependencyError for unknown ref |
| `TestScheduler_Schedule_InitialReady` | Units with no deps start in ready queue |
| `TestScheduler_Schedule_PendingWithDeps` | Units with deps start as pending |
| `TestScheduler_Transition_Valid` | Transitions unit and emits event |
| `TestScheduler_Transition_Invalid` | Returns error for invalid transition |
| `TestScheduler_GetState` | Returns correct state for unit |
| `TestScheduler_GetState_NotFound` | Returns nil, false for unknown unit |
| `TestScheduler_GetAllStates` | Returns snapshot of all states |
| `TestScheduler_ActiveCount` | Counts in_progress + pr_phase units |
| `TestScheduler_IsComplete_AllTerminal` | Returns true when all done |
| `TestScheduler_IsComplete_SomePending` | Returns false when work remains |
| `TestScheduler_HasFailures` | Returns true if any failed/blocked |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| None | N/A | Tests use mock events.Bus and inline units |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Schedule() must be called exactly once per Scheduler instance
- Initial ready evaluation: units with no dependencies go directly to ready queue
- Transition() emits events via the events.Bus (use events.UnitQueued, etc.)
- GetAllStates returns a deep copy to prevent external mutation
- Use RWMutex: RLock for reads (GetState, ActiveCount), Lock for writes

## NOT In Scope

- Dispatch logic (Task #5)
- Complete/Fail methods with propagation (Task #6)
- Worker pool integration (separate package)
