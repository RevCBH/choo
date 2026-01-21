---
unit: daemon-grpc
depends_on:
  - daemon-db
---

# DAEMON-GRPC Implementation Plan

## Overview

DAEMON-GRPC provides the gRPC interface for daemon communication, enabling CLI tools and external clients to manage jobs, stream events, and control the daemon lifecycle. This implementation is decomposed into seven atomic tasks:

1. Protocol buffer definitions and code generation
2. Core server types and constructor
3. Job lifecycle RPCs (StartJob, StopJob, GetJobStatus, ListJobs)
4. Event streaming (WatchJob) with replay support
5. Daemon lifecycle RPCs (Shutdown, Health)
6. Unix domain socket server setup
7. Integration tests and client utilities

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-proto-definitions.md | Define protobuf service and messages, generate Go code | None | `go build ./pkg/api/v1/...` |
| 2 | 02-server-types.md | Define GRPCServer struct, constructor, and helper types | #1 | `go build ./internal/daemon/...` |
| 3 | 03-job-lifecycle-rpcs.md | Implement StartJob, StopJob, GetJobStatus, ListJobs | #1, #2 | `go test ./internal/daemon/... -run TestGRPC_Job` |
| 4 | 04-watch-job-streaming.md | Implement WatchJob with event subscription and replay | #1, #2, #3 | `go test ./internal/daemon/... -run TestGRPC_Watch` |
| 5 | 05-daemon-lifecycle-rpcs.md | Implement Shutdown and Health RPCs | #1, #2 | `go test ./internal/daemon/... -run TestGRPC_Lifecycle` |
| 6 | 06-unix-socket-server.md | Implement Unix domain socket listener and server setup | #1, #2 | `go test ./internal/daemon/... -run TestServer_Socket` |
| 7 | 07-integration-tests.md | End-to-end tests with real gRPC client connections | #1-#6 | `go test ./internal/daemon/... -run TestGRPC_Integration` |

## Baseline Checks

```bash
go fmt ./internal/daemon/... && go vet ./internal/daemon/...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] `go test ./internal/daemon/...` passes with all tests
- [ ] Baseline checks pass
- [ ] Proto files compile without errors
- [ ] PR created and approved

## Reference

- Design spec: `specs/DAEMON-GRPC.md`
- Dependency: `specs/DAEMON-DB.md`
