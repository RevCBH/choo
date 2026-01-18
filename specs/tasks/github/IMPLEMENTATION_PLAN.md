---
unit: github
depends_on: []
---

# GITHUB Implementation Plan

## Overview

The GITHUB package manages GitHub PR lifecycle for the ralph orchestrator: PR creation (delegated to Claude via `gh` CLI), review polling via emoji reactions (Codex convention), and squash merge operations. This decomposition follows the module structure defined in the design spec.

## Decomposition Strategy

1. **Types first** - Define all core types (PRClient, ReviewState, PRInfo, etc.)
2. **Client foundation** - Authentication, HTTP client, rate limiting
3. **Review polling** - Emoji state machine, polling loop
4. **PR operations** - Create, update, merge, close
5. **Comments** - Fetch review comments for feedback loop

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-pr-types.md | Core types: PRClient, PRInfo, ReviewState, MergeResult, PRComment | None | `go build ./internal/github/...` |
| 2 | 02-client.md | PRClient constructor, auth (getToken), owner/repo detection | #1 | `go test ./internal/github/... -run TestGetToken` |
| 3 | 03-review-polling.md | Emoji state machine, GetReviewStatus, PollReview, WaitForApproval | #1, #2 | `go test ./internal/github/... -run TestReview` |
| 4 | 04-pr-operations.md | CreatePR, GetPR, UpdatePR, Merge, ClosePR | #1, #2 | `go test ./internal/github/... -run TestPR` |
| 5 | 05-comments.md | GetPRComments, GetUnaddressedComments | #1, #2 | `go test ./internal/github/... -run TestComments` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/github/... && go vet ./internal/github/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/GITHUB.md`
