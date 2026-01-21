---
unit: provider-interface
depends_on: []
---

# PROVIDER-INTERFACE Implementation Plan

## Overview

The provider interface defines the abstraction layer for multi-provider support in choo, enabling different CLI-based LLM tools (Claude CLI, Codex CLI) to execute tasks within the ralph inner loops. It includes the Provider interface, ProviderType constants, Config struct, and a factory function for configuration-driven instantiation.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-interface.md | Define Provider interface, ProviderType constants, and Config struct | None | `go build ./internal/provider/...` |
| 2 | 02-factory.md | Implement FromConfig factory function with tests | #1 | `go test ./internal/provider/... -run FromConfig` |

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

- Design spec: `/specs/PROVIDER-INTERFACE.md`
