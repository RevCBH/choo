# DAEMON-CORE — Daemon Process Lifecycle, Job Manager, and Resume Logic

## Overview

DAEMON-CORE defines the long-running daemon process that manages job execution for choo. The daemon provides a gRPC interface for CLI clients, manages multiple concurrent orchestrator instances through a job manager, and implements crash-resilient resume logic using SQLite-backed state persistence.

The daemon architecture enables:
- Background job execution decoupled from CLI sessions
- Concurrent execution of multiple jobs with configurable limits
- Automatic resume of interrupted jobs after daemon restart
- Graceful shutdown with safe checkpointing

## Requirements

### Functional Requirements

1. **Process Management**
   - Start as a background daemon process
   - Write PID file for process tracking
   - Listen on Unix domain socket for client connections
   - Handle graceful shutdown on SIGTERM/SIGINT

2. **Job Lifecycle**
   - Accept job start requests via gRPC
   - Create and manage Orchestrator instances per job
   - Track job state in SQLite database
   - Provide job status queries and event streaming

3. **Resume Capability**
   - On startup, query for jobs with `status='running'`
   - Validate repository and worktree state
   - Reconstruct scheduler DAG from persisted unit states
   - Resume orchestrators from last checkpoint

4. **Concurrency Control**
   - Enforce configurable maximum concurrent jobs
   - Isolate job event buses to prevent cross-contamination
   - Coordinate shutdown across all active jobs

### Performance Requirements

| Metric | Target |
|--------|--------|
| Startup time (no resume) | < 500ms |
| Job start latency | < 100ms |
| Max concurrent jobs | 10 (configurable) |
| gRPC response latency | < 50ms for status queries |
| Graceful shutdown timeout | 30s max |

### Constraints

- Unix domain sockets only (no TCP/network exposure)
- Single daemon instance per user (enforced via PID file)
- SQLite database for state persistence (no external dependencies)
- Resume only supports jobs from the same daemon version

## Design

### Module Structure

```
daemon/
├── daemon.go           # Daemon struct and lifecycle
├── config.go           # Configuration with defaults
├── job_manager.go      # Job tracking and orchestrator management
├── resume.go           # Resume logic and validation
├── grpc_server.go      # gRPC service implementation
└── pid.go              # PID file utilities
```

### Core Types

```go
// Daemon is the main daemon process coordinator.
type Daemon struct {
    cfg        *Config
    db         *db.DB
    jobManager *JobManager
    grpcServer *grpc.Server
    listener   net.Listener

    shutdownCh chan struct{}
    wg         sync.WaitGroup
}

// Config holds daemon configuration with sensible defaults.
type Config struct {
    SocketPath string // Default: ~/.choo/daemon.sock
    PIDFile    string // Default: ~/.choo/daemon.pid
    DBPath     string // Default: ~/.choo/choo.db
    MaxJobs    int    // Default: 10
}

// JobManager tracks and coordinates active jobs.
type JobManager struct {
    db      *db.DB
    maxJobs int

    mu      sync.RWMutex
    jobs    map[string]*ManagedJob

    eventBus *events.Bus
}

// ManagedJob represents an active job with its orchestrator.
type ManagedJob struct {
    ID           string
    Orchestrator *orchestrator.Orchestrator
    Cancel       context.CancelFunc
    Events       *events.Bus
    StartedAt    time.Time
}

// JobConfig contains parameters for starting a new job.
type JobConfig struct {
    RepoPath    string
    Spec        *spec.Spec
    DryRun      bool
    Concurrency int
}
```

### API Surface

#### Daemon Lifecycle

```go
// New creates a new daemon instance.
func New(cfg *Config) (*Daemon, error)

// Start begins the daemon, resumes jobs, and listens for connections.
// Blocks until shutdown is triggered.
func (d *Daemon) Start(ctx context.Context) error

// Shutdown initiates graceful shutdown.
func (d *Daemon) Shutdown()
```

#### Job Manager

