---
unit: feature-discovery
depends_on: []
---

# Feature Discovery Implementation Plan

## Overview

The Feature Discovery module provides PRD parsing and filesystem discovery for the PRD-driven workflow system. It handles parsing YAML frontmatter from PRD files stored in `docs/prds/`, discovering available PRDs, computing content hashes for drift detection, and defines feature/PRD lifecycle event types.

## Decomposition Strategy

The implementation is decomposed by concern:
1. **Types** - PRD struct, status constants, ValidationError
2. **Events** - Feature and PRD lifecycle event type constants
3. **Frontmatter** - YAML extraction between `---` delimiters
4. **Parser** - PRD parsing, body extraction, hash computation
5. **Validator** - Required field validation, ID format checking
6. **Discovery** - Filesystem walking, status filtering
7. **Repository** - Caching layer with drift detection

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-types.md | Define PRD struct and status constants | None | `go build ./internal/feature/...` |
| 2 | 02-events.md | Add feature and PRD event type constants | #1 | `go build ./internal/events/...` |
| 3 | 03-frontmatter.md | YAML frontmatter extraction logic | #1 | `go test ./internal/feature/... -run TestParseFrontmatter` |
| 4 | 04-parser.md | PRD parsing with body hash computation | #1, #3 | `go test ./internal/feature/... -run TestParsePRD` |
| 5 | 05-validator.md | PRD field validation and ID format checking | #1 | `go test ./internal/feature/... -run TestValidate` |
| 6 | 06-discovery.md | Filesystem discovery with status filtering | #1, #4, #5 | `go test ./internal/feature/... -run TestDiscover` |
| 7 | 07-repository.md | Caching repository with drift detection | #1, #4, #6 | `go test ./internal/feature/... -run TestRepository` |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
go fmt ./internal/feature/... && go vet ./internal/feature/...
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] `go test ./internal/feature/...` passes all tests
- [ ] PR created and merged

## Reference

- Design spec: `/specs/FEATURE-DISCOVERY.md`
