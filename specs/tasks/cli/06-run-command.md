---
task: 6
status: pending
backpressure: "go test ./internal/cli/... -run Run"
depends_on: [1, 3]
---

# Run Command

**Parent spec**: `/specs/CLI.md`
**Task**: #6 of 9 in implementation plan

## Objective

Implement the run command that executes orchestration with all supported flags.

## Dependencies

### External Specs (must be implemented)
- CONFIG - provides `Config` type
- DISCOVERY - provides `Discovery` type
- SCHEDULER - provides `Scheduler` type
- WORKER - provides `Pool` type

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `App` struct)
- Task #3 must be complete (provides: `SignalHandler`)

### Package Dependencies
- None beyond standard library and internal packages

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── run.go    # CREATE: Run command implementation
```

### Types to Implement

```go
// RunOptions holds flags for the run command
type RunOptions struct {
    Parallelism  int    // Max concurrent units (default: 4)
    TargetBranch string // Branch PRs target (default: main)
    DryRun       bool   // Show execution plan without running
    NoPR         bool   // Skip PR creation
    Unit         string // Run only specified unit (single-unit mode)
    SkipReview   bool   // Auto-merge without waiting for review
    TasksDir     string // Path to specs/tasks/ directory
}
```

### Functions to Implement

```go
// NewRunCmd creates the run command
func NewRunCmd(app *App) *cobra.Command {
    // Create command with Use: "run [tasks-dir]"
    // Add all flags: -p, -t, -n, --no-pr, --unit, --skip-review
    // Set defaults
    // Call RunOrchestrator in RunE
}

// RunOrchestrator executes the main orchestration loop
func (a *App) RunOrchestrator(ctx context.Context, opts RunOptions) error {
    // Validate options
    // Setup signal handler
    // Wire orchestrator components (calls WireOrchestrator)
    // Run discovery
    // Execute scheduler loop
    // Handle graceful shutdown
}

// Validate checks RunOptions for validity
func (opts RunOptions) Validate() error {
    // Parallelism must be > 0
    // TasksDir must not be empty
    // Return descriptive errors
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run Run -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestRunCmd_DefaultFlags` | Parallelism=4, Target=main |
| `TestRunCmd_CustomFlags` | All flags are correctly parsed |
| `TestRunOptions_Validate_Valid` | Valid options pass validation |
| `TestRunOptions_Validate_ZeroParallelism` | Returns error for parallelism=0 |
| `TestRunOptions_Validate_EmptyTasksDir` | Returns error for empty tasks dir |
| `TestRunOrchestrator_DryRun` | Dry run prints plan without executing |
| `TestRunOrchestrator_SingleUnit` | --unit flag filters to single unit |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Mock orchestrator | In-memory | Test run flow without real execution |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Flag defaults:
  - `-p, --parallelism`: 4
  - `-t, --target`: "main"
  - `-n, --dry-run`: false
  - `--no-pr`: false
  - `--unit`: "" (empty = all units)
  - `--skip-review`: false
  - `tasks-dir`: "specs/tasks"

- Dry-run mode should:
  - Load and validate configuration
  - Discover units and tasks
  - Print execution plan (which units, in what order)
  - Exit without running any tasks

- Exit codes:
  - 0: Success (all units complete)
  - 1: General error
  - 2: Invalid arguments
  - 130: Interrupted (SIGINT)

- Signal handling integration:
  - Register shutdown callback to save state
  - Cancel context on signal
  - Wait for current task to complete before exiting

## NOT In Scope

- Actual orchestration execution (task #9 wires components)
- Component creation (task #9)
- Progress display during run (future enhancement)
