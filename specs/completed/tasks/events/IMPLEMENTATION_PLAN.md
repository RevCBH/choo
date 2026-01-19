---
unit: events
depends_on: []
---

# EVENTS Implementation Plan

## Overview

The Events package provides a decoupled communication mechanism for choo through an event bus architecture. This unit implements the core event system with buffered channels, fan-out dispatch, and built-in handlers for logging and state persistence.

## Decomposition Strategy

The EVENTS spec naturally decomposes into three tasks following the types-first pattern:

1. **Event Types** - Core data structures and event type constants
2. **Event Bus** - Handler registration, buffered channel dispatch, fan-out
3. **Built-in Handlers** - LogHandler and StateHandler implementations

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-event-types.md | Event struct, EventType constants, builder methods | None | `go test ./internal/events/...` |
| 2 | 02-event-bus.md | Bus struct, Subscribe, Emit, dispatch loop, Close | #1 | `go test ./internal/events/...` |
| 3 | 03-handlers.md | LogHandler and StateHandler implementations | #1, #2 | `go test ./internal/events/...` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./... && go vet ./...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/EVENTS.md`
