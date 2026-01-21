# Task 01: Implement History Types

```yaml
task: 01-types
unit: history-store
depends_on: []
backpressure: "go build ./internal/history/..."
```

## Objective

Create the Go type definitions in `internal/history/types.go` implementing the canonical types from `specs/HISTORY-TYPES.md`.

## Requirements

1. Create `internal/history/types.go` with:
   - `RunStatus` type and constants (`running`, `completed`, `failed`, `stopped`)
   - `Run` struct with all fields per HISTORY-TYPES.md
   - `RunConfig` struct for creating runs
   - `RunResult` struct for completing runs
   - `EventRecord` struct for input events
   - `StoredEvent` struct for persisted events
   - `GraphData`, `GraphNode`, `GraphEdge` structs
   - `ListOptions` and `EventListOptions` for pagination
   - `RunList` and `EventList` response types
   - Event type constants (all `Event*` constants)

2. All types must use JSON struct tags matching the spec
3. Use `time.Time` for timestamps, `json.RawMessage` for payload
4. Use pointer types for optional fields (`*time.Time`, `*int`, `*string`)

## Acceptance Criteria

- [ ] All types from HISTORY-TYPES.md are implemented
- [ ] JSON tags match the spec exactly
- [ ] `go build ./internal/history/...` succeeds
- [ ] Types are documented with comments

## Files to Create/Modify

- `internal/history/types.go` (create)
