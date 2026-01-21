# DAEMON Implementation Plan

```yaml
unit: daemon
spec: ../DAEMON.md
depends_on: [history-store]
```

## Overview

Implement the long-running daemon process that owns history writes, serves the web UI, and coordinates with CLI processes via HTTP.

## Tasks

| # | Task | Description | Depends On |
|---|------|-------------|------------|
| 1 | [PID File](./01-pidfile.md) | Implement PID file management for single-instance | - |
| 2 | [Daemon Core](./02-daemon.md) | Implement Daemon struct and lifecycle | #1 |
| 3 | [Client](./03-client.md) | Implement Client for CLI-to-daemon communication | #2 |
| 4 | [HTTP Routes](./04-routes.md) | Implement HTTP endpoints for events and runs | #2 |
| 5 | [SSE Broadcast](./05-sse.md) | Implement Server-Sent Events for real-time updates | #4 |
| 6 | [CLI Integration](./06-cli.md) | Integrate daemon start/connect into CLI commands | #3, #5 |

## Baseline Checks

After all tasks complete:
```bash
go build ./...
go test ./internal/daemon/... -v
go vet ./internal/daemon/...
```
