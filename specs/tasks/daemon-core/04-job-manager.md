---
task: 4
status: complete
backpressure: "go test ./internal/daemon/... -run TestJobManager"
depends_on: [3]
---

# Job Manager Core

**Parent spec**: `specs/DAEMON-CORE.md`
**Task**: #4 of 7 in implementation plan

## Objective

Implement the JobManager for creating, tracking, and managing the lifecycle of concurrent orchestrator instances.

## Dependencies

### Task Dependencies (within this unit)
- Task #3 (ManagedJob and JobConfig types)

### Package Dependencies
- `sync` - for RWMutex
- `context` - for cancellation
- `github.com/oklog/ulid/v2` - for job ID generation
- `github.com/charlotte/internal/daemon/db` - for database operations
- `github.com/charlotte/internal/orchestrator` - for Orchestrator creation
- `github.com/charlotte/internal/events` - for event bus

## Deliverables

### Files to Create/Modify

```
internal/daemon/
└── job_manager.go    # CREATE: JobManager implementation
```

### Types to Implement

```go
// JobManager tracks and coordinates active jobs.
type JobManager struct {
    db      *db.DB
    maxJobs int

    mu      sync.RWMutex
    jobs    map[string]*ManagedJob

    eventBus *events.Bus  // Global daemon event bus
}
```

### Functions to Implement

```go
// NewJobManager creates a new job manager.
func NewJobManager(database *db.DB, maxJobs int) *JobManager {
    // Initialize with empty job map
    // Create global event bus for daemon-level events
}

// Start creates and starts a new job, returning the job ID.
func (jm *JobManager) Start(ctx context.Context, cfg JobConfig) (string, error) {
    // 1. Lock for write
    // 2. Enforce capacity limit (return error if at max)
    // 3. Generate unique job ID using ULID
    // 4. Create run record in SQLite (status='running')
    // 5. Create isolated event bus for this job
    // 6. Create orchestrator with job-specific config
    // 7. Register ManagedJob in map
    // 8. Start orchestrator in goroutine with cleanup on completion
    // 9. Return job ID
}

// Stop cancels a running job.
func (jm *JobManager) Stop(jobID string) error {
    // Get job from map
    // Call Cancel function
    // Update status in database
}

// StopAll cancels all running jobs.
func (jm *JobManager) StopAll() {
    // Iterate all jobs and cancel
    // Used during daemon shutdown
}

// Get returns a managed job by ID.
func (jm *JobManager) Get(jobID string) (*ManagedJob, bool) {
    // Read lock
    // Return from map
}

// List returns all active job IDs.
func (jm *JobManager) List() []string {
    // Read lock
    // Return slice of job IDs
}

// ActiveCount returns the number of currently running jobs.
func (jm *JobManager) ActiveCount() int {
    // Read lock
    // Return len(jobs)
}

// cleanup removes a completed job from tracking.
// Called when orchestrator goroutine exits.
func (jm *JobManager) cleanup(jobID string) {
    // Write lock
    // Remove from map
    // Update database status
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestJobManager
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestJobManager_Start` | Returns valid ULID job ID |
| `TestJobManager_Start` | Job appears in List() |
| `TestJobManager_Start_MaxJobs` | Returns error when at capacity |
| `TestJobManager_Stop` | Cancels running job |
| `TestJobManager_Stop_NotFound` | Returns error for invalid ID |
| `TestJobManager_Get` | Returns job for valid ID |
| `TestJobManager_Get_NotFound` | Returns false for invalid ID |
| `TestJobManager_StopAll` | Cancels all running jobs |
| `TestJobManager_Cleanup` | Removes job from tracking after completion |

### Test Fixtures

```go
func TestJobManager_Start(t *testing.T) {
    db := setupTestDB(t)
    jm := NewJobManager(db, 10)

    cfg := JobConfig{
        RepoPath:     "/tmp/repo",
        TasksDir:     "/tmp/tasks",
        TargetBranch: "main",
    }

    jobID, err := jm.Start(context.Background(), cfg)
    require.NoError(t, err)
    require.NotEmpty(t, jobID)

    // Verify ULID format (26 characters)
    assert.Len(t, jobID, 26)

    // Verify in list
    jobs := jm.List()
    assert.Contains(t, jobs, jobID)
}

func TestJobManager_Start_MaxJobs(t *testing.T) {
    db := setupTestDB(t)
    jm := NewJobManager(db, 2) // Only allow 2 jobs

    cfg := JobConfig{
        RepoPath:     "/tmp/repo",
        TasksDir:     "/tmp/tasks",
        TargetBranch: "main",
    }

    // Start 2 jobs successfully
    _, err := jm.Start(context.Background(), cfg)
    require.NoError(t, err)
    _, err = jm.Start(context.Background(), cfg)
    require.NoError(t, err)

    // Third job should fail
    _, err = jm.Start(context.Background(), cfg)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "max jobs")
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `ulid.Make().String()` for sortable, unique job IDs
- Orchestrator creation may be mocked in tests
- Job cleanup must be called even if orchestrator fails
- Use defer in goroutine to ensure cleanup runs

## NOT In Scope

- Event subscription and streaming (Task #5)
- Job resume logic (Task #6)
- gRPC server integration (handled by DAEMON-GRPC)
