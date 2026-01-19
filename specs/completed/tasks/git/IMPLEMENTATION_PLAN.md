---
unit: git
depends_on: [claude]
---

# GIT Implementation Plan

## Overview

The GIT package provides git operations for the Ralph orchestrator, enabling parallel unit execution through isolated worktrees and safe merging of concurrent PRs. This unit decomposes into 6 tasks covering: git command execution utilities, worktree management, branch naming with Claude, commit operations, and merge serialization with conflict resolution.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-exec.md | Git command execution utilities | None | `go test ./internal/git/...` |
| 2 | 02-worktree.md | Worktree creation/removal/setup | #1 | `go test ./internal/git/...` |
| 3 | 03-branch.md | Branch naming via Claude haiku | #1 | `go test ./internal/git/...` |
| 4 | 04-commit.md | Staging and commit operations | #1 | `go test ./internal/git/...` |
| 5 | 05-merge.md | Mutex-based merge serialization | #1, #3 | `go test ./internal/git/...` |
| 6 | 06-conflict.md | Conflict resolution with Claude | #5 | `go test ./internal/git/...` |

## Dependency Graph

```
01-exec ─┬─► 02-worktree
         ├─► 03-branch ──┬─► 05-merge ──► 06-conflict
         └─► 04-commit   │
                         │
```

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/git/... && go vet ./internal/git/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/GIT.md`
