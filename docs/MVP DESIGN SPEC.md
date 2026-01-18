# Ralph Orchestrator - Product Design Specification

## Document Info

| Field   | Value      |
| ------- | ---------- |
| Status  | Draft      |
| Author  | Bennett    |
| Created | 2026-01-18 |
| Target  | MVP        |

---

## 1. Overview

### 1.1 Problem Statement

The current `ralph.sh` script executes development tasks sequentially within a
single working directory. When a feature requires multiple independent units of
work (e.g., `app-shell`, `deck-list`, `config`), they must be executed serially
even when no dependencies exist between them. This wastes time and underutilizes
available parallelism.

Additionally, the current workflow requires manual PR creation, review
monitoring, and merge coordination.

### 1.2 Solution

Ralph Orchestrator is a Go application that:

1. Discovers units of work from the existing `specs/tasks/` directory structure
2. Builds a dependency graph and identifies parallelizable units
3. Creates isolated git worktrees for parallel execution
4. Runs the Ralph loop (ported from bash to Go) in each worktree
5. Manages the full PR lifecycle: create → review polling → feedback addressing
   → merge

### 1.3 Design Principles

1. **File-based state**: All state lives in frontmatter of existing spec files.
   No external database.
2. **Shell out to Claude CLI**: Use `claude` binary with OAuth, not API keys.
3. **Event-driven core**: Internal event bus enables future UIs (TUI, MCP, web)
   without architectural changes.
4. **Preserve existing semantics**: Same prompts, same backpressure model, same
   file structure as `ralph.sh`.
5. **Graceful degradation**: Single-unit mode behaves identically to `ralph.sh`.

### 1.4 Non-Goals (MVP)

- MCP server mode
- Interactive TUI (bubbletea)
- Web dashboard
- Cross-repository orchestration
- Custom Claude model selection

---

## 2. Architecture

### 2.1 Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              CLI (cobra)                                │
│                         choo <command>                            │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                             Orchestrator                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐   │
│  │  Discovery  │  │  Scheduler  │  │ Worker Pool │  │   PR Manager │   │
│  │             │  │             │  │             │  │              │   │
│  │ - Find units│  │ - Dep graph │  │ - Worktrees │  │ - Create PR  │   │
│  │ - Parse FM  │  │ - Ready Q   │  │ - Run loop  │  │ - Poll review│   │
│  │ - Validate  │  │ - Dispatch  │  │ - Events    │  │ - Feedback   │   │
│  └─────────────┘  └─────────────┘  └─────────────┘  └──────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                         Event Bus                                │   │
│  │                    chan Event (buffered)                         │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                    │                                    │
│         ┌──────────────────────────┼──────────────────────────┐        │
│         ▼                          ▼                          ▼        │
│  ┌─────────────┐           ┌─────────────┐           ┌─────────────┐   │
│  │ Log Emitter │           │State Writer │           │ (Future UI) │   │
│  └─────────────┘           └─────────────┘           └─────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
             ┌───────────┐   ┌───────────┐   ┌───────────┐
             │    Git    │   │  Claude   │   │  GitHub   │
             │ Worktrees │   │    CLI    │   │    API    │
             └───────────┘   └───────────┘   └───────────┘
