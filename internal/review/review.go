package review

import (
	"context"
	"fmt"
	"time"

	"github.com/RevCBH/choo/internal/events"
)

// Publisher abstracts event publishing for testing
type Publisher interface {
	Emit(e events.Event)
}

// Reviewer orchestrates the spec review loop
type Reviewer struct {
	config    ReviewConfig
	publisher Publisher
	taskTool  TaskInvoker
}

// NewReviewer creates a new Reviewer with the given configuration
func NewReviewer(config ReviewConfig, publisher Publisher, taskTool TaskInvoker) *Reviewer {
	return &Reviewer{
		config:    config,
		publisher: publisher,
		taskTool:  taskTool,
	}
}

// RunReviewLoop executes the full review loop for a feature
func (r *Reviewer) RunReviewLoop(ctx context.Context, feature, prdPath, specsPath string) (*ReviewSession, error) {
	session := &ReviewSession{
		Feature:      feature,
		PRDPath:      prdPath,
		SpecsPath:    specsPath,
		Config:       r.config,
		Iterations:   []IterationHistory{},
		FinalVerdict: "",
		BlockReason:  "",
	}

	// Emit SpecReviewStarted event
	r.publisher.Emit(events.Event{
		Type: SpecReviewStarted,
		Payload: ReviewStartedPayload{
			Feature:   feature,
			PRDPath:   prdPath,
			SpecsPath: specsPath,
		},
	})

	// Review loop
	for iteration := 1; iteration <= r.config.MaxIterations; iteration++ {
		// Call reviewWithRetry to get result
		result, err := r.reviewWithRetry(ctx, prdPath, specsPath)
		if err != nil {
			// Malformed output after retries - set blocked state
			reason := fmt.Sprintf("malformed output after %d retries: %v", r.config.RetryOnMalformed, err)
			session.FinalVerdict = "blocked"
			session.BlockReason = reason
			r.publishBlocked(session, reason)
			return session, nil
		}

		// Record iteration in session
		session.Iterations = append(session.Iterations, IterationHistory{
			Iteration: iteration,
			Result:    result,
			Timestamp: time.Now(),
		})

		// Emit SpecReviewIteration event
		r.publisher.Emit(events.Event{
			Type: SpecReviewIteration,
			Payload: map[string]interface{}{
				"feature":   feature,
				"iteration": iteration,
				"verdict":   result.Verdict,
				"scores":    result.Score,
			},
		})

		// Check verdict
		if result.Verdict == "pass" {
			// Set final verdict and emit pass event
			session.FinalVerdict = "pass"
			r.publisher.Emit(events.Event{
				Type: SpecReviewPassed,
				Payload: map[string]interface{}{
					"feature":    feature,
					"iterations": iteration,
					"scores":     result.Score,
				},
			})
			return session, nil
		}

		// Verdict is needs_revision
		if iteration < r.config.MaxIterations {
			// Emit feedback event
			r.publisher.Emit(events.Event{
				Type: SpecReviewFeedback,
				Payload: ReviewFeedbackPayload{
					Feature:   feature,
					Iteration: iteration,
					Feedback:  result.Feedback,
					Scores:    result.Score,
				},
			})

			// Apply feedback
			if err := r.applyFeedback(ctx, specsPath, result.Feedback); err != nil {
				// Feedback application failed - set blocked state
				reason := fmt.Sprintf("failed to apply feedback: %v", err)
				session.FinalVerdict = "blocked"
				session.BlockReason = reason
				r.publishBlocked(session, reason)
				return session, nil
			}
		}
	}

	// Max iterations exhausted - set blocked state
	reason := fmt.Sprintf("max iterations (%d) exhausted without passing review", r.config.MaxIterations)
	session.FinalVerdict = "blocked"
	session.BlockReason = reason
	r.publishBlocked(session, reason)
	return session, nil
}

// ReviewSpecs performs a single review invocation
func (r *Reviewer) ReviewSpecs(ctx context.Context, prdPath, specsPath string) (*ReviewResult, error) {
	// Invoke the reviewer to get raw output
	output, err := r.invokeReviewer(ctx, prdPath, specsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke reviewer: %w", err)
	}

	// Parse and validate the output
	result, err := ParseAndValidate(output)
	if err != nil {
		// Return error but preserve raw output for debugging
		return &ReviewResult{RawOutput: output}, err
	}

	return result, nil
}

// reviewWithRetry attempts review with configured retries on malformed output
func (r *Reviewer) reviewWithRetry(ctx context.Context, prdPath, specsPath string) (*ReviewResult, error) {
	var lastErr error

	for attempt := 0; attempt <= r.config.RetryOnMalformed; attempt++ {
		// Brief delay between retries (not on first attempt)
		if attempt > 0 {
			time.Sleep(1 * time.Second)
		}

		result, err := r.ReviewSpecs(ctx, prdPath, specsPath)
		if err == nil {
			// Success
			return result, nil
		}

		// Store error for potential return
		lastErr = err

		// Emit malformed event if we have raw output
		if result != nil && result.RawOutput != "" {
			r.publisher.Emit(events.Event{
				Type: SpecReviewMalformed,
				Payload: ReviewMalformedPayload{
					Feature:     "", // Feature name not available at this level
					RawOutput:   result.RawOutput,
					ParseError:  err.Error(),
					RetryNumber: attempt,
				},
			})
		}
	}

	// All retries exhausted
	return nil, lastErr
}

// invokeReviewer calls the Task tool with the review prompt
func (r *Reviewer) invokeReviewer(ctx context.Context, prdPath, specsPath string) (string, error) {
	prompt := fmt.Sprintf(`Review the generated specs for quality and completeness.

PRD: %s
Specs: %s

Review criteria:
1. COMPLETENESS: All PRD requirements have corresponding spec sections
2. CONSISTENCY: Types, interfaces, and naming are consistent throughout
3. TESTABILITY: Backpressure commands are specific and executable
4. ARCHITECTURE: Follows existing patterns in codebase

Output format (MUST be valid JSON):
{
  "verdict": "pass" | "needs_revision",
  "score": { "completeness": 0-100, "consistency": 0-100, "testability": 0-100, "architecture": 0-100 },
  "feedback": [
    { "section": "...", "issue": "...", "suggestion": "..." }
  ]
}`, prdPath, specsPath)

	return r.taskTool.InvokeTask(ctx, prompt, "general-purpose")
}

// applyFeedback applies feedback using FeedbackApplier
func (r *Reviewer) applyFeedback(ctx context.Context, specsPath string, feedback []ReviewFeedback) error {
	applier := NewFeedbackApplier(r.taskTool)
	return applier.ApplyFeedback(ctx, specsPath, feedback)
}

// publishBlocked emits a SpecReviewBlocked event with recovery info
func (r *Reviewer) publishBlocked(session *ReviewSession, reason string) {
	r.publisher.Emit(events.Event{
		Type: SpecReviewBlocked,
		Payload: ReviewBlockedPayload{
			Feature:      session.Feature,
			Reason:       reason,
			Iterations:   session.Iterations,
			Recovery:     []string{fmt.Sprintf("choo feature resume %s", session.Feature)},
			CurrentSpecs: session.SpecsPath,
		},
	})
}
