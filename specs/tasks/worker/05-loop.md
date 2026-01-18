---
task: 5
status: pending
backpressure: "go test ./internal/worker/... -run Loop"
depends_on: [1, 2, 3]
---

# Ralph Loop Implementation

**Parent spec**: `/specs/WORKER.md`
**Task**: #5 of 8 in implementation plan

## Objective

Implement the core Ralph loop that orchestrates task execution: find ready tasks, invoke Claude CLI via subprocess, validate completion, run backpressure, and commit.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides `Unit`, `Task`, `TaskStatus`, `ParseTaskFile`
- EVENTS - provides `Bus`, event types
- CLAUDE - provides `Client` (for subprocess invocation)

### Task Dependencies (within this unit)
- Task #1 (types-config) - provides `Worker`, `WorkerConfig`, `LoopState`, `LoopPhase`
- Task #2 (prompt) - provides `BuildTaskPrompt`, `TaskPrompt`
- Task #3 (backpressure) - provides `RunBackpressure`, `BackpressureResult`

### Package Dependencies
- `os/exec` - for Claude CLI subprocess
- `context` - for cancellation
- `strconv` - for max-turns argument

## Deliverables

### Files to Modify

```
internal/worker/
└── loop.go         # Add loop methods to existing file
```

### Functions to Implement

```go
// runTaskLoop executes the Ralph loop until all tasks complete or failure
func (w *Worker) runTaskLoop(ctx context.Context) error {
    // Loop:
    // 1. Find all ready tasks
    // 2. If none ready and all complete → return nil
    // 3. If none ready and some failed → return error
    // 4. Build prompt with all ready tasks
    // 5. Invoke Claude CLI subprocess
    // 6. After Claude returns, find which task was completed (check frontmatter)
    // 7. Run backpressure for completed task
    // 8. If backpressure fails → retry Claude
    // 9. Commit task changes
    // 10. Continue loop
}

// findReadyTasks returns tasks with satisfied dependencies and pending status
func (w *Worker) findReadyTasks() []*discovery.Task {
    // 1. Build set of completed task numbers
    // 2. For each pending task, check if all depends_on are in completed set
    // 3. Return slice of ready tasks
}

// invokeClaudeForTask runs Claude CLI as subprocess with the task prompt
// CRITICAL: Uses subprocess, NEVER the Claude API directly
func (w *Worker) invokeClaudeForTask(ctx context.Context, prompt TaskPrompt) error {
    // 1. Build args: --dangerously-skip-permissions, -p prompt.Content
    // 2. Optionally add --max-turns if configured
    // 3. Create exec.CommandContext for "claude" binary
    // 4. Set cmd.Dir to worktree path
    // 5. Connect stdout/stderr to logger
    // 6. Emit TaskClaudeInvoke event
    // 7. Run command
    // 8. Emit TaskClaudeDone event
    // 9. Return error if command failed
}

// verifyTaskComplete re-parses task file to check if status was updated
func (w *Worker) verifyTaskComplete(task *discovery.Task) (bool, error) {
    // 1. Call discovery.ParseTaskFile(task.FilePath)
    // 2. Return updated.Status == TaskStatusComplete
}

// executeTaskWithRetry runs Claude invocation with retry logic
func (w *Worker) executeTaskWithRetry(ctx context.Context, readyTasks []*discovery.Task) (*discovery.Task, error) {
    // 1. Build prompt with ready tasks
    // 2. Loop up to MaxClaudeRetries:
    //    a. Emit TaskClaudeInvoke event
    //    b. Invoke Claude
    //    c. Find which task was completed (scan all ready tasks)
    //    d. If a task was completed:
    //       - Run backpressure
    //       - If backpressure passes → return completed task
    //       - If backpressure fails → revert status, continue retry loop
    //    e. If no task completed → emit TaskRetry, continue
    // 3. Return error if max retries exceeded
}

// commitTask commits the completed task changes
func (w *Worker) commitTask(task *discovery.Task) error {
    // 1. Stage all changes: git add -A
    // 2. Create commit message: "feat(unit-id): complete task #N - Title"
    // 3. Commit with --no-verify to skip hooks
    // 4. Emit TaskCommitted event
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run Loop
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestFindReadyTasks_NoDependencies` | All pending tasks with empty depends_on are ready |
| `TestFindReadyTasks_WithDependencies` | Only tasks with completed deps are ready |
| `TestFindReadyTasks_AllComplete` | Returns empty slice when all tasks complete |
| `TestFindReadyTasks_BlockedTasks` | Tasks with unsatisfied deps are not ready |
| `TestInvokeClaudeForTask_BuildsCorrectArgs` | Args include --dangerously-skip-permissions and -p |
| `TestInvokeClaudeForTask_SetsWorkdir` | cmd.Dir is set to worktree path |
| `TestVerifyTaskComplete_Parses` | Correctly detects status changes |
| `TestCommitTask_MessageFormat` | Commit message matches expected format |

