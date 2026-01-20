---
task: 1
status: pending
backpressure: "go build ./internal/feature/..."
depends_on: []
---

# Prioritization Types

**Parent spec**: `/specs/FEATURE-PRIORITIZER.md`
**Task**: #1 of 5 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Define the core data structures for feature prioritization: PriorityResult, Recommendation, and PrioritizeOptions.

## Dependencies

### External Specs (must be implemented)
- FEATURE-DISCOVERY - provides `PRD` type (we extend, not duplicate)

### Task Dependencies (within this unit)
- None

### Package Dependencies
- Standard library only

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── prioritizer_types.go    # CREATE: Prioritization types
```

### Types to Implement

```go
// PriorityResult holds the analysis result from Claude
type PriorityResult struct {
    Recommendations []Recommendation `json:"recommendations"`
    DependencyGraph string           `json:"dependency_graph"`
    Analysis        string           `json:"analysis,omitempty"`
}

// Recommendation represents a single PRD recommendation
type Recommendation struct {
    PRDID      string   `json:"prd_id"`
    Title      string   `json:"title"`
    Priority   int      `json:"priority"` // 1 = highest
    Reasoning  string   `json:"reasoning"`
    DependsOn  []string `json:"depends_on"`
    EnablesFor []string `json:"enables_for"` // PRDs that depend on this
}

// PrioritizeOptions controls the prioritization behavior
type PrioritizeOptions struct {
    TopN       int  // Return top N recommendations (default: 3)
    ShowReason bool // Include detailed reasoning in output
}

// DefaultPrioritizeOptions returns options with sensible defaults
func DefaultPrioritizeOptions() PrioritizeOptions {
    return PrioritizeOptions{
        TopN:       3,
        ShowReason: false,
    }
}
```

### Functions to Implement

```go
// Validate checks that a PriorityResult is well-formed
func (r *PriorityResult) Validate() error {
    // Must have at least one recommendation
    // Each recommendation must have valid PRDID and Priority
}

// Truncate limits recommendations to the specified count
func (r *PriorityResult) Truncate(n int) {
    // Limit Recommendations slice to first n entries
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
| Build succeeds | No compilation errors |
| `DefaultPrioritizeOptions().TopN` | Returns 3 |
| `DefaultPrioritizeOptions().ShowReason` | Returns false |

### Test Fixtures

None required - pure type definitions and simple functions.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Types use JSON tags for serialization (Claude returns JSON)
- PriorityResult.Analysis is optional (only populated with --explain flag)
- EnablesFor shows downstream dependencies (features that depend on this one)
- Priority is 1-indexed (1 = highest priority, implement first)

## NOT In Scope

- PRD loading (Task #2)
- Prioritizer logic (Task #3)
- Response parsing (Task #4)
- CLI command (Task #5)
