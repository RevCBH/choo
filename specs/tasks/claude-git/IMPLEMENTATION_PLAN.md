---
unit: claude-git
depends_on: [escalation]
---

# CLAUDE-GIT Implementation Plan

## Overview

The claude-git unit delegates git operations (commit, push, PR creation) to Claude Code instead of the orchestrator executing them directly. This enables richer commit messages, contextual PR descriptions, and a consistent model where Claude handles all creative work while the orchestrator coordinates.

The implementation is decomposed into six atomic tasks:

1. **Retry utilities** - Exponential backoff retry for transient failures
2. **Prompt builders** - Git operation prompt construction
3. **Git helpers** - Verification methods (HEAD ref, branch existence, changed files)
4. **Commit delegate** - Worker method to commit via Claude
5. **Push delegate** - Worker method to push via Claude
6. **PR delegate** - Worker method to create PR via Claude with URL extraction

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-retry.md | RetryConfig, RetryResult, RetryWithBackoff | None | `go test ./internal/worker/... -run Retry` |
| 2 | 02-prompts.md | BuildCommitPrompt, BuildPushPrompt, BuildPRPrompt | None | `go test ./internal/worker/... -run Prompt` |
| 3 | 03-git-helpers.md | getHeadRef, hasNewCommit, branchExistsOnRemote, getChangedFiles | None | `go test ./internal/worker/... -run GitHelper` |
| 4 | 04-commit-delegate.md | commitViaClaudeCode worker method | 1, 2, 3 | `go test ./internal/worker/... -run CommitVia` |
| 5 | 05-push-delegate.md | pushViaClaudeCode worker method | 1, 2, 3 | `go test ./internal/worker/... -run PushVia` |
| 6 | 06-pr-delegate.md | createPRViaClaudeCode, extractPRURL | 1, 2 | `go test ./internal/worker/... -run PRVia` |

## Dependency Graph

```
          ┌─────────┐     ┌─────────┐     ┌─────────────┐
          │01-retry │     │02-prompts│     │03-git-helpers│
          └────┬────┘     └────┬────┘     └──────┬──────┘
               │               │                  │
               └───────┬───────┴──────────────────┘
                       │
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
  ┌───────────┐  ┌───────────┐  ┌───────────┐
  │04-commit  │  │05-push    │  │06-pr      │
  │  delegate │  │  delegate │  │  delegate │
  └───────────┘  └───────────┘  └───────────┘
```

## Baseline Checks

```bash
go fmt ./... && go vet ./...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] Unit tests cover success, failure, and retry scenarios
- [ ] Worker can commit, push, and create PRs via Claude delegation

## Files Created/Modified

```
internal/worker/
├── retry.go            # CREATE: Retry utilities
├── retry_test.go       # CREATE: Retry tests
├── prompt_git.go       # CREATE: Git operation prompts
├── prompt_git_test.go  # CREATE: Prompt tests
├── git_delegate.go     # CREATE: Claude delegation for git ops
├── git_delegate_test.go # CREATE: Delegation tests
└── worker.go           # MODIFY: Add escalator field, GitRetry config
```

## External Dependencies

- **ESCALATION spec** - Provides `escalate.Escalator` interface and `escalate.Escalation` type
- **WORKER spec** - Provides Worker struct and WorkerConfig
- **EVENTS spec** - Provides event emission for BranchPushed

## Reference

- Design spec: `/Users/bennett/conductor/workspaces/choo/las-vegas/specs/CLAUDE-GIT.md`
- Escalation spec: `/Users/bennett/conductor/workspaces/choo/las-vegas/specs/ESCALATION.md`
