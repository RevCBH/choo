# WORKER - Task Execution Engine for Ralph Orchestrator

## Overview

The Worker package implements the core task execution engine ("Ralph loop") that runs within a git worktree to complete a unit's tasks. Each Worker manages a single unit's lifecycle: worktree setup, iterative task execution with Claude CLI, backpressure validation, baseline checks, and PR creation.

Workers are spawned by the Pool and operate independently in parallel. They communicate progress via the Event Bus and never call the Claude API directly - all Claude interactions go through the `claude` CLI binary as a subprocess.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Worker Pool                                │
│                                                                         │
│   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐                  │
│   │  Worker 1   │   │  Worker 2   │   │  Worker 3   │   ...            │
│   │ (app-shell) │   │ (deck-list) │   │  (config)   │                  │
│   └──────┬──────┘   └──────┬──────┘   └──────┬──────┘                  │
│          │                 │                 │                          │
│          └─────────────────┼─────────────────┘                          │
│                            │                                            │
│                    ┌───────▼───────┐                                    │
│                    │   Event Bus   │                                    │
│                    └───────────────┘                                    │
└─────────────────────────────────────────────────────────────────────────┘
                             │
         ┌───────────────────┼───────────────────┐
         ▼                   ▼                   ▼
   ┌───────────┐       ┌───────────┐       ┌───────────┐
   │    Git    │       │  Claude   │       │  GitHub   │
   │ Worktrees │       │    CLI    │       │    API    │
   └───────────┘       └───────────┘       └───────────┘
```

## Requirements

### Functional Requirements

1. Create isolated git worktree for each unit execution
2. Execute the Ralph loop: find ready tasks, invoke Claude, validate, commit, repeat
3. Present ALL ready tasks to Claude and let Claude choose which one to work on
4. Run per-task backpressure commands before marking tasks complete
5. Run baseline checks (fmt, vet/clippy, typecheck) after all tasks complete
6. Commit changes per-task with `--no-verify` to skip hooks during iteration
7. Push branch and create PR after unit completion
8. Emit events for all state transitions (task started, completed, failed, etc.)
9. Support retry logic for Claude failures and backpressure failures
10. Clean up worktree on completion or failure

### Performance Requirements

| Metric | Target |
|--------|--------|
| Worktree creation | <5s |
| Task prompt construction | <10ms |
| Backpressure command timeout | 5 minutes default |
| Baseline check timeout | 10 minutes default |
| Event emission latency | <1ms |

### Constraints

- MUST use `claude` CLI subprocess, NEVER the Claude API directly
- MUST use `--dangerously-skip-permissions` flag for non-interactive execution
- MUST commit with `--no-verify` during task loop to avoid hook delays
- MUST run baseline checks at end of unit before PR creation
- Depends on: discovery (Unit, Task types), events (Bus), git, github, claude packages

## Design

### Module Structure

```
internal/worker/
├── worker.go           # Single unit worker implementation
├── pool.go             # Worker pool management
├── loop.go             # Ralph loop implementation
├── prompt.go           # Task prompt construction
├── backpressure.go     # Validation command runner
├── baseline.go         # Baseline checks (fmt, vet, typecheck)
└── execute.go          # Public Execute() entry point
```

### Core Types

```go
// internal/worker/worker.go

// Worker executes a single unit in an isolated worktree
type Worker struct {
    // Unit being executed
    unit *discovery.Unit

    // Configuration
    config WorkerConfig

    // Dependencies
    events  *events.Bus
    git     *git.WorktreeManager
    github  *github.PRClient
    claude  *claude.Client

    // Runtime state
    worktreePath string
    branch       string
    currentTask  *discovery.Task
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
    // RepoRoot is the absolute path to the main repository
    RepoRoot string

    // TargetBranch is the branch PRs will target (e.g., "main")
    TargetBranch string

    // WorktreeBase is the directory for worktrees (e.g., "/tmp/ralph-worktrees")
    WorktreeBase string

    // BaselineChecks are commands to run after all tasks complete
    BaselineChecks []BaselineCheck

    // MaxClaudeRetries is attempts before failing a task (default: 3)
    MaxClaudeRetries int

    // MaxBaselineRetries is attempts to fix baseline failures (default: 3)
    MaxBaselineRetries int

    // BackpressureTimeout is max time for backpressure commands
    BackpressureTimeout time.Duration

    // BaselineTimeout is max time for baseline checks
    BaselineTimeout time.Duration

    // NoPR skips PR creation when true
    NoPR bool
}

