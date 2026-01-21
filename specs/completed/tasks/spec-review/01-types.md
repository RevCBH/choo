---
task: 1
status: complete
backpressure: "go build ./internal/review/..."
depends_on: []
---

# Review Core Types

**Parent spec**: `/specs/SPEC-REVIEW.md`
**Task**: #1 of 6 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Define the core type definitions for the spec review system including ReviewResult, ReviewFeedback, ReviewSession, and configuration structs.

## Dependencies

### External Specs (must be implemented)
- FEATURE-DISCOVERY - provides event types pattern

### Task Dependencies (within this unit)
- None (this is the foundation task)

### Package Dependencies
- Standard library only (`time`)

## Deliverables

### Files to Create/Modify

```
internal/
└── review/
    └── types.go    # CREATE: Core review types
```

### Types to Implement

```go
// ReviewConfig configures the review loop behavior
type ReviewConfig struct {
    MaxIterations    int      // Maximum review iterations before blocking (default: 3)
    Criteria         []string // Review criteria to evaluate
    RetryOnMalformed int      // Retry attempts on malformed output (default: 1)
}

// DefaultReviewConfig returns sensible defaults
func DefaultReviewConfig() ReviewConfig

// ReviewResult represents the outcome of a single review
type ReviewResult struct {
    Verdict   string            `json:"verdict"`   // "pass" or "needs_revision"
    Score     map[string]int    `json:"score"`     // criteria -> score (0-100)
    Feedback  []ReviewFeedback  `json:"feedback"`  // Required when needs_revision
    RawOutput string            `json:"-"`         // For debugging malformed output
}

// ReviewFeedback represents a single piece of actionable feedback
type ReviewFeedback struct {
    Section    string `json:"section"`    // Spec section with issue
    Issue      string `json:"issue"`      // Description of the problem
    Suggestion string `json:"suggestion"` // How to fix it
}

// IterationHistory tracks review attempts for debugging
type IterationHistory struct {
    Iteration int           `json:"iteration"`
    Result    *ReviewResult `json:"result"`
    Timestamp time.Time     `json:"timestamp"`
}

// ReviewSession tracks the full review loop state
type ReviewSession struct {
    Feature      string             `json:"feature"`
    PRDPath      string             `json:"prd_path"`
    SpecsPath    string             `json:"specs_path"`
    Config       ReviewConfig       `json:"config"`
    Iterations   []IterationHistory `json:"iterations"`
    FinalVerdict string             `json:"final_verdict"` // "pass", "blocked", or ""
    BlockReason  string             `json:"block_reason"`  // Set when blocked
}
```

### Functions to Implement

```go
// DefaultReviewConfig returns sensible defaults
func DefaultReviewConfig() ReviewConfig {
    return ReviewConfig{
        MaxIterations:    3,
        Criteria:         []string{"completeness", "consistency", "testability", "architecture"},
        RetryOnMalformed: 1,
    }
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/review/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Compilation | Package compiles without errors |
| `TestDefaultReviewConfig` | Returns config with MaxIterations=3, 4 criteria, RetryOnMalformed=1 |

### Test Fixtures

No external fixtures required.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- ReviewConfig.Criteria should default to the four standard criteria
- All JSON tags should match the spec document exactly
- ReviewResult.RawOutput uses json:"-" to exclude from serialization
- Types should be exported for use by other packages

## NOT In Scope

- Schema validation logic (Task #2)
- Criteria definitions and descriptions (Task #3)
- Event types (Task #4)
- Any business logic beyond struct definitions
