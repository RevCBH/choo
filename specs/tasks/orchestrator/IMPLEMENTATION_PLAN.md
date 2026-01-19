---
unit: orchestrator
depends_on: [escalation]
---

# ORCHESTRATOR Implementation Plan

## Overview

The Orchestrator is the central coordination component that wires together all subsystems to execute units in parallel. It transforms the `choo run` command from a stub with TODO placeholders into a functioning orchestration engine.

This implementation is decomposed into six atomic tasks:
1. Core types and constructor
2. Main Run() loop with discovery and scheduling
3. Event handling and scheduler integration
4. Graceful shutdown with timeout
5. Dry-run mode for execution plan preview
6. CLI wire integration replacing TODOs

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types.md | Define Orchestrator struct, Config, Result, and helper types | None | `go build ./internal/orchestrator/...` |
| 2 | 02-main-loop.md | Implement Run() with discovery, scheduling, dispatch loop | #1 | `go test ./internal/orchestrator/... -run TestOrchestrator_Run` |
| 3 | 03-event-handling.md | Implement event subscription and handleEvent() | #1, #2 | `go test ./internal/orchestrator/... -run TestOrchestrator_HandleEvent` |
| 4 | 04-shutdown.md | Implement graceful shutdown with timeout | #1, #2 | `go test ./internal/orchestrator/... -run TestOrchestrator_Shutdown` |
| 5 | 05-dry-run.md | Implement dry-run mode for execution plan preview | #1, #2 | `go test ./internal/orchestrator/... -run TestOrchestrator_DryRun` |
| 6 | 06-wire-cli.md | Wire orchestrator into CLI run command | #1, #2, #3, #4, #5 | `go build ./cmd/choo/... && go test ./internal/cli/... -run TestRun` |

## Baseline Checks

```bash
go fmt ./... && go vet ./...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] `go test ./internal/orchestrator/...` passes with all tests
- [ ] Baseline checks pass
- [ ] `choo run specs/tasks/` executes correctly
- [ ] PR created and merged

## Reference

- Design spec: `/specs/ORCHESTRATOR.md`
- CLI run command: `/internal/cli/run.go`
- Wire infrastructure: `/internal/cli/wire.go`
