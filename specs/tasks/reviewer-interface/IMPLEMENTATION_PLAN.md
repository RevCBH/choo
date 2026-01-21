---
unit: reviewer-interface
depends_on: []
---

# REVIEWER-INTERFACE Implementation Plan

## Overview

The REVIEWER-INTERFACE unit defines the contract for advisory code review providers in Columbus. This is a foundational unit that other review-related units depend on. It introduces the Reviewer interface (separate from Provider, which handles task execution), along with ReviewResult and ReviewIssue types for structured feedback.

The design is intentionally simple: a single file containing the interface, result types, severity constants, and helper methods. This unit has no external dependencies and serves as the foundation for concrete reviewer implementations (Codex, Claude).

## Decomposition Strategy

This is a small, single-concern unit. All types belong in one file (`internal/provider/reviewer.go`) and are tightly coupled:

1. **Reviewer interface** - The core contract reviewers must satisfy
2. **ReviewResult** - Structured output containing issues and metadata
3. **ReviewIssue** - Individual finding with file, line, severity, message
4. **Severity constants** - The four severity levels (error, warning, suggestion, info)
5. **Helper methods** - HasErrors(), ErrorCount(), WarningCount(), IssuesByFile()

Since all components are interdependent and fit in a single file (~100 lines), this is implemented as a single task.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-reviewer-types.md | Define Reviewer interface, result types, severity constants, and helper methods | None | `go test ./internal/provider/... -run TestReview` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/provider/... && go vet ./internal/provider/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `go test ./internal/provider/...` passes all tests
- [ ] PR created and merged

## Reference

- Design spec: `specs/REVIEWER-INTERFACE.md`
- Related: Provider interface in `internal/provider/provider.go`
