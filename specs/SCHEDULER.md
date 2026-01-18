# SCHEDULER — Dependency Graph and Unit Dispatch for Ralph Orchestrator

## Overview

The Scheduler package manages the execution order of units by building a dependency graph, maintaining unit state machines, and dispatching ready units to the worker pool. It ensures that units are executed only when their dependencies are satisfied and respects the configured parallelism limit.

The scheduler receives discovered units from the discovery package, constructs a directed acyclic graph (DAG) of dependencies, and continuously evaluates which units are ready for execution. It handles unit lifecycle transitions, failure propagation, and provides the ready queue that workers consume from.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Scheduler                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌──────────────────┐         ┌──────────────────┐                     │
│   │  Dependency      │         │   State          │                     │
│   │  Graph           │         │   Machine        │                     │
│   │                  │         │                  │                     │
│   │  - Build DAG     │         │  - Transitions   │                     │
│   │  - Topo sort     │         │  - Validation    │                     │
│   │  - Cycle detect  │         │  - Events        │                     │
│   └────────┬─────────┘         └────────┬─────────┘                     │
│            │                            │                                │
│            └──────────┬─────────────────┘                                │
│                       ▼                                                  │
│            ┌──────────────────┐                                          │
│            │   Ready Queue    │                                          │
│            │                  │                                          │
│            │  - Pending units │                                          │
│            │  - Ready units   │                                          │
│            │  - Dispatch      │                                          │
│            └────────┬─────────┘                                          │
│                     │                                                    │
│                     ▼                                                    │
│            ┌──────────────────┐                                          │
│            │   Dispatcher     │                                          │
│            │                  │                                          │
│            │  - Select next   │                                          │
│            │  - Parallelism   │                                          │
│            │  - Emit events   │                                          │
│            └──────────────────┘                                          │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
       ┌─────────────┐                 ┌─────────────┐
       │   Worker    │                 │   Events    │
       │   Pool      │                 │   Bus       │
       └─────────────┘                 └─────────────┘
```

## Requirements

### Functional Requirements

1. Build a dependency graph from discovered units
2. Detect and reject circular dependencies during graph construction
3. Maintain unit state machine with transitions: pending -> ready -> in_progress -> complete/failed/blocked
4. Move units from pending to ready when all their depends_on units are complete
5. Provide a ready queue of units available for dispatch
6. Dispatch next ready unit to worker pool, respecting parallelism limit
7. Handle unit completion events and re-evaluate pending units
8. Handle unit failure and propagate blocked status to dependent units
9. Support PR lifecycle states: pr_open, in_review, merging
10. Emit events for all state transitions
11. Provide topological ordering for deterministic execution order
12. Track running and pr_phase units against parallelism limit

### Performance Requirements

| Metric | Target |
|--------|--------|
| Graph construction time | O(V + E) where V=units, E=dependencies |
| Ready queue evaluation | <1ms for 100 units |
| Dispatch latency | <100us per dispatch |
| Memory per unit | <1KB overhead |

### Constraints

- Imports: Unit, Task types from discovery package
- Depends on events package for event emission
- Must be thread-safe for concurrent access from workers
- Parallelism limit applies to running + pr_phase units combined
- Must preserve execution order determinism for reproducible runs

## Design

### Module Structure

```
internal/scheduler/
├── scheduler.go        # Main Scheduler type, public API
├── graph.go            # Dependency graph construction, cycle detection
├── state.go            # Unit state machine, transitions
├── queue.go            # Ready queue implementation
└── dispatch.go         # Dispatch logic, parallelism control
```

### Core Types

```go
// internal/scheduler/scheduler.go

// Scheduler manages unit execution order and dispatch
type Scheduler struct {
    // Configuration
    maxParallelism int

    // Dependency graph
    graph *Graph

    // State tracking
    states map[string]*UnitState

    // Ready queue (thread-safe)
    ready *ReadyQueue

    // Event emission
    events *events.Bus

    // Synchronization
    mu sync.RWMutex
}

// Schedule represents the execution plan
type Schedule struct {
    // TopologicalOrder is the units in valid execution order
    TopologicalOrder []string

    // Levels groups units by dependency depth (0 = no deps)
    Levels [][]string

    // MaxParallelism is the configured limit
    MaxParallelism int
}
```

```go
// internal/scheduler/graph.go