```go
// Start creates and starts a new job, returning the job ID.
func (jm *JobManager) Start(ctx context.Context, cfg JobConfig) (string, error)

// Stop cancels a running job.
func (jm *JobManager) Stop(jobID string) error

// Get returns a managed job by ID.
func (jm *JobManager) Get(jobID string) (*ManagedJob, bool)

// List returns all active job IDs.
func (jm *JobManager) List() []string

// Subscribe returns an event channel for a specific job.
func (jm *JobManager) Subscribe(jobID string) (<-chan events.Event, func())
```

#### gRPC Service

```protobuf
service DaemonService {
    rpc StartJob(StartJobRequest) returns (StartJobResponse);
    rpc StopJob(StopJobRequest) returns (StopJobResponse);
    rpc GetJobStatus(GetJobStatusRequest) returns (GetJobStatusResponse);
    rpc ListJobs(ListJobsRequest) returns (ListJobsResponse);
    rpc WatchJob(WatchJobRequest) returns (stream JobEvent);
    rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);
    rpc Health(HealthRequest) returns (HealthResponse);
}
```

## Implementation Notes

### Daemon Startup Sequence

```go
func (d *Daemon) Start(ctx context.Context) error {
    // 1. Write PID file (fails if daemon already running)
    if err := d.writePIDFile(); err != nil {
        return err
    }
    defer os.Remove(d.cfg.PIDFile)

    // 2. Resume any interrupted jobs from previous run
    if err := d.resumeJobs(ctx); err != nil {
        log.Printf("WARN: failed to resume jobs: %v", err)
        // Continue startup - resume failures are non-fatal
    }

    // 3. Create Unix socket listener (remove stale socket first)
    if err := os.Remove(d.cfg.SocketPath); err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("failed to remove old socket: %w", err)
    }

    listener, err := net.Listen("unix", d.cfg.SocketPath)
    if err != nil {
        return fmt.Errorf("failed to create socket: %w", err)
    }
    d.listener = listener

    // 4. Start gRPC server
    d.grpcServer = grpc.NewServer()
    apiv1.RegisterDaemonServiceServer(d.grpcServer, NewGRPCServer(d.jobManager, d.db))

    d.wg.Add(1)
    go func() {
        defer d.wg.Done()
        if err := d.grpcServer.Serve(listener); err != nil {
            log.Printf("gRPC server error: %v", err)
        }
    }()

    log.Printf("Daemon listening on %s", d.cfg.SocketPath)

    // 5. Wait for shutdown signal
    <-d.shutdownCh
    return d.gracefulShutdown(ctx)
}
```

### Job Start Flow

```go
func (jm *JobManager) Start(ctx context.Context, cfg JobConfig) (string, error) {
    jm.mu.Lock()
    defer jm.mu.Unlock()

    // 1. Enforce capacity limit
    if len(jm.jobs) >= jm.maxJobs {
        return "", fmt.Errorf("max jobs (%d) reached", jm.maxJobs)
    }

    // 2. Generate unique job ID
    jobID := ulid.Make().String()

    // 3. Create run record in SQLite (status='running')
    now := time.Now()
    run := &db.Run{
        ID:            jobID,
        RepoPath:      cfg.RepoPath,
        Status:        db.RunStatusRunning,
        DaemonVersion: Version,
        StartedAt:     &now,
    }
    if err := jm.db.CreateRun(ctx, run); err != nil {
        return "", fmt.Errorf("failed to create run record: %w", err)
    }

    // 4. Create isolated event bus for this job
    jobEvents := events.NewBus()

    // 5. Create orchestrator with job-specific config
    jobCtx, cancel := context.WithCancel(ctx)
    orch, err := orchestrator.New(orchestrator.Config{
        DB:          jm.db,
        RunID:       jobID,
        RepoPath:    cfg.RepoPath,
        Spec:        cfg.Spec,
        Events:      jobEvents,
        Concurrency: cfg.Concurrency,
    })
    if err != nil {
        cancel()
        return "", fmt.Errorf("failed to create orchestrator: %w", err)
    }

    // 6. Register managed job
    jm.jobs[jobID] = &ManagedJob{
        ID:           jobID,
        Orchestrator: orch,
        Cancel:       cancel,
        Events:       jobEvents,
        StartedAt:    time.Now(),
    }

    // 7. Start orchestrator in background
    go func() {
        defer jm.cleanup(jobID)
        if err := orch.Run(jobCtx); err != nil {
            log.Printf("Job %s failed: %v", jobID, err)
        }
    }()

    return jobID, nil
}
```

