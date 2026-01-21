---
unit: provider-integration
depends_on:
  - provider-interface
  - provider-implementations
  - provider-config
---

# PROVIDER-INTEGRATION Implementation Plan

## Overview

The provider-integration unit modifies the worker and orchestrator packages to use the Provider abstraction instead of hardcoded Claude CLI invocation. This enables configurable provider selection per-unit based on a precedence chain (force flag > frontmatter > CLI flag > env var > config > default).

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-worker-types.md | Update Worker and WorkerDeps with Provider field | None | `go build ./internal/worker/...` |
| 2 | 02-invoke-provider.md | Rename invokeClaudeForTask to invokeProvider | #1 | `go build ./internal/worker/...` |
| 3 | 03-pool-factory.md | Add provider factory to Pool | #1, #2 | `go test ./internal/worker/... -run Pool` |
| 4 | 04-orchestrator.md | Add resolveProviderForUnit to Orchestrator | #3 | `go test ./internal/orchestrator/... -run Provider` |

## Dependency Graph

```
01-worker-types ──┬──► 02-invoke-provider ──┬──► 03-pool-factory ──► 04-orchestrator
                  └────────────────────────┘
```

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/worker/... ./internal/orchestrator/... && go vet ./internal/worker/... ./internal/orchestrator/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/PROVIDER-INTEGRATION.md`
- Provider interface: `/specs/PROVIDER.md`
