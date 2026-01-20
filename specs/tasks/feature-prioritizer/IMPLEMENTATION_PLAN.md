---
unit: feature-prioritizer
depends_on: [feature-discovery]
---

# Feature Prioritizer Implementation Plan

## Overview

The Feature Prioritizer analyzes PRDs in `docs/prds/` and recommends which feature to implement next. It uses Claude to evaluate dependencies, refactoring impact, and codebase state. The system extends the PRD loading capabilities from feature-discovery and adds a new `choo next-feature` CLI command.

## Decomposition Strategy

The implementation is decomposed by concern:
1. **Types** - PriorityResult, Recommendation types for prioritization output
2. **PRD Loading** - Load and parse PRD files with frontmatter extraction
3. **Prioritizer Core** - Prioritizer struct, prompt building, agent invocation
4. **Response Parsing** - Parse Claude JSON response with validation
5. **CLI Command** - `choo next-feature` command with flags and output formatting

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types.md | Define PriorityResult and Recommendation types | None | `go build ./internal/feature/...` |
| 2 | 02-prd-loader.md | PRD loading with frontmatter parsing | #1 | `go test ./internal/feature/... -run TestLoadPRD` |
| 3 | 03-prioritizer.md | Prioritizer struct with prompt building | #1, #2 | `go test ./internal/feature/... -run TestPrioritizer` |
| 4 | 04-response-parser.md | Parse and validate Claude JSON response | #1 | `go test ./internal/feature/... -run TestParse` |
| 5 | 05-cli-command.md | next-feature command with output formatting | #1, #3, #4 | `go test ./internal/cli/... -run TestNextFeature` |

## Dependency Graph

```
01-types ─────┬─► 02-prd-loader ─┐
              │                   │
              ├─► 04-response ────┼─► 03-prioritizer ─┐
              │                   │                    │
              └───────────────────┴────────────────────┴─► 05-cli-command
```

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/feature/... ./internal/cli/... && go vet ./internal/feature/... ./internal/cli/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `go test ./internal/feature/... ./internal/cli/...` passes
- [ ] PR created and merged

## Reference

- Design spec: `/specs/FEATURE-PRIORITIZER.md`
- Dependency: `/specs/FEATURE-DISCOVERY.md` (provides PRD type foundation)