// Graph represents the unit dependency DAG
type Graph struct {
    // Nodes are unit IDs
    nodes map[string]bool

    // Edges map from unit ID to its dependencies
    // edges["app-shell"] = ["project-setup", "config"]
    edges map[string][]string

    // Reverse edges for dependent lookup
    // dependents["config"] = ["app-shell", "deck-list"]
    dependents map[string][]string
}

// CycleError indicates a circular dependency was detected
type CycleError struct {
    // Cycle contains the unit IDs forming the cycle
    Cycle []string
}

func (e *CycleError) Error() string {
    return fmt.Sprintf("circular dependency detected: %s", strings.Join(e.Cycle, " -> "))
}

// MissingDependencyError indicates a referenced dependency doesn't exist
type MissingDependencyError struct {
    Unit       string
    Dependency string
}

func (e *MissingDependencyError) Error() string {
    return fmt.Sprintf("unit %q depends on unknown unit %q", e.Unit, e.Dependency)
}
```

```go
// internal/scheduler/state.go

// UnitState tracks the current state of a unit
type UnitState struct {
    // UnitID is the unit identifier
    UnitID string

    // Status is the current state
    Status UnitStatus

    // BlockedBy contains unit IDs blocking this unit (if blocked)
    BlockedBy []string

    // StartedAt is when the unit entered in_progress
    StartedAt *time.Time

    // CompletedAt is when the unit reached a terminal state
    CompletedAt *time.Time

    // Error contains failure details if status is failed
    Error error
}

// UnitStatus represents the unit's lifecycle state
type UnitStatus string

const (
    // StatusPending - waiting for dependencies
    StatusPending UnitStatus = "pending"

    // StatusReady - dependencies satisfied, waiting for worker
    StatusReady UnitStatus = "ready"

    // StatusInProgress - worker is executing tasks
    StatusInProgress UnitStatus = "in_progress"

    // StatusPROpen - PR created, waiting for review
    StatusPROpen UnitStatus = "pr_open"

    // StatusInReview - reviewer has reacted with eyes emoji
    StatusInReview UnitStatus = "in_review"

    // StatusMerging - approved, in merge queue
    StatusMerging UnitStatus = "merging"

    // StatusComplete - successfully merged
    StatusComplete UnitStatus = "complete"

    // StatusFailed - unrecoverable error
    StatusFailed UnitStatus = "failed"

    // StatusBlocked - dependency failed
    StatusBlocked UnitStatus = "blocked"
)

// IsTerminal returns true if the status is a final state
func (s UnitStatus) IsTerminal() bool {
    return s == StatusComplete || s == StatusFailed || s == StatusBlocked
}

// IsActive returns true if the unit is consuming a parallelism slot
func (s UnitStatus) IsActive() bool {
    return s == StatusInProgress || s == StatusPROpen ||
           s == StatusInReview || s == StatusMerging
}
```

```go
// internal/scheduler/queue.go

// ReadyQueue manages units ready for dispatch
type ReadyQueue struct {
    // Queue of ready unit IDs (FIFO within same priority)
    queue []string

    // Set for O(1) membership checks
    set map[string]bool

    mu sync.Mutex
}
```

```go
// internal/scheduler/dispatch.go

// DispatchResult represents the outcome of a dispatch attempt
type DispatchResult struct {
    // Unit is the dispatched unit ID, empty if none available
    Unit string

    // Dispatched is true if a unit was dispatched
    Dispatched bool

    // Reason explains why no unit was dispatched
    Reason DispatchBlockReason
}

// DispatchBlockReason explains why dispatch didn't occur
type DispatchBlockReason string

const (
    // ReasonNone - dispatch succeeded
    ReasonNone DispatchBlockReason = ""

    // ReasonNoReady - no units in ready queue
    ReasonNoReady DispatchBlockReason = "no_ready_units"

    // ReasonAtCapacity - parallelism limit reached
    ReasonAtCapacity DispatchBlockReason = "at_capacity"

    // ReasonAllComplete - all units have completed
    ReasonAllComplete DispatchBlockReason = "all_complete"

    // ReasonAllBlocked - remaining units are blocked
    ReasonAllBlocked DispatchBlockReason = "all_blocked"
)
```

### API Surface

```go
// internal/scheduler/scheduler.go