```

### 2.2 Package Structure

```
choo/
├── cmd/
│   └── choo/
│       └── main.go                 # Entry point
│
├── internal/
│   ├── discovery/
│   │   ├── discovery.go            # Find units, parse structure
│   │   ├── frontmatter.go          # YAML frontmatter parser
│   │   └── validation.go           # Validate unit/task structure
│   │
│   ├── scheduler/
│   │   ├── graph.go                # Dependency graph, topological sort
│   │   ├── scheduler.go            # Ready queue, dispatch logic
│   │   └── state.go                # Track unit states
│   │
│   ├── worker/
│   │   ├── worker.go               # Single unit execution
│   │   ├── loop.go                 # Ralph loop (ported from bash)
│   │   ├── prompt.go               # Prompt construction
│   │   └── backpressure.go         # Validation runner
│   │
│   ├── claude/
│   │   ├── claude.go               # Claude CLI subprocess interface
│   │   └── output.go               # Output parsing
│   │
│   ├── git/
│   │   ├── worktree.go             # Worktree create/cleanup
│   │   ├── branch.go               # Branch operations
│   │   ├── commit.go               # Commit, push
│   │   └── merge.go                # Rebase, conflict detection
│   │
│   ├── github/
│   │   ├── client.go               # GitHub API client
│   │   ├── pr.go                   # PR create, update, merge
│   │   └── review.go               # Review status polling (emoji state machine)
│   │
│   ├── events/
│   │   ├── bus.go                  # Event bus implementation
│   │   ├── types.go                # Event type definitions
│   │   └── handlers.go             # Built-in handlers (logging, state)
│   │
│   └── config/
│       └── config.go               # Configuration loading
│
├── go.mod
├── go.sum
└── README.md
```

---

## 3. Data Model

### 3.1 Directory Structure (Existing - Unchanged)

```
specs/
├── README.md
├── APP-SHELL.md                    # Unit design spec
├── DECK-LIST.md
└── tasks/
    ├── app-shell/
    │   ├── IMPLEMENTATION_PLAN.md  # Unit manifest + orchestrator state
    │   ├── 01-nav-types.md         # Task spec with frontmatter
    │   ├── 02-navigation.md
    │   ├── 03-app-shell.md
    │   └── 04-route-setup.md
    └── deck-list/
        ├── IMPLEMENTATION_PLAN.md
        ├── 01-deck-card.md
        └── 02-deck-grid.md
```

### 3.2 IMPLEMENTATION_PLAN.md Frontmatter

The orchestrator extends the existing format with runtime state fields:

```yaml
---
# === Author-provided (required) ===
unit: app-shell
depends_on: [project-setup, config, model-download]  # Other unit IDs

# === Orchestrator-managed (auto-populated) ===
orch_status: in_progress    # pending | in_progress | pr_open | in_review | merging | complete | failed
orch_branch: ralph/app-shell-a1b2c3
orch_worktree: /tmp/ralph-worktrees/app-shell
orch_pr_number: 42
orch_started_at: 2025-01-18T10:30:00Z
orch_completed_at: null
---

# APP-SHELL Implementation Plan
...
```

### 3.3 Task Frontmatter (Existing - Unchanged)

```yaml
---
task: 1
status: complete        # pending | in_progress | complete | failed
backpressure: "cd koe && pnpm typecheck"
depends_on: []          # Task numbers within this unit
---

# Navigation Types
...
```

### 3.4 Go Types

```go
// internal/discovery/types.go

type Unit struct {
    // Parsed from directory/files
    ID       string   // directory name, e.g., "app-shell"
    Path     string   // absolute path to unit directory
    SpecPath string   // path to parent spec (e.g., specs/APP-SHELL.md)
    
    // Parsed from IMPLEMENTATION_PLAN.md frontmatter
    DependsOn []string // other unit IDs
    
    // Orchestrator state (from frontmatter, updated at runtime)
    Status      UnitStatus
    Branch      string
    Worktree    string
    PRNumber    int
    StartedAt   *time.Time
    CompletedAt *time.Time
    
    // Parsed from task files
    Tasks []*Task
}

type UnitStatus string

const (
    UnitStatusPending    UnitStatus = "pending"
    UnitStatusInProgress UnitStatus = "in_progress"
    UnitStatusPROpen     UnitStatus = "pr_open"
    UnitStatusInReview   UnitStatus = "in_review"
    UnitStatusMerging    UnitStatus = "merging"
    UnitStatusComplete   UnitStatus = "complete"
    UnitStatusFailed     UnitStatus = "failed"
)

type Task struct {
    // Parsed from frontmatter
    Number       int
    Status       TaskStatus
    Backpressure string
    DependsOn    []int // task numbers within unit
    
    // Parsed from file
    FilePath string // relative to unit dir, e.g., "01-nav-types.md"
    Title    string // from first H1
    Content  string // full markdown content
}

type TaskStatus string

const (
    TaskStatusPending    TaskStatus = "pending"
    TaskStatusInProgress TaskStatus = "in_progress"
    TaskStatusComplete   TaskStatus = "complete"
    TaskStatusFailed     TaskStatus = "failed"
)
```

---

## 4. Core Workflows

### 4.1 Discovery Flow

```
Input: specs/tasks/ directory path

