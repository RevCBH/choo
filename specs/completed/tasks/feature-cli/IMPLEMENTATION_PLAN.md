---
unit: feature-cli
depends_on: [feature-workflow]
---

# FEATURE-CLI Implementation Plan

## Overview

This unit provides the CLI commands for managing feature development workflows. It exposes `choo feature start`, `choo feature status`, and `choo feature resume` commands that orchestrate the end-to-end workflow from PRD to committed specs and tasks.

## Decomposition Strategy

The CLI is decomposed into these concerns:
1. **State types** - FeatureStatus constants and FeatureState struct
2. **PRD store** - PRD file operations and frontmatter management
3. **Parent command** - Feature command group registration
4. **Start command** - Workflow initiation with dry-run support
5. **Status command** - Display status with JSON output option
6. **Resume command** - Resume blocked workflows

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-state-types.md | Feature state constants and types | None | `go build ./internal/feature/...` |
| 2 | 02-prd-store.md | PRD file parsing and frontmatter updates | #1 | `go test ./internal/feature/... -run PRD` |
| 3 | 03-parent-cmd.md | Feature parent command registration | None | `go build ./internal/cli/...` |
| 4 | 04-start-cmd.md | Feature start command implementation | #1, #2, #3 | `go test ./internal/cli/... -run FeatureStart` |
| 5 | 05-status-cmd.md | Feature status command implementation | #1, #2, #3 | `go test ./internal/cli/... -run FeatureStatus` |
| 6 | 06-resume-cmd.md | Feature resume command implementation | #1, #2, #3, #4 | `go test ./internal/cli/... -run FeatureResume` |

## Dependency Graph

```
01-state-types ─────┬─► 02-prd-store ─┬─► 04-start-cmd ─► 06-resume-cmd
                    │                 │
                    │                 ├─► 05-status-cmd
                    │                 │
03-parent-cmd ──────┴─────────────────┘
```

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/feature/... ./internal/cli/... && go vet ./internal/feature/... ./internal/cli/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `go test ./internal/feature/... ./internal/cli/...` passes
- [ ] PR created and merged

## Reference

- Design spec: `/specs/FEATURE-CLI.md`
- Parent workflow spec: `/specs/FEATURE-WORKFLOW.md`