// BaselineCheck represents a single baseline validation command
type BaselineCheck struct {
    // Name identifies the check (e.g., "rust-fmt", "typescript")
    Name string

    // Command is the shell command to run
    Command string

    // Pattern is the glob pattern for files this check applies to
    Pattern string
}
```

```go
// internal/worker/pool.go

// Pool manages a collection of workers executing units in parallel
type Pool struct {
    // Configuration
    maxWorkers int
    config     WorkerConfig

    // Dependencies
    events *events.Bus
    git    *git.WorktreeManager
    github *github.PRClient
    claude *claude.Client

    // Runtime state
    workers   map[string]*Worker  // unit ID -> worker
    mu        sync.Mutex
    wg        sync.WaitGroup
}

// PoolStats holds current pool statistics
type PoolStats struct {
    ActiveWorkers   int
    CompletedUnits  int
    FailedUnits     int
    TotalTasks      int
    CompletedTasks  int
}
```

```go
// internal/worker/loop.go

// LoopState tracks the Ralph loop execution state
type LoopState struct {
    // Iteration counter
    Iteration int

    // Current phase
    Phase LoopPhase

    // Tasks by status
    ReadyTasks     []*discovery.Task
    CompletedTasks []*discovery.Task
    FailedTasks    []*discovery.Task

    // Current task being worked on (set after Claude picks)
    CurrentTask *discovery.Task
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
```

```go
// internal/worker/prompt.go

// TaskPrompt contains the constructed prompt for Claude
type TaskPrompt struct {
    // Content is the full prompt text
    Content string

    // ReadyTasks lists the tasks included in this prompt
    ReadyTasks []*discovery.Task
}
```

```go
// internal/worker/backpressure.go

// BackpressureResult holds the result of a backpressure command
type BackpressureResult struct {
    // Success indicates if the command passed
    Success bool

    // Output is the combined stdout/stderr
    Output string

    // Duration is how long the command took
    Duration time.Duration

    // ExitCode is the command's exit code
    ExitCode int
}
```

### Imports (from other packages)

```go
import (
    "github.com/ralph-orch/internal/discovery"  // Unit, Task, UnitStatus, TaskStatus
    "github.com/ralph-orch/internal/events"     // Bus, Event, EventType
    "github.com/ralph-orch/internal/git"        // WorktreeManager
    "github.com/ralph-orch/internal/github"     // PRClient
    "github.com/ralph-orch/internal/claude"     // Client
)
```

### Exports (public API)

```go
// internal/worker/worker.go

// NewWorker creates a worker for executing a unit
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) *Worker

// Run executes the unit through all phases: setup, task loop, baseline, PR
func (w *Worker) Run(ctx context.Context) error

// WorkerDeps bundles worker dependencies for injection
type WorkerDeps struct {
    Events *events.Bus
    Git    *git.WorktreeManager
    GitHub *github.PRClient
    Claude *claude.Client
}
```

```go
// internal/worker/pool.go

// NewPool creates a worker pool with the specified parallelism
func NewPool(maxWorkers int, cfg WorkerConfig, deps WorkerDeps) *Pool

// Submit queues a unit for execution
func (p *Pool) Submit(unit *discovery.Unit) error

// Wait blocks until all submitted units complete
func (p *Pool) Wait() error

// Stats returns current pool statistics
func (p *Pool) Stats() PoolStats

// Shutdown gracefully stops all workers
func (p *Pool) Shutdown(ctx context.Context) error
```

```go
// internal/worker/execute.go

// Execute runs a single unit to completion (convenience wrapper)
// This is the primary entry point for single-unit execution
func Execute(ctx context.Context, unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) error
```

### API Surface

```go
// internal/worker/loop.go

// runTaskLoop executes the Ralph loop until all tasks complete or failure
func (w *Worker) runTaskLoop(ctx context.Context) error

// findReadyTasks returns tasks with satisfied dependencies and pending status
func (w *Worker) findReadyTasks() []*discovery.Task

// executeTask runs Claude on a single task with retry logic
func (w *Worker) executeTask(ctx context.Context, task *discovery.Task) error
```

```go
// internal/worker/prompt.go

// BuildTaskPrompt constructs the Claude prompt for ready tasks
func BuildTaskPrompt(readyTasks []*discovery.Task) TaskPrompt

