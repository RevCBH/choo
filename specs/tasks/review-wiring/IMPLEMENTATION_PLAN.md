---
unit: review-wiring
depends_on: [review-config, codex-reviewer, claude-reviewer, review-worker]
---

# Review Wiring Implementation Plan

## Overview

The Review Wiring unit is the final integration unit for the advisory code review system. It wires up all review components: resolves the reviewer based on configuration, injects the reviewer into Worker construction, and adds the pre-merge review call after all units complete their merge to the feature branch.

This unit depends on:
- **review-config**: Provides `CodeReviewConfig` and config loading
- **codex-reviewer**: Provides `CodexReviewer` implementation
- **claude-reviewer**: Provides `ClaudeReviewer` implementation
- **review-worker**: Provides `runCodeReview()` and Worker reviewer field

## Decomposition Strategy

The implementation is decomposed into two focused tasks:
1. **Reviewer Resolution** - Implement `resolveReviewer()` in orchestrator and inject reviewer into Worker construction via the provider factory pattern
2. **Pre-merge Review Integration** - Wire `runCodeReview()` call into the Worker's pre-merge flow, replacing the placeholder

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-reviewer-resolution.md | Implement resolveReviewer() and inject into WorkerDeps | None | `go test ./internal/orchestrator/... -run TestResolveReviewer` |
| 2 | 02-pre-merge-integration.md | Wire runCodeReview() into mergeToFeatureBranch | #1 | `go test ./internal/worker/... -run TestRunCodeReview` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/orchestrator/... ./internal/worker/... && go vet ./internal/orchestrator/... ./internal/worker/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `go test ./internal/orchestrator/... ./internal/worker/...` passes all tests
- [ ] PR created and merged

## Reference

- Design spec: `/docs/prd/CODE-REVIEW.md`
- Orchestrator: `/internal/orchestrator/orchestrator.go`
- Worker: `/internal/worker/worker.go`
