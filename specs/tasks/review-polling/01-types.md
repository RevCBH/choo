---
task: 1
status: pending
backpressure: "go build ./internal/github/..."
depends_on: []
---

# Add ReviewPollerConfig Type

**Parent spec**: `/specs/REVIEW-POLLING.md`
**Task**: #1 of 5 in implementation plan

## Objective

Add the ReviewPollerConfig type to centralize polling configuration and add RequireCI field for CI integration.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None

## Deliverables

### Files to Modify
```
internal/github/
└── review.go    # MODIFY: Add ReviewPollerConfig type
```

### Types to Implement

Add this type to `internal/github/review.go` after the existing type definitions:

```go
// ReviewPollerConfig holds configuration for the review poller
type ReviewPollerConfig struct {
	PollInterval  time.Duration // Time between polls (default 30s)
	ReviewTimeout time.Duration // Max time to wait for approval (default 2h)
	RequireCI     bool          // Whether to require CI pass before merge
}

// DefaultReviewPollerConfig returns the default polling configuration
func DefaultReviewPollerConfig() ReviewPollerConfig {
	return ReviewPollerConfig{
		PollInterval:  30 * time.Second,
		ReviewTimeout: 2 * time.Hour,
		RequireCI:     false,
	}
}
```

## Implementation Notes

The existing PRClientConfig already has PollInterval and ReviewTimeout fields. ReviewPollerConfig provides a focused configuration type specifically for the polling behavior, with the addition of RequireCI for future CI integration.

This type will be used by the polling loop to configure its behavior. The defaults match the existing PRClient defaults.

## Backpressure

### Validation Command
```bash
go build ./internal/github/...
```

## NOT In Scope
- Modifying PRClient to use ReviewPollerConfig (uses existing fields)
- CI integration logic (RequireCI is a placeholder for future work)
- Changes to existing GetReviewStatus or PollReview functions