// New creates a new Scheduler with the given event bus and parallelism limit
func New(events *events.Bus, maxParallelism int) *Scheduler

// Schedule builds the execution plan from discovered units
// Returns error if dependencies are invalid (cycles, missing refs)
func (s *Scheduler) Schedule(units []*discovery.Unit) (*Schedule, error)

// Dispatch attempts to dispatch the next ready unit
// Returns the dispatched unit ID or empty if none available
func (s *Scheduler) Dispatch() DispatchResult

// ReadyQueue returns the current list of ready unit IDs
func (s *Scheduler) ReadyQueue() []string

// Complete marks a unit as complete and re-evaluates pending units
func (s *Scheduler) Complete(unitID string)

// Fail marks a unit as failed and propagates blocked status to dependents
func (s *Scheduler) Fail(unitID string, err error)

// Transition moves a unit to a new status
// Returns error if the transition is invalid
func (s *Scheduler) Transition(unitID string, to UnitStatus) error

// GetState returns the current state of a unit
func (s *Scheduler) GetState(unitID string) (*UnitState, bool)

// GetAllStates returns a snapshot of all unit states
func (s *Scheduler) GetAllStates() map[string]*UnitState

// ActiveCount returns the number of units consuming parallelism slots
func (s *Scheduler) ActiveCount() int

// IsComplete returns true if all units have reached terminal states
func (s *Scheduler) IsComplete() bool

// HasFailures returns true if any units failed or are blocked
func (s *Scheduler) HasFailures() bool
```

```go
// internal/scheduler/graph.go

// NewGraph constructs a dependency graph from units
// Returns error if cycles or missing dependencies are detected
func NewGraph(units []*discovery.Unit) (*Graph, error)

// TopologicalSort returns unit IDs in valid execution order
func (g *Graph) TopologicalSort() ([]string, error)

// GetDependencies returns the direct dependencies of a unit
func (g *Graph) GetDependencies(unitID string) []string

// GetDependents returns units that depend on the given unit
func (g *Graph) GetDependents(unitID string) []string

// GetLevels returns units grouped by dependency depth
// Level 0 contains units with no dependencies
func (g *Graph) GetLevels() [][]string
```

```go
// internal/scheduler/state.go

// ValidTransitions defines allowed state transitions
var ValidTransitions = map[UnitStatus][]UnitStatus{
    StatusPending:    {StatusReady, StatusBlocked},
    StatusReady:      {StatusInProgress, StatusBlocked},
    StatusInProgress: {StatusPROpen, StatusComplete, StatusFailed},
    StatusPROpen:     {StatusInReview, StatusComplete, StatusFailed},
    StatusInReview:   {StatusMerging, StatusPROpen, StatusFailed},
    StatusMerging:    {StatusComplete, StatusFailed},
    StatusComplete:   {}, // terminal
    StatusFailed:     {}, // terminal
    StatusBlocked:    {}, // terminal
}

// CanTransition checks if a transition from -> to is valid
func CanTransition(from, to UnitStatus) bool

// NewUnitState creates initial state for a unit
func NewUnitState(unitID string) *UnitState
```

```go
// internal/scheduler/queue.go

// NewReadyQueue creates an empty ready queue
func NewReadyQueue() *ReadyQueue

// Push adds a unit ID to the ready queue
func (q *ReadyQueue) Push(unitID string)

// Pop removes and returns the next ready unit ID
// Returns empty string if queue is empty
func (q *ReadyQueue) Pop() string

// Peek returns the next ready unit ID without removing it
func (q *ReadyQueue) Peek() string

// Contains checks if a unit ID is in the queue
func (q *ReadyQueue) Contains(unitID string) bool

// Len returns the number of units in the queue
func (q *ReadyQueue) Len() int

// Remove removes a specific unit from the queue
func (q *ReadyQueue) Remove(unitID string) bool
```

### State Machine

```
                              Schedule()
                                  │
                                  ▼
                          ┌─────────────┐
                          │   pending   │
                          └──────┬──────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
    deps satisfied        dep failed          (wait)
              │                  │                  │
              ▼                  ▼                  │
      ┌─────────────┐    ┌─────────────┐           │
      │    ready    │    │   blocked   │◄──────────┘
      └──────┬──────┘    └─────────────┘
             │                  │
        Dispatch()         (terminal)
             │
             ▼
      ┌─────────────┐
      │ in_progress │
      └──────┬──────┘
             │
    ┌────────┼────────┐
    │        │        │
  PR open  Complete  Fail
    │        │        │
    ▼        │        ▼
