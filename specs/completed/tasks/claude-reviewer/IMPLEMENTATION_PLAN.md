---
unit: claude-reviewer
depends_on: [reviewer-interface]
---

# CLAUDE-REVIEWER Implementation Plan

## Overview

The ClaudeReviewer implements the Reviewer interface for automated code review using Claude CLI. It retrieves the git diff between HEAD and a base branch, constructs a review prompt requesting structured JSON output, invokes Claude with non-interactive flags, and parses the response into actionable review issues.

The implementation is decomposed into three atomic tasks: the core reviewer implementation, the prompt builder, and the JSON extraction/parsing logic.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-reviewer-impl.md | ClaudeReviewer struct, constructor, Name(), Review(), getDiff() | None | `go test ./internal/provider/... -run TestClaudeReviewer` |
| 2 | 02-prompt-builder.md | BuildClaudeReviewPrompt function | None | `go test ./internal/provider/... -run TestBuildClaudeReviewPrompt` |
| 3 | 03-json-extraction.md | JSON extraction and parseOutput (extractJSON, code fence handling, brace matching) | #1, #2 | `go test ./internal/provider/... -run TestExtract` |

## Baseline Checks

```bash
go fmt ./... && go vet ./...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] `go test ./internal/provider/...` passes with all tests
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/CLAUDE-REVIEWER.md`
- Dependency: `/specs/REVIEWER-INTERFACE.md`
