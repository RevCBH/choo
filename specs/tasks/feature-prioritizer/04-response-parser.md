---
task: 4
status: pending
backpressure: "go test ./internal/feature/... -run TestParse"
depends_on: [1]
---

# Response Parser

**Parent spec**: `/specs/FEATURE-PRIORITIZER.md`
**Task**: #4 of 5 in implementation plan

## Objective

Implement robust parsing and validation of Claude's JSON response, handling both raw JSON and markdown-wrapped responses.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `PriorityResult`, `Recommendation`)

### Package Dependencies
- `encoding/json` - JSON parsing
- `strings` - String manipulation for JSON extraction

## Deliverables

### Files to Create/Modify

```
internal/feature/
├── response.go       # CREATE
└── response_test.go  # CREATE
```

### Functions to Implement

```go
// ParsePriorityResponse parses Claude's response into a PriorityResult
// Handles both raw JSON and markdown-wrapped JSON (```json ... ```)
func ParsePriorityResponse(response string) (*PriorityResult, error)

// extractJSON extracts JSON content from a response string
// Handles: raw JSON, ```json ... ``` wrapper, ``` ... ``` wrapper
func extractJSON(response string) (string, error)

// validateRecommendations checks that all recommendations are valid
func validateRecommendations(recs []Recommendation) error
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestParse -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestParsePriorityResponse_RawJSON` | Parses raw JSON response |
| `TestParsePriorityResponse_MarkdownWrapped` | Extracts JSON from ```json block |
| `TestParsePriorityResponse_PlainCodeBlock` | Extracts JSON from ``` block |
| `TestParsePriorityResponse_WithPreamble` | Handles text before JSON |
| `TestParsePriorityResponse_EmptyRecommendations` | Returns error |
| `TestParsePriorityResponse_MissingPRDID` | Returns error with context |
| `TestParsePriorityResponse_InvalidPriority` | Returns error for priority <= 0 |
| `TestExtractJSON_NoJSON` | Returns error when no JSON found |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### JSON Extraction Algorithm

1. Check if response starts with `{` (raw JSON)
2. Look for ` ```json ` marker and extract until closing ` ``` `
3. Look for plain ` ``` ` marker
4. Find first `{` and match closing `}` by counting depth

### Validation Rules

1. Non-empty recommendations: at least one recommendation required
2. Valid PRDID: each recommendation must have non-empty `prd_id`
3. Valid Priority: each recommendation must have `priority > 0`

### Error Messages

Include context for debugging:
- Position of error when possible
- The offending value
- What the correct format should be

```go
func validateRecommendations(recs []Recommendation) error {
    if len(recs) == 0 {
        return fmt.Errorf("no recommendations in response")
    }
    for i, rec := range recs {
        if rec.PRDID == "" {
            return fmt.Errorf("recommendation %d missing prd_id", i)
        }
        if rec.Priority <= 0 {
            return fmt.Errorf("recommendation %d (%s) has invalid priority: %d", i, rec.PRDID, rec.Priority)
        }
    }
    return nil
}
```

## NOT In Scope

- Prioritizer orchestration (Task #3)
- CLI output formatting (Task #5)
- Retry logic for malformed responses
