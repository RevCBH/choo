---
task: 3
status: pending
backpressure: "go test ./internal/feature/... -run TestParseFrontmatter"
depends_on: [1]
---

# Frontmatter Parser

**Parent spec**: `/specs/FEATURE-DISCOVERY.md`
**Task**: #3 of 7 in implementation plan

## Objective

Implement YAML frontmatter extraction from markdown content, splitting content into frontmatter bytes and body bytes.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: PRD type for context)

### Package Dependencies
- Standard library only (`bytes`)

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    ├── frontmatter.go       # CREATE: Frontmatter extraction logic
    └── frontmatter_test.go  # CREATE: Tests for frontmatter extraction
```

### Functions to Implement

```go
// parseFrontmatter splits content into frontmatter YAML and body markdown
// Content must start with "---\n" followed by YAML, then "\n---\n" delimiter
// Returns the frontmatter bytes (without delimiters) and body bytes
func parseFrontmatter(content []byte) (frontmatter []byte, body []byte, err error)
```

### Implementation Logic

```go
func parseFrontmatter(content []byte) (frontmatter []byte, body []byte, err error) {
    // Content must start with "---\n"
    if !bytes.HasPrefix(content, []byte("---\n")) {
        return nil, nil, fmt.Errorf("missing frontmatter delimiter")
    }

    // Find closing delimiter
    rest := content[4:] // Skip opening "---\n"
    idx := bytes.Index(rest, []byte("\n---\n"))
    if idx == -1 {
        // Try "---" at end of file (no trailing newline)
        idx = bytes.Index(rest, []byte("\n---"))
        if idx == -1 || idx+4 != len(rest) {
            return nil, nil, fmt.Errorf("missing closing frontmatter delimiter")
        }
        frontmatter = rest[:idx]
        body = nil
        return frontmatter, body, nil
    }

    frontmatter = rest[:idx]
    body = rest[idx+5:] // Skip "\n---\n"

    return frontmatter, body, nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestParseFrontmatter -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestParseFrontmatter_Valid` | Extracts frontmatter between `---` delimiters |
| `TestParseFrontmatter_NoOpeningDelimiter` | Returns error for content without `---\n` prefix |
| `TestParseFrontmatter_NoClosingDelimiter` | Returns error when closing `---` missing |
| `TestParseFrontmatter_EmptyBody` | Returns empty body when content ends at `---` |
| `TestParseFrontmatter_TrailingWhitespace` | Handles body with leading newlines correctly |

### Test Fixtures

Test cases embedded in test file:

```go
// Valid frontmatter with body
var validWithBody = `---
prd_id: test-feature
title: Test Feature
status: draft
---

# Test Feature

This is the body.
`

// Valid frontmatter at end of file
var validNoBody = `---
prd_id: test-feature
title: Test Feature
status: draft
---`

// Missing opening delimiter
var noOpening = `prd_id: test-feature
title: Test Feature
---
# Body`

// Missing closing delimiter
var noClosing = `---
prd_id: test-feature
title: Test Feature
# Body without closing delimiter`
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- The opening `---\n` must be at byte 0 (no leading whitespace)
- Closing delimiter can be `\n---\n` (body follows) or `\n---` at EOF (no body)
- Frontmatter bytes do not include the `---` delimiters
- Body bytes do not include the leading `---\n` after frontmatter
- Empty body is valid (returns nil/empty slice)

## NOT In Scope

- YAML parsing of frontmatter content (Task #4)
- File I/O operations (Task #4)
- Validation of frontmatter fields (Task #5)