// BuildBaselineFixPrompt constructs the prompt for fixing baseline failures
func BuildBaselineFixPrompt(checkOutput string, baselineCommands string) string
```

```go
// internal/worker/backpressure.go

// RunBackpressure executes a task's backpressure command
func RunBackpressure(ctx context.Context, command string, workdir string, timeout time.Duration) BackpressureResult

// ValidateTaskComplete checks if task status was updated to complete
func ValidateTaskComplete(task *discovery.Task) bool
```

```go
// internal/worker/baseline.go

// RunBaselineChecks executes all baseline checks for the unit
func RunBaselineChecks(ctx context.Context, checks []BaselineCheck, workdir string, timeout time.Duration) (bool, string)

// BaselineCheckResult holds results for a single check
type BaselineCheckResult struct {
    Check   BaselineCheck
    Passed  bool
    Output  string
}
```

### Worker Execution Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                     Phase 1: Setup                              │
├─────────────────────────────────────────────────────────────────┤
│  1. Create worktree: git worktree add <path> -b <branch>        │
│  2. Update unit frontmatter: orch_status=in_progress            │
│  3. Emit UnitStarted event                                      │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Phase 2: Task Loop                            │
├─────────────────────────────────────────────────────────────────┤
│  while true:                                                    │
│    1. Find all "ready" tasks (pending + deps satisfied)         │
│    2. If none ready:                                            │
│       - All complete → break to Phase 2.5                       │
│       - Some failed → emit UnitFailed, return error             │
│    3. Build prompt with ALL ready tasks                         │
│    4. Invoke Claude CLI (claude chooses one task)               │
│    5. Emit TaskClaudeDone event                                 │
│    6. Check task frontmatter status:                            │
│       - If status != complete → retry (back to step 4)          │
│    7. Run backpressure command for completed task               │
│    8. If backpressure fails:                                    │
│       - Revert status to in_progress                            │
│       - Retry (back to step 4)                                  │
│    9. Commit with --no-verify                                   │
│   10. Emit TaskCompleted event                                  │
│   11. Continue (find new ready tasks)                           │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                Phase 2.5: Baseline Checks                       │
├─────────────────────────────────────────────────────────────────┤
│  1. Run baseline checks (fmt, vet/clippy, typecheck)            │
│  2. If checks fail:                                             │
│     a. Invoke Claude with baseline-fix prompt                   │
│     b. Commit fixes with --no-verify                            │
│     c. Re-run baseline checks                                   │
│     d. If still failing after 3 attempts → emit UnitFailed      │
│  3. Commit baseline fixes (if any) with proper message          │
│  4. Continue to Phase 3                                         │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Phase 3: PR Lifecycle                          │
├─────────────────────────────────────────────────────────────────┤
│  1. Push branch to remote                                       │
│  2. Create PR via GitHub API                                    │
│  3. Update unit frontmatter: orch_status=pr_open                │
│  4. Emit PRCreated event                                        │
│  5. Enter review loop (handled by PR Manager)                   │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Phase 4: Cleanup                             │
├─────────────────────────────────────────────────────────────────┤
│  1. Remove worktree: git worktree remove <path>                 │
│  2. Delete local branch (if merged)                             │
│  3. Emit UnitCompleted event                                    │
└─────────────────────────────────────────────────────────────────┘
```

### Task Prompt Format

The prompt presents all ready tasks and instructs Claude to choose one:

```go
func BuildTaskPrompt(readyTasks []*discovery.Task) TaskPrompt {
    var taskList strings.Builder
    for _, t := range readyTasks {
        fmt.Fprintf(&taskList, "### Task #%d: %s\n", t.Number, t.Title)
        fmt.Fprintf(&taskList, "- File: %s\n", t.FilePath)
        fmt.Fprintf(&taskList, "- Backpressure: `%s`\n\n", t.Backpressure)
    }

    content := fmt.Sprintf(`You are executing a Ralph task. Follow these instructions exactly.

## Ready Tasks

The following tasks have all dependencies satisfied. Choose ONE to implement:

%s

## Instructions
1. Choose one task from the ready list above
2. Read that task's spec file completely
3. Implement ONLY what is specified - nothing more, nothing less
4. Run the backpressure validation command from the task's frontmatter
5. If validation fails, fix the issues and re-run until it passes
6. When the backpressure check passes, UPDATE THE FRONTMATTER STATUS to complete:
   - Edit the task spec file
   - Change `+"`status: in_progress`"+` to `+"`status: complete`"+`
