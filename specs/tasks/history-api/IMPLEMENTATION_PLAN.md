# HISTORY-API Implementation Plan

```yaml
unit: history-api
spec: ../HISTORY-API.md
depends_on: [daemon]
```

## Overview

Implement the HTTP API endpoints for querying historical run data. These endpoints are served by the daemon and consumed by the web UI.

## Tasks

| # | Task | Description | Depends On |
|---|------|-------------|------------|
| 1 | [Types](./01-types.md) | Implement API request/response types | - |
| 2 | [Handlers](./02-handlers.md) | Implement HTTP handlers for history endpoints | #1 |
| 3 | [Routes](./03-routes.md) | Register history routes in daemon | #2 |

## Baseline Checks

After all tasks complete:
```bash
go build ./...
go test ./internal/web/... -v
go vet ./internal/web/...
```
