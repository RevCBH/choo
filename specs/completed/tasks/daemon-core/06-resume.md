---
task: 6
status: complete
backpressure: "go test ./internal/daemon/... -run TestResume"
depends_on: [4]
---

# Job Resume Logic

**Parent spec**: `specs/DAEMON-CORE.md`
**Task**: #6 of 7 in implementation plan

## Objective

Implement job resume logic and validation for crash recovery after daemon restart.

## Dependencies

### Task Dependencies (within this unit)
- Task #4 (JobManager core implementation)

### Package Dependencies
- `os` - for filesystem checks
- `github.com/charlotte/internal/daemon/db` - for database queries
- `log` - for resume logging

## Deliverables

### Files to Create/Modify

```
internal/daemon/
└── resume.go    # CREATE: Resume logic and validation
```

### Types to Implement

```go
// ResumeResult contains the outcome of a job resume attempt.
type ResumeResult struct {
    JobID    string
    Success  bool
    Error    error
    Skipped  bool   // True if job was not resumable
    Reason   string // Human-readable explanation
}
```

### Functions to Implement

```go
// ResumeJobs queries for interrupted jobs and attempts to resume them.
// Returns results for each job attempted.
func (jm *JobManager) ResumeJobs(ctx context.Context) []ResumeResult {
    // 1. Query runs with status='running' from database
    // 2. For each run, attempt resume
    // 3. Collect results (success or failure reason)
    // 4. Return all results
}

// Resume attempts to resume a specific job from its persisted state.
func (jm *JobManager) Resume(ctx context.Context, runID string, cfg JobConfig, units []*db.UnitRecord) error {
    // 1. Validate daemon version matches
    // 2. Validate repository still exists
    // 3. Validate worktrees for in-progress units
    // 4. Create orchestrator in resume mode
    // 5. Register managed job
    // 6. Start orchestrator
}

// validateRepoExists checks if the repository path is still valid.
func validateRepoExists(repoPath string) error {
    // Check path exists
    // Check it's a directory
    // Check it contains .git
}

// validateWorktrees checks worktree validity for in-progress units.
// Returns updated unit records with failed status for invalid worktrees.
func validateWorktrees(units []*db.UnitRecord) []*db.UnitRecord {
    // For each unit with status='running' and worktree_path set
    // Check if worktree path exists
    // If not, mark unit as failed with error message
}

// markJobFailed updates a job's database status to failed.
func (jm *JobManager) markJobFailed(ctx context.Context, runID string, reason string) error {
    // Update run status in database
    // Include error message
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestResume
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestResumeJobs_NoJobs` | Returns empty slice when no running jobs |
| `TestResumeJobs_SingleJob` | Successfully resumes one interrupted job |
| `TestResumeJobs_MultipleJobs` | Resumes multiple jobs, reports each result |
| `TestResume_ValidState` | Job resumes with valid repo and worktrees |
| `TestResume_VersionMismatch` | Returns error if daemon version differs |
| `TestResume_MissingRepo` | Returns error if repository deleted |
| `TestResume_InvalidWorktree` | Marks unit as failed if worktree missing |
| `TestValidateRepoExists_Valid` | Returns nil for valid git repo |
| `TestValidateRepoExists_Missing` | Returns error for missing path |
| `TestValidateRepoExists_NotGit` | Returns error for non-git directory |
| `TestValidateWorktrees_AllValid` | Returns units unchanged |
| `TestValidateWorktrees_SomeMissing` | Updates status for missing worktrees |

### Test Fixtures

```go
func TestResumeJobs_NoJobs(t *testing.T) {
    db := setupTestDB(t)
    jm := NewJobManager(db, 10)

    results := jm.ResumeJobs(context.Background())
    assert.Empty(t, results)
}

func TestResume_MissingRepo(t *testing.T) {
    db := setupTestDB(t)
    jm := NewJobManager(db, 10)

    // Create a run record with non-existent repo
    run := &db.Run{
        ID:       ulid.Make().String(),
        RepoPath: "/nonexistent/repo",
        Status:   db.RunStatusRunning,
    }
    require.NoError(t, db.CreateRun(run))

    results := jm.ResumeJobs(context.Background())
    require.Len(t, results, 1)
    assert.False(t, results[0].Success)
    assert.Contains(t, results[0].Error.Error(), "no longer exists")
}

func TestValidateWorktrees_SomeMissing(t *testing.T) {
    units := []*db.UnitRecord{
        {
            ID:           "run1_unit1",
            Status:       string(db.UnitStatusRunning),
            WorktreePath: stringPtr("/valid/path"), // Would be mocked
        },
        {
            ID:           "run1_unit2",
            Status:       string(db.UnitStatusRunning),
            WorktreePath: stringPtr("/nonexistent/worktree"),
        },
    }

    updated := validateWorktrees(units)

    // First unit unchanged (assuming mock setup)
    // Second unit should be marked failed
    assert.Equal(t, string(db.UnitStatusFailed), updated[1].Status)
    assert.Contains(t, *updated[1].Error, "worktree no longer exists")
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Resume failures are logged but don't stop daemon startup
- Each job is resumed independently; one failure doesn't affect others
- Jobs marked as failed remain in database for inspection
- Worktree validation is best-effort; some edge cases may slip through
- Resume only supports jobs from the same daemon version (check version in run metadata)

## NOT In Scope

- Daemon startup sequence (Task #7)
- Orchestrator resume mode implementation (handled by orchestrator package)
- gRPC status reporting (handled by DAEMON-GRPC)
