package worker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
)

// LoopState tracks the Ralph loop execution state
type LoopState struct {
	Iteration      int
	Phase          LoopPhase
	ReadyTasks     []*discovery.Task
	CompletedTasks []*discovery.Task
	FailedTasks    []*discovery.Task
	CurrentTask    *discovery.Task
}

// LoopPhase indicates the current loop phase
type LoopPhase string

const (
	PhaseTaskSelection  LoopPhase = "task_selection"
	PhaseClaudeInvoke   LoopPhase = "claude_invoke"
	PhaseBackpressure   LoopPhase = "backpressure"
	PhaseCommit         LoopPhase = "commit"
	PhaseBaselineChecks LoopPhase = "baseline_checks"
	PhaseBaselineFix    LoopPhase = "baseline_fix"
	PhasePRCreation     LoopPhase = "pr_creation"
)

// findReadyTasks returns tasks with satisfied dependencies and pending status
func (w *Worker) findReadyTasks() []*discovery.Task {
	// 1. Build set of completed task numbers
	completedSet := make(map[int]bool)
	for _, task := range w.unit.Tasks {
		if task.Status == discovery.TaskStatusComplete {
			completedSet[task.Number] = true
		}
	}

	// 2. For each pending task, check if all depends_on are in completed set
	var ready []*discovery.Task
	for _, task := range w.unit.Tasks {
		if task.Status != discovery.TaskStatusPending {
			continue
		}

		// Check if all dependencies are satisfied
		allDepsComplete := true
		for _, dep := range task.DependsOn {
			if !completedSet[dep] {
				allDepsComplete = false
				break
			}
		}

		if allDepsComplete {
			ready = append(ready, task)
		}
	}

	return ready
}

// invokeClaudeForTask runs Claude CLI as subprocess with the task prompt
// CRITICAL: Uses subprocess, NEVER the Claude API directly
// NOTE: We do NOT pass --max-turns to Claude. Each Claude invocation is ONE Ralph turn.
// Claude should have unlimited turns to complete the task within a single invocation.
// MaxClaudeRetries controls how many times Ralph will invoke Claude, not Claude's internal turns.
func (w *Worker) invokeClaudeForTask(ctx context.Context, prompt TaskPrompt) error {
	// Build args: --dangerously-skip-permissions, -p prompt.Content
	// No --max-turns: Claude gets unlimited turns per invocation
	args := []string{
		"--dangerously-skip-permissions",
		"-p", prompt.Content,
	}

	// Create exec.CommandContext for "claude" binary
	cmd := exec.CommandContext(ctx, "claude", args...)

	// Set cmd.Dir to worktree path
	cmd.Dir = w.worktreePath

	// Create log file for Claude output
	logDir := filepath.Join(w.config.WorktreeBase, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create log directory: %v\n", err)
	}

	logFile, err := os.Create(filepath.Join(logDir, fmt.Sprintf("claude-%s-%d.log", w.unit.ID, time.Now().Unix())))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create log file: %v\n", err)
		// Fall back to inheriting stdout/stderr (unless suppressed)
		if !w.config.SuppressOutput {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
	} else {
		defer logFile.Close()
		// Write prompt to log file first
		fmt.Fprintf(logFile, "=== PROMPT ===\n%s\n=== END PROMPT ===\n\n", prompt.Content)
		fmt.Fprintf(logFile, "=== CLAUDE OUTPUT ===\n")

		if w.config.SuppressOutput {
			// TUI mode: only write to log file
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		} else {
			// Non-TUI mode: tee output to both log file and stdout/stderr
			cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
			cmd.Stderr = io.MultiWriter(os.Stderr, logFile)
			fmt.Fprintf(os.Stderr, "Claude output logging to: %s\n", logFile.Name())
		}
	}

	// Emit TaskClaudeInvoke event
	if w.events != nil {
		evt := events.NewEvent(events.TaskClaudeInvoke, w.unit.ID)
		if w.currentTask != nil {
			evt = evt.WithTask(w.currentTask.Number).WithPayload(map[string]any{
				"title": w.currentTask.Title,
			})
		}
		w.events.Emit(evt)
	}

	// Run command
	runErr := cmd.Run()

	// Write completion status to log
	if logFile != nil {
		fmt.Fprintf(logFile, "\n=== END CLAUDE OUTPUT ===\n")
		if runErr != nil {
			fmt.Fprintf(logFile, "Exit error: %v\n", runErr)
		} else {
			fmt.Fprintf(logFile, "Exit: success\n")
		}
	}

	// Emit TaskClaudeDone event
	if w.events != nil {
		evt := events.NewEvent(events.TaskClaudeDone, w.unit.ID)
		if w.currentTask != nil {
			evt = evt.WithTask(w.currentTask.Number)
		}
		if runErr != nil {
			evt = evt.WithError(runErr)
		}
		w.events.Emit(evt)
	}

	// Return error if command failed
	return runErr
}

