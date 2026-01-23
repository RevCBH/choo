---
# === Author-provided (required) ===
unit: gitops-mock
depends_on: [gitops]

# === Orchestrator-managed (auto-populated at runtime) ===
# orch_status: pending
# orch_branch: ralph/gitops-mock-xxxxxx
# orch_worktree: .ralph/worktrees/gitops-mock
# orch_pr_number: null
# orch_started_at: null
# orch_completed_at: null
---

# GITOPS-MOCK Implementation Plan

## Overview

GITOPS-MOCK provides a testable mock implementation of the GitOps interface. It enables tests to verify git operation behavior without executing actual git commands, with configurable stub responses, call tracking, and assertion helpers.

The decomposition follows these concerns:
- Mock structure with stub fields and call tracking
- Safety feature simulation (AllowDestructive, BranchGuard, AuditLogger)
- Assertion helpers for common test patterns
- Tests to verify the mock itself works correctly

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-mock-structure.md | MockGitOps struct, constructors, stub fields | None | `go build ./internal/git/...` |
| 2 | 02-method-implementations.md | All interface method implementations with call recording | #1 | `go test ./internal/git/... -run TestMockGitOps_Methods` |
| 3 | 03-safety-simulation.md | AllowDestructive checks, BranchGuard simulation, audit capture | #2 | `go test ./internal/git/... -run TestMockGitOps_Safety` |
| 4 | 04-assertions.md | AssertCalled, AssertNotCalled, AssertCallOrder, etc. | #2 | `go test ./internal/git/... -run TestMockGitOps_Assert` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/git/... && go vet ./internal/git/...
go test ./internal/git/... -v
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] MockGitOps implements GitOps interface
- [ ] Safety simulation matches real GitOps behavior
- [ ] Baseline checks pass (go fmt, go vet, go test)
- [ ] PR created and merged

## Reference

- Design spec: `/specs/GITOPS-MOCK.md`
- Parent interface: `/specs/GITOPS.md`
