package worker

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/anthropics/choo/internal/claude"
	"github.com/anthropics/choo/internal/discovery"
	"github.com/anthropics/choo/internal/events"
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
func (w *Worker) invokeClaudeForTask(ctx context.Context, prompt TaskPrompt) error {
	// 1. Emit TaskClaudeInvoke event
	if w.events != nil {
		evt := events.NewEvent(events.TaskClaudeInvoke, w.unit.ID)
		if w.currentTask != nil {
			evt = evt.WithTask(w.currentTask.Number)
		}
		w.events.Emit(evt)
	}

	// 2. Build execute options
	opts := claude.ExecuteOptions{
		Prompt:                     prompt.Content,
		WorkDir:                    w.worktreePath,
		MaxTurns:                   w.config.MaxClaudeRetries,
		DangerouslySkipPermissions: true,
		Timeout:                    10 * time.Minute, // Default timeout
		Stdout:                     nil,              // Inherit from parent
		Stderr:                     nil,              // Inherit from parent
	}

	// 3. Execute via Claude client
	result, err := w.claude.Execute(ctx, opts)

	// 4. Emit TaskClaudeDone event
	if w.events != nil {
		evt := events.NewEvent(events.TaskClaudeDone, w.unit.ID)
		if w.currentTask != nil {
			evt = evt.WithTask(w.currentTask.Number)
		}
		if err != nil {
			evt = evt.WithError(err)
		}
		w.events.Emit(evt)
	}

	// 5. Return error if execution failed
	if err != nil {
		return err
	}
	if result != nil && !result.Success {
		return fmt.Errorf("claude execution failed with exit code %d", result.ExitCode)
	}

	return nil
}

// verifyTaskComplete re-parses task file to check if status was updated
func (w *Worker) verifyTaskComplete(task *discovery.Task) (bool, error) {
	// 1. Call discovery.ParseTaskFile(task.FilePath)
	// Use worktreePath since Claude edits files in the worktree, not the main repo
	taskPath := filepath.Join(w.worktreePath, task.FilePath)
	updated, err := discovery.ParseTaskFile(taskPath)
	if err != nil {
		return false, err
	}

	// 2. Return updated.Status == TaskStatusComplete
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
		// a. Emit TaskClaudeInvoke event (done inside invokeClaudeForTask)
		// b. Invoke Claude
		err := w.invokeClaudeForTask(ctx, prompt)
		if err != nil {
			// If Claude invocation itself failed, continue retrying
			if w.events != nil {
				evt := events.NewEvent(events.TaskRetry, w.unit.ID)
				evt = evt.WithPayload(map[string]interface{}{
					"attempt": attempt + 1,
					"reason":  "claude_invocation_failed",
					"error":   err.Error(),
				})
				w.events.Emit(evt)
			}
			continue
		}

		// c. Find which task was completed (scan all ready tasks)
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
				evt := events.NewEvent(events.TaskBackpressure, w.unit.ID).WithTask(completedTask.Number)
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

			// If backpressure fails → emit validation fail, continue retry loop
			if w.events != nil {
				evt := events.NewEvent(events.TaskValidationFail, w.unit.ID).WithTask(completedTask.Number)
				evt = evt.WithPayload(map[string]interface{}{
					"output":    result.Output,
					"exit_code": result.ExitCode,
				})
				w.events.Emit(evt)
			}

			// Note: We don't revert status here as the spec doesn't mention it
			// The retry will just try again
		}

		// e. If no task completed → emit TaskRetry, continue
		if w.events != nil {
			evt := events.NewEvent(events.TaskRetry, w.unit.ID)
			evt = evt.WithPayload(map[string]interface{}{
				"attempt": attempt + 1,
				"reason":  "no_task_completed",
			})
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
