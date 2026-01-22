package worker

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

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

// runReviewFixLoop attempts to fix review issues up to MaxFixIterations times.
// Returns true if all issues were resolved (a fix was committed).
func (w *Worker) runReviewFixLoop(ctx context.Context, issues []provider.ReviewIssue) bool {
	maxIterations := 1
	if w.reviewConfig != nil {
		maxIterations = w.reviewConfig.MaxFixIterations
	}

	for i := 0; i < maxIterations; i++ {
		// Emit fix attempt event
		if w.events != nil {
			evt := events.NewEvent(events.CodeReviewFixAttempt, w.unit.ID).
				WithPayload(map[string]any{
					"iteration":      i + 1,
					"max_iterations": maxIterations,
				})
			w.events.Emit(evt)
		}

		if w.reviewConfig != nil && w.reviewConfig.Verbose {
			fmt.Fprintf(os.Stderr, "Fix attempt %d/%d\n", i+1, maxIterations)
		}

		// Build fix prompt and invoke provider
		fixPrompt := BuildReviewFixPrompt(issues)
		if err := w.invokeProviderForFix(ctx, fixPrompt); err != nil {
			fmt.Fprintf(os.Stderr, "Fix attempt failed: %v\n", err)
			w.cleanupWorktree(ctx) // Reset any partial changes
			continue
		}

		// Commit any fix changes
		committed, err := w.commitReviewFixes(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to commit review fixes: %v\n", err)
			w.cleanupWorktree(ctx)
			continue
		}

		if committed {
			if w.events != nil {
				evt := events.NewEvent(events.CodeReviewFixApplied, w.unit.ID).
					WithPayload(map[string]any{"iteration": i + 1})
				w.events.Emit(evt)
			}
			return true // Success
		}

		// No changes made - provider didn't fix anything
		if w.reviewConfig != nil && w.reviewConfig.Verbose {
			fmt.Fprintf(os.Stderr, "No changes made by fix attempt %d\n", i+1)
		}
	}

	// Cleanup any uncommitted changes left by fix attempts
	w.cleanupWorktree(ctx)
	return false
}

// invokeProviderForFix asks the task provider to address review issues.
// Uses the same provider that executed the unit tasks.
func (w *Worker) invokeProviderForFix(ctx context.Context, fixPrompt string) error {
	if w.provider == nil {
		return fmt.Errorf("no provider configured")
	}

	// Invoke provider with fix prompt
	// stdout discarded (we only care about file changes)
	// stderr passed through for visibility
	return w.provider.Invoke(ctx, fixPrompt, w.worktreePath, io.Discard, os.Stderr)
}

// commitReviewFixes commits any changes made during the fix attempt.
// Returns (true, nil) if changes were committed, (false, nil) if no changes.
func (w *Worker) commitReviewFixes(ctx context.Context) (bool, error) {
	// 1. Check for staged/unstaged changes using git status
	hasChanges, err := w.hasUncommittedChanges(ctx)
	if err != nil {
		return false, fmt.Errorf("checking for changes: %w", err)
	}
	if !hasChanges {
		return false, nil // No changes to commit
	}

	// 2. Stage all changes
	if _, err := w.runner().Exec(ctx, w.worktreePath, "add", "-A"); err != nil {
		return false, fmt.Errorf("staging changes: %w", err)
	}

	// 3. Commit with standardized message (--no-verify to skip hooks)
	commitMsg := "fix: address code review feedback"
	if _, err := w.runner().Exec(ctx, w.worktreePath, "commit", "-m", commitMsg, "--no-verify"); err != nil {
		return false, fmt.Errorf("committing changes: %w", err)
	}

	return true, nil
}

// hasUncommittedChanges returns true if there are staged or unstaged changes.
func (w *Worker) hasUncommittedChanges(ctx context.Context) (bool, error) {
	// git status --porcelain returns empty string if clean
	output, err := w.runner().Exec(ctx, w.worktreePath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

// cleanupWorktree resets any uncommitted changes left by failed fix attempts.
// This ensures the worktree is clean before proceeding to merge.
// Errors are logged but not returned - cleanup is best-effort.
func (w *Worker) cleanupWorktree(ctx context.Context) {
	// Guard: skip if no worktree path configured (prevents accidental cleanup of cwd)
	if w.worktreePath == "" {
		return
	}

	// 1. Reset staged changes (git reset)
	if _, err := w.runner().Exec(ctx, w.worktreePath, "reset", "HEAD"); err != nil {
		if w.reviewConfig != nil && w.reviewConfig.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: git reset failed: %v\n", err)
		}
		// Continue with cleanup
	}

	// 2. Clean untracked files (git clean -fd)
	if _, err := w.runner().Exec(ctx, w.worktreePath, "clean", "-fd"); err != nil {
		if w.reviewConfig != nil && w.reviewConfig.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: git clean failed: %v\n", err)
		}
		// Continue with cleanup
	}

	// 3. Restore modified files (git checkout .)
	if _, err := w.runner().Exec(ctx, w.worktreePath, "checkout", "."); err != nil {
		if w.reviewConfig != nil && w.reviewConfig.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: git checkout failed: %v\n", err)
		}
	}
}