### Resume Logic

```go
func (d *Daemon) resumeJobs(ctx context.Context) error {
    // Query for runs that were in progress when daemon stopped
    runs, err := d.db.GetRunsByStatus(ctx, db.RunStatusRunning)
    if err != nil {
        return fmt.Errorf("failed to query running runs: %w", err)
    }

    if len(runs) == 0 {
        log.Println("No jobs to resume")
        return nil
    }

    log.Printf("Found %d jobs to resume", len(runs))

    var resumeErrors []error
    for _, run := range runs {
        if err := d.resumeJob(ctx, run); err != nil {
            resumeErrors = append(resumeErrors, fmt.Errorf("job %s: %w", run.ID, err))
            // Mark as failed rather than leaving in running state
            _ = d.markJobFailed(ctx, run.ID, err.Error())
        }
    }

    if len(resumeErrors) > 0 {
        return fmt.Errorf("resume errors: %v", resumeErrors)
    }
    return nil
}

func (d *Daemon) resumeJob(ctx context.Context, run *db.Run) error {
    // 1. Validate daemon version matches
    if run.DaemonVersion != Version {
        return fmt.Errorf("version mismatch: run created by %s, current daemon is %s", run.DaemonVersion, Version)
    }

    // 2. Validate repository still exists
    if _, err := os.Stat(run.RepoPath); os.IsNotExist(err) {
        return fmt.Errorf("repository no longer exists: %s", run.RepoPath)
    }

    // 3. Get unit states from database
    units, err := d.db.GetUnitsByRunID(ctx, run.ID)
    if err != nil {
        return fmt.Errorf("failed to get units: %w", err)
    }

    // 4. Validate worktrees are still valid for in-progress units
    for _, unit := range units {
        if unit.Status == db.UnitStatusRunning && unit.WorktreePath != "" {
            if _, err := os.Stat(unit.WorktreePath); os.IsNotExist(err) {
                // Mark unit as failed - worktree was cleaned up
                unit.Status = db.UnitStatusFailed
                unit.Error = "worktree no longer exists after restart"
            }
        }
    }

    // 5. Reconstruct job config from run record
    cfg := JobConfig{
        RepoPath: run.RepoPath,
        // Spec is loaded from run metadata
    }

    // 6. Resume via job manager with existing run ID
    return d.jobManager.Resume(ctx, run.ID, cfg, units)
}
```

### Graceful Shutdown

```go
func (d *Daemon) gracefulShutdown(ctx context.Context) error {
    log.Println("Starting graceful shutdown...")

    // 1. Stop accepting new connections
    d.grpcServer.GracefulStop()

    // 2. Signal all jobs to stop at safe points
    d.jobManager.StopAll()

    // 3. Wait for jobs with timeout
    shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    done := make(chan struct{})
    go func() {
        d.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        log.Println("Graceful shutdown complete")
        return nil
    case <-shutdownCtx.Done():
        log.Println("Shutdown timeout - forcing exit")
        return fmt.Errorf("shutdown timeout exceeded")
    }
}
```

### PID File Handling

```go
func (d *Daemon) writePIDFile() error {
    // Check if daemon already running
    if data, err := os.ReadFile(d.cfg.PIDFile); err == nil {
        pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
        if pid > 0 {
            // Check if process exists
            if process, err := os.FindProcess(pid); err == nil {
                if err := process.Signal(syscall.Signal(0)); err == nil {
                    return fmt.Errorf("daemon already running with PID %d", pid)
                }
            }
        }
        // Stale PID file - remove it
        os.Remove(d.cfg.PIDFile)
    }

    // Write current PID
    return os.WriteFile(d.cfg.PIDFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}
```

## Testing Strategy

### Unit Tests

