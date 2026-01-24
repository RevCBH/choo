---
unit: container-daemon
depends_on:
  - container-manager
  - json-events
---

# CONTAINER-DAEMON Implementation Plan

## Overview

CONTAINER-DAEMON integrates container-based job execution into the daemon. When container mode is enabled, the daemon creates isolated containers for each job, passes credentials via environment variables, streams container logs in real-time, and bridges parsed JSON events to the web UI. After all unit branches merge, the archive step moves completed specs to `specs/completed/`.

The implementation is decomposed into six atomic tasks following a types-first approach:

1. **Config Types** - Extend daemon config with container options
2. **Log Streamer** - Parse JSON events from container stdout
3. **Container Job Manager** - Dispatch jobs to containers
4. **Archive Command** - Move completed specs to specs/completed/
5. **CLI Extensions** - Add container-mode flags
6. **Orchestrator Completion** - Run archive step on completion

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-config-types.md | Container config types and status constants | None | `go build ./internal/daemon/...` |
| 2 | 02-log-streamer.md | Log streamer with JSON event parsing | #1 | `go test ./internal/daemon/... -run TestLogStreamer` |
| 3 | 03-container-job.md | Container job dispatch and lifecycle | #1, #2 | `go test ./internal/daemon/... -run TestContainerJob` |
| 4 | 04-archive.md | Archive command implementation | None | `go test ./internal/cli/... -run TestArchive` |
| 5 | 05-cli-extensions.md | Container mode CLI flags | #1 | `go build ./cmd/choo/...` |
| 6 | 06-orchestrator-complete.md | Orchestrator complete() with archive step | #4 | `go test ./internal/orchestrator/... -run TestComplete` |

## Dependency Graph

```
Task 1 (config-types) ─┬─► Task 2 (log-streamer) ─► Task 3 (container-job)
                       │
                       └─► Task 5 (cli-extensions)

Task 4 (archive) ─────────► Task 6 (orchestrator-complete)
```

## External Dependencies

These units must be implemented before CONTAINER-DAEMON:

- **CONTAINER-MANAGER** provides:
  - `container.Manager` interface
  - `container.ContainerConfig` struct
  - `container.ContainerID` type
  - `container.Create()`, `container.Start()`, `container.Wait()`, `container.Logs()` methods

- **JSON-EVENTS** provides:
  - `events.JSONEvent` wire format type
  - `events.ParseJSONEvent()` function
  - Event type constants and payload structures

## Baseline Checks

```bash
go fmt ./internal/daemon/... && go vet ./internal/daemon/...
go fmt ./internal/cli/... && go vet ./internal/cli/...
go fmt ./internal/orchestrator/... && go vet ./internal/orchestrator/...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `choo daemon start --container-mode` starts daemon with container support
- [ ] Container job creates container, streams logs, bridges events
- [ ] `choo archive` moves completed specs to `specs/completed/`
- [ ] PR created and merged

## Reference

- Design spec: `/specs/CONTAINER-DAEMON.md`
- Container manager: `/specs/CONTAINER-MANAGER.md`
- JSON events: `/specs/JSON-EVENTS.md`
