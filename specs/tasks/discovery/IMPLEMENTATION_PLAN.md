---
unit: discovery
depends_on: []
---

# Discovery Implementation Plan

## Overview

The Discovery package finds, parses, and validates units and tasks from the `specs/tasks/` directory structure. It transforms the file-based specification format into strongly-typed Go data structures that the Scheduler and Worker components consume.

## Decomposition Strategy

The implementation is decomposed by concern:
1. **Types** - Core data structures (Unit, Task, status enums)
2. **Frontmatter** - YAML parsing from markdown files
3. **Discovery** - File/directory walking and glob matching
4. **Validation** - Structure validation and error aggregation

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-core-types.md | Define Unit, Task, and status enum types | None | `go build ./internal/discovery/...` |
| 2 | 02-frontmatter-parsing.md | YAML frontmatter extraction and parsing | #1 | `go test ./internal/discovery/... -run TestParse` |
| 3 | 03-file-discovery.md | Directory walking and glob-based file discovery | #1, #2 | `go test ./internal/discovery/... -run TestDiscover` |
| 4 | 04-validation.md | Validation logic and error aggregation | #1, #2, #3 | `go test ./internal/discovery/... -run TestValidate` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/discovery/... && go vet ./internal/discovery/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `go test ./internal/discovery/...` passes all tests
- [ ] PR created and merged

## Reference

- Design spec: `/Users/bennett/conductor/workspaces/choo/lahore/specs/DISCOVERY.md`
