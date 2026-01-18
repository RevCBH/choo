---
task: 6
status: pending
backpressure: "go test ./internal/scheduler/... -run TestFailure"
depends_on: [4, 5]
---

# Failure Propagation and Completion

**Parent spec**: `/specs/SCHEDULER.md`
**Task**: #6 of 6 in implementation plan

## Objective

Implement Complete() and Fail() methods with dependency re-evaluation and blocked status propagation.

## Dependencies

### External Specs (must be implemented)
- EVENTS - provides event types for UnitCompleted, UnitFailed, UnitBlocked, UnitQueued

### Task Dependencies (within this unit)
- Task #4 (provides: Scheduler struct, states, graph, ready queue, evaluateReady)
- Task #5 (provides: Dispatch method, DispatchResult types)

### Package Dependencies
- Standard library only (`time`)

## Deliverables

### Files to Create/Modify

```
internal/
└── scheduler/
    ├── complete.go       # CREATE: Complete and Fail methods
    └── complete_test.go  # CREATE: Completion and failure tests
```

### Functions to Implement

```go
// internal/scheduler/complete.go

// Complete marks a unit as complete and re-evaluates pending units
// Emits UnitCompleted event and potentially UnitQueued for dependents
func (s *Scheduler) Complete(unitID string)

// Fail marks a unit as failed and propagates blocked status to dependents
// Emits UnitFailed event and UnitBlocked for affected dependents
func (s *Scheduler) Fail(unitID string, err error)

// propagateBlocked recursively marks dependents as blocked
// Called with lock held
func (s *Scheduler) propagateBlocked(failedID, currentID string)
```

## Backpressure

### Validation Command

```bash
go test ./internal/scheduler/... -run TestFailure -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestComplete_SetsStatus` | Unit status becomes complete |
| `TestComplete_SetsCompletedAt` | Unit state has CompletedAt timestamp |
| `TestComplete_EmitsEvent` | UnitCompleted event emitted |
| `TestComplete_UnblocksDependents` | Dependent with all deps complete moves to ready |
| `TestComplete_PartialUnblock` | Dependent with remaining deps stays pending |
| `TestComplete_EmitsQueuedEvent` | UnitQueued event for newly ready units |
| `TestFail_SetsStatus` | Unit status becomes failed |
| `TestFail_SetsError` | Unit state has Error field set |
| `TestFail_EmitsEvent` | UnitFailed event emitted with error message |
| `TestFail_BlocksDependents` | Direct dependents become blocked |
| `TestFail_BlocksTransitive` | Transitive dependents become blocked |
| `TestFail_RecordsBlockedBy` | BlockedBy field contains original failed unit |
| `TestFail_EmitsBlockedEvents` | UnitBlocked event for each blocked unit |
| `TestFail_RemovesFromReadyQueue` | Blocked units removed from ready queue |
| `TestFail_SkipsTerminalUnits` | Already complete/failed units not affected |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| None | N/A | Tests use mock events.Bus and inline unit construction |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Complete() must re-evaluate ALL dependents of the completed unit
- A dependent becomes ready only when ALL its dependencies are complete
- Fail() uses recursive propagation: blocked units' dependents are also blocked
- BlockedBy should contain the root failed unit ID, not intermediate blocked ones
- Remove units from ready queue before marking as blocked
- Use graph.GetDependents() to find affected units
- Both methods need full lock (not RLock) since they modify state

## NOT In Scope

- Retry logic (future enhancement)
- Partial failure recovery
- Worker pool notification (workers poll Dispatch)
