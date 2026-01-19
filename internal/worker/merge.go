package worker

import (
	"context"
	"fmt"
	"strings"

	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
)

// MergeConfig holds configuration for merge operations
type MergeConfig struct {
	// TargetBranch is the branch to rebase onto and merge into
	TargetBranch string

	// MaxConflictRetries is the max attempts for conflict resolution
	MaxConflictRetries int

	// RetryConfig configures backoff behavior
	RetryConfig RetryConfig
}

// ConflictInfo contains information about detected conflicts
type ConflictInfo struct {
	// Files is the list of files with conflicts
	Files []string

	// TargetBranch is the branch being rebased onto
	TargetBranch string

	// SourceBranch is the branch being rebased
	SourceBranch string
}

// mergeWithConflictResolution performs a full merge with conflict handling
// This is called by the worker after PR approval
//
//nolint:unused // WIP: will be wired up when conflict resolution is fully integrated
func (w *Worker) mergeWithConflictResolution(ctx context.Context) error {
	// Fetch latest
	if err := git.Fetch(ctx, w.config.RepoRoot, w.config.TargetBranch); err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	// Try rebase
	targetRef := fmt.Sprintf("origin/%s", w.config.TargetBranch)
	hasConflicts, err := git.Rebase(ctx, w.worktreePath, targetRef)
	if err != nil {
		return fmt.Errorf("rebase failed: %w", err)
	}

	if !hasConflicts {
		// No conflicts, force push and merge
		return w.forcePushAndMerge(ctx)
	}

	// Get conflicted files
	conflictedFiles, err := git.GetConflictedFiles(ctx, w.worktreePath)
	if err != nil {
		return fmt.Errorf("failed to get conflicted files: %w", err)
	}

	// Emit conflict event
	if w.events != nil {
		evt := events.NewEvent(events.PRConflict, w.unit.ID).
			WithPayload(map[string]any{
				"files": conflictedFiles,
			})
		w.events.Emit(evt)
	}

	// Delegate conflict resolution to Claude
	prompt := BuildConflictPrompt(w.config.TargetBranch, conflictedFiles)

	retryResult := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
		if err := w.invokeClaude(ctx, prompt); err != nil {
			return err
		}

		// Verify rebase completed (no longer in rebase state)
		inRebase, err := git.IsRebaseInProgress(ctx, w.worktreePath)
		if err != nil {
			return err
		}
		if inRebase {
			// Claude didn't complete the rebase
			return fmt.Errorf("claude did not complete rebase")
		}
		return nil
	})

	if !retryResult.Success {
		// Clean up - abort the rebase
		_ = git.AbortRebase(ctx, w.worktreePath)

		// Escalate to user
		if w.escalator != nil {
			_ = w.escalator.Escalate(ctx, escalate.Escalation{
				Severity: escalate.SeverityBlocking,
				Unit:     w.unit.ID,
				Title:    "Failed to resolve merge conflicts",
				Message: fmt.Sprintf(
					"Claude could not resolve conflicts after %d attempts",
					retryResult.Attempts,
				),
				Context: map[string]string{
					"files":  strings.Join(conflictedFiles, ", "),
					"target": w.config.TargetBranch,
					"error":  retryResult.LastErr.Error(),
				},
			})
		}

		return retryResult.LastErr
	}

	return w.forcePushAndMerge(ctx)
}

// forcePushAndMerge pushes the rebased branch and merges via GitHub API
//
//nolint:unused // WIP: called by mergeWithConflictResolution
func (w *Worker) forcePushAndMerge(ctx context.Context) error {
	// Force push the rebased branch
	if err := git.ForcePushWithLease(ctx, w.worktreePath); err != nil {
		return fmt.Errorf("force push failed: %w", err)
	}

	// Merge via GitHub API
	if _, err := w.github.Merge(ctx, w.prNumber); err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	// Emit PRMerged event
	if w.events != nil {
		evt := events.NewEvent(events.PRMerged, w.unit.ID).WithPR(w.prNumber)
		w.events.Emit(evt)
	}

	return nil
}