┌─────────┐  │  ┌─────────────┐
│ pr_open │  │  │   failed    │
└────┬────┘  │  └─────────────┘
     │       │        │
  review     │   (terminal)
     │       │
     ▼       │
┌──────────┐ │
│in_review │ │
└────┬─────┘ │
     │       │
  approve    │
     │       │
     ▼       │
┌──────────┐ │
│ merging  │ │
└────┬─────┘ │
     │       │
   merge     │
     │       │
     ▼       ▼
┌─────────────────┐
│    complete     │
└─────────────────┘
      │
  (terminal)
```

### Scheduler Flow

```
Input: []*discovery.Unit, maxParallelism int

Initialization:
  1. Build dependency graph from units
  2. Validate graph (no cycles, no missing refs)
  3. Compute topological sort
  4. Initialize all units as pending
  5. Evaluate initial ready set (units with no deps)

Main Loop (driven by events and Dispatch calls):
  1. On Dispatch():
     a. Check parallelism: if ActiveCount() >= maxParallelism, return AtCapacity
     b. Pop next unit from ready queue
     c. If none, return NoReady or AllComplete/AllBlocked
     d. Transition unit: ready -> in_progress
     e. Emit UnitStarted event
     f. Return unit ID

  2. On Complete(unitID):
     a. Transition unit: -> complete
     b. Emit UnitCompleted event
     c. For each dependent of unitID:
        - Check if all its dependencies are complete
        - If yes: transition pending -> ready, push to ready queue
        - Emit UnitQueued event

  3. On Fail(unitID, err):
     a. Transition unit: -> failed
     b. Emit UnitFailed event
     c. For each dependent of unitID:
        - Transition pending -> blocked
        - Record BlockedBy = [unitID]
        - Emit UnitBlocked event
        - Recursively propagate blocked to their dependents
```

### Dependency Resolution Example

```
Given units:
  app-shell:  depends_on: [project-setup, config]
  deck-list:  depends_on: [config]
  config:     depends_on: [project-setup]
  project-setup: depends_on: []

Graph:
  project-setup ──► config ──► app-shell
                      │
                      └──► deck-list

Topological order: [project-setup, config, app-shell, deck-list]
                   or [project-setup, config, deck-list, app-shell]

Levels:
  Level 0: [project-setup]
  Level 1: [config]
  Level 2: [app-shell, deck-list]

With parallelism=2:
  t=0: Dispatch project-setup (active=1)
  t=1: project-setup completes, config becomes ready
  t=1: Dispatch config (active=1)
  t=2: config completes, app-shell and deck-list become ready
  t=2: Dispatch app-shell (active=1)
  t=2: Dispatch deck-list (active=2)
  t=3: Both complete
```

## Implementation Notes

### Thread Safety

The scheduler is accessed concurrently by multiple workers reporting completion. All state mutations must be protected:

```go
func (s *Scheduler) Complete(unitID string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    state := s.states[unitID]
    if state == nil {
        return
    }

    now := time.Now()
    state.Status = StatusComplete
    state.CompletedAt = &now

    s.events.Emit(events.Event{
        Type: events.UnitCompleted,
        Unit: unitID,
    })

    // Re-evaluate dependents
    for _, depID := range s.graph.GetDependents(unitID) {
        s.evaluateReady(depID)
    }
}

