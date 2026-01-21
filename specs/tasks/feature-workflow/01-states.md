---
task: 1
status: complete
backpressure: "go build ./internal/feature/..."
depends_on: []
---

# Feature Status States

**Parent spec**: `/specs/FEATURE-WORKFLOW.md`
**Task**: #1 of 6 in implementation plan

## Objective

Define the FeatureStatus type, status constants, and state transition validation logic.

## Dependencies

### External Specs (must be implemented)
- None (foundational types)

### Task Dependencies (within this unit)
- None

### Package Dependencies
- Standard library only

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── states.go    # CREATE: State definitions and validation
```

### Types to Implement

```go
// FeatureStatus represents the current state of a feature workflow
type FeatureStatus string

const (
    StatusPending         FeatureStatus = "pending"
    StatusGeneratingSpecs FeatureStatus = "generating_specs"
    StatusReviewingSpecs  FeatureStatus = "reviewing_specs"
    StatusUpdatingSpecs   FeatureStatus = "updating_specs"
    StatusReviewBlocked   FeatureStatus = "review_blocked"
    StatusValidatingSpecs FeatureStatus = "validating_specs"
    StatusGeneratingTasks FeatureStatus = "generating_tasks"
    StatusSpecsCommitted  FeatureStatus = "specs_committed"
    StatusInProgress      FeatureStatus = "in_progress"
    StatusUnitsComplete   FeatureStatus = "units_complete"
    StatusPROpen          FeatureStatus = "pr_open"
    StatusComplete        FeatureStatus = "complete"
    StatusFailed          FeatureStatus = "failed"
)

// ValidTransitions defines allowed state transitions
var ValidTransitions = map[FeatureStatus][]FeatureStatus{
    StatusPending:         {StatusGeneratingSpecs, StatusFailed},
    StatusGeneratingSpecs: {StatusReviewingSpecs, StatusFailed},
    StatusReviewingSpecs:  {StatusUpdatingSpecs, StatusReviewBlocked, StatusValidatingSpecs, StatusFailed},
    StatusUpdatingSpecs:   {StatusReviewingSpecs, StatusFailed},
    StatusReviewBlocked:   {StatusReviewingSpecs, StatusFailed},
    StatusValidatingSpecs: {StatusGeneratingTasks, StatusFailed},
    StatusGeneratingTasks: {StatusSpecsCommitted, StatusFailed},
    StatusSpecsCommitted:  {StatusInProgress, StatusFailed},
    StatusInProgress:      {StatusUnitsComplete, StatusFailed},
    StatusUnitsComplete:   {StatusPROpen, StatusFailed},
    StatusPROpen:          {StatusComplete, StatusFailed},
    StatusComplete:        {},
    StatusFailed:          {},
}
```

### Functions to Implement

```go
// CanTransition checks if a state transition is valid
func CanTransition(from, to FeatureStatus) bool

// ParseFeatureStatus converts a string to FeatureStatus with validation
func ParseFeatureStatus(s string) (FeatureStatus, error)

// IsTerminal returns true if the status is a terminal state
func (s FeatureStatus) IsTerminal() bool

// CanResume returns true if the status supports resume
func (s FeatureStatus) CanResume() bool
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
| `CanTransition(StatusPending, StatusGeneratingSpecs)` | Returns `true` |
| `CanTransition(StatusPending, StatusComplete)` | Returns `false` |
| `CanTransition(StatusReviewBlocked, StatusReviewingSpecs)` | Returns `true` |
| `CanTransition(StatusComplete, StatusPending)` | Returns `false` |
| `ParseFeatureStatus("pending")` | Returns `StatusPending, nil` |
| `ParseFeatureStatus("invalid")` | Returns error |
| `StatusComplete.IsTerminal()` | Returns `true` |
| `StatusFailed.IsTerminal()` | Returns `true` |
| `StatusInProgress.IsTerminal()` | Returns `false` |
| `StatusReviewBlocked.CanResume()` | Returns `true` |
| `StatusInProgress.CanResume()` | Returns `false` |

### Test Fixtures

None required - pure type definitions and validation functions.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Empty string should parse to `StatusPending` (default state)
- Status parsing should be case-sensitive
- Terminal states have no valid outgoing transitions
- Only `StatusReviewBlocked` supports resume (returns to reviewing)

## NOT In Scope

- Commit operations (Task #2)
- Drift detection (Task #3)
- Completion logic (Task #4)
- Review cycle management (Task #5)
- Workflow orchestration (Task #6)
