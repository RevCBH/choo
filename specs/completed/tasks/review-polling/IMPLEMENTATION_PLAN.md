---
unit: review-polling
depends_on: [github, events, worker]
---

# REVIEW-POLLING Implementation Plan

## Overview

This unit implements PR review monitoring using GitHub reactions as state indicators. The system polls PRs for emoji reactions (eyes for "in review", thumbs-up for "approved") and handles feedback by delegating to Claude Code.

The existing codebase already has:
- Core review types (ReviewStatus, ReviewState, PollResult, Reaction) in `internal/github/review.go`
- Basic polling (GetReviewStatus, PollReview, WaitForApproval) in `internal/github/review.go`
- PR comment fetching in `internal/github/comments.go`
- Event types for PR lifecycle in `internal/events/types.go`

This implementation adds:
- ReviewPollerConfig for configurable polling behavior
- Error resilience in the polling loop (continue on transient errors)
- BuildFeedbackPrompt for constructing Claude prompts from PR comments
- FeedbackHandler for orchestrating feedback response via Claude

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types.md | Add ReviewPollerConfig type | None | go build ./internal/github/... |
| 2 | 02-status-detection.md | Enhance GetReviewStatus with LastActivity tracking | #1 | go test ./internal/github/... -run TestGetReviewStatus |
| 3 | 03-poll-loop.md | Add error resilience to PollReview | #2 | go test ./internal/github/... -run TestPollReview |
| 4 | 04-feedback-prompt.md | BuildFeedbackPrompt function | None | go test ./internal/worker/... -run TestBuildFeedbackPrompt |
| 5 | 05-feedback-handler.md | FeedbackHandler struct and HandleFeedback method | #4 | go test ./internal/worker/... -run TestHandleFeedback |

## Baseline Checks

```bash
go fmt ./... && go vet ./...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] ReviewPollerConfig is integrated into PRClient
- [ ] PollReview continues polling on transient errors
- [ ] BuildFeedbackPrompt includes all comment details
- [ ] FeedbackHandler retries and escalates on failure
- [ ] PR created and merged

## Reference

- Design spec: `/specs/REVIEW-POLLING.md`
- Existing review implementation: `/internal/github/review.go`
- Existing comment implementation: `/internal/github/comments.go`
- Event types: `/internal/events/types.go`