// verifyTaskComplete re-parses task file to check if status was updated
func (w *Worker) verifyTaskComplete(task *discovery.Task) (bool, error) {
	// unit.Path may be relative (e.g., specs/tasks/web) or absolute
	// task.FilePath is relative to unit dir (e.g., 01-types.md)
	// We need: worktreePath/unit.Path/task.FilePath

	// If unit.Path is absolute, make it relative to RepoRoot
	unitPath := w.unit.Path
	if filepath.IsAbs(unitPath) {
		var err error
		unitPath, err = filepath.Rel(w.config.RepoRoot, unitPath)
		if err != nil {
			return false, fmt.Errorf("failed to get relative unit path: %w", err)
		}
	}

	// Construct full task path in worktree:
	// e.g., .ralph/worktrees/web/specs/tasks/web/01-types.md
	taskPath := filepath.Join(w.worktreePath, unitPath, task.FilePath)

	updated, err := discovery.ParseTaskFile(taskPath)
	if err != nil {
		return false, fmt.Errorf("failed to parse task file %s: %w", taskPath, err)
	}

	// Return true if status is complete
	return updated.Status == discovery.TaskStatusComplete, nil
}

// commitTask commits the completed task changes
func (w *Worker) commitTask(task *discovery.Task) error {
	// 1. Stage all changes: git add -A
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = w.worktreePath
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// 2. Create commit message: "feat(unit-id): complete task #N - Title"
	msg := fmt.Sprintf("feat(%s): complete task #%d - %s", w.unit.ID, task.Number, task.Title)

	// 3. Commit with --no-verify to skip hooks
	commitCmd := exec.Command("git", "commit", "-m", msg, "--no-verify")
	commitCmd.Dir = w.worktreePath
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	// 4. Emit TaskCommitted event
	if w.events != nil {
		evt := events.NewEvent(events.TaskCommitted, w.unit.ID).WithTask(task.Number)
		w.events.Emit(evt)
	}

	return nil
}