### Test Implementation

```go
func TestFindReadyTasks_NoDependencies(t *testing.T) {
    unit := &discovery.Unit{
        Tasks: []*discovery.Task{
            {Number: 1, Status: discovery.TaskStatusPending, DependsOn: []int{}},
            {Number: 2, Status: discovery.TaskStatusPending, DependsOn: []int{}},
        },
    }

    w := &Worker{unit: unit}
    ready := w.findReadyTasks()

    if len(ready) != 2 {
        t.Errorf("expected 2 ready tasks, got %d", len(ready))
    }
}

func TestFindReadyTasks_WithDependencies(t *testing.T) {
    unit := &discovery.Unit{
        Tasks: []*discovery.Task{
            {Number: 1, Status: discovery.TaskStatusComplete, DependsOn: []int{}},
            {Number: 2, Status: discovery.TaskStatusPending, DependsOn: []int{1}},
            {Number: 3, Status: discovery.TaskStatusPending, DependsOn: []int{2}},
        },
    }

    w := &Worker{unit: unit}
    ready := w.findReadyTasks()

    if len(ready) != 1 {
        t.Errorf("expected 1 ready task, got %d", len(ready))
    }
    if ready[0].Number != 2 {
        t.Errorf("expected task 2 to be ready, got task %d", ready[0].Number)
    }
}

func TestFindReadyTasks_AllComplete(t *testing.T) {
    unit := &discovery.Unit{
        Tasks: []*discovery.Task{
            {Number: 1, Status: discovery.TaskStatusComplete, DependsOn: []int{}},
            {Number: 2, Status: discovery.TaskStatusComplete, DependsOn: []int{1}},
        },
    }

    w := &Worker{unit: unit}
    ready := w.findReadyTasks()

    if len(ready) != 0 {
        t.Errorf("expected 0 ready tasks, got %d", len(ready))
    }
}

func TestFindReadyTasks_BlockedTasks(t *testing.T) {
    unit := &discovery.Unit{
        Tasks: []*discovery.Task{
            {Number: 1, Status: discovery.TaskStatusPending, DependsOn: []int{}},
            {Number: 2, Status: discovery.TaskStatusPending, DependsOn: []int{1}},
            {Number: 3, Status: discovery.TaskStatusPending, DependsOn: []int{1, 2}},
        },
    }

    w := &Worker{unit: unit}
    ready := w.findReadyTasks()

    // Only task 1 should be ready
    if len(ready) != 1 {
        t.Errorf("expected 1 ready task, got %d", len(ready))
    }
    if ready[0].Number != 1 {
        t.Errorf("expected task 1 to be ready")
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (tests mock Claude invocation)
- [x] Runs in <60 seconds

## Implementation Notes

- CRITICAL: Claude CLI is invoked via `exec.CommandContext("claude", args...)`, NEVER via API
- Use `--dangerously-skip-permissions` flag for non-interactive execution
- After Claude returns, scan ALL ready tasks to find which one was completed
- Re-parse task files using discovery.ParseTaskFile to get updated status
- Commit with `--no-verify` to skip pre-commit hooks during iteration

## NOT In Scope

- Worktree creation/cleanup (task #6)
- Baseline checks phase (task #6)
- PR creation (task #6)
- Pool management (task #7)
