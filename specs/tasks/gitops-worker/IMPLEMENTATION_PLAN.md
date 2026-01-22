---
# === Author-provided (required) ===
unit: gitops-worker
depends_on: [gitops, gitops-mock]

# === Orchestrator-managed (auto-populated at runtime) ===
# orch_status: pending
# orch_branch: ralph/gitops-worker-xxxxxx
# orch_worktree: .ralph/worktrees/gitops-worker
# orch_pr_number: null
# orch_started_at: null
# orch_completed_at: null
---

# GITOPS-WORKER Implementation Plan

## Overview

GITOPS-WORKER migrates the Worker package from raw `git.Runner` + `worktreePath` pattern to the safe `git.GitOps` interface. This addresses the production bug where tests ran destructive git commands on the actual repository.

The migration follows a phased approach:
- Phase 1: Add GitOps field alongside existing runner (non-breaking)
- Phase 2: Migrate critical methods (cleanupWorktree, commitReviewFixes, hasUncommittedChanges)
- Phase 3: Full migration and runner removal (future unit)

This unit implements Phase 1-2.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-worker-struct.md | Add gitOps field to Worker struct, update WorkerDeps | None | `go build ./internal/worker/...` |
| 2 | 02-constructor.md | Update NewWorker to create GitOps from WorktreeBase | #1 | `go test ./internal/worker/... -run TestNewWorker` |
| 3 | 03-cleanup-worktree.md | Migrate cleanupWorktree to use GitOps | #2 | `go test ./internal/worker/... -run TestCleanupWorktree` |
| 4 | 04-commit-review-fixes.md | Migrate commitReviewFixes and hasUncommittedChanges | #2 | `go test ./internal/worker/... -run TestCommitReviewFixes` |
| 5 | 05-test-migration.md | Update existing tests to use MockGitOps | #3, #4 | `go test ./internal/worker/... -v` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/worker/... && go vet ./internal/worker/...
go test ./internal/worker/... -v
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Worker uses GitOps for cleanup and commit operations
- [ ] Tests use MockGitOps instead of stubbed Runner
- [ ] Baseline checks pass (go fmt, go vet, go test)
- [ ] PR created and merged

## Reference

- Design spec: `/specs/GITOPS-WORKER.md`
- Dependencies: `/specs/GITOPS.md`, `/specs/GITOPS-MOCK.md`