// executeTaskWithRetry runs Claude invocation with retry logic
func (w *Worker) executeTaskWithRetry(ctx context.Context, readyTasks []*discovery.Task) (*discovery.Task, error) {
	// 1. Build prompt with ready tasks
	prompt := BuildTaskPrompt(readyTasks)

	// 2. Loop up to MaxClaudeRetries
	maxRetries := w.config.MaxClaudeRetries
	if maxRetries <= 0 {
		maxRetries = 1 // Default to at least one attempt
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Set currentTask to first ready task for event emission
		if len(readyTasks) > 0 {
			w.currentTask = readyTasks[0]
		}

		// a. Emit TaskClaudeInvoke event (done inside invokeClaudeForTask)
		// b. Invoke Claude
		claudeErr := w.invokeClaudeForTask(ctx, prompt)

		// c. Find which task was completed (scan all ready tasks)
		// IMPORTANT: Check for completion EVEN if Claude returned an error,
		// because Claude might complete the task and then hit max-turns or other limits
		var completedTask *discovery.Task
		for _, task := range readyTasks {
			complete, err := w.verifyTaskComplete(task)
			if err != nil {
				// Error parsing task file, continue to next task
				continue
			}
			if complete {
				completedTask = task
				break
			}
		}

		// d. If a task was completed
		if completedTask != nil {
			// Run backpressure
			if w.events != nil {
				evt := events.NewEvent(events.TaskBackpressure, w.unit.ID).WithTask(completedTask.Number).WithPayload(map[string]any{
					"title": completedTask.Title,
				})
				w.events.Emit(evt)
			}

			result := RunBackpressure(ctx, completedTask.Backpressure, w.worktreePath, w.config.BackpressureTimeout)

			// If backpressure passes → return completed task
			if result.Success {
				if w.events != nil {
					evt := events.NewEvent(events.TaskValidationOK, w.unit.ID).WithTask(completedTask.Number)
					w.events.Emit(evt)
				}
				return completedTask, nil
			}

			// If backpressure fails → emit validation fail and retry event, continue retry loop
			if w.events != nil {
				evt := events.NewEvent(events.TaskValidationFail, w.unit.ID).WithTask(completedTask.Number)
				evt = evt.WithPayload(map[string]any{
					"output":    result.Output,
					"exit_code": result.ExitCode,
				})
				w.events.Emit(evt)

				retryEvt := events.NewEvent(events.TaskRetry, w.unit.ID).WithTask(completedTask.Number)
				retryEvt = retryEvt.WithPayload(map[string]any{
					"attempt": attempt + 1,
					"reason":  "backpressure_failed",
				})
				w.events.Emit(retryEvt)
			}

			// Note: We don't revert status here as the spec doesn't mention it
			// The retry will just try again
			continue
		}

		// e. If no task completed → emit TaskRetry, continue
		if w.events != nil {
			evt := events.NewEvent(events.TaskRetry, w.unit.ID)
			reason := "no_task_completed"
			if claudeErr != nil {
				reason = "claude_invocation_failed"
			}
			payload := map[string]any{
				"attempt": attempt + 1,
				"reason":  reason,
			}
			if claudeErr != nil {
				payload["claude_error"] = claudeErr.Error()
			}
			evt = evt.WithPayload(payload)
			w.events.Emit(evt)
		}
	}

	// 3. Return error if max retries exceeded
	return nil, fmt.Errorf("max retries (%d) exceeded without completing a task", maxRetries)
}

// runTaskLoop executes the Ralph loop until all tasks complete or failure
func (w *Worker) runTaskLoop(ctx context.Context) error {
	for {
		// 1. Find all ready tasks
		readyTasks := w.findReadyTasks()

		// 2. If none ready and all complete → return nil
		if len(readyTasks) == 0 {
			allComplete := true
			anyFailed := false
			for _, task := range w.unit.Tasks {
				if task.Status == discovery.TaskStatusFailed {
					anyFailed = true
				}
				if task.Status != discovery.TaskStatusComplete {
					allComplete = false
				}
			}

			if allComplete {
				return nil
			}

			// 3. If none ready and some failed → return error
			if anyFailed {
				return fmt.Errorf("some tasks failed and no tasks are ready")
			}

			// If none ready but not all complete and none failed, that means blocked
			return fmt.Errorf("no tasks ready but not all complete (circular dependency or missing tasks)")
		}

		// 4-7. Execute task with retry (builds prompt, invokes Claude, runs backpressure)
		completedTask, err := w.executeTaskWithRetry(ctx, readyTasks)
		if err != nil {
			return fmt.Errorf("failed to complete task: %w", err)
		}

		// 8. Update the task status in our unit
		for _, task := range w.unit.Tasks {
			if task.Number == completedTask.Number {
				task.Status = discovery.TaskStatusComplete
				break
			}
		}

		// 9. Commit task changes
		if err := w.commitTask(completedTask); err != nil {
			return fmt.Errorf("failed to commit task #%d: %w", completedTask.Number, err)
		}

		// 10. Continue loop
	}
}
