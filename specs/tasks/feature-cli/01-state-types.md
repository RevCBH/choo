---
task: 1
status: complete
backpressure: "go build ./internal/feature/..."
depends_on: []
---

# Feature State Types

**Parent spec**: `/specs/FEATURE-CLI.md`
**Task**: #1 of 6 in implementation plan

## Objective

Define the feature workflow state constants, state struct, and valid transition map.

## Dependencies

### External Specs (must be implemented)
- None for this task

### Task Dependencies (within this unit)
- None (this is the foundation)

### Package Dependencies
- `time` (standard library)
- `gopkg.in/yaml.v3`

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── state.go    # CREATE: State types and transitions
```

### Types to Implement

```go
// FeatureStatus represents the current state of a feature workflow
type FeatureStatus string

const (
    StatusNotStarted      FeatureStatus = "not_started"
    StatusGeneratingSpecs FeatureStatus = "generating_specs"
    StatusReviewingSpecs  FeatureStatus = "reviewing_specs"
    StatusReviewBlocked   FeatureStatus = "review_blocked"
    StatusValidatingSpecs FeatureStatus = "validating_specs"
    StatusGeneratingTasks FeatureStatus = "generating_tasks"
    StatusSpecsCommitted  FeatureStatus = "specs_committed"
)

// FeatureState holds the complete state of a feature workflow
type FeatureState struct {
    PRDID            string        `yaml:"prd_id"`
    Status           FeatureStatus `yaml:"feature_status"`
    Branch           string        `yaml:"branch"`
    StartedAt        time.Time     `yaml:"started_at"`
    ReviewIterations int           `yaml:"review_iterations"`
    MaxReviewIter    int           `yaml:"max_review_iter"`
    LastFeedback     string        `yaml:"last_feedback,omitempty"`
    SpecCount        int           `yaml:"spec_count,omitempty"`
    TaskCount        int           `yaml:"task_count,omitempty"`
}

// ValidTransitions defines allowed state transitions
var ValidTransitions = map[FeatureStatus][]FeatureStatus{
    StatusNotStarted:      {StatusGeneratingSpecs},
    StatusGeneratingSpecs: {StatusReviewingSpecs, StatusReviewBlocked},
    StatusReviewingSpecs:  {StatusValidatingSpecs, StatusReviewBlocked},
    StatusReviewBlocked:   {StatusReviewingSpecs, StatusValidatingSpecs, StatusGeneratingTasks},
    StatusValidatingSpecs: {StatusGeneratingTasks},
    StatusGeneratingTasks: {StatusSpecsCommitted},
}
```

### Functions to Implement

```go
// CanTransition checks if transitioning from one state to another is valid
func CanTransition(from, to FeatureStatus) bool {
    // Look up allowed transitions from current state
    // Return true if 'to' state is in the allowed list
}

// IsTerminal returns true if the status is a terminal state
func (s FeatureStatus) IsTerminal() bool {
    // StatusSpecsCommitted is terminal for the CLI workflow
}

// IsBlocked returns true if the workflow is in a blocked state
func (s FeatureStatus) IsBlocked() bool {
    // Only StatusReviewBlocked is a blocked state
}

// String returns the string representation of the status
func (s FeatureStatus) String() string {
    return string(s)
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/feature/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestFeatureStatus_String` | `StatusNotStarted.String() == "not_started"` |
| `TestCanTransition_Valid` | `CanTransition(StatusNotStarted, StatusGeneratingSpecs) == true` |
| `TestCanTransition_Invalid` | `CanTransition(StatusNotStarted, StatusSpecsCommitted) == false` |
| `TestFeatureStatus_IsTerminal` | `StatusSpecsCommitted.IsTerminal() == true` |
| `TestFeatureStatus_IsBlocked` | `StatusReviewBlocked.IsBlocked() == true` |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use the exact FeatureStatus values from the design spec
- ValidTransitions map should cover all non-terminal states
- Terminal states have empty allowed transition slices
- FeatureState struct uses YAML tags for frontmatter serialization

## NOT In Scope

- PRD file operations (task #2)
- Workflow execution logic (FEATURE-WORKFLOW spec)
- CLI command implementations (tasks #3-6)
