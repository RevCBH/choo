---
unit: provider-config
depends_on: []
---

# PROVIDER-CONFIG Implementation Plan

## Overview

The provider-config unit implements multi-provider support for task execution in choo. It defines types, resolution logic, CLI flags, and frontmatter parsing for provider selection. The system enables Claude and Codex as task execution providers with a five-level precedence chain for configuration.

**Decomposition Strategy**: Types-first, then resolution logic, then CLI integration, finally frontmatter extension.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types.md | Add ProviderConfig and ProviderSettings types to config | None | `go build ./internal/config/...` |
| 2 | 02-resolution.md | Implement ResolveProvider function with precedence chain | #1 | `go test ./internal/config/... -run Provider` |
| 3 | 03-cli-flags.md | Add --provider and --force-task-provider CLI flags | #1 | `go build ./cmd/choo/...` |
| 4 | 04-frontmatter.md | Add Provider field to UnitFrontmatter | None | `go test ./internal/discovery/... -run Frontmatter` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/config/... && go vet ./internal/config/...
go fmt ./internal/cli/... && go vet ./internal/cli/...
go fmt ./internal/discovery/... && go vet ./internal/discovery/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/PROVIDER-CONFIG.md`
