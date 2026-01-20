---
task: 2
status: complete
backpressure: "go test ./internal/web/... -run TestStore"
depends_on: [1]
---

# Implement State Store

**Parent spec**: `/specs/WEB.md`
**Task**: #2 of 7 in implementation plan

## Objective

Implement the in-memory state store that tracks orchestration status, processes events, and provides state snapshots.

## Dependencies

### Task Dependencies (within this unit)
- #1 (types.go) - Event, UnitState, GraphData, StateSnapshot types

### Package Dependencies
- `encoding/json`
- `sync`

## Deliverables

### Files to Create

```
internal/web/
├── store.go       # CREATE: Store implementation
└── store_test.go  # CREATE: Store tests
```

### Types to Implement

```go
package web

import (
    "encoding/json"
    "sync"
)

// Store maintains the current orchestration state.
// It is safe for concurrent access.
type Store struct {
    mu          sync.RWMutex
    connected   bool              // true when orchestrator is connected
    status      string            // "waiting", "running", "completed", "failed"
    startedAt   time.Time
    parallelism int
    graph       *GraphData
    units       map[string]*UnitState
}
```

### Functions to Implement

```go
// NewStore creates an empty state store in "waiting" status.
func NewStore() *Store

// HandleEvent processes an event and updates state accordingly.
// Thread-safe. Event type determines state transition:
//   - orch.started: set status="running", store graph, init units
//   - unit.queued: set unit status to "ready"
//   - unit.started: set unit status to "in_progress", set startedAt
//   - task.started: increment currentTask
//   - unit.completed: set unit status to "complete"
//   - unit.failed: set unit status to "failed", store error
//   - unit.blocked: set unit status to "blocked"
//   - orch.completed: set status="completed"
//   - orch.failed: set status="failed"
func (s *Store) HandleEvent(e *Event)

// Snapshot returns the current state as a StateSnapshot.
// Thread-safe for concurrent reads.
func (s *Store) Snapshot() *StateSnapshot

// Graph returns the dependency graph, or nil if not yet received.
// Thread-safe.
func (s *Store) Graph() *GraphData

// SetConnected updates the connection status.
// Called when orchestrator connects/disconnects from socket.
func (s *Store) SetConnected(connected bool)

// Reset clears all state for a new run.
// Returns store to "waiting" status with no units.
func (s *Store) Reset()
```

### Tests to Implement

```go
// store_test.go

func TestStore_NewStore(t *testing.T)
// - NewStore returns store with status="waiting"
// - connected=false, empty units

func TestStore_HandleOrchStarted(t *testing.T)
// - Sets status to "running"
// - Stores parallelism from payload
// - Stores graph from payload
// - Initializes unit states from graph nodes

func TestStore_HandleUnitLifecycle(t *testing.T)
// - unit.queued sets status to "ready"
// - unit.started sets status to "in_progress"
// - unit.completed sets status to "complete"
// - unit.failed sets status to "failed" with error

func TestStore_HandleTaskStarted(t *testing.T)
// - task.started increments currentTask

func TestStore_HandleOrchCompleted(t *testing.T)
// - orch.completed sets status to "completed"

func TestStore_HandleOrchFailed(t *testing.T)
// - orch.failed sets status to "failed"

func TestStore_SummaryCalculation(t *testing.T)
// - Summary counts match unit statuses
// - Total equals number of units

func TestStore_SetConnected(t *testing.T)
// - SetConnected(true) sets connected=true
// - SetConnected(false) sets connected=false

func TestStore_Reset(t *testing.T)
// - Reset clears units
// - Reset sets status to "waiting"

func TestStore_ConcurrentAccess(t *testing.T)
// - Multiple goroutines can read/write safely
```

## Backpressure

### Validation Command

```bash
go test ./internal/web/... -run TestStore -v
```

### Must Pass
- All TestStore* tests pass
- No data races (run with `-race`)

### CI Compatibility
- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `sync.RWMutex` for concurrent access: `RLock` for reads, `Lock` for writes
- Initialize `units` map in `NewStore` to avoid nil map panic
- `HandleEvent` should silently ignore unknown event types
- When handling `unit.*` events, check if unit exists in map before updating
- `Snapshot` should return a copy of unit slice, not reference to internal map
- `Graph()` should return the stored graph pointer (shared reference is fine for read-only)

### State Update Mapping

| Event | State Update |
|-------|--------------|
| `orch.started` | set status="running", store graph, init units from nodes |
| `unit.queued` | set unit status to "ready" |
| `unit.started` | set unit status to "in_progress", set startedAt |
| `task.started` | increment currentTask |
| `unit.completed` | set unit status to "complete" |
| `unit.failed` | set unit status to "failed", store error |
| `unit.blocked` | set unit status to "blocked" |
| `orch.completed` | set status="completed" |
| `orch.failed` | set status="failed" |

## NOT In Scope

- SSE hub (task #3)
- Socket handling (task #4)
- HTTP handlers (task #5)
- Server lifecycle (task #6)
