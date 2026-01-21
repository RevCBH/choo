---
unit: provider-implementations
depends_on:
  - provider-interface
---

# Provider Implementations Plan

## Overview

The provider implementations unit contains concrete implementations of the Provider interface for invoking AI coding assistants as subprocesses. This unit implements ClaudeProvider and CodexProvider, both following the same pattern of spawning a subprocess with tool-specific arguments.

This unit depends on `provider-interface` because it implements the `Provider` interface defined there.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-claude.md | Implement ClaudeProvider with tests | None | `go test ./internal/provider/... -run Claude` |
| 2 | 02-codex.md | Implement CodexProvider with tests | None | `go test ./internal/provider/... -run Codex` |

Note: Tasks 1 and 2 have no internal dependencies and can be implemented in parallel.

## Baseline Checks

```bash
go fmt ./... && go vet ./...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] `go test -v ./internal/provider/... -run Claude` passes
- [ ] `go test -v ./internal/provider/... -run Codex` passes
- [ ] PR created and merged

## Reference

- Design spec: `/specs/PROVIDER-IMPLEMENTATIONS.md`
- Interface spec: `/specs/PROVIDER-INTERFACE.md`
