---
unit: cli
depends_on: [discovery, scheduler, worker, git, github, events, config]
---

# CLI Implementation Plan

## Overview

The CLI package provides the command-line interface for choo using the Cobra framework. It serves as the entry point that wires together all other components. This unit implements five commands (run, status, resume, cleanup, version), signal handling for graceful shutdown, and rich status output with progress bars.

## Decomposition Strategy

The CLI is decomposed into these concerns:
1. **Root command and App struct** - Foundation that other commands attach to
2. **Display formatting** - Progress bars and status symbols (shared utility)
3. **Signal handling** - Graceful shutdown infrastructure
4. **Individual commands** - run, status, resume, cleanup, version
5. **Component wiring** - Dependency injection and orchestrator assembly

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-root-command.md | Root Cobra command and App struct | None | `go build ./internal/cli/...` |
| 2 | 02-display.md | Progress bars and status symbols | None | `go test ./internal/cli/... -run Display` |
| 3 | 03-signals.md | Signal handler for graceful shutdown | #1 | `go test ./internal/cli/... -run Signal` |
| 4 | 04-version-command.md | Version command implementation | #1 | `go test ./internal/cli/... -run Version` |
| 5 | 05-status-command.md | Status command with formatting | #1, #2 | `go test ./internal/cli/... -run Status` |
| 6 | 06-run-command.md | Run command with options | #1, #3 | `go test ./internal/cli/... -run Run` |
| 7 | 07-resume-command.md | Resume command implementation | #1, #6 | `go test ./internal/cli/... -run Resume` |
| 8 | 08-cleanup-command.md | Cleanup command implementation | #1 | `go test ./internal/cli/... -run Cleanup` |
| 9 | 09-wire.md | Component assembly and wiring | #1, #6 | `go test ./internal/cli/... -run Wire` |

## Dependency Graph

```
01-root-command ─┬─► 03-signals ─────────────────┐
                 │                               │
                 ├─► 04-version                  │
                 │                               ▼
02-display ──────┼─► 05-status          06-run ─► 07-resume
                 │                         │
                 ├─► 08-cleanup            │
                 │                         │
                 └─────────────────────────┴─► 09-wire
```

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/cli/... && go vet ./internal/cli/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `go test ./internal/cli/...` passes
- [ ] PR created and merged

## Reference

- Design spec: `/specs/CLI.md`
