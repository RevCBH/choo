package worker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
)

// prURLPattern matches GitHub PR URLs
//
//nolint:unused // WIP: used in integration tests for PR creation
var prURLPattern = regexp.MustCompile(`https://github\.com/[^/]+/[^/]+/pull/\d+`)

func (w *Worker) runner() git.Runner {
	if w.gitRunner != nil {
		return w.gitRunner
	}
	return git.DefaultRunner()
}

// getHeadRef returns the current HEAD commit SHA
func (w *Worker) getHeadRef(ctx context.Context) (string, error) {
	out, err := w.runner().Exec(ctx, w.worktreePath, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
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
	out, err := w.runner().Exec(ctx, w.worktreePath, "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// getChangedFiles returns list of modified/added/deleted files
func (w *Worker) getChangedFiles(ctx context.Context) ([]string, error) {
	out, err := w.runner().Exec(ctx, w.worktreePath, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var files []string
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if len(line) >= 3 {
			// Format: "XY filename" where XY is status
			files = append(files, strings.TrimSpace(line[3:]))
		}
	}
	return files, nil
}

// commitViaClaudeCode invokes Claude to stage and commit changes
//
//nolint:unused // WIP: will be wired up when git delegation is fully integrated
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
			_ = w.escalator.Escalate(ctx, escalate.Escalation{
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

// pushViaClaudeCode invokes Claude to push the branch to remote
//
//nolint:unused // WIP: will be wired up when git delegation is fully integrated
func (w *Worker) pushViaClaudeCode(ctx context.Context) error {
	prompt := BuildPushPrompt(w.branch)

	result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
		if err := w.invokeClaude(ctx, prompt); err != nil {
			return err
		}

		// Verify branch exists on remote
		exists, err := w.branchExistsOnRemote(ctx, w.branch)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("branch not found on remote after push")
		}
		return nil
	})

	if !result.Success {
		if w.escalator != nil {
			_ = w.escalator.Escalate(ctx, escalate.Escalation{
				Severity: escalate.SeverityBlocking,
				Unit:     w.unit.ID,
				Title:    "Failed to push branch",
				Message:  fmt.Sprintf("Claude could not push after %d attempts", result.Attempts),
				Context: map[string]string{
					"branch": w.branch,
					"error":  result.LastErr.Error(),
				},
			})
		}
		return result.LastErr
	}

	// Emit BranchPushed event on success
	if w.events != nil {
		evt := events.NewEvent(events.BranchPushed, w.unit.ID).
			WithPayload(map[string]any{"branch": w.branch})
		w.events.Emit(evt)
	}

	return nil
}

// invokeClaude invokes Claude CLI with the given prompt (no output capture)
func (w *Worker) invokeClaude(ctx context.Context, prompt string) error {
	taskPrompt := TaskPrompt{Content: prompt}
	return w.invokeClaudeForTask(ctx, taskPrompt)
}

// invokeClaudeInDir invokes Claude CLI in a specific directory (for RepoRoot conflicts)
func (w *Worker) invokeClaudeInDir(ctx context.Context, dir, prompt string) error {
	args := []string{
		"--dangerously-skip-permissions",
		"-p", prompt,
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = dir

	// Create log file for Claude output
	logDir := filepath.Join(w.config.WorktreeBase, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create log directory: %v\n", err)
	}

	logFile, err := os.Create(filepath.Join(logDir, fmt.Sprintf("claude-merge-%s-%d.log", w.unit.ID, time.Now().Unix())))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create log file: %v\n", err)
		if !w.config.SuppressOutput {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
	} else {
		defer logFile.Close()
		fmt.Fprintf(logFile, "=== Claude invocation for merge conflict resolution ===\n")
		fmt.Fprintf(logFile, "Directory: %s\n", dir)
		fmt.Fprintf(logFile, "Prompt:\n%s\n", prompt)
		fmt.Fprintf(logFile, "=== Output ===\n")

		if w.config.SuppressOutput {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		} else {
			cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
			cmd.Stderr = io.MultiWriter(os.Stderr, logFile)
		}
	}

	return cmd.Run()
}

// invokeClaudeWithOutputImpl is the default implementation
//
//nolint:unused // WIP: used in integration tests for PR creation
func (w *Worker) invokeClaudeWithOutputImpl(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude",
		"--dangerously-skip-permissions",
		"-p", prompt,
	)
	cmd.Dir = w.worktreePath

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(out), nil
}

// extractPRURL extracts a GitHub PR URL from Claude's output
//
//nolint:unused // WIP: used in integration tests for PR creation
func extractPRURL(output string) string {
	match := prURLPattern.FindString(output)
	return match
}

// createPRViaClaudeCode invokes Claude to create the PR
//
//nolint:unused // WIP: used in integration tests for PR creation
func (w *Worker) createPRViaClaudeCode(ctx context.Context) (string, error) {
	prompt := BuildPRPrompt(w.branch, w.config.TargetBranch, w.unit.ID)

	var prURL string

	result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
		var output string
		var err error

		// Use the mock function if set, otherwise use default implementation
		if w.invokeClaudeWithOutput != nil {
			output, err = w.invokeClaudeWithOutput(ctx, prompt)
		} else {
			output, err = w.invokeClaudeWithOutputImpl(ctx, prompt)
		}

		if err != nil {
			return err
		}

		// Extract PR URL from output
		url := extractPRURL(output)
		if url == "" {
			return fmt.Errorf("could not find PR URL in claude output")
		}

		prURL = url
		return nil
	})

	if !result.Success {
		if w.escalator != nil {
			_ = w.escalator.Escalate(ctx, escalate.Escalation{
				Severity: escalate.SeverityBlocking,
				Unit:     w.unit.ID,
				Title:    "Failed to create PR",
				Message:  fmt.Sprintf("Claude could not create PR after %d attempts", result.Attempts),
				Context: map[string]string{
					"branch": w.branch,
					"target": w.config.TargetBranch,
					"error":  result.LastErr.Error(),
				},
			})
		}
		return "", result.LastErr
	}

	return prURL, nil
}
