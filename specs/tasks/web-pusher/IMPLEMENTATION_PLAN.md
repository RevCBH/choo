---
unit: web-pusher
depends_on: []
---

# WEB-PUSHER Implementation Plan

## Overview

Implements the SocketPusher that connects `choo run` to a web UI via Unix socket. The pusher subscribes to the event bus and forwards events as JSON over a Unix domain socket, enabling real-time visualization of orchestration progress.

This implementation is decomposed into three atomic tasks:
1. Wire event types and payload structures
2. Core SocketPusher with connection management and event forwarding
3. CLI --web flag integration

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types.md | Define WireEvent, GraphPayload, NodePayload, EdgePayload, PusherConfig | None | `go build ./internal/web/...` |
| 2 | 02-pusher.md | Implement SocketPusher with Start, Close, Connected, SetGraph | #1 | `go test ./internal/web/... -run TestSocketPusher` |
| 3 | 03-cli-flag.md | Add --web flag to run command | #2 | `go build ./cmd/choo/... && go test ./internal/cli/... -run TestRunOptions` |

## Baseline Checks

```bash
go build ./... && go test ./... && go vet ./...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] `go test ./internal/web/...` passes with all tests
- [ ] Baseline checks pass
- [ ] `choo run --web` connects to socket and forwards events
- [ ] PR created and merged

## Reference

- Events package: `/internal/events/types.go`
- Scheduler graph: `/internal/scheduler/graph.go`
- CLI run command: `/internal/cli/run.go`
