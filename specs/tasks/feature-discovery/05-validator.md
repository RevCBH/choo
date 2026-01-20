---
task: 5
status: pending
backpressure: "go test ./internal/feature/... -run TestValidate"
depends_on: [1]
---

# PRD Validator

**Parent spec**: `/specs/FEATURE-DISCOVERY.md`
**Task**: #5 of 7 in implementation plan

## Objective

Implement PRD validation logic including required field checking and PRD ID format validation for git branch compatibility.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `PRD` struct, `ValidationError`, status constants)

### Package Dependencies
- `regexp` - ID format validation

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    ├── validation.go       # CREATE: Validation logic
    └── validation_test.go  # CREATE: Validation tests
```

### Functions to Implement

```go
// ValidatePRD checks that all required fields are present and valid
// Returns nil if valid, ValidationError if invalid
func ValidatePRD(prd *PRD) error

// validatePRDID checks that the ID is valid for use in git branch names
func validatePRDID(id string) error
```

### Implementation Logic

```go
// validPRDID matches lowercase alphanumeric with hyphens
// Must start and end with alphanumeric, 2-50 characters
var validPRDID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func validatePRDID(id string) error {
    if len(id) < 2 {
        return ValidationError{
            Field:   "prd_id",
            Message: "too short (minimum 2 characters)",
        }
    }
    if len(id) > 50 {
        return ValidationError{
            Field:   "prd_id",
            Message: "too long (maximum 50 characters)",
        }
    }
    if !validPRDID.MatchString(id) {
        return ValidationError{
            Field:   "prd_id",
            Message: "must be lowercase alphanumeric with hyphens, no leading/trailing hyphens",
        }
    }
    return nil
}

func ValidatePRD(prd *PRD) error {
    if prd.ID == "" {
        return ValidationError{Field: "prd_id", Message: "required"}
    }
    if err := validatePRDID(prd.ID); err != nil {
        return err
    }
    if prd.Title == "" {
        return ValidationError{Field: "title", Message: "required"}
    }
    if prd.Status == "" {
        return ValidationError{Field: "status", Message: "required"}
    }
    if !IsValidPRDStatus(prd.Status) {
        return ValidationError{
            Field:   "status",
            Message: fmt.Sprintf("invalid value %q, must be one of: draft, approved, in_progress, complete, archived", prd.Status),
        }
    }
    return nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestValidate -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestValidatePRD_Valid` | Returns nil for valid PRD |
| `TestValidatePRD_MissingID` | Returns ValidationError with Field="prd_id" |
| `TestValidatePRD_MissingTitle` | Returns ValidationError with Field="title" |
| `TestValidatePRD_MissingStatus` | Returns ValidationError with Field="status" |
| `TestValidatePRD_InvalidStatus` | Returns ValidationError for unknown status |
| `TestValidatePRDID_TooShort` | Returns error for single character ID |
| `TestValidatePRDID_TooLong` | Returns error for ID over 50 characters |
| `TestValidatePRDID_InvalidChars` | Returns error for uppercase or special characters |
| `TestValidatePRDID_LeadingHyphen` | Returns error for `-test-feature` |
| `TestValidatePRDID_TrailingHyphen` | Returns error for `test-feature-` |
| `TestValidatePRDID_Valid` | Returns nil for `test-feature-01` |

### Test Fixtures

```go
var validPRD = &PRD{
    ID:     "test-feature",
    Title:  "Test Feature",
    Status: PRDStatusDraft,
}

var testCases = []struct {
    name    string
    prd     *PRD
    wantErr string
}{
    {
        name:    "missing prd_id",
        prd:     &PRD{Title: "Test", Status: "draft"},
        wantErr: "prd_id",
    },
    {
        name:    "missing title",
        prd:     &PRD{ID: "test", Status: "draft"},
        wantErr: "title",
    },
    {
        name:    "missing status",
        prd:     &PRD{ID: "test", Title: "Test"},
        wantErr: "status",
    },
    {
        name:    "invalid status",
        prd:     &PRD{ID: "test", Title: "Test", Status: "unknown"},
        wantErr: "status",
    },
    {
        name:    "invalid prd_id format",
        prd:     &PRD{ID: "Test Feature!", Title: "Test", Status: "draft"},
        wantErr: "prd_id",
    },
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- PRD IDs are used in git branch names (`feature/<prd_id>`), so format is strict
- Regex requires 2+ chars: start with alnum, middle can have hyphens, end with alnum
- Single character IDs are rejected (regex requires start AND end alnum)
- Validation returns first error found (not all errors)
- Use `ValidationError` type for consistent error formatting

## NOT In Scope

- Parsing PRD content (Task #4)
- Cross-PRD validation (dependency checking)
- Feature status validation (only PRD status validated here)
