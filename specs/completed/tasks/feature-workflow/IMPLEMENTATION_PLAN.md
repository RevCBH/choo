---
unit: feature-workflow
depends_on: [feature-discovery, feature-branch, spec-review]
---

# Feature Workflow Implementation Plan

## Overview

The Feature Workflow unit implements the complete feature lifecycle state machine including spec commits, review cycles, auto-triggered completion, and drift detection. This orchestrates spec generation through final PR merge.

## Decomposition Strategy

The implementation is decomposed by module:
1. **States** - State type definitions and transition validation
2. **Commit** - Spec commit operations with push retry
3. **Drift** - PRD drift detection and impact assessment
4. **Completion** - Unit completion checking and feature PR creation
5. **Review Cycle** - Review iteration management and blocking
6. **Workflow** - Main state machine orchestration

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-states.md | Define FeatureStatus type and transition validation | None | `go build ./internal/feature/...` |
| 2 | 02-commit.md | Implement spec commit operations | #1 | `go test ./internal/feature/... -run TestCommit` |
| 3 | 03-drift.md | Implement drift detection and assessment | #1 | `go test ./internal/feature/... -run TestDrift` |
| 4 | 04-completion.md | Implement completion checking and PR creation | #1 | `go test ./internal/feature/... -run TestCompletion` |
| 5 | 05-review-cycle.md | Implement review cycle management | #1 | `go test ./internal/feature/... -run TestReview` |
| 6 | 06-workflow.md | Wire state machine orchestration | #1, #2, #3, #4, #5 | `go test ./internal/feature/... -run TestWorkflow` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/feature/... && go vet ./internal/feature/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `go test ./internal/feature/...` passes all tests
- [ ] PR created and merged

## Reference

- Design spec: `/specs/FEATURE-WORKFLOW.md`