7. Do NOT move on to other tasks - stop after completing one

## Critical
- Choose ONE task and complete it fully
- You MUST update the spec file's frontmatter status to `+"`complete`"+` when done
- The backpressure command MUST pass before marking complete
- Do not refactor unrelated code
- Do not add features not in the spec
- NEVER run tests in watch mode. Always use flags to run tests once and exit.
`,
        taskList.String(),
    )

    return TaskPrompt{
        Content:    content,
        ReadyTasks: readyTasks,
    }
}
```

### Claude CLI Invocation

Workers always invoke Claude via subprocess:

```go
func (w *Worker) invokeClaudeForTask(ctx context.Context, prompt TaskPrompt) error {
    args := []string{
        "--dangerously-skip-permissions",
        "-p", prompt.Content,
    }

    if w.config.MaxTurns > 0 {
        args = append(args, "--max-turns", strconv.Itoa(w.config.MaxTurns))
    }

    cmd := exec.CommandContext(ctx, "claude", args...)
    cmd.Dir = w.worktreePath
    cmd.Stdout = w.logger
    cmd.Stderr = w.logger

    w.events.Emit(events.Event{
        Type: events.TaskClaudeInvoke,
        Unit: w.unit.ID,
    })

    err := cmd.Run()

    w.events.Emit(events.Event{
        Type: events.TaskClaudeDone,
        Unit: w.unit.ID,
    })

    return err
}
```

### Backpressure Validation

Each task has a backpressure command that must pass before the task is considered complete:

```go
func RunBackpressure(ctx context.Context, command string, workdir string, timeout time.Duration) BackpressureResult {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    start := time.Now()

    cmd := exec.CommandContext(ctx, "sh", "-c", command)
    cmd.Dir = workdir

    output, err := cmd.CombinedOutput()

    exitCode := 0
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            exitCode = exitErr.ExitCode()
        }
    }

    return BackpressureResult{
        Success:  err == nil,
        Output:   string(output),
        Duration: time.Since(start),
        ExitCode: exitCode,
    }
}
```

### Commit Strategy

Tasks are committed individually with `--no-verify` to skip pre-commit hooks during iteration:

```go
func (w *Worker) commitTask(task *discovery.Task) error {
    message := fmt.Sprintf("feat(%s): complete task #%d - %s",
        w.unit.ID, task.Number, task.Title)

    // Stage all changes
    if err := w.git.AddAll(w.worktreePath); err != nil {
        return fmt.Errorf("git add: %w", err)
    }

    // Commit with --no-verify to skip hooks during task loop
    if err := w.git.Commit(w.worktreePath, message, "--no-verify"); err != nil {
        return fmt.Errorf("git commit: %w", err)
    }

    w.events.Emit(events.Event{
        Type: events.TaskCommitted,
        Unit: w.unit.ID,
        Task: &task.Number,
    })

    return nil
}
```

### Baseline Checks

Run after all tasks complete, before PR creation:

```go
func RunBaselineChecks(ctx context.Context, checks []BaselineCheck, workdir string, timeout time.Duration) (bool, string) {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    var failures []string
    allPassed := true

    for _, check := range checks {
        cmd := exec.CommandContext(ctx, "sh", "-c", check.Command)
        cmd.Dir = workdir

        output, err := cmd.CombinedOutput()
        if err != nil {
            allPassed = false
            failures = append(failures, fmt.Sprintf("=== %s ===\n%s", check.Name, output))
        }
    }

    return allPassed, strings.Join(failures, "\n\n")
}
```

### Baseline Fix Prompt

When baseline checks fail, Claude is invoked to fix the issues:

```go
func BuildBaselineFixPrompt(checkOutput string, baselineCommands string) string {
    return fmt.Sprintf(`You are fixing baseline check failures. Follow these instructions exactly.

## Baseline Check Failures

The following baseline checks failed after completing all tasks:

%s

## Baseline Commands
%s

## Instructions

1. Review the error output above
2. Fix the issues (formatting, linting, type errors)
3. Re-run the baseline checks to verify fixes
4. Do NOT commit - the orchestrator will commit for you

## Critical
- Only fix issues reported by baseline checks
- Do not refactor or change logic
- Do not modify test assertions
- These are lint/format fixes only
`,
        checkOutput,
        baselineCommands,
    )
}
```

## Implementation Notes

### Ready Task Detection

A task is "ready" when:
1. Its status is `pending`
2. All tasks in its `depends_on` list have status `complete`

