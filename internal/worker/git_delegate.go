package worker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/anthropics/choo/internal/escalate"
)

// getHeadRef returns the current HEAD commit SHA
func (w *Worker) getHeadRef(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = w.worktreePath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// hasNewCommit checks if HEAD has moved since the given ref
func (w *Worker) hasNewCommit(ctx context.Context, sinceRef string) (bool, error) {
	currentHead, err := w.getHeadRef(ctx)
	if err != nil {
		return false, err
	}
	return currentHead != sinceRef, nil
}

// branchExistsOnRemote checks if a branch exists on the remote
func (w *Worker) branchExistsOnRemote(ctx context.Context, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", "origin", branch)
	cmd.Dir = w.worktreePath
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// getChangedFiles returns list of modified/added/deleted files
func (w *Worker) getChangedFiles(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = w.worktreePath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) >= 3 {
			// Format: "XY filename" where XY is status
			files = append(files, strings.TrimSpace(line[3:]))
		}
	}
	return files, nil
}

// commitViaClaudeCode invokes Claude to stage and commit changes
func (w *Worker) commitViaClaudeCode(ctx context.Context, taskTitle string) error {
	// Get the HEAD ref before invoking Claude
	headBefore, err := w.getHeadRef(ctx)
	if err != nil {
		return fmt.Errorf("failed to get HEAD ref: %w", err)
	}

	files, _ := w.getChangedFiles(ctx)
	prompt := BuildCommitPrompt(taskTitle, files)

	result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
		if err := w.invokeClaude(ctx, prompt); err != nil {
			return err
		}

		// Verify commit was created
		hasCommit, err := w.hasNewCommit(ctx, headBefore)
		if err != nil {
			return err
		}
		if !hasCommit {
			return fmt.Errorf("claude did not create a commit")
		}
		return nil
	})

	if !result.Success {
		if w.escalator != nil {
			w.escalator.Escalate(ctx, escalate.Escalation{
				Severity: escalate.SeverityBlocking,
				Unit:     w.unit.ID,
				Title:    "Failed to commit changes",
				Message:  fmt.Sprintf("Claude could not commit after %d attempts", result.Attempts),
				Context: map[string]string{
					"task":  taskTitle,
					"error": result.LastErr.Error(),
				},
			})
		}
		return result.LastErr
	}

	return nil
}

// invokeClaude invokes Claude CLI with the given prompt (no output capture)
func (w *Worker) invokeClaude(ctx context.Context, prompt string) error {
	taskPrompt := TaskPrompt{Content: prompt}
	return w.invokeClaudeForTask(ctx, taskPrompt)
}