func (s *Scheduler) evaluateReady(unitID string) {
    // Called with lock held
    state := s.states[unitID]
    if state == nil || state.Status != StatusPending {
        return
    }

    // Check all dependencies
    for _, depID := range s.graph.GetDependencies(unitID) {
        depState := s.states[depID]
        if depState == nil || depState.Status != StatusComplete {
            return
        }
    }

    // All deps complete, move to ready
    state.Status = StatusReady
    s.ready.Push(unitID)

    s.events.Emit(events.Event{
        Type: events.UnitQueued,
        Unit: unitID,
    })
}
```

### Cycle Detection

Uses Kahn's algorithm during topological sort to detect cycles:

```go
func (g *Graph) TopologicalSort() ([]string, error) {
    // Calculate in-degrees
    inDegree := make(map[string]int)
    for node := range g.nodes {
        inDegree[node] = len(g.edges[node])
    }

    // Find nodes with no incoming edges
    var queue []string
    for node, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, node)
        }
    }

    var result []string
    for len(queue) > 0 {
        node := queue[0]
        queue = queue[1:]
        result = append(result, node)

        // Reduce in-degree of dependents
        for _, dep := range g.dependents[node] {
            inDegree[dep]--
            if inDegree[dep] == 0 {
                queue = append(queue, dep)
            }
        }
    }

    // If we didn't visit all nodes, there's a cycle
    if len(result) != len(g.nodes) {
        cycle := g.findCycle()
        return nil, &CycleError{Cycle: cycle}
    }

    return result, nil
}
```

### Failure Propagation

When a unit fails, all transitive dependents are marked as blocked:

```go
func (s *Scheduler) Fail(unitID string, err error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    state := s.states[unitID]
    if state == nil {
        return
    }

    now := time.Now()
    state.Status = StatusFailed
    state.CompletedAt = &now
    state.Error = err

    s.events.Emit(events.Event{
        Type:  events.UnitFailed,
        Unit:  unitID,
        Error: err.Error(),
    })

    // Propagate blocked status to all dependents
    s.propagateBlocked(unitID, unitID)
}

func (s *Scheduler) propagateBlocked(failedID, currentID string) {
    // Called with lock held
    for _, depID := range s.graph.GetDependents(currentID) {
        state := s.states[depID]
        if state == nil || state.Status.IsTerminal() {
            continue
        }

        // Remove from ready queue if present
        s.ready.Remove(depID)

        state.Status = StatusBlocked
        state.BlockedBy = append(state.BlockedBy, failedID)
        now := time.Now()
        state.CompletedAt = &now

        s.events.Emit(events.Event{
            Type: events.UnitBlocked,
            Unit: depID,
            Payload: map[string]any{
                "blocked_by": failedID,
            },
        })

        // Recursively propagate
        s.propagateBlocked(failedID, depID)
    }
}
```

### Parallelism Tracking

The parallelism limit applies to units in active states (in_progress through merging):

```go
func (s *Scheduler) ActiveCount() int {
    s.mu.RLock()
    defer s.mu.RUnlock()

    count := 0
    for _, state := range s.states {
        if state.Status.IsActive() {
            count++
        }
    }
    return count
}

func (s *Scheduler) Dispatch() DispatchResult {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Check parallelism limit
    active := 0
    for _, state := range s.states {
        if state.Status.IsActive() {
            active++
        }
    }
    if active >= s.maxParallelism {
        return DispatchResult{Reason: ReasonAtCapacity}
    }

    // Try to get next ready unit
    unitID := s.ready.Pop()
    if unitID == "" {
        if s.IsComplete() {
            return DispatchResult{Reason: ReasonAllComplete}
        }
        if s.allBlockedOrComplete() {
            return DispatchResult{Reason: ReasonAllBlocked}
        }
        return DispatchResult{Reason: ReasonNoReady}
    }

    // Transition to in_progress
    state := s.states[unitID]
    now := time.Now()
    state.Status = StatusInProgress
    state.StartedAt = &now

    s.events.Emit(events.Event{
        Type: events.UnitStarted,
        Unit: unitID,
    })

    return DispatchResult{
        Unit:       unitID,
        Dispatched: true,
    }
}
```

## Testing Strategy

### Unit Tests

```go
// internal/scheduler/graph_test.go

