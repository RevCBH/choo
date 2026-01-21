---
unit: daemon-core
depends_on:
  - daemon-db
  - daemon-grpc
---

# DAEMON-CORE Implementation Plan

## Overview

The DAEMON-CORE unit implements the long-running daemon process that manages job execution for choo. This includes process lifecycle management (PID files, signal handling), the JobManager for concurrent orchestrator instances, and crash-resilient resume logic using SQLite-backed state persistence.

## Decomposition Strategy

The implementation is decomposed by module:
1. **Config** - Configuration types with sensible defaults
2. **PID** - PID file management for single-instance enforcement
3. **Job Manager Types** - ManagedJob and JobConfig types
4. **Job Manager Core** - Job creation, tracking, and lifecycle
5. **Job Manager Events** - Event subscription and streaming
6. **Resume** - Job resume logic and validation
7. **Daemon Lifecycle** - Main daemon struct and startup/shutdown

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-config.md | Define Config struct with defaults and validation | None | `go build ./internal/daemon/...` |
| 2 | 02-pid.md | Implement PID file utilities for single-instance enforcement | None | `go test ./internal/daemon/... -run TestPID` |
| 3 | 03-job-types.md | Define ManagedJob and JobConfig types | None | `go build ./internal/daemon/...` |
| 4 | 04-job-manager.md | Implement job creation, tracking, and lifecycle | #3 | `go test ./internal/daemon/... -run TestJobManager` |
| 5 | 05-job-events.md | Implement event subscription and streaming per job | #4 | `go test ./internal/daemon/... -run TestJobEvents` |
| 6 | 06-resume.md | Implement job resume logic and validation | #4 | `go test ./internal/daemon/... -run TestResume` |
| 7 | 07-daemon.md | Wire daemon lifecycle with startup and shutdown | #1, #2, #4, #6 | `go test ./internal/daemon/... -run TestDaemon` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/daemon/... && go vet ./internal/daemon/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `go test ./internal/daemon/...` passes all tests
- [ ] PR created and merged

## Reference

- Design spec: `specs/DAEMON-CORE.md`
- Related: `specs/DAEMON-DB.md` (database layer)
- Related: `specs/DAEMON-GRPC.md` (gRPC interface)
