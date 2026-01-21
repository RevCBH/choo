---
unit: daemon-client
depends_on:
  - daemon-grpc
---

# DAEMON-CLIENT Implementation Plan

## Overview

The daemon-client unit provides a Go wrapper around the gRPC interface for CLI tools to communicate with the Charlotte daemon. It handles connection management over Unix sockets, translates between Go types and protobuf messages, and provides streaming support for real-time job event watching.

The implementation is decomposed by concern:
1. Type definitions (client-facing types decoupled from protobuf)
2. Protobuf conversion utilities
3. Connection lifecycle (connect/close)
4. Job management methods (start, stop, list, status)
5. Event streaming with callback pattern
6. Health and shutdown operations

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-client-types.md | Define Client struct and client-side types | None | `go build ./internal/client/...` |
| 2 | 02-protobuf-conversion.md | Implement protobuf to/from client type converters | #1 | `go test ./internal/client/... -run TestProto` |
| 3 | 03-connection-management.md | Implement New() and Close() for connection lifecycle | #1 | `go build ./internal/client/...` |
| 4 | 04-job-lifecycle.md | Implement StartJob, StopJob, ListJobs, GetJobStatus | #2, #3 | `go test ./internal/client/... -run TestJob` |
| 5 | 05-event-streaming.md | Implement WatchJob with callback-based event handler | #2, #3 | `go test ./internal/client/... -run TestWatch` |
| 6 | 06-health-shutdown.md | Implement Health and Shutdown methods | #3 | `go test ./internal/client/... -run TestHealth` |

## Baseline Checks

```bash
go fmt ./internal/client/... && go vet ./internal/client/...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and approved

## Reference

- Design spec: `specs/DAEMON-CLIENT.md`
