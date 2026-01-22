---
unit: review-worker
depends_on:
  - reviewer-interface
  - review-config
---

# REVIEW-WORKER Implementation Plan

## Overview

The REVIEW-WORKER unit integrates advisory code review into the Worker. Review runs after unit tasks complete (before merge) and is strictly advisory - it never blocks the merge operation. If issues are found, the system attempts fixes using the configured provider up to MaxFixIterations times. Failed fix attempts trigger worktree cleanup (reset, clean, checkout) to ensure a clean state for merge.

Key design principles:
- **Non-blocking**: Review errors and issues never propagate to block merge
- **Advisory**: All output is informational (stderr) and event-driven
- **Self-healing**: Worktree cleanup after failed fixes ensures merge can proceed
- **Configurable**: Fix iterations controllable via MaxFixIterations (0 = review-only)

## Decomposition Strategy

The implementation is decomposed by responsibility:

1. **Review Events** - Event types for the code review lifecycle (must exist before orchestration emits them)
2. **Review Orchestration** - Main `runCodeReview()` entry point with nil-reviewer handling and flow control
3. **Fix Loop** - `runReviewFixLoop()` and `invokeProviderForFix()` for iterative fix attempts
4. **Commit and Cleanup** - `commitReviewFixes()` and `cleanupWorktree()` for git operations

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-review-events.md | Add code review event types to events package | None | `go build ./internal/events/...` |
| 2 | 02-review-orchestration.md | Implement runCodeReview entry point and flow | #1 | `go test ./internal/worker/... -run TestRunCodeReview` |
| 3 | 03-fix-loop.md | Implement fix loop and provider invocation | #2 | `go test ./internal/worker/... -run TestReviewFixLoop` |
| 4 | 04-commit-cleanup.md | Implement commit and worktree cleanup operations | #3 | `go test ./internal/worker/... -run TestCommitReviewFixes` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/worker/... ./internal/events/... && go vet ./internal/worker/... ./internal/events/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `go test ./internal/worker/... -run TestReview` passes all review-related tests
- [ ] Review integration does not block merge on any error path
- [ ] PR created and merged

## Reference

- Design spec: `specs/REVIEW-WORKER.md`
- Related: `specs/REVIEWER-INTERFACE.md` (provides Reviewer interface and ReviewResult types)
- Related: `specs/REVIEW-CONFIG.md` (provides CodeReviewConfig)
