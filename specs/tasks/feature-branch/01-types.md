---
task: 1
status: pending
backpressure: "go build ./internal/feature/..."
depends_on: []
---

# Feature Types

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/oslo/specs/FEATURE-BRANCH.md`
**Task**: #1 of 4 in implementation plan

## Objective

Define the Feature struct and Status enum for tracking feature lifecycle state.

## Dependencies

### External Specs (must be implemented)
- FEATURE-DISCOVERY - provides `PRD` type from `internal/feature/types.go`

### Task Dependencies (within this unit)
- None

### Package Dependencies
- Standard library only (`time` package)

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── feature.go    # CREATE: Feature struct and status constants
```

### Types to Implement

```go
// Status represents the current state of a feature
type Status string

const (
    StatusPending    Status = "pending"     // Feature created, no work started
    StatusInProgress Status = "in_progress" // Specs being implemented
    StatusComplete   Status = "complete"    // All specs merged to feature branch
    StatusMerged     Status = "merged"      // Feature branch merged to main
)

// Feature represents a feature being developed from a PRD
type Feature struct {
    PRD       *PRD      // Reference to the source PRD
    Branch    string    // Feature branch name (e.g., "feature/streaming-events")
    Status    Status    // Current lifecycle state
    StartedAt time.Time // When feature work began
}
```

### Functions to Implement

```go
// NewFeature creates a Feature from a PRD
// Sets Branch to "feature/<prd.ID>", Status to StatusPending, StartedAt to time.Now()
func NewFeature(p *PRD) *Feature

// GetBranch returns the feature branch name
func (f *Feature) GetBranch() string

// SetStatus updates the feature status
func (f *Feature) SetStatus(status Status)

// IsComplete returns true if all specs have been merged to the feature branch
func (f *Feature) IsComplete() bool

// IsMerged returns true if the feature branch has been merged to main
func (f *Feature) IsMerged() bool
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
| `NewFeature(prd)` | Returns Feature with correct Branch format |
| `NewFeature(prd).Status` | Returns `StatusPending` |
| `feature.IsComplete()` | Returns false when Status != StatusComplete |
| `feature.IsMerged()` | Returns false when Status != StatusMerged |

### Test Fixtures

None required - pure type definitions and simple methods.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- The PRD type is defined in `internal/feature/types.go` from the feature-discovery spec
- Branch name is constructed as `feature/<prd.ID>` in NewFeature
- Status constants must match the spec exactly (lowercase with underscores)
- StartedAt should be set to `time.Now()` in NewFeature

## NOT In Scope

- BranchManager operations (Task #2)
- Configuration (Task #3)
- CLI integration (Task #4)
