---
task: 2
status: pending
backpressure: "go test ./internal/review/... -run Schema"
depends_on: [1]
---

# Schema Validation

**Parent spec**: `/specs/SPEC-REVIEW.md`
**Task**: #2 of 6 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement JSON extraction from reviewer output and schema validation to ensure reviewer responses conform to the required structure.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: ReviewResult, ReviewFeedback types)

### Package Dependencies
- Standard library (`encoding/json`, `strings`, `fmt`)

## Deliverables

### Files to Create/Modify

```
internal/
└── review/
    └── schema.go    # CREATE: Schema validation logic
```

### Types to Implement

```go
// SchemaError represents a validation failure
type SchemaError struct {
    Field   string
    Message string
}

func (e SchemaError) Error() string {
    return fmt.Sprintf("schema validation failed: %s - %s", e.Field, e.Message)
}

// ValidVerdicts defines acceptable verdict values
var ValidVerdicts = []string{"pass", "needs_revision"}

// RequiredScoreCriteria defines the criteria that must have scores
var RequiredScoreCriteria = []string{"completeness", "consistency", "testability", "architecture"}
```

### Functions to Implement

```go
// ParseAndValidate parses JSON output and validates against required schema
func ParseAndValidate(output string) (*ReviewResult, error)

// extractJSON finds and extracts JSON object from surrounding text
func extractJSON(output string) string

// isValidVerdict checks if verdict is in ValidVerdicts
func isValidVerdict(verdict string) bool
```

### Validation Rules

1. **JSON Extraction**: Find first `{` and last `}` to extract JSON object
2. **Verdict Validation**: Must be "pass" or "needs_revision"
3. **Score Object**: Must exist and contain all four criteria
4. **Score Range**: Each score must be 0-100
5. **Feedback Required**: When verdict is "needs_revision", feedback array must have at least one item
6. **Feedback Structure**: Each feedback item must have non-empty section, issue, and suggestion

## Backpressure

### Validation Command

```bash
go test ./internal/review/... -run Schema -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestParseAndValidate_ValidPass` | Parses valid pass verdict with all scores |
| `TestParseAndValidate_ValidNeedsRevision` | Parses needs_revision with feedback array |
| `TestParseAndValidate_InvalidVerdict` | Returns SchemaError for invalid verdict like "maybe" |
| `TestParseAndValidate_MissingScore` | Returns SchemaError when score object missing |
| `TestParseAndValidate_NeedsRevisionWithoutFeedback` | Returns SchemaError when needs_revision has empty feedback |
| `TestParseAndValidate_ExtractsJSONFromText` | Extracts JSON when surrounded by explanatory text |
| `TestParseAndValidate_ScoreOutOfRange` | Returns SchemaError for score > 100 or < 0 |
| `TestParseAndValidate_MissingCriterion` | Returns SchemaError when any criterion score missing |
| `TestExtractJSON_NoJSON` | Returns empty string when no JSON found |
| `TestExtractJSON_ValidJSON` | Extracts JSON object correctly |

### Test Fixtures

No external fixtures required. Use inline JSON strings.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- JSON extraction handles Claude's tendency to include explanatory text around JSON
- SchemaError provides field-specific error messages for debugging
- Validation order: JSON extraction, JSON parsing, verdict, scores, feedback
- Return early on first validation failure

## NOT In Scope

- Criteria definitions (Task #3)
- Event emission (Task #4)
- Feedback application (Task #5)
- Review loop orchestration (Task #6)
