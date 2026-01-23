---
unit: json-events
depends_on: []
---

# JSON-EVENTS Implementation Plan

## Overview

The JSON-EVENTS module enables structured event communication between `choo run` processes and the daemon when running inside containers. When `choo run` executes non-interactively (no TTY attached), it emits events as JSON lines to stdout. The daemon reads container stdout, parses these JSON lines, and bridges the events to the web UI via the existing event bus.

## Decomposition Strategy

The JSON-EVENTS spec naturally decomposes into four tasks:

1. **Wire Format Types** - JSONEvent struct for serialization with proper JSON tags
2. **JSON Emitter** - Thread-safe JSON line writer with mutex protection
3. **JSON Line Reader** - Stream reader for JSON lines with large payload handling
4. **TTY Detection and Bus Integration** - IsJSONMode function and EmitRaw method

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-wire-types.md | JSONEvent struct with JSON tags, optional fields | None | `go build ./internal/events/...` |
| 2 | 02-json-emitter.md | JSONEmitter struct, Emit method, JSONEmitterHandler | #1 | `go test ./internal/events/... -run TestJSONEmitter` |
| 3 | 03-json-reader.md | JSONLineReader, ParseJSONEvent, 64KB buffer | #1 | `go test ./internal/events/... -run TestJSONLineReader` |
| 4 | 04-tty-detection.md | IsJSONMode function, EmitRaw method on Bus | #1, #2, #3 | `go test ./internal/events/... -run TestIsJSONMode` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./... && go vet ./...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/JSON-EVENTS.md`
- Events spec: `/specs/completed/EVENTS.md`