```go
func (w *Worker) findReadyTasks() []*discovery.Task {
    completedNums := make(map[int]bool)
    for _, t := range w.unit.Tasks {
        if t.Status == discovery.TaskStatusComplete {
            completedNums[t.Number] = true
        }
    }

    var ready []*discovery.Task
    for _, t := range w.unit.Tasks {
        if t.Status != discovery.TaskStatusPending {
            continue
        }

        depsOK := true
        for _, dep := range t.DependsOn {
            if !completedNums[dep] {
                depsOK = false
                break
            }
        }

        if depsOK {
            ready = append(ready, t)
        }
    }

    return ready
}
```

### Status Verification After Claude

After Claude invocation, the worker must verify the task was actually completed:

```go
func (w *Worker) verifyTaskComplete(task *discovery.Task) (bool, error) {
    // Re-parse the task file to get updated frontmatter
    updated, err := discovery.ParseTaskFile(task.FilePath)
    if err != nil {
        return false, fmt.Errorf("parse task file: %w", err)
    }

    return updated.Status == discovery.TaskStatusComplete, nil
}
```

### Retry Logic

The worker retries Claude invocation up to `MaxClaudeRetries` times:

```go
func (w *Worker) executeTaskWithRetry(ctx context.Context, task *discovery.Task) error {
    var lastErr error

    for attempt := 1; attempt <= w.config.MaxClaudeRetries; attempt++ {
        w.events.Emit(events.Event{
            Type:    events.TaskClaudeInvoke,
            Unit:    w.unit.ID,
            Task:    &task.Number,
            Payload: map[string]int{"attempt": attempt},
        })

        if err := w.invokeClaudeForTask(ctx, task); err != nil {
            lastErr = err
            w.events.Emit(events.Event{
                Type:  events.TaskRetry,
                Unit:  w.unit.ID,
                Task:  &task.Number,
                Error: err.Error(),
            })
            continue
        }

        // Check if task was marked complete
        complete, err := w.verifyTaskComplete(task)
        if err != nil {
            return fmt.Errorf("verify task: %w", err)
        }

        if complete {
            return nil
        }

        // Task not marked complete, retry
        lastErr = fmt.Errorf("task not marked complete after Claude invocation")
    }

    return fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

### Event Emission

Workers emit events at key state transitions:

| Event | When |
|-------|------|
| UnitStarted | After worktree creation |
| TaskStarted | When starting to work on a task |
| TaskClaudeInvoke | Before Claude CLI invocation |
| TaskClaudeDone | After Claude CLI returns |
| TaskBackpressure | Before running backpressure command |
| TaskValidationOK | Backpressure passed |
| TaskValidationFail | Backpressure failed |
| TaskCommitted | After git commit |
| TaskCompleted | Task fully done |
| TaskRetry | Retrying task |
| TaskFailed | Task exhausted retries |
| UnitFailed | Unit cannot continue |
| PRCreated | After PR creation |
| UnitCompleted | Unit fully done |

### Branch Naming

Branches follow the pattern `ralph/<unit-id>-<short-hash>`:

```go
func (w *Worker) generateBranchName() string {
    hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", w.unit.ID, time.Now().UnixNano())))
    shortHash := hex.EncodeToString(hash[:3])
    return fmt.Sprintf("ralph/%s-%s", w.unit.ID, shortHash)
}
```

## Testing Strategy

### Unit Tests

```go
// internal/worker/prompt_test.go

func TestBuildTaskPrompt_SingleTask(t *testing.T) {
    tasks := []*discovery.Task{
        {Number: 1, Title: "Nav Types", FilePath: "01-nav-types.md", Backpressure: "pnpm typecheck"},
    }

    prompt := BuildTaskPrompt(tasks)

    if !strings.Contains(prompt.Content, "Task #1: Nav Types") {
        t.Error("prompt should contain task title")
    }
    if !strings.Contains(prompt.Content, "pnpm typecheck") {
        t.Error("prompt should contain backpressure command")
    }
}

