---
task: 3
status: complete
backpressure: "go test ./internal/review/... -run Criteria"
depends_on: [1]
---

# Review Criteria Definitions

**Parent spec**: `/specs/SPEC-REVIEW.md`
**Task**: #3 of 6 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Define the review criteria with descriptions and minimum acceptable scores for spec quality evaluation.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: ReviewConfig type)

### Package Dependencies
- Standard library only

## Deliverables

### Files to Create/Modify

```
internal/
└── review/
    └── criteria.go    # CREATE: Criteria definitions
```

### Types to Implement

```go
// Criterion defines a review criterion with its evaluation parameters
type Criterion struct {
    Name        string
    Description string
    MinScore    int // Minimum acceptable score (default: 70)
}
```

### Functions to Implement

```go
// DefaultCriteria returns the standard review criteria
func DefaultCriteria() []Criterion {
    return []Criterion{
        {
            Name:        "completeness",
            Description: "All PRD requirements have corresponding spec sections",
            MinScore:    70,
        },
        {
            Name:        "consistency",
            Description: "Types, interfaces, and naming are consistent throughout",
            MinScore:    70,
        },
        {
            Name:        "testability",
            Description: "Backpressure commands are specific and executable",
            MinScore:    70,
        },
        {
            Name:        "architecture",
            Description: "Follows existing patterns in codebase",
            MinScore:    70,
        },
    }
}

// GetCriterion returns a criterion by name, or nil if not found
func GetCriterion(name string) *Criterion

// CriteriaNames returns the list of criterion names
func CriteriaNames() []string

// IsPassing checks if all scores meet minimum thresholds
func IsPassing(scores map[string]int) bool
```

## Backpressure

### Validation Command

```bash
go test ./internal/review/... -run Criteria -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestDefaultCriteria_Count` | Returns exactly 4 criteria |
| `TestDefaultCriteria_Names` | Contains completeness, consistency, testability, architecture |
| `TestDefaultCriteria_MinScores` | All criteria have MinScore of 70 |
| `TestGetCriterion_Found` | Returns correct criterion for "completeness" |
| `TestGetCriterion_NotFound` | Returns nil for unknown criterion |
| `TestCriteriaNames` | Returns slice of 4 criterion names |
| `TestIsPassing_AllPass` | Returns true when all scores >= 70 |
| `TestIsPassing_OneFails` | Returns false when any score < 70 |
| `TestIsPassing_BoundaryScore` | Returns true when score == 70 exactly |

### Test Fixtures

No external fixtures required.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Criteria descriptions match the reviewer prompt in the spec
- MinScore of 70 is the default threshold for all criteria
- IsPassing uses DefaultCriteria() internally to check thresholds
- GetCriterion is case-sensitive for criterion names

## NOT In Scope

- Schema validation (Task #2)
- Event emission (Task #4)
- Feedback application (Task #5)
- Review loop orchestration (Task #6)
- Configurable per-criterion thresholds (future enhancement)
