---
task: 2
status: complete
backpressure: "go test ./internal/scheduler/... -run TestState"
depends_on: []
---

# State Machine and Transitions

**Parent spec**: `/specs/SCHEDULER.md`
**Task**: #2 of 6 in implementation plan

## Objective

Implement the unit status enum, state machine transitions, and UnitState tracking structure.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None (independent of Task #1)

### Package Dependencies
- Standard library only (`time`)

## Deliverables

### Files to Create/Modify

```
internal/
└── scheduler/
    ├── state.go       # CREATE: UnitState and status types
    └── state_test.go  # CREATE: State machine tests
```

### Types to Implement

```go
// internal/scheduler/state.go

// UnitStatus represents the unit's lifecycle state
type UnitStatus string

const (
    StatusPending    UnitStatus = "pending"
    StatusReady      UnitStatus = "ready"
    StatusInProgress UnitStatus = "in_progress"
    StatusPROpen     UnitStatus = "pr_open"
    StatusInReview   UnitStatus = "in_review"
    StatusMerging    UnitStatus = "merging"
    StatusComplete   UnitStatus = "complete"
    StatusFailed     UnitStatus = "failed"
    StatusBlocked    UnitStatus = "blocked"
)

// UnitState tracks the current state of a unit
type UnitState struct {
    UnitID      string
    Status      UnitStatus
    BlockedBy   []string
    StartedAt   *time.Time
    CompletedAt *time.Time
    Error       error
}

// ValidTransitions defines allowed state transitions
var ValidTransitions = map[UnitStatus][]UnitStatus{
    StatusPending:    {StatusReady, StatusBlocked},
    StatusReady:      {StatusInProgress, StatusBlocked},
    StatusInProgress: {StatusPROpen, StatusComplete, StatusFailed},
    StatusPROpen:     {StatusInReview, StatusComplete, StatusFailed},
    StatusInReview:   {StatusMerging, StatusPROpen, StatusFailed},
    StatusMerging:    {StatusComplete, StatusFailed},
    StatusComplete:   {},
    StatusFailed:     {},
    StatusBlocked:    {},
}
```

### Functions to Implement

```go
// IsTerminal returns true if the status is a final state
func (s UnitStatus) IsTerminal() bool

// IsActive returns true if the unit is consuming a parallelism slot
func (s UnitStatus) IsActive() bool

// CanTransition checks if a transition from -> to is valid
func CanTransition(from, to UnitStatus) bool

// NewUnitState creates initial state for a unit (status = pending)
func NewUnitState(unitID string) *UnitState
```

## Backpressure

### Validation Command

```bash
go test ./internal/scheduler/... -run TestState -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestUnitStatus_IsTerminal` | complete, failed, blocked return true |
| `TestUnitStatus_IsTerminal_NonTerminal` | pending, ready, in_progress, pr_open, in_review, merging return false |
| `TestUnitStatus_IsActive` | in_progress, pr_open, in_review, merging return true |
| `TestUnitStatus_IsActive_Inactive` | pending, ready, complete, failed, blocked return false |
| `TestCanTransition_Valid` | All ValidTransitions entries return true |
| `TestCanTransition_Invalid` | pending->complete returns false |
| `TestCanTransition_Terminal` | complete->anything returns false |
| `TestNewUnitState` | Creates state with pending status and correct ID |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| None | N/A | Tests use inline construction |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Terminal states: complete, failed, blocked (no outgoing transitions)
- Active states consume parallelism slots: in_progress, pr_open, in_review, merging
- The state machine diagram in the spec shows all valid paths
- StatusInReview can go back to StatusPROpen (reviewer removes eyes emoji)

## NOT In Scope

- Actual state transitions (done in Scheduler.Transition, Task #4)
- Event emission on transitions (Task #4)
- BlockedBy propagation logic (Task #6)