| Test Case | Description |
|-----------|-------------|
| `TestDaemon_New` | Verify daemon initializes with defaults |
| `TestDaemon_PIDFile_AlreadyRunning` | Verify error when daemon already running |
| `TestDaemon_PIDFile_StalePID` | Verify stale PID file is cleaned up |
| `TestJobManager_Start` | Verify job creation and ID generation |
| `TestJobManager_MaxJobs` | Verify capacity limit enforcement |
| `TestJobManager_Stop` | Verify job cancellation |
| `TestResume_NoJobs` | Verify clean startup with no running jobs |
| `TestResume_ValidJob` | Verify job resumes with valid state |
| `TestResume_MissingRepo` | Verify job marked failed if repo deleted |
| `TestResume_InvalidWorktree` | Verify unit marked failed if worktree missing |

### Integration Tests

| Test Case | Description |
|-----------|-------------|
| `TestDaemon_StartStop` | Full daemon lifecycle with gRPC client |
| `TestDaemon_MultipleJobs` | Concurrent job execution |
| `TestDaemon_ResumeAfterCrash` | Simulate crash and verify resume |
| `TestDaemon_GracefulShutdown` | Verify jobs checkpoint on shutdown |
| `TestGRPC_StreamEvents` | Verify event streaming to client |

### Manual Testing

```bash
# Start daemon in foreground for debugging
choo daemon --foreground --verbose

# In another terminal, start a job
choo run spec.yaml

# Verify daemon state
cat ~/.choo/daemon.pid
ls -la ~/.choo/daemon.sock

# Test resume: kill daemon with SIGKILL, restart, verify job continues
kill -9 $(cat ~/.choo/daemon.pid)
choo daemon start
choo status  # Should show resumed job
```

## Design Decisions

### Unix Socket vs TCP

**Decision**: Use Unix domain socket only.

**Rationale**:
- Security: No network exposure, file permissions control access
- Performance: Lower latency than TCP loopback
- Simplicity: No port conflicts, no firewall concerns
- Tradeoff: Cannot run CLI on remote machine (acceptable for v1)

### Single Daemon Instance

**Decision**: Enforce single daemon per user via PID file.

**Rationale**:
- Avoids race conditions on SQLite database
- Simplifies job coordination
- Clear ownership of Unix socket
- Alternative considered: Multiple daemons with separate databases (rejected - complexity)

### Resume on Startup vs Explicit Command

**Decision**: Automatically resume on daemon startup.

**Rationale**:
- Crash recovery should be automatic
- User expectation: jobs survive daemon restart
- No manual intervention required
- Tradeoff: Startup slightly slower if many jobs to resume

### Isolated Event Buses per Job

**Decision**: Each job gets its own event bus instance.

**Rationale**:
- Prevents event cross-contamination between jobs
- Subscribers only see events for their job
- Simplifies cleanup when job completes
- Memory overhead acceptable (event buses are lightweight)

### Resume Failure Handling

**Decision**: Mark jobs as failed if resume validation fails, continue with other jobs.

**Rationale**:
- Partial resume better than no resume
- Failed jobs visible in status
- User can investigate and re-run manually
- Alternative: Fail entire daemon startup (rejected - too disruptive)

## Future Enhancements

1. **Remote CLI Support**: Add optional TCP listener with mTLS for remote CLI connections
2. **Job Priorities**: Allow priority levels affecting scheduling order
3. **Resource Limits**: Per-job memory and CPU constraints
4. **Metrics Endpoint**: Prometheus-compatible metrics for monitoring
5. **Health Checks**: Liveness and readiness endpoints for orchestration
6. **Job Queuing**: Queue jobs beyond max limit instead of rejecting
7. **Checkpoint API**: Explicit checkpoint command for safe pause/resume

## References

- [gRPC Go Documentation](https://grpc.io/docs/languages/go/)
- [Unix Domain Sockets](https://man7.org/linux/man-pages/man7/unix.7.html)
- [ULID Specification](https://github.com/ulid/spec)
- Internal: `pkg/orchestrator/orchestrator.go` - Orchestrator interface
- Internal: `pkg/db/db.go` - SQLite database layer
- Internal: `pkg/events/bus.go` - Event bus implementation
