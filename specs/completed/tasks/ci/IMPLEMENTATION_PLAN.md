---
unit: ci
depends_on: []
---

# CI Implementation Plan

## Overview

The CI component provides automated testing via GitHub Actions and a status check API for programmatic verification. This implementation is decomposed into three tasks:

1. **Workflow Configuration** - The GitHub Actions workflow file itself
2. **Check Types** - Data structures for representing check run status
3. **Check Status API** - Methods for querying and waiting on check status

The workflow file is independent infrastructure. The Go code follows a types-first approach where types are defined before the methods that use them.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-workflow.md | Create GitHub Actions CI workflow | None | File exists and is valid YAML |
| 2 | 02-check-types.md | CheckStatus, CheckRun, CheckRunsResponse types | None | go build ./internal/github/... |
| 3 | 03-check-status.md | GetCheckStatus, getCheckRuns, WaitForChecks methods | #2 | go test ./internal/github/... -run Check |

## Baseline Checks

```bash
go fmt ./... && go vet ./...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] CI workflow triggers on PR
- [ ] `go test -v ./internal/github/... -run Check` passes
- [ ] PR created and merged

## Reference

- Design spec: `/specs/CI.md`
