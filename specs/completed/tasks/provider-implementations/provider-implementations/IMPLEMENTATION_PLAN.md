---
unit: provider-implementations
depends_on:
  - provider-interface
---

# Implementation Plan: provider-implementations

## Overview

This unit implements the Claude and Codex providers that satisfy the Provider interface defined in PROVIDER-INTERFACE. Both providers invoke CLI tools as subprocesses with appropriate flags for automated operation.

## Design Spec Reference

[PROVIDER-IMPLEMENTATIONS](/specs/PROVIDER-IMPLEMENTATIONS.md)

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-claude.md | Implement ClaudeProvider with Invoke and Name methods | provider-interface | `go build ./internal/provider/...` |
| 2 | 02-codex.md | Implement CodexProvider with Invoke and Name methods | provider-interface, #1 | `go test ./internal/provider/... -run Provider` |

## Dependency Graph

```
provider-interface
       │
       ▼
┌──────────────┐
│  01-claude   │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  02-codex    │
└──────────────┘
```

## Baseline Checks

Before starting implementation:

```bash
# Verify provider interface exists
go build ./internal/provider/...

# Verify interface is defined
grep -q "type Provider interface" internal/provider/provider.go
```

## Completion Criteria

- [ ] ClaudeProvider implements Provider interface
- [ ] CodexProvider implements Provider interface
- [ ] Both providers handle context cancellation
- [ ] Both providers wrap errors with provider name
- [ ] Unit tests pass for both providers
- [ ] `go test ./internal/provider/...` passes
