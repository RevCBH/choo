---
unit: scheduler
depends_on: [discovery, events]
---

# SCHEDULER Implementation Plan

## Overview

The Scheduler package manages unit execution order via dependency graph construction, state machine management, and dispatch coordination. This decomposition separates the concerns into: graph types and algorithms, state machine logic, ready queue data structure, and dispatch coordination.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-graph-types.md | Graph data structures and construction | None | `go test ./internal/scheduler/... -run TestGraph` |
| 2 | 02-state-machine.md | Unit status types and transition validation | None | `go test ./internal/scheduler/... -run TestState` |
| 3 | 03-ready-queue.md | Thread-safe ready queue implementation | None | `go test ./internal/scheduler/... -run TestReadyQueue` |
| 4 | 04-scheduler-core.md | Scheduler struct and Schedule() method | #1, #2, #3 | `go test ./internal/scheduler/... -run TestScheduler` |
| 5 | 05-dispatch.md | Dispatch logic and parallelism control | #4 | `go test ./internal/scheduler/... -run TestDispatch` |
| 6 | 06-failure-propagation.md | Complete/Fail methods and blocked propagation | #4, #5 | `go test ./internal/scheduler/... -run TestFailure` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/scheduler/... && go vet ./internal/scheduler/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] Full test suite: `go test ./internal/scheduler/... -v`
- [ ] PR created and merged

## Reference

- Design spec: `/specs/SCHEDULER.md`
- Dependency: `/specs/DISCOVERY.md` (provides Unit type)
- Dependency: `/specs/EVENTS.md` (provides Bus type)
