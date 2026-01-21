---
unit: daemon-db
depends_on: []
---

# DAEMON-DB Implementation Plan

## Overview

DAEMON-DB provides the SQLite-based persistent storage layer for the daemon architecture. It stores workflow runs, work units, and an event log for debugging and replay. The implementation uses `modernc.org/sqlite` (pure Go, no CGO) with WAL mode for concurrent access.

Decomposition strategy: Types first, then connection/migration infrastructure, followed by entity-specific CRUD operations (runs, units, events), and finally integration tests.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types.md | Status constants and record types | None | `go build ./internal/daemon/db/...` |
| 2 | 02-connection.md | DB connection, WAL mode, schema migrations | #1 | `go test ./internal/daemon/db/... -run TestOpen` |
| 3 | 03-runs.md | Run CRUD operations | #2 | `go test ./internal/daemon/db/... -run TestRun` |
| 4 | 04-units.md | Unit CRUD operations | #3 | `go test ./internal/daemon/db/... -run TestUnit` |
| 5 | 05-events.md | Event logging and query operations | #3 | `go test ./internal/daemon/db/... -run TestEvent` |
| 6 | 06-integration.md | Integration tests for cascade delete and resumability | #4, #5 | `go test ./internal/daemon/db/... -v` |

## Baseline Checks

```bash
go fmt ./internal/daemon/db/... && go vet ./internal/daemon/db/...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and approved

## Reference

- Design spec: `specs/DAEMON-DB.md`
