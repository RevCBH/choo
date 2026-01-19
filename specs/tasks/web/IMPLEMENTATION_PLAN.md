---
unit: web
depends_on: []
---

# WEB Implementation Plan

## Overview

The Web package provides a real-time monitoring interface for the choo orchestrator through HTTP and Server-Sent Events (SSE). It runs as an independent daemon (`choo web`) that receives events from `choo run` via Unix socket, maintains an in-memory state store, and broadcasts updates to connected browsers.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types.md | Define shared types (Event, GraphData, UnitState, etc.) | None | `go build ./internal/web/...` |
| 2 | 02-store.md | In-memory state store with event handling | #1 | `go test ./internal/web/... -run TestStore` |
| 3 | 03-sse.md | SSE hub for browser connection management | #1 | `go test ./internal/web/... -run TestHub` |
| 4 | 04-socket.md | Unix socket listener for orchestrator events | #1, #2, #3 | `go test ./internal/web/... -run TestSocket` |
| 5 | 05-handlers.md | HTTP handlers for API endpoints | #1, #2, #3 | `go test ./internal/web/... -run TestHandler` |
| 6 | 06-server.md | Main server wiring and lifecycle | #1-#5 | `go test ./internal/web/... -run TestServer` |
| 7 | 07-cli.md | CLI command `choo web` | #6 | `go build ./... && choo web --help` |

## Baseline Checks

```bash
go build ./... && go test ./... && go vet ./...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `choo web` starts and serves HTTP on :8080
- [ ] Unix socket accepts connections and processes events
- [ ] SSE broadcasts events to connected browsers
- [ ] State persists after orchestrator disconnects
- [ ] Graceful shutdown cleans up socket file
- [ ] PR created and merged

## Reference

- Design spec: `/specs/WEB.md`