1. List subdirectories of specs/tasks/
2. For each subdirectory:
   a. Check for IMPLEMENTATION_PLAN.md (skip if missing)
   b. Check for [0-9][0-9]-*.md files (skip if none)
   c. Parse IMPLEMENTATION_PLAN.md frontmatter
   d. Parse each task file's frontmatter
   e. Validate:
      - Unit has `unit` field
      - All tasks have `task`, `status`, `backpressure` fields
      - Task numbers are sequential starting from 1
      - Task `depends_on` references valid task numbers
   f. Build Unit struct

3. Validate cross-unit dependencies:
   - All `depends_on` references exist
   - No circular dependencies

Output: []*Unit with dependency graph
```

### 4.2 Scheduler Flow

```
Input: []*Unit, max_parallelism int

State:
  - pending:    units waiting on dependencies
  - ready:      units with satisfied deps, waiting for worker
  - running:    units currently executing
  - pr_phase:   units in PR/review/merge cycle
  - complete:   done
  - failed:     error state

Loop:
  1. Move units from pending → ready if all deps complete
  2. While len(running) + len(pr_phase) < max_parallelism AND len(ready) > 0:
     a. Pop unit from ready queue
     b. Dispatch to worker pool
     c. Move to running
  3. Wait for events
  4. On UnitPROpen: move running → pr_phase
  5. On UnitComplete: move → complete, re-evaluate pending
  6. On UnitFailed: move → failed, optionally fail dependents
  7. Repeat until all complete or fatal error
```

### 4.3 Worker Flow (Single Unit)

```
Input: Unit, repo_root string

Phase 1: Setup
  1. Create worktree: git worktree add <path> -b <branch> <target_branch>
  2. Update unit frontmatter: orch_status=in_progress, orch_branch, orch_worktree
  3. Emit UnitStarted event

Phase 2: Task Loop (Ralph loop)
  while true:
    1. Find all "ready" tasks (pending tasks whose depends_on are all complete)
    2. If none found:
       - If all complete → break to Phase 2.5 (Baseline Checks)
       - If some failed/blocked → emit UnitFailed, return error
    3. Build prompt with ALL ready tasks (see §5.1)
       - Claude chooses which task to work on
    4. Emit TaskStarted event (after Claude picks)
    5. Invoke Claude CLI (see §5.2)
    6. Emit TaskClaudeDone event
    7. Check task status in frontmatter:
       - If status != complete → retry (back to step 5)
       - If status == complete → continue
    8. Run backpressure command for completed task
    9. If backpressure fails:
        - Revert status to in_progress
        - Retry (back to step 5)
    10. Commit changes
    11. Emit TaskCompleted event
    12. Continue to next iteration (find new ready tasks)

Phase 2.5: Baseline Checks (end of unit)
  1. Run baseline checks (fmt, vet/clippy, typecheck)
  2. If checks fail:
     a. Invoke Claude CLI with baseline-fix prompt
     b. Commit fixes
     c. Re-run baseline checks
     d. If still failing after 3 attempts, emit UnitFailed
  3. Continue to Phase 3

Phase 3: PR Lifecycle
  1. Push branch
  2. Create PR via GitHub API
  3. Update unit frontmatter: orch_status=pr_open, orch_pr_number
  4. Emit PRCreated event
  5. Enter review loop (see §4.4)

Phase 4: Cleanup
  1. Remove worktree: git worktree remove <path>
  2. Delete local branch
  3. Emit UnitCompleted event
```

### 4.4 PR Review Loop

```
Input: Unit with open PR

Poll interval: 30 seconds

Loop:
  1. Fetch PR reactions via GitHub API
  2. Check for 👀 (eyes) reaction:
     - Present → review in progress, continue polling
  3. Check for 👍 (thumbs up) reaction:
     - Present → approved, proceed to merge
  4. Check for review comments:
     - If comments exist AND no 👀 AND no 👍:
       - Changes requested, address feedback
  5. Sleep poll interval
  6. Repeat

On approval:
  1. Update unit frontmatter: orch_status=merging
  2. Acquire merge lock (see §4.5)
  3. Fetch latest target branch
  4. Rebase if needed
  5. If conflicts:
     a. Invoke Claude to resolve
     b. Push with --force-with-lease
  6. Merge PR via GitHub API
  7. Release merge lock
  8. Update unit frontmatter: orch_status=complete
  9. Emit PRMerged, UnitCompleted events

