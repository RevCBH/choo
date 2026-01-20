---
unit: spec-review
depends_on: [feature-discovery]
---

# SPEC-REVIEW Implementation Plan

## Overview

The Spec Review system provides automated quality assurance for generated specifications through a Claude-based reviewer agent. It validates specs against completeness, consistency, testability, and architecture criteria before proceeding to implementation.

This unit implements the review loop orchestration, schema validation for reviewer output, feedback application, and event emission for state transitions. The system handles malformed outputs gracefully, applies feedback iteratively, and transitions to blocked states when human intervention is required.

## Decomposition Strategy

The SPEC-REVIEW spec decomposes into six tasks following a types-first pattern:

1. **Core Types** - ReviewResult, ReviewFeedback, ReviewSession, config types
2. **Schema Validation** - JSON parsing, extraction, and validation against schema
3. **Criteria Definitions** - Review criteria with descriptions and minimum scores
4. **Events** - Event types and payload structs for review workflow
5. **Feedback Application** - Logic to apply reviewer feedback via Task tool
6. **Review Loop** - Main orchestration with retry, iteration tracking, and blocking

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types.md | Core review types and config structs | None | `go build ./internal/review/...` |
| 2 | 02-schema.md | JSON extraction and schema validation | #1 | `go test ./internal/review/... -run Schema` |
| 3 | 03-criteria.md | Review criteria definitions and defaults | #1 | `go test ./internal/review/... -run Criteria` |
| 4 | 04-events.md | Event types and payload structs | #1 | `go build ./internal/review/...` |
| 5 | 05-feedback.md | Feedback application via Task tool | #1, #4 | `go test ./internal/review/... -run Feedback` |
| 6 | 06-loop.md | Review loop orchestration | #1, #2, #3, #4, #5 | `go test ./internal/review/... -run ReviewLoop` |

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

- Design spec: `/specs/SPEC-REVIEW.md`
