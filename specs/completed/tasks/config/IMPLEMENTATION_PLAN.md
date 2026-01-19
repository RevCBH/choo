---
unit: config
depends_on: []
---

# CONFIG Implementation Plan

## Overview

The config package handles loading, validating, and providing access to Ralph Orchestrator configuration. This unit implements the complete configuration system with support for YAML files, environment variable overrides, and GitHub auto-detection.

**Decomposition Strategy**: Types-first, then layer by concern (defaults, env, validation, github detection), finally the main LoadConfig orchestration.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-core-types.md | Define all config structs and types | None | `go build ./internal/config/...` |
| 2 | 02-defaults.md | Default value constants and DefaultConfig() | #1 | `go test ./internal/config/... -run TestDefault` |
| 3 | 03-env-overrides.md | Environment variable parsing and application | #1 | `go test ./internal/config/... -run TestEnv` |
| 4 | 04-validation.md | Config validation logic and ValidationError | #1 | `go test ./internal/config/... -run TestValidat` |
| 5 | 05-github-detection.md | GitHub owner/repo auto-detection from git remote | #1 | `go test ./internal/config/... -run TestGitHub` |
| 6 | 06-load-config.md | Main LoadConfig() function and YAML parsing | #2, #3, #4, #5 | `go test ./internal/config/... -v` |

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

- Design spec: `/specs/CONFIG.md`
