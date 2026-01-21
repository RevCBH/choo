package worker

import (
	"context"
	"fmt"
	"os"

	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/provider"
)

// runCodeReview performs advisory code review after task completion.
// This function NEVER returns an error that blocks the merge.
// All review failures are logged but do not prevent merge.
func (w *Worker) runCodeReview(ctx context.Context) {
	// 1. Check if reviewer is nil (disabled)
	if w.reviewer == nil {
		return
	}

	// 2. Emit CodeReviewStarted event
	if w.events != nil {
		evt := events.NewEvent(events.CodeReviewStarted, w.unit.ID)
		w.events.Emit(evt)
	}

	// 3. Determine base ref for comparison (local feature branch)
	baseRef := w.getBaseRef()

	// 4. Invoke reviewer
	result, err := w.reviewer.Review(ctx, w.worktreePath, baseRef)
	if err != nil {
		// Log error but don't fail
		if w.reviewConfig != nil && w.reviewConfig.Verbose {
			fmt.Fprintf(os.Stderr, "Code review failed to run: %v\n", err)
		}
		if w.events != nil {
			evt := events.NewEvent(events.CodeReviewFailed, w.unit.ID).
				WithPayload(map[string]any{"error": err.Error()})
			w.events.Emit(evt)
		}
		return // Proceed to merge anyway
	}

	// 5. Check result - no issues means passed
	if result.Passed || len(result.Issues) == 0 {
		if w.reviewConfig != nil && w.reviewConfig.Verbose {
			fmt.Fprintf(os.Stderr, "Code review passed: %s\n", result.Summary)
		}
		if w.events != nil {
			evt := events.NewEvent(events.CodeReviewPassed, w.unit.ID).
				WithPayload(map[string]any{"summary": result.Summary})
			w.events.Emit(evt)
		}
		return
	}

	// 6. Issues found - always log (actionable information)
	fmt.Fprintf(os.Stderr, "Code review found %d issues\n", len(result.Issues))
	if w.events != nil {
		evt := events.NewEvent(events.CodeReviewIssuesFound, w.unit.ID).
			WithPayload(map[string]any{
				"count":  len(result.Issues),
				"issues": result.Issues,
			})
		w.events.Emit(evt)
	}

	// 7. Attempt fix loop if configured
	if w.reviewConfig != nil && w.reviewConfig.MaxFixIterations > 0 {
		w.runReviewFixLoop(ctx, result.Issues)
	}

	// 8. Merge proceeds regardless of fix outcome (handled by caller)
}

// getBaseRef returns the base branch reference for diff comparison.
// Uses the local feature branch which may contain prior unit merges
// that haven't been pushed yet.
func (w *Worker) getBaseRef() string {
	// Primary: use target branch (the local feature branch)
	if w.config.TargetBranch != "" {
		return w.config.TargetBranch
	}
	// Fallback: main
	return "main"
}

// runReviewFixLoop attempts to fix review issues iteratively.
// This is a stub for Task #3 implementation.
func (w *Worker) runReviewFixLoop(ctx context.Context, issues []provider.ReviewIssue) {
	// TODO: Task #3 will implement this
}
