---
task: 3
status: pending
backpressure: "go build ./internal/daemon/..."
depends_on: []
---

# Job Manager Types

**Parent spec**: `specs/DAEMON-CORE.md`
**Task**: #3 of 7 in implementation plan

## Objective

Define the ManagedJob and JobConfig types that represent active jobs and job creation parameters.

## Dependencies

### Task Dependencies (within this unit)
- None (foundational types)

### Package Dependencies
- `context` - for cancellation
- `time` - for timestamps
- `github.com/charlotte/internal/orchestrator` - for Orchestrator type
- `github.com/charlotte/internal/events` - for event bus

## Deliverables

### Files to Create/Modify

```
internal/daemon/
└── job_types.go    # CREATE: Job-related type definitions
```

### Types to Implement

```go
// ManagedJob represents an active job with its orchestrator.
// It tracks the running orchestrator instance and provides
// cancellation and event access.
type ManagedJob struct {
    ID           string
    Orchestrator *orchestrator.Orchestrator
    Cancel       context.CancelFunc
    Events       *events.Bus
    StartedAt    time.Time
    Config       JobConfig
}

// JobConfig contains parameters for starting a new job.
type JobConfig struct {
    RepoPath      string       // Absolute path to git repository
    TasksDir      string       // Directory containing task definitions
    TargetBranch  string       // Base branch for PRs (e.g., "main")
    FeatureBranch string       // Optional: for feature mode
    DryRun        bool         // If true, don't create PRs or merge
    Concurrency   int          // Max parallel units (0 = default)
}

// JobState represents a snapshot of job state for status queries.
type JobState struct {
    ID            string
    Status        string       // "running", "completed", "failed", "cancelled"
    StartedAt     time.Time
    CompletedAt   *time.Time
    Error         *string
    UnitsTotal    int
    UnitsComplete int
    UnitsFailed   int
}
```

### Functions to Implement

```go
// Validate checks the JobConfig for required fields.
func (c *JobConfig) Validate() error {
    // RepoPath must be non-empty and absolute
    // TasksDir must be non-empty
    // TargetBranch must be non-empty
}

// IsRunning returns true if the job is still executing.
func (j *ManagedJob) IsRunning() bool {
    // Check if orchestrator is still running
    // This may involve checking context cancellation
}

// State returns a snapshot of the job's current state.
func (j *ManagedJob) State() JobState {
    // Build state from orchestrator status
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/daemon/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| `JobConfig{}.Validate()` | Returns error for empty RepoPath |
| `JobConfig{RepoPath: "/repo"}.Validate()` | Returns error for empty TasksDir |
| `JobConfig{RepoPath: "/repo", TasksDir: "/tasks"}.Validate()` | Returns error for empty TargetBranch |
| Valid JobConfig | Validate returns nil |
| `ManagedJob.State()` | Returns JobState with correct ID |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- ManagedJob holds a reference to the orchestrator, not ownership
- Cancel function must be safe to call multiple times
- Events bus is isolated per job to prevent cross-contamination
- JobState is a value type for safe concurrent access

## NOT In Scope

- JobManager implementation (Task #4)
- Event subscription logic (Task #5)
- Orchestrator creation (handled in Task #4)