func TestBuildTaskPrompt_MultipleTasks(t *testing.T) {
    tasks := []*discovery.Task{
        {Number: 1, Title: "Task A", FilePath: "01-a.md", Backpressure: "cmd-a"},
        {Number: 2, Title: "Task B", FilePath: "02-b.md", Backpressure: "cmd-b"},
        {Number: 3, Title: "Task C", FilePath: "03-c.md", Backpressure: "cmd-c"},
    }

    prompt := BuildTaskPrompt(tasks)

    // All tasks should be listed
    for _, task := range tasks {
        if !strings.Contains(prompt.Content, task.Title) {
            t.Errorf("prompt should contain task %q", task.Title)
        }
    }

    // Should instruct to choose ONE
    if !strings.Contains(prompt.Content, "Choose ONE") {
        t.Error("prompt should instruct to choose one task")
    }
}
```

```go
// internal/worker/backpressure_test.go

func TestRunBackpressure_Success(t *testing.T) {
    result := RunBackpressure(context.Background(), "exit 0", t.TempDir(), time.Minute)

    if !result.Success {
        t.Error("expected success for exit 0")
    }
    if result.ExitCode != 0 {
        t.Errorf("expected exit code 0, got %d", result.ExitCode)
    }
}

func TestRunBackpressure_Failure(t *testing.T) {
    result := RunBackpressure(context.Background(), "exit 1", t.TempDir(), time.Minute)

    if result.Success {
        t.Error("expected failure for exit 1")
    }
    if result.ExitCode != 1 {
        t.Errorf("expected exit code 1, got %d", result.ExitCode)
    }
}

func TestRunBackpressure_Timeout(t *testing.T) {
    result := RunBackpressure(context.Background(), "sleep 10", t.TempDir(), 100*time.Millisecond)

    if result.Success {
        t.Error("expected failure for timeout")
    }
}
```

```go
// internal/worker/loop_test.go

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
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Single task unit | Unit with 1 task, mock Claude that marks complete |
| Multi-task linear | Tasks 1 -> 2 -> 3, verify sequential execution |
| Multi-task parallel ready | Tasks 1,2,3 all independent, verify Claude sees all |
| Backpressure retry | Mock Claude completes, backpressure fails, verify retry |
| Baseline fix loop | All tasks complete, baseline fails, verify fix attempt |

### Manual Testing

- [ ] Worker creates worktree in correct location
- [ ] Worker commits each task with correct message format
- [ ] Worker runs backpressure commands from task frontmatter
- [ ] Worker retries when backpressure fails
- [ ] Worker runs baseline checks after all tasks
- [ ] Worker invokes Claude with baseline-fix prompt when needed
- [ ] Worker creates PR with correct branch name
- [ ] Worker cleans up worktree on completion
- [ ] Worker emits events at all expected transitions

## Design Decisions

### Why Agent-Driven Task Selection?

Presenting all ready tasks and letting Claude choose provides several benefits:
1. Claude can make intelligent decisions about task ordering based on context
2. Reduces orchestrator complexity - no need for priority algorithms
3. Allows Claude to batch related work naturally
4. Maintains flexibility as task dependencies become more complex

Alternative: Strict FIFO or priority-based selection by orchestrator. Rejected because it removes agent autonomy and may not choose optimal task order.

### Why --no-verify for Task Commits?

Pre-commit hooks (formatting, linting) can be slow and may fail during iteration when code is incomplete. Running hooks on every task commit would:
1. Slow down the iteration cycle
2. Fail unnecessarily on intermediate states
3. Duplicate work that baseline checks will do anyway

The baseline checks phase ensures code quality before PR creation.

### Why Per-Task Backpressure Instead of Per-Unit?

Per-task backpressure provides:
1. Earlier feedback - failures caught immediately after each task
2. More specific validation - each task can have its own relevant checks
3. Better debuggability - know exactly which task's changes broke what

Per-unit backpressure would delay all validation until the end, making failures harder to diagnose.

### Why Baseline Checks Separate from Backpressure?

Baseline checks are project-wide quality gates (fmt, lint, typecheck) while backpressure is task-specific validation. Keeping them separate:
1. Allows tasks to have focused backpressure commands
2. Ensures consistent quality gates across all tasks
3. Catches cross-task issues that individual backpressure might miss

## Future Enhancements

1. Parallel task execution within a unit (when multiple ready tasks have no shared files)
2. Task-level timeouts with configurable limits
3. Checkpoint/resume within a unit (not just between units)
4. Custom baseline checks per-unit via frontmatter
5. Streaming Claude output to event bus for real-time display

## References

- [PRD Section 4.3: Worker Flow](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [PRD Section 5.1: Task Execution Prompt](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [PRD Section 5.2: Claude CLI Invocation](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [PRD Section 5.5: Baseline Fix Prompt](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