func TestGraph_Build(t *testing.T) {
    tests := []struct {
        name    string
        units   []*discovery.Unit
        wantErr bool
    }{
        {
            name: "simple chain",
            units: []*discovery.Unit{
                {ID: "a", DependsOn: []string{}},
                {ID: "b", DependsOn: []string{"a"}},
                {ID: "c", DependsOn: []string{"b"}},
            },
            wantErr: false,
        },
        {
            name: "cycle detected",
            units: []*discovery.Unit{
                {ID: "a", DependsOn: []string{"c"}},
                {ID: "b", DependsOn: []string{"a"}},
                {ID: "c", DependsOn: []string{"b"}},
            },
            wantErr: true,
        },
        {
            name: "missing dependency",
            units: []*discovery.Unit{
                {ID: "a", DependsOn: []string{"nonexistent"}},
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := NewGraph(tt.units)
            if (err != nil) != tt.wantErr {
                t.Errorf("NewGraph() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

func TestGraph_TopologicalSort(t *testing.T) {
    units := []*discovery.Unit{
        {ID: "d", DependsOn: []string{"b", "c"}},
        {ID: "b", DependsOn: []string{"a"}},
        {ID: "c", DependsOn: []string{"a"}},
        {ID: "a", DependsOn: []string{}},
    }

    graph, err := NewGraph(units)
    if err != nil {
        t.Fatalf("NewGraph() error = %v", err)
    }

    order, err := graph.TopologicalSort()
    if err != nil {
        t.Fatalf("TopologicalSort() error = %v", err)
    }

    // Verify a comes before b and c
    aIdx := indexOf(order, "a")
    bIdx := indexOf(order, "b")
    cIdx := indexOf(order, "c")
    dIdx := indexOf(order, "d")

    if aIdx > bIdx || aIdx > cIdx {
        t.Errorf("a must come before b and c, got order: %v", order)
    }
    if bIdx > dIdx || cIdx > dIdx {
        t.Errorf("b and c must come before d, got order: %v", order)
    }
}
```

```go
// internal/scheduler/state_test.go

func TestUnitStatus_IsTerminal(t *testing.T) {
    terminals := []UnitStatus{StatusComplete, StatusFailed, StatusBlocked}
    nonTerminals := []UnitStatus{StatusPending, StatusReady, StatusInProgress,
                                  StatusPROpen, StatusInReview, StatusMerging}

    for _, s := range terminals {
        if !s.IsTerminal() {
            t.Errorf("%v.IsTerminal() = false, want true", s)
        }
    }

    for _, s := range nonTerminals {
        if s.IsTerminal() {
            t.Errorf("%v.IsTerminal() = true, want false", s)
        }
    }
}

func TestCanTransition(t *testing.T) {
    tests := []struct {
        from, to UnitStatus
        want     bool
    }{
        {StatusPending, StatusReady, true},
        {StatusPending, StatusBlocked, true},
        {StatusPending, StatusComplete, false},
        {StatusReady, StatusInProgress, true},
        {StatusInProgress, StatusPROpen, true},
        {StatusInProgress, StatusComplete, true},
        {StatusComplete, StatusPending, false}, // terminal
    }

    for _, tt := range tests {
        name := fmt.Sprintf("%v->%v", tt.from, tt.to)
        t.Run(name, func(t *testing.T) {
            if got := CanTransition(tt.from, tt.to); got != tt.want {
                t.Errorf("CanTransition(%v, %v) = %v, want %v",
                    tt.from, tt.to, got, tt.want)
            }
        })
    }
}
```

```go
// internal/scheduler/scheduler_test.go

func TestScheduler_Dispatch(t *testing.T) {
    bus := events.NewBus(100)
    defer bus.Close()

    sched := New(bus, 2) // max 2 parallel

    units := []*discovery.Unit{
        {ID: "a", DependsOn: []string{}},
        {ID: "b", DependsOn: []string{}},
        {ID: "c", DependsOn: []string{}},
    }

    _, err := sched.Schedule(units)
    if err != nil {
        t.Fatalf("Schedule() error = %v", err)
    }

    // Should dispatch first two
    r1 := sched.Dispatch()
    if !r1.Dispatched {
        t.Error("first dispatch should succeed")
    }

    r2 := sched.Dispatch()
    if !r2.Dispatched {
        t.Error("second dispatch should succeed")
    }

    // Third should be blocked by parallelism
    r3 := sched.Dispatch()
    if r3.Dispatched {
        t.Error("third dispatch should be blocked")
    }
    if r3.Reason != ReasonAtCapacity {
        t.Errorf("reason = %v, want %v", r3.Reason, ReasonAtCapacity)
    }
}

func TestScheduler_FailurePropagation(t *testing.T) {
    bus := events.NewBus(100)
    defer bus.Close()

    sched := New(bus, 4)

    units := []*discovery.Unit{
        {ID: "a", DependsOn: []string{}},
        {ID: "b", DependsOn: []string{"a"}},
        {ID: "c", DependsOn: []string{"b"}},
    }

    _, err := sched.Schedule(units)
    if err != nil {
        t.Fatalf("Schedule() error = %v", err)
    }

    // Dispatch and fail unit a
    sched.Dispatch() // dispatches "a"
    sched.Fail("a", errors.New("test error"))

    // b and c should be blocked
    stateB, _ := sched.GetState("b")
    stateC, _ := sched.GetState("c")

    if stateB.Status != StatusBlocked {
        t.Errorf("b status = %v, want blocked", stateB.Status)
    }
    if stateC.Status != StatusBlocked {
        t.Errorf("c status = %v, want blocked", stateC.Status)
    }
}
```

```go
// internal/scheduler/queue_test.go

func TestReadyQueue_FIFO(t *testing.T) {
    q := NewReadyQueue()

    q.Push("a")
    q.Push("b")
    q.Push("c")

    if got := q.Pop(); got != "a" {
        t.Errorf("first Pop() = %q, want %q", got, "a")
    }
    if got := q.Pop(); got != "b" {
        t.Errorf("second Pop() = %q, want %q", got, "b")
    }
    if got := q.Pop(); got != "c" {
        t.Errorf("third Pop() = %q, want %q", got, "c")
    }
    if got := q.Pop(); got != "" {
        t.Errorf("fourth Pop() = %q, want empty", got)
    }
}

func TestReadyQueue_Remove(t *testing.T) {
    q := NewReadyQueue()

    q.Push("a")
    q.Push("b")
    q.Push("c")

    if !q.Remove("b") {
        t.Error("Remove(b) should return true")
    }

    if q.Contains("b") {
        t.Error("b should not be in queue after removal")
    }

    if got := q.Len(); got != 2 {
        t.Errorf("Len() = %d, want 2", got)
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Diamond dependency | A <- B, A <- C, B <- D, C <- D; verify D runs after B and C |
| Parallel independence | 4 units with no deps, parallelism=4; all dispatch immediately |
| Chain execution | A <- B <- C <- D; verify sequential order |
| Partial failure | A <- B <- C, B fails; verify C blocked, A unaffected |
| Resume from state | Pre-populate with mixed states; verify correct ready queue |

### Manual Testing

- [ ] Create units with cycle, verify error message is clear
- [ ] Run with parallelism=1, verify sequential execution
- [ ] Kill orchestrator mid-run, verify state allows resume
- [ ] Create deep dependency chain, verify no stack overflow
- [ ] Test with 50+ units for performance baseline

## Design Decisions

### Why FIFO Ready Queue?

Simpler than priority queuing and provides deterministic order. When multiple units become ready simultaneously (same dependency level), they execute in discovery order. This makes debugging easier and ensures reproducible runs.

Alternatives considered: priority queue (by estimated duration), random (for load distribution). These add complexity without clear MVP benefit.

### Why Combined Parallelism Limit?

Counting both running and pr_phase units against the limit prevents resource exhaustion. A unit in PR review still consumes a worktree and may need sudden work (conflict resolution, feedback addressing). Separating the limits would complicate reasoning about resource usage.

### Why Propagate Blocked Immediately?

Eager failure propagation allows the scheduler to immediately recalculate which units can proceed and which are doomed. This provides faster feedback to users and allows early termination when all remaining units are blocked.

Alternative: lazy evaluation (check on dispatch). This delays feedback and complicates IsComplete() logic.

### Why Events for All Transitions?

Every state transition emits an event to:
1. Enable logging without coupling to log implementation
2. Support future UIs (TUI, web dashboard) without scheduler changes
3. Allow state persistence handler to sync frontmatter
4. Facilitate testing by capturing event sequences

## Future Enhancements

1. Priority scheduling based on critical path analysis
2. Estimated duration tracking for better parallelism decisions
3. Unit retry with backoff for transient failures
4. Pause/resume individual units
5. Dynamic parallelism adjustment based on system load

## References

- [PRD Section 4.2: Scheduler Flow](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [Discovery Package Types](/Users/bennett/conductor/workspaces/choo/lahore/specs/DISCOVERY.md)
- [Events Package](/Users/bennett/conductor/workspaces/choo/lahore/specs/EVENTS.md)
