---
unit: container-manager
depends_on: []
---

# CONTAINER-MANAGER Implementation Plan

## Overview

The container-manager package provides CLI-based Docker/Podman container lifecycle management for isolated job execution. Rather than using Docker's Go SDK, it shells out to the CLI tools, keeping the implementation simple and portable across Docker and Podman.

**Decomposition Strategy**: Types-first (core types and interface), then runtime detection, finally the full CLIManager implementation with all lifecycle methods.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-core-types.md | ContainerID, ContainerConfig, Manager interface | None | `go build ./internal/container/...` |
| 2 | 02-runtime-detection.md | DetectRuntime() function | None | `go test ./internal/container/... -run TestDetectRuntime` |
| 3 | 03-cli-manager.md | CLIManager with all Manager methods | #1, #2 | `go test ./internal/container/... -run TestCLIManager` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/container/... && go vet ./internal/container/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/CONTAINER-MANAGER.md`
