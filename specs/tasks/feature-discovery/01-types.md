---
task: 1
status: pending
backpressure: "go build ./internal/feature/..."
depends_on: []
---

# Core Types

**Parent spec**: `/specs/FEATURE-DISCOVERY.md`
**Task**: #1 of 7 in implementation plan

## Objective

Define the core data structures for PRD representation including the PRD struct, status constants, and ValidationError type.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None

### Package Dependencies
- Standard library only (`time`, `fmt`)

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── types.go    # CREATE: PRD struct and status constants
```

### Types to Implement

```go
// PRD represents a Product Requirements Document
type PRD struct {
    // Required fields (from frontmatter)
    ID     string `yaml:"prd_id"`
    Title  string `yaml:"title"`
    Status string `yaml:"status"` // draft | approved | in_progress | complete | archived

    // Optional dependency hints
    DependsOn []string `yaml:"depends_on,omitempty"`

    // Complexity estimates
    EstimatedUnits int `yaml:"estimated_units,omitempty"`
    EstimatedTasks int `yaml:"estimated_tasks,omitempty"`

    // Orchestrator-managed fields (updated at runtime)
    FeatureBranch        string     `yaml:"feature_branch,omitempty"`
    FeatureStatus        string     `yaml:"feature_status,omitempty"`
    FeatureStartedAt     *time.Time `yaml:"feature_started_at,omitempty"`
    FeatureCompletedAt   *time.Time `yaml:"feature_completed_at,omitempty"`
    SpecReviewIterations int        `yaml:"spec_review_iterations,omitempty"`
    LastSpecReview       *time.Time `yaml:"last_spec_review,omitempty"`

    // File metadata (not in frontmatter)
    FilePath string `yaml:"-"`
    Body     string `yaml:"-"` // Markdown content after frontmatter
    BodyHash string `yaml:"-"` // SHA-256 for drift detection
}

// PRDStatus values for the status field
const (
    PRDStatusDraft      = "draft"
    PRDStatusApproved   = "approved"
    PRDStatusInProgress = "in_progress"
    PRDStatusComplete   = "complete"
    PRDStatusArchived   = "archived"
)

// FeatureStatus values for orchestrator-managed feature_status field
const (
    FeatureStatusPending         = "pending"
    FeatureStatusGeneratingSpecs = "generating_specs"
    FeatureStatusReviewingSpecs  = "reviewing_specs"
    FeatureStatusReviewBlocked   = "review_blocked"
    FeatureStatusValidatingSpecs = "validating_specs"
    FeatureStatusGeneratingTasks = "generating_tasks"
    FeatureStatusSpecsCommitted  = "specs_committed"
    FeatureStatusInProgress      = "in_progress"
    FeatureStatusUnitsComplete   = "units_complete"
    FeatureStatusPROpen          = "pr_open"
    FeatureStatusComplete        = "complete"
    FeatureStatusFailed          = "failed"
)

// ValidationError represents a frontmatter validation failure
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("invalid PRD: %s: %s", e.Field, e.Message)
}
```

### Functions to Implement

```go
// validPRDStatuses returns the set of valid PRD status values
func validPRDStatuses() []string

// validFeatureStatuses returns the set of valid feature status values
func validFeatureStatuses() []string

// IsValidPRDStatus checks if a status string is a valid PRD status
func IsValidPRDStatus(s string) bool

// IsValidFeatureStatus checks if a status string is a valid feature status
func IsValidFeatureStatus(s string) bool
```

## Backpressure

### Validation Command

```bash
go build ./internal/feature/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| `PRDStatusDraft` | Equals `"draft"` |
| `PRDStatusApproved` | Equals `"approved"` |
| `FeatureStatusPending` | Equals `"pending"` |
| `ValidationError{Field: "x", Message: "y"}.Error()` | Returns `"invalid PRD: x: y"` |
| `IsValidPRDStatus("draft")` | Returns `true` |
| `IsValidPRDStatus("invalid")` | Returns `false` |

### Test Fixtures

None required - pure type definitions and simple validation functions.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `yaml` struct tags for frontmatter fields that will be serialized
- Use `yaml:"-"` for fields that should not appear in frontmatter (FilePath, Body, BodyHash)
- Time fields use pointers (`*time.Time`) to represent optional/nullable timestamps
- Status validation functions are simple slice contains checks

## NOT In Scope

- Event types (Task #2)
- Frontmatter parsing logic (Task #3)
- PRD parsing from files (Task #4)
- Validation logic (Task #5)