On feedback:
  1. Fetch PR comments via GitHub API
  2. Build feedback prompt (see §5.3)
  3. Invoke Claude CLI
  4. Commit and push
  5. Emit PRFeedbackAddressed event
  6. Return to poll loop
```

### 4.5 Merge Serialization

```go
// Simple mutex-based FCFS

var mergeMutex sync.Mutex

func (w *Worker) mergeWithLock(pr *PullRequest) error {
    w.events.Emit(MergeQueueJoined, w.unit)
    
    mergeMutex.Lock()
    defer mergeMutex.Unlock()
    
    // Rebase onto fresh target
    if err := w.git.Fetch(); err != nil {
        return err
    }
    
    hasConflicts, err := w.git.Rebase(w.targetBranch)
    if err != nil {
        return err
    }
    
    if hasConflicts {
        w.events.Emit(MergeConflict, w.unit)
        if err := w.resolveConflictsWithClaude(); err != nil {
            return err
        }
        if err := w.git.Push("--force-with-lease"); err != nil {
            return err
        }
    }
    
    if err := w.github.MergePR(pr); err != nil {
        return err
    }
    
    w.events.Emit(MergeCompleted, w.unit)
    return nil
}
```

---

## 5. Claude CLI Interface

### 5.1 Task Execution Prompt

Updated to present all ready tasks and let Claude choose:

```go
func (w *Worker) buildTaskPrompt(readyTasks []*Task) string {
    var taskList strings.Builder
    for _, t := range readyTasks {
        fmt.Fprintf(&taskList, "### Task #%d: %s\n", t.Number, t.Title)
        fmt.Fprintf(&taskList, "- File: %s\n", t.FilePath)
        fmt.Fprintf(&taskList, "- Backpressure: `%s`\n\n", t.Backpressure)
    }

    return fmt.Sprintf(`You are executing a Ralph task. Follow these instructions exactly.

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
}
```

When only one task is ready, the prompt simplifies to that single task.

### 5.2 Claude CLI Invocation

```go
// internal/claude/claude.go

type Client struct {
    Command string // default: "claude"
    Logger  io.Writer
}

type InvokeOptions struct {
    WorkingDir string
    Prompt     string
    MaxTurns   int // 0 = unlimited (claude's default)
}

func (c *Client) Invoke(ctx context.Context, opts InvokeOptions) error {
    args := []string{
        "--dangerously-skip-permissions",
        "-p", opts.Prompt,
    }
    
    if opts.MaxTurns > 0 {
        args = append(args, "--max-turns", strconv.Itoa(opts.MaxTurns))
    }
    
    cmd := exec.CommandContext(ctx, c.Command, args...)
    cmd.Dir = opts.WorkingDir
    cmd.Stdout = c.Logger
    cmd.Stderr = c.Logger
    
    return cmd.Run()
}
```

### 5.3 PR Feedback Prompt

```go
func (w *Worker) buildFeedbackPrompt(comments []PRComment) string {
    var commentBlock strings.Builder
    for _, c := range comments {
        fmt.Fprintf(&commentBlock, "### %s\n", c.Path)
        if c.Line > 0 {
            fmt.Fprintf(&commentBlock, "Line %d:\n", c.Line)
        }
        fmt.Fprintf(&commentBlock, "%s\n\n", c.Body)
    }
    
    return fmt.Sprintf(`You are addressing PR review feedback. Follow these instructions exactly.

## PR Feedback

The following review comments were left on PR #%d:

%s

## Instructions

1. Read each comment carefully
2. Make the requested changes
3. Run validation: %s
4. Ensure all checks pass
5. Do NOT commit - the orchestrator will commit for you

## Critical
- Address ALL comments
- Do not make unrelated changes
- Stay focused on the feedback
`,
        w.unit.PRNumber,
        commentBlock.String(),
        w.fullValidationCommand(),
    )
}
```

### 5.4 Conflict Resolution Prompt

```go
func (w *Worker) buildConflictPrompt(conflicts []string) string {
    return fmt.Sprintf(`You are resolving git merge conflicts. Follow these instructions exactly.

## Conflicts

The following files have conflicts after rebasing onto %s:

%s

## Instructions

1. Open each conflicted file
2. Resolve conflicts by keeping the correct code (usually combining both changes logically)
3. Remove all conflict markers (<<<<<<<, =======, >>>>>>>)
4. Run validation: %s
5. Ensure the code compiles and tests pass
6. Stage resolved files with `+"`git add <file>`"+`
7. Complete rebase with `+"`git rebase --continue`"+`

## Critical
- Preserve functionality from BOTH branches where possible
- If uncertain, prefer the incoming changes (your branch)
- Do not leave any conflict markers
`,
        w.targetBranch,
        strings.Join(conflicts, "\n"),
        w.fullValidationCommand(),
    )
}
```

### 5.5 Baseline Fix Prompt

```go
func (w *Worker) buildBaselineFixPrompt(checkOutput string) string {
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
        w.baselineChecksCommand(),
    )
}
```

---

## 6. Event System

### 6.1 Event Types

```go
// internal/events/types.go

type Event struct {
    Time    time.Time   `json:"time"`
    Type    EventType   `json:"type"`
    Unit    string      `json:"unit,omitempty"`
    Task    *int        `json:"task,omitempty"`
    PR      *int        `json:"pr,omitempty"`
    Payload any         `json:"payload,omitempty"`
    Error   string      `json:"error,omitempty"`
}

type EventType string

const (
    // Orchestrator lifecycle
    OrchStarted   EventType = "orch.started"
    OrchCompleted EventType = "orch.completed"
    OrchFailed    EventType = "orch.failed"
    
    // Unit lifecycle
    UnitQueued    EventType = "unit.queued"
    UnitStarted   EventType = "unit.started"
    UnitCompleted EventType = "unit.completed"
    UnitFailed    EventType = "unit.failed"
    
    // Task lifecycle
    TaskStarted       EventType = "task.started"
    TaskClaudeInvoke  EventType = "task.claude.invoke"
    TaskClaudeDone    EventType = "task.claude.done"
    TaskBackpressure  EventType = "task.backpressure"
    TaskValidationOK  EventType = "task.validation.ok"
    TaskValidationFail EventType = "task.validation.fail"
    TaskCommitted     EventType = "task.committed"
    TaskCompleted     EventType = "task.completed"
    TaskRetry         EventType = "task.retry"
    TaskFailed        EventType = "task.failed"
    
    // PR lifecycle
    PRCreated          EventType = "pr.created"
    PRReviewPending    EventType = "pr.review.pending"
    PRReviewInProgress EventType = "pr.review.in_progress"
    PRReviewApproved   EventType = "pr.review.approved"
    PRFeedbackReceived EventType = "pr.feedback.received"
    PRFeedbackAddressed EventType = "pr.feedback.addressed"
    PRMergeQueued      EventType = "pr.merge.queued"
    PRConflict         EventType = "pr.conflict"
    PRMerged           EventType = "pr.merged"
    PRFailed           EventType = "pr.failed"
    
    // Git operations
    WorktreeCreated EventType = "worktree.created"
    WorktreeRemoved EventType = "worktree.removed"
    BranchPushed    EventType = "branch.pushed"
)
```

### 6.2 Event Bus

```go
// internal/events/bus.go

type Handler func(Event)

type Bus struct {
    handlers []Handler
    ch       chan Event
    done     chan struct{}
}

func NewBus(bufferSize int) *Bus {
    b := &Bus{
        ch:   make(chan Event, bufferSize),
        done: make(chan struct{}),
    }
    go b.loop()
    return b
}

func (b *Bus) Subscribe(h Handler) {
    b.handlers = append(b.handlers, h)
}

func (b *Bus) Emit(e Event) {
    e.Time = time.Now()
    select {
    case b.ch <- e:
    default:
        // Buffer full, log warning but don't block
        log.Printf("WARN: event buffer full, dropping %s", e.Type)
    }
}

func (b *Bus) loop() {
    for {
        select {
        case e := <-b.ch:
            for _, h := range b.handlers {
                h(e)
            }
        case <-b.done:
            return
        }
    }
}

func (b *Bus) Close() {
    close(b.done)
}
```

### 6.3 Built-in Handlers

```go
// Logging handler (MVP)
func LogHandler(e Event) {
    taskStr := ""
    if e.Task != nil {
        taskStr = fmt.Sprintf(" task=#%d", *e.Task)
    }
    prStr := ""
    if e.PR != nil {
        prStr = fmt.Sprintf(" pr=#%d", *e.PR)
    }
    
    log.Printf("[%s] %s%s%s", e.Type, e.Unit, taskStr, prStr)
}

// State persistence handler
func StateHandler(units map[string]*Unit) Handler {
    return func(e Event) {
        unit, ok := units[e.Unit]
        if !ok {
            return
        }
        
        // Update in-memory state based on event
        switch e.Type {
        case UnitStarted:
            unit.Status = UnitStatusInProgress
        case UnitCompleted:
            unit.Status = UnitStatusComplete
        // ... etc
        }
        
        // Persist to frontmatter
        if err := writeUnitFrontmatter(unit); err != nil {
            log.Printf("ERROR: failed to persist state: %v", err)
        }
    }
}
```

---

## 7. GitHub Integration

### 7.1 Authentication

Use `gh` CLI token or `GITHUB_TOKEN` environment variable:

```go
func getGitHubToken() (string, error) {
    // 1. Check environment
    if token := os.Getenv("GITHUB_TOKEN"); token != "" {
        return token, nil
    }
    
    // 2. Try gh CLI
    cmd := exec.Command("gh", "auth", "token")
    out, err := cmd.Output()
    if err == nil {
        return strings.TrimSpace(string(out)), nil
    }
    
    return "", fmt.Errorf("no GitHub token found: set GITHUB_TOKEN or run 'gh auth login'")
}
```

### 7.2 PR Review Status (Emoji State Machine)

```go
type ReviewStatus string

const (
    ReviewPending    ReviewStatus = "pending"     // No reactions
    ReviewInProgress ReviewStatus = "in_progress" // 👀 present
    ReviewApproved   ReviewStatus = "approved"    // 👍 present
    ReviewChanges    ReviewStatus = "changes"     // Comments but no 👀/👍
)

func (c *GitHubClient) GetReviewStatus(prNumber int) (ReviewStatus, error) {
    // GET /repos/{owner}/{repo}/issues/{issue_number}/reactions
    reactions, err := c.getReactions(prNumber)
    if err != nil {
        return "", err
    }
    
    hasEyes := false
    hasThumbsUp := false
    
    for _, r := range reactions {
        switch r.Content {
        case "eyes":
            hasEyes = true
        case "+1":
            hasThumbsUp = true
        }
    }
    
    if hasThumbsUp {
        return ReviewApproved, nil
    }
    if hasEyes {
        return ReviewInProgress, nil
    }
    
    // Check for comments
    comments, err := c.getPRComments(prNumber)
    if err != nil {
        return "", err
    }
    
    if len(comments) > 0 {
        return ReviewChanges, nil
    }
    
    return ReviewPending, nil
}
```

---

## 8. CLI Interface

### 8.1 Commands

```
choo - Parallel development task orchestrator

Usage:
  choo [command]

Commands:
  run       Execute units in parallel
  status    Show current progress
  resume    Resume from last state
  cleanup   Remove worktrees and reset state
  version   Print version information

Flags:
  -h, --help      Show help
  -v, --verbose   Verbose output
```

### 8.2 Run Command

```
choo run [tasks-dir]

Execute development units in parallel with PR lifecycle management.

Arguments:
  tasks-dir    Path to specs/tasks/ directory (default: ./specs/tasks/)

Flags:
  -p, --parallelism int    Max concurrent units (default: 4)
  -t, --target string      Target branch for PRs (default: main)
  -n, --dry-run            Show what would be done
      --no-pr              Skip PR creation, just execute tasks
      --unit string        Run only specified unit
      --skip-review        Auto-merge without waiting for review (dangerous)

Examples:
  # Run all units with default parallelism
  choo run specs/tasks/

  # Run with 2 parallel workers
  choo run -p 2 specs/tasks/

  # Run single unit (like ralph.sh)
  choo run --unit app-shell specs/tasks/

  # Dry run to see execution plan
  choo run -n specs/tasks/
```

### 8.3 Status Output

```
$ choo status specs/tasks/

═══════════════════════════════════════════════════════════════
Ralph Orchestrator Status
Target: main | Parallelism: 4
═══════════════════════════════════════════════════════════════

 [app-shell] ████████████████████ 100% (complete)
   ✓ #1  01-nav-types.md
   ✓ #2  02-navigation.md
   ✓ #3  03-app-shell.md
   ✓ #4  04-route-setup.md
   PR #42 merged

 [deck-list] ████████░░░░░░░░░░░░  40% (in_progress)
   ✓ #1  01-deck-card.md
   ● #2  02-deck-grid.md         ← executing
   ○ #3  03-deck-page.md

 [config] ░░░░░░░░░░░░░░░░░░░░   0% (pending)
   → blocked by: project-setup

───────────────────────────────────────────────────────────────
 Units: 3 | Complete: 1 | In Progress: 1 | Pending: 1
 Tasks: 9 | Complete: 5 | In Progress: 1 | Pending: 3
═══════════════════════════════════════════════════════════════
```

---

## 9. Configuration

### 9.1 Configuration File (Optional)

```yaml
# .choo.yaml (optional, in repo root)

target_branch: main
parallelism: 4

github:
  owner: auto        # Detected from git remote
  repo: auto         # Detected from git remote
  
worktree:
  base_path: /tmp/ralph-worktrees
  
claude:
  command: claude    # Or path to binary
  max_turns: 0       # 0 = unlimited
  
baseline_checks:
  - name: rust-fmt
    command: "cd koe/src-tauri && cargo fmt --check"
    pattern: "*.rs"
  - name: rust-clippy
    command: "cd koe/src-tauri && cargo clippy -- -D warnings"
    pattern: "*.rs"
  - name: typescript
    command: "cd koe && pnpm typecheck"
    pattern: "*.ts,*.tsx"
```

### 9.2 Environment Variables

| Variable              | Description        | Default                |
| --------------------- | ------------------ | ---------------------- |
| `GITHUB_TOKEN`        | GitHub API token   | (try `gh auth token`)  |
| `RALPH_CLAUDE_CMD`    | Claude CLI command | `claude`               |
| `RALPH_WORKTREE_BASE` | Worktree directory | `/tmp/ralph-worktrees` |
| `RALPH_LOG_LEVEL`     | Log verbosity      | `info`                 |

---

## 10. Error Handling

### 10.1 Retry Policy

| Failure                  | Retry? | Max Attempts                      | Backoff                           |
| ------------------------ | ------ | --------------------------------- | --------------------------------- |
| Claude CLI exit non-zero | Yes    | 3                                 | None                              |
| Backpressure fails       | Yes    | ∞ (until success or manual abort) | None                              |
| GitHub API 5xx           | Yes    | 5                                 | Exponential (1s, 2s, 4s, 8s, 16s) |
| GitHub API 4xx           | No     | -                                 | -                                 |
| Git push rejected        | Yes    | 3                                 | Linear (fetch, rebase, retry)     |
| Merge conflict           | Yes    | 3                                 | Invoke Claude                     |

### 10.2 Failure Modes

| Scenario                 | Behavior                                                             |
| ------------------------ | -------------------------------------------------------------------- |
| Unit fails all retries   | Mark failed, continue other units                                    |
| Unit dependency fails    | Mark dependents as blocked                                           |
| All units blocked/failed | Exit with error                                                      |
| Interrupt (SIGINT)       | Graceful shutdown: finish current Claude turn, commit progress, exit |
| Worktree left behind     | `choo cleanup` removes orphans                                 |

### 10.3 Recovery

State is persisted in frontmatter after each task completion. To resume:

```bash
# Resumes from last committed state
choo resume specs/tasks/
```

---

## 11. Future Extensibility Hooks

### 11.1 MCP Server Mode (Future)

The event bus architecture enables MCP without core changes:

```go
// Future: internal/mcp/server.go

func (s *MCPServer) tools() []mcp.Tool {
    return []mcp.Tool{
        {Name: "ralph_run", Handler: s.handleRun},
        {Name: "ralph_status", Handler: s.handleStatus},
        {Name: "ralph_pause", Handler: s.handlePause},
        {Name: "ralph_resume", Handler: s.handleResume},
    }
}

func (s *MCPServer) handleRun(params map[string]any) (any, error) {
    // Dispatch to orchestrator
    go s.orch.Run(params["path"].(string))
    return map[string]string{"status": "started"}, nil
}

// Events stream to MCP notifications
func (s *MCPServer) eventHandler(e Event) {
    s.notify("ralph/event", e)
}
```

### 11.2 TUI Mode (Future)

Bubbletea model subscribes to event bus:

```go
// Future: internal/tui/model.go

type Model struct {
    units    []*Unit
    events   chan Event
    // ... bubbletea state
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case Event:
        // Update display state from event
        return m.handleEvent(msg), nil
    }
    return m, nil
}
```

### 11.3 Web Dashboard (Future)

HTTP server with WebSocket event streaming:

```go
// Future: internal/web/server.go

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
    conn, _ := upgrader.Upgrade(w, r, nil)
    
    s.bus.Subscribe(func(e Event) {
        conn.WriteJSON(e)
    })
    
    // Keep connection open
    select {}
}
```

---

## 12. Testing Strategy

### 12.1 Unit Tests

| Package     | Key Tests                                           |
| ----------- | --------------------------------------------------- |
| `discovery` | Frontmatter parsing, validation, edge cases         |
| `scheduler` | Dependency graph, topological sort, cycle detection |
| `worker`    | Prompt construction, state machine transitions      |
| `github`    | Review status parsing, mock API responses           |

### 12.2 Integration Tests

| Scenario              | Setup                                  |
| --------------------- | -------------------------------------- |
| Single unit execution | Fixture with 2-task unit, mock Claude  |
| Parallel execution    | Fixture with 3 independent units       |
| Dependency chain      | A → B → C linear dependency            |
| PR feedback loop      | Mock GitHub API with comment injection |

### 12.3 E2E Tests

Run against real repos with `--dry-run` and `--no-pr` flags.

---

## 13. Implementation Plan

### Phase 1: Core Engine (Week 1)

| Unit      | Tasks                                             |
| --------- | ------------------------------------------------- |
| discovery | Parse frontmatter, find units, validate structure |
| scheduler | Dependency graph, ready queue, basic dispatch     |
| worker    | Port ralph loop, prompt building, backpressure    |
| claude    | CLI subprocess interface                          |
| git       | Worktree create/cleanup, branch ops, commit       |
| cli       | `run` and `status` commands                       |

### Phase 2: PR Lifecycle (Week 2)

| Unit     | Tasks                             |
| -------- | --------------------------------- |
| github   | API client, PR create/merge       |
| review   | Emoji state machine, polling loop |
| feedback | Comment fetching, feedback prompt |
| merge    | FCFS queue, conflict resolution   |

### Phase 3: Polish (Week 3)

| Unit     | Tasks                           |
| -------- | ------------------------------- |
| recovery | Resume command, orphan cleanup  |
| config   | Config file loading, validation |
| events   | Event bus, logging handler      |
| testing  | Integration tests, fixtures     |

---

## 14. Open Questions

1. **PR title/body template**: Should these be configurable, or derive from
   IMPLEMENTATION_PLAN.md?

2. **Review timeout**: Should there be a max wait time for review before
   alerting/failing?

3. **Conflict resolution limits**: After N failed conflict resolutions, should
   the unit be parked for manual intervention?

4. **Baseline checks**: Should these be configurable per-repo or hardcoded
   initially?

5. **Worktree location**: `/tmp` is ephemeral; should we default to
   `.ralph/worktrees/` in repo?

---

## 15. Appendix: Comparison with ralph.sh

| Feature               | ralph.sh | choo |
| --------------------- | -------- | ---------- |
| Single unit execution | ✓        | ✓          |
| Multi-unit sequential | ✓        | ✓          |
| Multi-unit parallel   | ✗        | ✓          |
| Git worktrees         | ✗        | ✓          |
| PR creation           | ✗        | ✓          |
| Review polling        | ✗        | ✓          |
| Feedback addressing   | ✗        | ✓          |
| Merge coordination    | ✗        | ✓          |
| Event streaming       | ✗        | ✓          |
| Configuration file    | ✗        | ✓          |
| Claude CLI (OAuth)    | ✓        | ✓          |
| File-based state      | ✓        | ✓          |
| Same prompts          | ✓        | ✓          |
