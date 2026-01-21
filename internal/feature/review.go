package feature

import (
	"context"
	"errors"
	"fmt"
)

// Spec represents a specification to be reviewed (placeholder type)
type Spec struct {
	Path    string
	Content string
}

// ReviewResult represents the outcome of a single review
type ReviewResult struct {
	Verdict  string                   `json:"verdict"`
	Score    map[string]int           `json:"score"`
	Feedback []ReviewFeedback         `json:"feedback"`
}

// ReviewFeedback represents a single piece of actionable feedback
type ReviewFeedback struct {
	Section    string `json:"section"`
	Issue      string `json:"issue"`
	Suggestion string `json:"suggestion"`
}

// Reviewer is the interface for spec review operations
type Reviewer interface {
	Review(ctx context.Context, specs []Spec) (*ReviewResult, error)
}

// ReviewCycle manages the review iteration loop
type ReviewCycle struct {
	reviewer      Reviewer
	maxIterations int
	transitionFn  func(FeatureStatus) error
	escalateFn    func(string, error) error
}

// ReviewCycleConfig configures the review cycle
type ReviewCycleConfig struct {
	MaxIterations int // default 3
}

// ResumeOptions configures how to resume from blocked state
type ResumeOptions struct {
	SkipReview bool   // Skip directly to validation
	Message    string // User-provided context for resume
}

// MalformedReviewError indicates that the review output was malformed
type MalformedReviewError struct {
	msg string
}

func (e *MalformedReviewError) Error() string {
	return fmt.Sprintf("malformed review output: %s", e.msg)
}

// NewReviewCycle creates a review cycle manager
func NewReviewCycle(reviewer Reviewer, cfg ReviewCycleConfig) *ReviewCycle {
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 3
	}

	return &ReviewCycle{
		reviewer:      reviewer,
		maxIterations: cfg.MaxIterations,
		transitionFn:  func(FeatureStatus) error { return nil },
		escalateFn:    func(string, error) error { return nil },
	}
}

// Run executes the review cycle until pass, blocked, or error
func (rc *ReviewCycle) Run(ctx context.Context, specs []Spec) error {
	for iteration := 0; iteration < rc.maxIterations; iteration++ {
		result, err := rc.reviewer.Review(ctx, specs)
		if err != nil {
			if isMalformedOutput(err) {
				_ = rc.transitionFn(StatusReviewBlocked)
				return rc.escalateFn("Review produced malformed output", err)
			}
			return err
		}

		switch result.Verdict {
		case "pass":
			return rc.transitionFn(StatusValidatingSpecs)
		case "needs_revision":
			// Transition to updating specs
			if err := rc.transitionFn(StatusUpdatingSpecs); err != nil {
				return err
			}
			// After updating, transition back to reviewing
			if err := rc.transitionFn(StatusReviewingSpecs); err != nil {
				return err
			}
		default:
			// Unknown verdict treated as error
			return fmt.Errorf("unknown review verdict: %s", result.Verdict)
		}
	}

	// Max iterations reached
	_ = rc.transitionFn(StatusReviewBlocked)
	return rc.escalateFn("Max review iterations reached", nil)
}

// Resume continues from blocked state
func (rc *ReviewCycle) Resume(ctx context.Context, opts ResumeOptions) error {
	if opts.SkipReview {
		// Skip directly to validation
		return rc.transitionFn(StatusValidatingSpecs)
	}

	// Transition back to reviewing and run one iteration
	if err := rc.transitionFn(StatusReviewingSpecs); err != nil {
		return err
	}

	result, err := rc.reviewer.Review(ctx, []Spec{})
	if err != nil {
		if isMalformedOutput(err) {
			_ = rc.transitionFn(StatusReviewBlocked)
			return rc.escalateFn("Review produced malformed output", err)
		}
		return err
	}

	switch result.Verdict {
	case "pass":
		return rc.transitionFn(StatusValidatingSpecs)
	case "needs_revision":
		// Transition to updating specs
		if err := rc.transitionFn(StatusUpdatingSpecs); err != nil {
			return err
		}
		// After updating, transition back to reviewing
		if err := rc.transitionFn(StatusReviewingSpecs); err != nil {
			return err
		}
		// Continue with run cycle
		return rc.Run(ctx, []Spec{})
	default:
		return fmt.Errorf("unknown review verdict: %s", result.Verdict)
	}
}

// isMalformedOutput checks if error indicates malformed review output
func isMalformedOutput(err error) bool {
	var malformedErr *MalformedReviewError
	return errors.As(err, &malformedErr)
}
