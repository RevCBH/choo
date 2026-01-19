---
unit: worker
depends_on: [discovery, events, git, github, claude]
---

# WORKER Implementation Plan

## Overview

The Worker package implements the core task execution engine ("Ralph loop") that runs within a git worktree to complete a unit's tasks. This implementation is decomposed into 8 atomic tasks that build up the worker functionality incrementally.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types-config.md | Core types and WorkerConfig | None | `go build ./internal/worker/...` |
| 2 | 02-prompt.md | Task prompt construction | None | `go test ./internal/worker/... -run Prompt` |
| 3 | 03-backpressure.md | Backpressure command runner | None | `go test ./internal/worker/... -run Backpressure` |
| 4 | 04-baseline.md | Baseline checks runner | None | `go test ./internal/worker/... -run Baseline` |
| 5 | 05-loop.md | Ralph loop implementation | #1, #2, #3 | `go test ./internal/worker/... -run Loop` |
| 6 | 06-worker.md | Single unit worker | #1, #4, #5 | `go test ./internal/worker/... -run Worker` |
| 7 | 07-pool.md | Worker pool management | #6 | `go test ./internal/worker/... -run Pool` |
| 8 | 08-execute.md | Public Execute entry point | #6, #7 | `go test ./internal/worker/...` |

## Dependency Graph

```
01-types-config ──┬──────────────────────────────┐
                  │                              │
02-prompt ────────┼──► 05-loop ──► 06-worker ───┼──► 08-execute
                  │              ▲               │
03-backpressure ──┘              │               │
                                 │               │
04-baseline ─────────────────────┘               │
                                                 │
                                 07-pool ────────┘
```

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/worker/... && go vet ./internal/worker/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/WORKER.md`
