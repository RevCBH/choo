---
unit: escalation
depends_on: []
---

# ESCALATION Implementation Plan

## Overview

The escalation system provides a pluggable notification interface for alerting users when Claude cannot complete operations. This unit implements multiple notification backends (terminal, Slack, webhook) with a consistent interface and a factory function for configuration-driven instantiation.

The implementation is decomposed into six atomic tasks, building from the core interface through each backend to the factory function.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-interface.md | Define Escalator interface, Severity type, and Escalation struct | None | `go build ./internal/escalate/...` |
| 2 | 02-terminal.md | Implement Terminal escalator (stderr with emoji indicators) | #1 | `go test ./internal/escalate/... -run Terminal` |
| 3 | 03-slack.md | Implement Slack escalator (webhook with Block Kit) | #1 | `go test ./internal/escalate/... -run Slack` |
| 4 | 04-webhook.md | Implement generic Webhook escalator (JSON POST) | #1 | `go test ./internal/escalate/... -run Webhook` |
| 5 | 05-multi.md | Implement Multi escalator (concurrent fan-out) | #1 | `go test ./internal/escalate/... -run Multi` |
| 6 | 06-factory.md | Implement FromConfig factory function | #1, #2, #3, #4, #5 | `go test ./internal/escalate/... -run FromConfig` |

## Baseline Checks

```bash
go fmt ./... && go vet ./...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] `go test ./internal/escalate/...` passes with all tests
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/ESCALATION.md`
