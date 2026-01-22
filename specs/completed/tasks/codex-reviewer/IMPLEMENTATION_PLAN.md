---
unit: codex-reviewer
depends_on: [reviewer-interface]
---

# CODEX-REVIEWER Implementation Plan

## Overview

The CodexReviewer implements the Reviewer interface using the OpenAI Codex CLI. It invokes `codex review --base <baseBranch>` and parses the output into structured ReviewResult objects. The implementation handles exit codes carefully: exit code 0 means passed, exit code 1 means issues found (not an error), and exit code 2+ indicates execution errors.

This is decomposed into two tasks:

1. **Reviewer Implementation** - The CodexReviewer struct implementing the Reviewer interface with CLI invocation and exit code handling
2. **Output Parser** - Regex-based parsing of codex output into ReviewIssue structs with file, line, severity, and message extraction

The parser is separated because it has distinct test fixtures and the parsing logic is complex enough to warrant isolated testing.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-reviewer-impl.md | CodexReviewer struct with CLI invocation and exit code handling | None | go build ./internal/provider/... |
| 2 | 02-output-parser.md | Regex parsing of codex output into ReviewIssue structs | #1 | go test ./internal/provider/... -run TestCodexReviewer |

## Baseline Checks

```bash
go fmt ./... && go vet ./...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] CodexReviewer implements Reviewer interface
- [ ] Exit code 0 returns Passed=true
- [ ] Exit code 1 returns Passed=false with parsed issues
- [ ] Exit code 2+ returns error
- [ ] All four severity levels (error, warning, suggestion, info) are parsed correctly
- [ ] PR created and merged

## Reference

- Design spec: `/specs/CODEX-REVIEWER.md`
- Interface spec: `/specs/REVIEWER-INTERFACE.md`
