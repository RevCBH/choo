---
task: 1
status: pending
backpressure: "go build ./internal/worker/..."
depends_on: []
---

# Worker Types and Configuration

**Parent spec**: `/specs/WORKER.md`
**Task**: #1 of 8 in implementation plan

## Objective

Define the core types and configuration structures for the Worker package.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides `Unit`, `Task`, `UnitStatus`, `TaskStatus`
- EVENTS - provides `Bus`, `Event`, `EventType`
- GIT - provides `WorktreeManager`
- GITHUB - provides `PRClient`
- CLAUDE - provides `Client`

### Task Dependencies (within this unit)
- None (first task)

### Package Dependencies
- Standard library only for this task

## Deliverables

### Files to Create

```
internal/worker/
├── worker.go       # Worker struct and WorkerConfig
├── pool.go         # Pool struct and PoolStats (types only)
└── loop.go         # LoopState and LoopPhase types
```

### Types to Implement

```go
// internal/worker/worker.go

// Worker executes a single unit in an isolated worktree
type Worker struct {
    unit         *discovery.Unit
    config       WorkerConfig
    events       *events.Bus
    git          *git.WorktreeManager
    github       *github.PRClient
    claude       *claude.Client
    worktreePath string
    branch       string
    currentTask  *discovery.Task
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
    RepoRoot            string
    TargetBranch        string
    WorktreeBase        string
    BaselineChecks      []BaselineCheck
    MaxClaudeRetries    int
    MaxBaselineRetries  int
    BackpressureTimeout time.Duration
    BaselineTimeout     time.Duration
    NoPR                bool
}

// BaselineCheck represents a single baseline validation command
type BaselineCheck struct {
    Name    string
    Command string
    Pattern string
}

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

// Pool manages a collection of workers executing units in parallel
type Pool struct {
    maxWorkers int
    config     WorkerConfig
    events     *events.Bus
    git        *git.WorktreeManager
    github     *github.PRClient
    claude     *claude.Client
    workers    map[string]*Worker
    mu         sync.Mutex
    wg         sync.WaitGroup
}

// PoolStats holds current pool statistics
type PoolStats struct {
    ActiveWorkers  int
    CompletedUnits int
    FailedUnits    int
    TotalTasks     int
    CompletedTasks int
}
```

```go
// internal/worker/loop.go

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
```

### Functions to Implement

```go
// internal/worker/worker.go

// NewWorker creates a worker for executing a unit
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) *Worker {
    return &Worker{
        unit:   unit,
        config: cfg,
        events: deps.Events,
        git:    deps.Git,
        github: deps.GitHub,
        claude: deps.Claude,
    }
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/worker/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| Types are exported | `Worker`, `WorkerConfig`, `Pool`, `PoolStats`, `LoopState`, `LoopPhase` accessible |
| Constants defined | All `LoopPhase` constants compile |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Import discovery, events, git, github, and claude packages
- Use placeholder interfaces if dependency packages aren't implemented yet
- WorkerConfig should have sensible zero-value defaults where possible

## NOT In Scope

- Worker.Run() implementation (task #6)
- Pool methods (task #7)
- Loop execution logic (task #5)
