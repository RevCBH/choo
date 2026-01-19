---
unit: conflict-resolution
depends_on: [claude-git]
---

# CONFLICT-RESOLUTION Implementation Plan

## Overview

The CONFLICT-RESOLUTION module handles merge conflicts that arise when multiple units merge to main in parallel. When a worker attempts to rebase its branch onto the target branch and encounters conflicts, the system delegates resolution to Claude Code rather than attempting automated resolution.

This implementation is decomposed into 4 tasks:
1. Git helper functions for conflict detection and rebase state management
2. Conflict prompt builder for Claude delegation
3. Main merge flow with conflict resolution retry loop
4. Force push and PR merge with event emission

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-git-helpers.md | Git conflict detection helpers (IsRebaseInProgress, AbortRebase, GetConflictedFiles) | None | `go test ./internal/git/... -run "Rebase\|Conflict"` |
| 2 | 02-conflict-prompt.md | Conflict resolution prompt builder | None | `go test ./internal/worker/... -run ConflictPrompt` |
| 3 | 03-merge-flow.md | Retry utilities and mergeWithConflictResolution worker method | 1, 2 | `go test ./internal/worker/... -run "Retry\|MergeConflict"` |
| 4 | 04-force-push-merge.md | forcePushAndMerge, PR merge, event emission | 3 | `go test ./internal/worker/... -run ForcePush` |

## Baseline Checks

```bash
go fmt ./... && go vet ./...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `go test ./internal/git/... -run "Rebase|Conflict"` passes
- [ ] `go test ./internal/worker/... -run "ConflictPrompt|MergeConflict|ForcePush"` passes
- [ ] PR created and merged

## Reference

- Design spec: `/specs/CONFLICT-RESOLUTION.md`
- GIT spec: `/specs/completed/GIT.md`
- EVENTS spec: `/specs/completed/EVENTS.md`
