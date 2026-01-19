---
task: 2
status: pending
backpressure: "go build ./internal/github/..."
depends_on: []
---

# Check Status Types

**Parent spec**: `/specs/CI.md`
**Task**: #2 of 3 in implementation plan

## Objective

Define the data types for representing GitHub Actions check run status: `CheckStatus`, `CheckRun`, and `CheckRunsResponse`.

## Dependencies

### Task Dependencies (within this unit)
- None (types are independent)

### External Dependencies
- `internal/github` package must exist (already present)

## Deliverables

### Files to Create/Modify
```
internal/github/
└── checks.go    # CREATE: Check status types
```

### Content

```go
// internal/github/checks.go
package github

// CheckStatus represents the aggregated status of CI checks for a commit
type CheckStatus string

const (
	// CheckPending indicates one or more checks are still running
	CheckPending CheckStatus = "pending"
	// CheckSuccess indicates all checks completed successfully
	CheckSuccess CheckStatus = "success"
	// CheckFailure indicates one or more checks failed
	CheckFailure CheckStatus = "failure"
)

// CheckRun represents a single GitHub Actions check run
type CheckRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`     // queued, in_progress, completed
	Conclusion string `json:"conclusion"` // success, failure, cancelled, skipped
}

// CheckRunsResponse represents the GitHub API response for check runs
type CheckRunsResponse struct {
	TotalCount int        `json:"total_count"`
	CheckRuns  []CheckRun `json:"check_runs"`
}
```

## Backpressure

### Validation Command
```bash
go build ./internal/github/...
```

### Success Criteria
- Package compiles without errors
- Types `CheckStatus`, `CheckRun`, `CheckRunsResponse` are defined

## NOT In Scope
- API methods that use these types (Task #3)
- Test file for types (no logic to test)
- Any workflow file changes
