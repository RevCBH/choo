---
task: 3
status: pending
backpressure: "go test ./internal/scheduler/... -run TestReadyQueue"
depends_on: []
---

# Ready Queue Implementation

**Parent spec**: `/specs/SCHEDULER.md`
**Task**: #3 of 6 in implementation plan

## Objective

Implement the thread-safe FIFO ready queue for units awaiting dispatch.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None (independent of Tasks #1 and #2)

### Package Dependencies
- Standard library only (`sync`)

## Deliverables

### Files to Create/Modify

```
internal/
└── scheduler/
    ├── queue.go       # CREATE: ReadyQueue implementation
    └── queue_test.go  # CREATE: Queue unit tests
```

### Types to Implement

```go
// internal/scheduler/queue.go

// ReadyQueue manages units ready for dispatch
type ReadyQueue struct {
    // queue of ready unit IDs (FIFO within same priority)
    queue []string

    // set for O(1) membership checks
    set map[string]bool

    mu sync.Mutex
}
```

### Functions to Implement

```go
// NewReadyQueue creates an empty ready queue
func NewReadyQueue() *ReadyQueue

// Push adds a unit ID to the ready queue
// No-op if unit is already in queue
func (q *ReadyQueue) Push(unitID string)

// Pop removes and returns the next ready unit ID
// Returns empty string if queue is empty
func (q *ReadyQueue) Pop() string

// Peek returns the next ready unit ID without removing it
// Returns empty string if queue is empty
func (q *ReadyQueue) Peek() string

// Contains checks if a unit ID is in the queue
func (q *ReadyQueue) Contains(unitID string) bool

// Len returns the number of units in the queue
func (q *ReadyQueue) Len() int

// Remove removes a specific unit from the queue
// Returns true if unit was found and removed
func (q *ReadyQueue) Remove(unitID string) bool

// List returns a copy of all unit IDs currently in queue
func (q *ReadyQueue) List() []string
```

## Backpressure

### Validation Command

```bash
go test ./internal/scheduler/... -run TestReadyQueue -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestReadyQueue_FIFO` | Push a,b,c; Pop returns a,b,c in order |
| `TestReadyQueue_PopEmpty` | Pop on empty queue returns "" |
| `TestReadyQueue_Peek` | Peek returns next without removal |
| `TestReadyQueue_PeekEmpty` | Peek on empty queue returns "" |
| `TestReadyQueue_Contains` | Returns true for queued, false for not queued |
| `TestReadyQueue_Len` | Returns correct count |
| `TestReadyQueue_Remove` | Removes item from middle of queue |
| `TestReadyQueue_RemoveNotFound` | Returns false for missing item |
| `TestReadyQueue_PushDuplicate` | Push same ID twice only adds once |
| `TestReadyQueue_List` | Returns copy of queue contents |
| `TestReadyQueue_Concurrent` | Push/Pop from multiple goroutines safely |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| None | N/A | Tests use inline construction |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Must be thread-safe: multiple workers may call Pop() concurrently
- Use both slice (for order) and map (for O(1) Contains/Remove)
- Remove must update both slice and map
- Push should be idempotent (no duplicate entries)
- Concurrent test should use sync.WaitGroup with 100+ goroutines

## NOT In Scope

- Priority scheduling (FIFO only for MVP)
- Timeout/deadline tracking
- Integration with Scheduler (Task #4)
