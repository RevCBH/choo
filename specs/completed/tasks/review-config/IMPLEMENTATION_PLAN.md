---
unit: review-config
depends_on: []
---

# REVIEW-CONFIG Implementation Plan

## Overview

The review-config unit adds configuration types and loading logic for the advisory code review system. It extends the existing config package with a `CodeReviewConfig` struct that controls whether code review runs, which provider to use, iteration limits, and verbosity settings.

**Decomposition Strategy**: Types-first with defaults, then validation and integration into the main Config struct. This is a lightweight config extension (no CLI commands).

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types-defaults.md | CodeReviewConfig struct, ReviewProviderType, DefaultCodeReviewConfig(), IsReviewOnlyMode() | None | `go build ./internal/config/...` |
| 2 | 02-validation-integration.md | Validate() method, integration into Config struct and LoadConfig | #1 | `go test ./internal/config/... -run TestCodeReview` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/config/... && go vet ./internal/config/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/REVIEW-CONFIG.md`
