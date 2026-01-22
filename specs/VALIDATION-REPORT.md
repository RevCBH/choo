# Spec Validation Report

Generated: 2026-01-21

## Summary

| Category | Errors | Warnings | Auto-fixed |
|----------|--------|----------|------------|
| Type consistency | 2 | 0 | 2 |
| Interface alignment | 2 | 0 | 2 |
| Dependency validity | 0 | 0 | 0 |
| Import/export balance | 0 | 0 | 0 |
| Naming consistency | 3 | 0 | 3 |

**Total: 7 issues found, 7 auto-fixed**

## Errors (must fix before proceeding)

None remaining after auto-fixes.

## Warnings (should fix)

None.

## Actions Taken (fixes applied)

### 1. Type Consistency: JSONEvent wire format alignment (JSON-EVENTS)

**File**: `/Users/bennett/conductor/workspaces/choo/dakar/specs/JSON-EVENTS.md`

**Problem**: JSON-EVENTS defined `jsonEvent` as a type alias for `Event`, which uses `json:"time"` for the timestamp field. However, the PRD (section 4.3) defines the wire format with `json:"timestamp"`.

**Fix Applied**: Changed `jsonEvent = Event` alias to a proper `JSONEvent` struct matching the PRD wire format:
```go
type JSONEvent struct {
    Type      string                 `json:"type"`
    Timestamp time.Time              `json:"timestamp"`
    Unit      string                 `json:"unit,omitempty"`
    Task      *int                   `json:"task,omitempty"`
    PR        *int                   `json:"pr,omitempty"`
    Payload   map[string]interface{} `json:"payload,omitempty"`
    Error     string                 `json:"error,omitempty"`
}
```

### 2. Type Consistency: JSONEvent type reference (CONTAINER-DAEMON)

**File**: `/Users/bennett/conductor/workspaces/choo/dakar/specs/CONTAINER-DAEMON.md`

**Problem**: CONTAINER-DAEMON defined its own `JSONEvent` struct that duplicated the definition from JSON-EVENTS.

**Fix Applied**: Changed to import from JSON-EVENTS:
```go
type JSONEvent = events.JSONEvent
```

### 3. Interface Alignment: container.Runtime vs container.Manager (CONTAINER-DAEMON)

**File**: `/Users/bennett/conductor/workspaces/choo/dakar/specs/CONTAINER-DAEMON.md`

**Problem**: CONTAINER-DAEMON used `container.Runtime` interface, but CONTAINER-MANAGER defines `container.Manager`.

**Fix Applied**: Changed all references from `runtime container.Runtime` to `manager container.Manager` in:
- `LogStreamer` struct field
- `NewLogStreamer` function parameter

### 4. Interface Alignment: Logs() method signature (CONTAINER-DAEMON)

**File**: `/Users/bennett/conductor/workspaces/choo/dakar/specs/CONTAINER-DAEMON.md`

**Problem**: CONTAINER-DAEMON called `Logs()` with a `LogsOptions` struct that doesn't exist in CONTAINER-MANAGER.

**Fix Applied**: Simplified to match CONTAINER-MANAGER's signature:
```go
// Before
reader, err := s.runtime.Logs(ctx, s.containerID, container.LogsOptions{...})

// After
reader, err := s.manager.Logs(ctx, container.ContainerID(s.containerID))
```

### 5. Naming Consistency: Event type format (CONTAINER-DAEMON)

**File**: `/Users/bennett/conductor/workspaces/choo/dakar/specs/CONTAINER-DAEMON.md`

**Problem**: Used underscore-separated event types (`unit_started`, `task_completed`) instead of PRD's dot-separated format (`unit.started`, `task.completed`).

**Fix Applied**: Changed all event type strings to use dot notation:
- `"unit_started"` -> `"unit.started"`
- `"task_completed"` -> `"task.completed"`

### 6. Naming Consistency: Wire format timestamp field (JSON-EVENTS)

**File**: `/Users/bennett/conductor/workspaces/choo/dakar/specs/JSON-EVENTS.md`

**Problem**: Example JSON in the spec used `"time"` field name instead of PRD's `"timestamp"`.

**Fix Applied**: Updated all JSON examples to use `"timestamp"`:
```json
// Before
{"time":"2024-01-15T10:30:00Z","type":"unit.started",...}

// After
{"type":"unit.started","timestamp":"2024-01-15T10:30:00Z",...}
```

### 7. Naming Consistency: Test case JSON (CONTAINER-DAEMON)

**File**: `/Users/bennett/conductor/workspaces/choo/dakar/specs/CONTAINER-DAEMON.md`

**Problem**: Test case JSON strings used underscore event types.

**Fix Applied**: Updated test JSON to use correct dot notation.

## Remaining Issues

None. All issues have been auto-fixed.

## Dependency Graph Validation

All `depends_on` references were validated:

| Spec | Dependencies | Status |
|------|--------------|--------|
| CONTAINER-MANAGER | (none) | OK |
| JSON-EVENTS | EVENTS | OK - exists at `specs/completed/EVENTS.md` |
| CONTAINER-IMAGE | (none) | OK |
| CONTAINER-DAEMON | DAEMON-CORE, CONTAINER-MANAGER, JSON-EVENTS | OK - all exist |

## Import/Export Balance

| Type | Exported By | Imported By | Status |
|------|-------------|-------------|--------|
| `ContainerID` | CONTAINER-MANAGER | CONTAINER-DAEMON | OK |
| `ContainerConfig` | CONTAINER-MANAGER | CONTAINER-DAEMON | OK |
| `Manager` interface | CONTAINER-MANAGER | CONTAINER-DAEMON | OK |
| `JSONEvent` | JSON-EVENTS | CONTAINER-DAEMON | OK |
| `JSONEmitter` | JSON-EVENTS | (internal use) | OK |
| `JSONLineReader` | JSON-EVENTS | CONTAINER-DAEMON | OK |
| `Event` | EVENTS (completed) | JSON-EVENTS | OK |

## Notes for Implementers

1. **JSONEvent vs Event**: The internal `Event` type (from `internal/events/types.go`) uses `Time` field with `json:"time"` tag. The wire format `JSONEvent` uses `Timestamp` with `json:"timestamp"` tag. The `JSONEmitter` must convert between these formats when serializing, and `JSONLineReader` must convert when parsing.

2. **Container Manager Interface**: CONTAINER-DAEMON should import the `Manager` interface from `internal/container`, not define its own. The `Logs()` method signature in CONTAINER-MANAGER is simpler than what CONTAINER-DAEMON originally specified - it returns a combined stdout/stderr stream with follow behavior built-in.

3. **Event Type Constants**: Both specs should use the `EventType` constants from `internal/events/types.go` (e.g., `events.UnitStarted`) rather than string literals where possible, to prevent typos.
