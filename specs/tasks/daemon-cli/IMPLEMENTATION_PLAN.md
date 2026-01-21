---
unit: daemon-cli
depends_on:
  - daemon-client
  - daemon-core
---

# DAEMON-CLI Implementation Plan

## Overview

DAEMON-CLI provides the command-line interface for interacting with the choo daemon. This unit adds daemon management commands (start, stop, status), job control commands (jobs, watch, stop), and extends the existing run command with daemon mode support.

The decomposition follows a layered approach:
1. Display helpers provide formatting primitives
2. Daemon commands enable process lifecycle management
3. Job commands provide job monitoring and control
4. Run command integration enables daemon-mode execution

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-display-helpers.md | Event and job display formatting utilities | None | `go test ./internal/cli/... -run TestDisplay` |
| 2 | 02-daemon-commands.md | Daemon start, stop, status commands | #1 | `go test ./internal/cli/... -run TestDaemon` |
| 3 | 03-jobs-command.md | Job listing command with status filter | #1 | `go test ./internal/cli/... -run TestJobs` |
| 4 | 04-watch-command.md | Event stream attachment command | #1 | `go test ./internal/cli/... -run TestWatch` |
| 5 | 05-stop-command.md | Job stop command with force flag | None | `go test ./internal/cli/... -run TestStop` |
| 6 | 06-run-daemon-mode.md | Extend run command with daemon mode | #1, #4 | `go test ./internal/cli/... -run TestRunDaemon` |

## Baseline Checks

```bash
go fmt ./internal/cli/... && go vet ./internal/cli/...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and approved

## Reference

- Design spec: `specs/DAEMON-CLI.md`
- Client spec: `specs/DAEMON-CLIENT.md`
- Core spec: `specs/DAEMON-CORE.md`
