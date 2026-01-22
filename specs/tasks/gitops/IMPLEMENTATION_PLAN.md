---
# === Author-provided (required) ===
unit: gitops
depends_on: []

# === Orchestrator-managed (auto-populated at runtime) ===
# orch_status: pending
# orch_branch: ralph/gitops-xxxxxx
# orch_worktree: .ralph/worktrees/gitops
# orch_pr_number: null
# orch_started_at: null
# orch_completed_at: null
---

# GITOPS Implementation Plan

## Overview

GitOps provides a safe, validated interface for executing git operations bound to specific repository paths. This unit implements the core interface, constructor with path validation, and all git operation methods.

The decomposition follows the module structure in the spec:
- Error types and option types first (compile-time validation)
- Per-repo lock (concurrency safety)
- Interface and constructor (path validation at construction)
- Read operations (no lock required)
- Write operations (require lock, branch guard, destructive checks)

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-errors-types.md | Error variables and option types | None | `go build ./internal/git/...` |
| 2 | 02-result-types.md | StatusResult, Commit, and option structs | #1 | `go build ./internal/git/...` |
| 3 | 03-lock.md | Per-repo write lock implementation | None | `go test ./internal/git/... -run TestRepoLock` |
| 4 | 04-interface-constructor.md | GitOps interface and NewGitOps with validation | #1, #2 | `go test ./internal/git/... -run TestNewGitOps` |
| 5 | 05-read-operations.md | Status, RevParse, Diff, Log, CurrentBranch, BranchExists | #4 | `go test ./internal/git/... -run TestGitOps_Read` |
| 6 | 06-write-operations.md | Add, AddAll, Reset, Commit, CheckoutBranch | #3, #4 | `go test ./internal/git/... -run TestGitOps_Write` |
| 7 | 07-destructive-remote-merge.md | CheckoutFiles, Clean, ResetHard, Fetch, Push, Merge, MergeAbort | #3, #6 | `go test ./internal/git/... -run TestGitOps_Destructive` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/git/... && go vet ./internal/git/...
go test ./internal/git/... -v
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass (go fmt, go vet, go test)
- [ ] PR created and merged

## Reference

- Design spec: `/specs/GITOPS.md`
- PRD: `/docs/prd/safe-git-operations.md`
