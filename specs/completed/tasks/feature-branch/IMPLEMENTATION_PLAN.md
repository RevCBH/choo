---
unit: feature-branch
depends_on: [feature-discovery]
---

# Feature Branch Implementation Plan

## Overview

The Feature Branch module provides feature branch creation, lifecycle management, and orchestrator integration. It builds on the PRD types from feature-discovery to manage dedicated branches for PRD-driven development workflows.

## Decomposition Strategy

The implementation is decomposed by concern:
1. **Types** - Feature struct and status constants
2. **Branch Manager** - Core branch operations (create, exists, checkout, delete)
3. **Config Integration** - FeatureConfig with branch prefix setting
4. **CLI Integration** - --feature flag support in run command

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types.md | Feature struct and status constants | None | `go build ./internal/feature/...` |
| 2 | 02-branch-manager.md | BranchManager with CRUD operations | #1 | `go test ./internal/feature/... -run TestBranchManager` |
| 3 | 03-config.md | FeatureConfig and defaults | #1 | `go build ./internal/config/...` |
| 4 | 04-cli-integration.md | --feature flag in run command | #2, #3 | `go build ./cmd/oslo/...` |

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

- Design spec: `/Users/bennett/conductor/workspaces/choo/oslo/specs/FEATURE-BRANCH.md`
