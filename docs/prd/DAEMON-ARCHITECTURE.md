---
prd_id: daemon-architecture
title: "Charlotte Daemon Architecture"
status: draft
depends_on:
  - mvp-orchestrator
  - web-ui
# Orchestrator-managed fields
# feature_branch: feature/daemon-architecture
# feature_status: pending
# spec_review_iterations: 0
---

# Charlotte Daemon Architecture

## Document Info

| Field   | Value      |
| ------- | ---------- |
| Status  | Draft      |
| Author  | Claude     |
| Created | 2026-01-20 |
| Target  | v0.5       |

---

## 1. Overview

### 1.1 Problem Statement

Charlotte currently operates as a CLI-based orchestrator where each `choo run` invocation creates a new process that directly owns workflow execution. This architecture has several limitations:

1. **No Concurrent Jobs**: Running multiple jobs on different feature branches requires multiple terminal sessions
2. **No Resumability**: If the CLI process crashes or is interrupted, job state is lost and resumption requires manual intervention
3. **Tight Coupling**: UI clients (web dashboard, TUI) must embed orchestration logic or rely on file-system polling
4. **No Job Isolation**: Multiple invocations can interfere with each other's state

### 1.2 Solution

Convert Charlotte from a CLI-based orchestrator to a daemon-based architecture where:

1. A **long-running daemon process** owns all workflow execution
2. The **CLI becomes a thin gRPC client** that sends commands to the daemon
3. **SQLite provides persistent state** for resumability after daemon restarts
4. **Multiple concurrent jobs** can run in isolation with proper resource management

### 1.3 Architecture Overview

```
┌─────────────┐     gRPC      ┌─────────────────────────────────────────┐
│  choo CLI   │◄─────────────►│             Daemon Process              │
│  (client)   │               │                                         │
└─────────────┘               │  ┌─────────┐    ┌──────────────────┐   │
                              │  │  gRPC   │    │   Job Manager    │   │
                              │  │ Server  │◄──►│  (runs N jobs)   │   │
                              │  └─────────┘    └──────────────────┘   │
                              │                          │              │
                              │           ┌──────────────┼──────┐      │
                              │           ▼              ▼      ▼      │
                              │    ┌─────────────┐ ┌─────────────┐     │
                              │    │Orchestrator │ │Orchestrator │ ... │
                              │    │  (job 1)    │ │  (job 2)    │     │
                              │    └─────────────┘ └─────────────┘     │
                              │                          │              │
                              │                 ┌────────┴────────┐    │
                              │                 │    SQLite DB    │    │
                              │                 │  (state store)  │    │
                              │                 └─────────────────┘    │
                              └─────────────────────────────────────────┘
```

### 1.4 Design Principles

1. **Persistent State**: SQLite database stores all run/unit/event state for resumability
2. **gRPC Interface**: Clean separation between CLI client and daemon execution
3. **Job Isolation**: Each job gets its own Orchestrator instance with isolated event bus
4. **Graceful Degradation**: Single-job mode behaves identically to current CLI behavior
5. **Resume on Restart**: Daemon automatically resumes `running` jobs on startup

### 1.5 Non-Goals (Initial Implementation)

- Remote daemon access (network binding beyond localhost)
- Authentication/authorization between CLI and daemon
- Distributed daemon cluster
- Cross-machine job migration

---

## 2. Data Model

### 2.1 SQLite Schema

The daemon uses SQLite for persistent state storage. The schema supports full job resumability:

```sql
-- Runs table: Top-level workflow executions
CREATE TABLE runs (
    id              TEXT PRIMARY KEY,       -- ULID for sortable unique IDs
    feature_branch  TEXT NOT NULL,
    repo_path       TEXT NOT NULL,
    target_branch   TEXT NOT NULL,
    tasks_dir       TEXT NOT NULL,
    parallelism     INTEGER NOT NULL,
    status          TEXT NOT NULL,          -- pending/running/completed/failed/cancelled
    started_at      DATETIME,
    completed_at    DATETIME,
    error           TEXT,
    config_json     TEXT,                   -- Serialized Config struct
    UNIQUE(feature_branch, repo_path)       -- One active run per branch/repo
);

-- Units table: Individual work units within a run
CREATE TABLE units (
    id              TEXT PRIMARY KEY,       -- run_id + "_" + unit_id
    run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    unit_id         TEXT NOT NULL,
    status          TEXT NOT NULL,          -- pending/in_progress/pr_open/complete/failed
    branch          TEXT,
    worktree_path   TEXT,
    started_at      DATETIME,
    completed_at    DATETIME,
    error           TEXT,
    UNIQUE(run_id, unit_id)
);

-- Events table: Event log for replay and debugging
CREATE TABLE events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    sequence        INTEGER NOT NULL,       -- Per-run sequence number
    event_type      TEXT NOT NULL,
    unit_id         TEXT,
    payload_json    TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(run_id, sequence)
);

-- Indexes for common queries
CREATE INDEX idx_runs_status ON runs(status);
CREATE INDEX idx_units_run_id ON units(run_id);
CREATE INDEX idx_units_status ON units(status);
CREATE INDEX idx_events_run_id ON events(run_id);
CREATE INDEX idx_events_sequence ON events(run_id, sequence);
```

### 2.2 Go Types

```go
// internal/db/types.go

type RunStatus string

const (
    RunStatusPending   RunStatus = "pending"
    RunStatusRunning   RunStatus = "running"
    RunStatusCompleted RunStatus = "completed"
    RunStatusFailed    RunStatus = "failed"
    RunStatusCancelled RunStatus = "cancelled"
)

type Run struct {
    ID            string     `db:"id"`
    FeatureBranch string     `db:"feature_branch"`
    RepoPath      string     `db:"repo_path"`
    TargetBranch  string     `db:"target_branch"`
    TasksDir      string     `db:"tasks_dir"`
    Parallelism   int        `db:"parallelism"`
    Status        RunStatus  `db:"status"`
    StartedAt     *time.Time `db:"started_at"`
    CompletedAt   *time.Time `db:"completed_at"`
    Error         *string    `db:"error"`
    ConfigJSON    string     `db:"config_json"`
}

type UnitRecord struct {
    ID           string     `db:"id"`
    RunID        string     `db:"run_id"`
    UnitID       string     `db:"unit_id"`
    Status       string     `db:"status"`
    Branch       *string    `db:"branch"`
    WorktreePath *string    `db:"worktree_path"`
    StartedAt    *time.Time `db:"started_at"`
    CompletedAt  *time.Time `db:"completed_at"`
    Error        *string    `db:"error"`
}

type EventRecord struct {
    ID          int64     `db:"id"`
    RunID       string    `db:"run_id"`
    Sequence    int       `db:"sequence"`
    EventType   string    `db:"event_type"`
    UnitID      *string   `db:"unit_id"`
    PayloadJSON *string   `db:"payload_json"`
    CreatedAt   time.Time `db:"created_at"`
}
```

---

## 3. gRPC Interface

### 3.1 Service Definition

```protobuf
// proto/choo/v1/daemon.proto

syntax = "proto3";

package choo.v1;

option go_package = "github.com/charlotte/pkg/api/v1;apiv1";

import "google/protobuf/timestamp.proto";

service DaemonService {
    // Job lifecycle
    rpc StartJob(StartJobRequest) returns (StartJobResponse);
    rpc StopJob(StopJobRequest) returns (StopJobResponse);
    rpc GetJobStatus(GetJobStatusRequest) returns (GetJobStatusResponse);
    rpc ListJobs(ListJobsRequest) returns (ListJobsResponse);

    // Event streaming
    rpc WatchJob(WatchJobRequest) returns (stream JobEvent);

    // Daemon lifecycle
    rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);
    rpc Health(HealthRequest) returns (HealthResponse);
}

// StartJob creates and starts a new job
message StartJobRequest {
    string tasks_dir = 1;
    string target_branch = 2;
    string feature_branch = 3;  // Optional: for feature mode
    int32 parallelism = 4;
    string repo_path = 5;
}

message StartJobResponse {
    string job_id = 1;
    string status = 2;
}

// StopJob gracefully stops a running job
message StopJobRequest {
    string job_id = 1;
    bool force = 2;  // If true, kill immediately without waiting
}

message StopJobResponse {
    bool success = 1;
    string message = 2;
}

// GetJobStatus returns current status of a job
message GetJobStatusRequest {
    string job_id = 1;
}

message GetJobStatusResponse {
    string job_id = 1;
    string status = 2;
    google.protobuf.Timestamp started_at = 3;
    google.protobuf.Timestamp completed_at = 4;
    string error = 5;
    repeated UnitStatus units = 6;
}

message UnitStatus {
    string unit_id = 1;
    string status = 2;
    int32 tasks_complete = 3;
    int32 tasks_total = 4;
    int32 pr_number = 5;
}

// ListJobs returns all jobs, optionally filtered by status
message ListJobsRequest {
    repeated string status_filter = 1;  // Empty = all statuses
}

message ListJobsResponse {
    repeated JobSummary jobs = 1;
}

message JobSummary {
    string job_id = 1;
    string feature_branch = 2;
    string status = 3;
    google.protobuf.Timestamp started_at = 4;
    int32 units_complete = 5;
    int32 units_total = 6;
}

// WatchJob streams events for a running job
message WatchJobRequest {
    string job_id = 1;
    int32 from_sequence = 2;  // Resume from sequence number (0 = beginning)
}

message JobEvent {
    int32 sequence = 1;
    string event_type = 2;
    string unit_id = 3;
    string payload_json = 4;
    google.protobuf.Timestamp timestamp = 5;
}

// Shutdown gracefully shuts down the daemon
message ShutdownRequest {
    bool wait_for_jobs = 1;  // If true, wait for running jobs to complete
    int32 timeout_seconds = 2;
}

message ShutdownResponse {
    bool success = 1;
    int32 jobs_stopped = 2;
}

// Health check
message HealthRequest {}

message HealthResponse {
    bool healthy = 1;
    int32 active_jobs = 2;
    string version = 3;
}
```

### 3.2 gRPC Server Implementation

```go
// internal/daemon/grpc.go

type GRPCServer struct {
    apiv1.UnimplementedDaemonServiceServer

    jobManager *JobManager
    db         *db.DB
}

func NewGRPCServer(jm *JobManager, db *db.DB) *GRPCServer {
    return &GRPCServer{
        jobManager: jm,
        db:         db,
    }
}

func (s *GRPCServer) StartJob(ctx context.Context, req *apiv1.StartJobRequest) (*apiv1.StartJobResponse, error) {
    jobID, err := s.jobManager.Start(ctx, JobConfig{
        TasksDir:      req.TasksDir,
        TargetBranch:  req.TargetBranch,
        FeatureBranch: req.FeatureBranch,
        Parallelism:   int(req.Parallelism),
        RepoPath:      req.RepoPath,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to start job: %v", err)
    }

    return &apiv1.StartJobResponse{
        JobId:  jobID,
        Status: "running",
    }, nil
}

func (s *GRPCServer) WatchJob(req *apiv1.WatchJobRequest, stream apiv1.DaemonService_WatchJobServer) error {
    // Subscribe to job events
    events, unsub := s.jobManager.Subscribe(req.JobId, int(req.FromSequence))
    defer unsub()

    for event := range events {
        if err := stream.Send(eventToProto(event)); err != nil {
            return err
        }
    }

    return nil
}
```

---

## 4. Daemon Core

### 4.1 Daemon Lifecycle

```go
// internal/daemon/daemon.go

type Daemon struct {
    cfg        *Config
    db         *db.DB
    jobManager *JobManager
    grpcServer *grpc.Server
    listener   net.Listener

    shutdownCh chan struct{}
    wg         sync.WaitGroup
}

type Config struct {
    SocketPath string // Default: ~/.choo/daemon.sock
    PIDFile    string // Default: ~/.choo/daemon.pid
    DBPath     string // Default: ~/.choo/choo.db
    MaxJobs    int    // Default: 10
}

func New(cfg *Config) (*Daemon, error) {
    // Initialize SQLite database
    database, err := db.Open(cfg.DBPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Initialize job manager
    jm := NewJobManager(database, cfg.MaxJobs)

    return &Daemon{
        cfg:        cfg,
        db:         database,
        jobManager: jm,
        shutdownCh: make(chan struct{}),
    }, nil
}

func (d *Daemon) Start(ctx context.Context) error {
    // Write PID file
    if err := d.writePIDFile(); err != nil {
        return err
    }
    defer os.Remove(d.cfg.PIDFile)

    // Resume any interrupted jobs
    if err := d.resumeJobs(ctx); err != nil {
        log.Printf("WARN: failed to resume jobs: %v", err)
    }

    // Create Unix socket listener
    if err := os.Remove(d.cfg.SocketPath); err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("failed to remove old socket: %w", err)
    }

    listener, err := net.Listen("unix", d.cfg.SocketPath)
    if err != nil {
        return fmt.Errorf("failed to create socket: %w", err)
    }
    d.listener = listener

    // Create and start gRPC server
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

    // Wait for shutdown signal
    <-d.shutdownCh
    return d.gracefulShutdown(ctx)
}

func (d *Daemon) Shutdown() {
    close(d.shutdownCh)
}

func (d *Daemon) gracefulShutdown(ctx context.Context) error {
    log.Println("Initiating graceful shutdown...")

    // Stop accepting new requests
    d.grpcServer.GracefulStop()

    // Wait for running jobs to reach safe point
    if err := d.jobManager.StopAll(ctx); err != nil {
        log.Printf("WARN: some jobs did not stop cleanly: %v", err)
    }

    // Close database
    if err := d.db.Close(); err != nil {
        log.Printf("WARN: database close error: %v", err)
    }

    d.wg.Wait()
    log.Println("Shutdown complete")
    return nil
}
```

### 4.2 Job Manager

```go
// internal/daemon/jobmanager.go

type JobManager struct {
    db      *db.DB
    maxJobs int

    mu      sync.RWMutex
    jobs    map[string]*ManagedJob

    eventBus *events.Bus
}

type ManagedJob struct {
    ID          string
    Orchestrator *orchestrator.Orchestrator
    Cancel      context.CancelFunc
    Events      *events.Bus
    StartedAt   time.Time
}

func NewJobManager(db *db.DB, maxJobs int) *JobManager {
    return &JobManager{
        db:       db,
        maxJobs:  maxJobs,
        jobs:     make(map[string]*ManagedJob),
        eventBus: events.NewBus(1000),
    }
}

func (jm *JobManager) Start(ctx context.Context, cfg JobConfig) (string, error) {
    jm.mu.Lock()
    defer jm.mu.Unlock()

    // Check capacity
    if len(jm.jobs) >= jm.maxJobs {
        return "", fmt.Errorf("max jobs (%d) reached", jm.maxJobs)
    }

    // Generate job ID
    jobID := ulid.Make().String()

    // Create run record
    run := &db.Run{
        ID:            jobID,
        FeatureBranch: cfg.FeatureBranch,
        RepoPath:      cfg.RepoPath,
        TargetBranch:  cfg.TargetBranch,
        TasksDir:      cfg.TasksDir,
        Parallelism:   cfg.Parallelism,
        Status:        db.RunStatusRunning,
    }
    if err := jm.db.CreateRun(ctx, run); err != nil {
        return "", fmt.Errorf("failed to create run record: %w", err)
    }

    // Create isolated event bus for this job
    jobEvents := events.NewBus(1000)

    // Subscribe to persist events to SQLite
    jobEvents.Subscribe(jm.createEventPersister(jobID))

    // Create orchestrator
    orch, err := orchestrator.New(orchestrator.Config{
        TasksDir:     cfg.TasksDir,
        TargetBranch: cfg.TargetBranch,
        Parallelism:  cfg.Parallelism,
        RepoPath:     cfg.RepoPath,
        EventBus:     jobEvents,
    })
    if err != nil {
        return "", fmt.Errorf("failed to create orchestrator: %w", err)
    }

    // Start orchestrator in background
    jobCtx, cancel := context.WithCancel(ctx)
    job := &ManagedJob{
        ID:           jobID,
        Orchestrator: orch,
        Cancel:       cancel,
        Events:       jobEvents,
        StartedAt:    time.Now(),
    }
    jm.jobs[jobID] = job

    go jm.runJob(jobCtx, job)

    return jobID, nil
}

func (jm *JobManager) runJob(ctx context.Context, job *ManagedJob) {
    defer func() {
        jm.mu.Lock()
        delete(jm.jobs, job.ID)
        jm.mu.Unlock()
    }()

    err := job.Orchestrator.Run(ctx)

    // Update run status
    status := db.RunStatusCompleted
    var errMsg *string
    if err != nil {
        status = db.RunStatusFailed
        msg := err.Error()
        errMsg = &msg
    }

    if updateErr := jm.db.UpdateRunStatus(context.Background(), job.ID, status, errMsg); updateErr != nil {
        log.Printf("ERROR: failed to update run status: %v", updateErr)
    }
}

func (jm *JobManager) Subscribe(jobID string, fromSequence int) (<-chan events.Event, func()) {
    jm.mu.RLock()
    job, ok := jm.jobs[jobID]
    jm.mu.RUnlock()

    if !ok {
        // Job not running, replay from database
        return jm.replayEvents(jobID, fromSequence)
    }

    // Subscribe to live events
    ch := make(chan events.Event, 100)
    job.Events.Subscribe(func(e events.Event) {
        select {
        case ch <- e:
        default:
            // Drop if buffer full
        }
    })

    return ch, func() { close(ch) }
}

func (jm *JobManager) createEventPersister(runID string) events.Handler {
    var sequence int32

    return func(e events.Event) {
        seq := atomic.AddInt32(&sequence, 1)

        record := &db.EventRecord{
            RunID:     runID,
            Sequence:  int(seq),
            EventType: string(e.Type),
            UnitID:    e.Unit,
        }

        if e.Payload != nil {
            data, _ := json.Marshal(e.Payload)
            payload := string(data)
            record.PayloadJSON = &payload
        }

        if err := jm.db.InsertEvent(context.Background(), record); err != nil {
            log.Printf("ERROR: failed to persist event: %v", err)
        }
    }
}
```

### 4.3 Resume Logic

```go
// internal/daemon/resume.go

func (d *Daemon) resumeJobs(ctx context.Context) error {
    // Query for runs with status = 'running'
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
        }
    }

    if len(resumeErrors) > 0 {
        return fmt.Errorf("failed to resume %d jobs: %v", len(resumeErrors), resumeErrors)
    }

    return nil
}

func (d *Daemon) resumeJob(ctx context.Context, run *db.Run) error {
    // Validate repository still exists
    if _, err := os.Stat(run.RepoPath); os.IsNotExist(err) {
        return d.markJobFailed(ctx, run.ID, "repository no longer exists")
    }

    // Get unit states from database
    units, err := d.db.GetUnitsForRun(ctx, run.ID)
    if err != nil {
        return fmt.Errorf("failed to get units: %w", err)
    }

    // Validate worktrees are still valid
    for _, unit := range units {
        if unit.WorktreePath != nil {
            if _, err := os.Stat(*unit.WorktreePath); os.IsNotExist(err) {
                // Worktree gone, mark unit for restart
                if err := d.db.UpdateUnitStatus(ctx, unit.ID, "pending"); err != nil {
                    return fmt.Errorf("failed to reset unit %s: %w", unit.UnitID, err)
                }
            }
        }
    }

    // Reconstruct scheduler DAG from units table
    unitStates := make(map[string]string)
    for _, u := range units {
        unitStates[u.UnitID] = u.Status
    }

    // Resume via job manager
    return d.jobManager.Resume(ctx, run, unitStates)
}

func (d *Daemon) markJobFailed(ctx context.Context, runID, reason string) error {
    return d.db.UpdateRunStatus(ctx, runID, db.RunStatusFailed, &reason)
}
```

---

## 5. CLI Client

### 5.1 Client Implementation

```go
// internal/client/client.go

type Client struct {
    conn   *grpc.ClientConn
    daemon apiv1.DaemonServiceClient
}

func New(socketPath string) (*Client, error) {
    conn, err := grpc.Dial(
        "unix://"+socketPath,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to connect to daemon: %w", err)
    }

    return &Client{
        conn:   conn,
        daemon: apiv1.NewDaemonServiceClient(conn),
    }, nil
}

func (c *Client) StartJob(ctx context.Context, cfg JobConfig) (string, error) {
    resp, err := c.daemon.StartJob(ctx, &apiv1.StartJobRequest{
        TasksDir:      cfg.TasksDir,
        TargetBranch:  cfg.TargetBranch,
        FeatureBranch: cfg.FeatureBranch,
        Parallelism:   int32(cfg.Parallelism),
        RepoPath:      cfg.RepoPath,
    })
    if err != nil {
        return "", err
    }
    return resp.JobId, nil
}

func (c *Client) WatchJob(ctx context.Context, jobID string, handler func(events.Event)) error {
    stream, err := c.daemon.WatchJob(ctx, &apiv1.WatchJobRequest{
        JobId:        jobID,
        FromSequence: 0,
    })
    if err != nil {
        return err
    }

    for {
        event, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }
        handler(protoToEvent(event))
    }
}

func (c *Client) Close() error {
    return c.conn.Close()
}
```

### 5.2 CLI Commands

```
choo daemon start    # Start daemon in foreground
choo daemon stop     # Graceful shutdown (wait for jobs)
choo daemon status   # Show daemon info (PID, socket, active jobs)

choo run [dir]       # Start job via daemon, attach to event stream
choo watch <job-id>  # Attach to running job's event stream
choo jobs            # List all jobs (with status filter)
choo jobs --status running
choo stop <job-id>   # Stop a running job
```

### 5.3 Updated Run Command

```go
// internal/cli/run.go

func runCmd() *cobra.Command {
    var (
        parallelism   int
        targetBranch  string
        featureBranch string
        useDaemon     bool
    )

    cmd := &cobra.Command{
        Use:   "run [tasks-dir]",
        Short: "Execute units in parallel",
        RunE: func(cmd *cobra.Command, args []string) error {
            tasksDir := "specs/tasks"
            if len(args) > 0 {
                tasksDir = args[0]
            }

            if useDaemon {
                return runWithDaemon(cmd.Context(), tasksDir, parallelism, targetBranch, featureBranch)
            }
            return runInline(cmd.Context(), tasksDir, parallelism, targetBranch)
        },
    }

    cmd.Flags().IntVarP(&parallelism, "parallelism", "p", 4, "Max concurrent units")
    cmd.Flags().StringVarP(&targetBranch, "target", "t", "main", "Target branch")
    cmd.Flags().StringVar(&featureBranch, "feature", "", "Feature branch (feature mode)")
    cmd.Flags().BoolVar(&useDaemon, "use-daemon", true, "Use daemon mode")

    return cmd
}

func runWithDaemon(ctx context.Context, tasksDir string, parallelism int, target, feature string) error {
    client, err := client.New(defaultSocketPath())
    if err != nil {
        return fmt.Errorf("failed to connect to daemon: %w (is daemon running?)", err)
    }
    defer client.Close()

    repoPath, _ := os.Getwd()

    jobID, err := client.StartJob(ctx, client.JobConfig{
        TasksDir:      tasksDir,
        TargetBranch:  target,
        FeatureBranch: feature,
        Parallelism:   parallelism,
        RepoPath:      repoPath,
    })
    if err != nil {
        return err
    }

    fmt.Printf("Started job %s\n", jobID)

    // Attach to event stream
    return client.WatchJob(ctx, jobID, func(e events.Event) {
        // Display events to user
        displayEvent(e)
    })
}
```

---

## 6. Database Layer

### 6.1 Connection Management

```go
// internal/db/db.go

type DB struct {
    conn *sql.DB
}

func Open(path string) (*DB, error) {
    // Use modernc.org/sqlite for pure Go SQLite (no CGO)
    conn, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, err
    }

    // Enable WAL mode for better concurrency
    if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
        return nil, fmt.Errorf("failed to enable WAL: %w", err)
    }

    // Enable foreign keys
    if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
        return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
    }

    db := &DB{conn: conn}

    // Run migrations
    if err := db.migrate(); err != nil {
        return nil, fmt.Errorf("migration failed: %w", err)
    }

    return db, nil
}

func (db *DB) migrate() error {
    _, err := db.conn.Exec(schema)
    return err
}

func (db *DB) Close() error {
    return db.conn.Close()
}
```

### 6.2 CRUD Operations

```go
// internal/db/runs.go

func (db *DB) CreateRun(ctx context.Context, run *Run) error {
    now := time.Now()
    run.StartedAt = &now

    _, err := db.conn.ExecContext(ctx, `
        INSERT INTO runs (id, feature_branch, repo_path, target_branch, tasks_dir,
                         parallelism, status, started_at, config_json)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, run.ID, run.FeatureBranch, run.RepoPath, run.TargetBranch, run.TasksDir,
       run.Parallelism, run.Status, run.StartedAt, run.ConfigJSON)

    return err
}

func (db *DB) GetRun(ctx context.Context, id string) (*Run, error) {
    var run Run
    err := db.conn.QueryRowContext(ctx, `
        SELECT id, feature_branch, repo_path, target_branch, tasks_dir,
               parallelism, status, started_at, completed_at, error, config_json
        FROM runs WHERE id = ?
    `, id).Scan(&run.ID, &run.FeatureBranch, &run.RepoPath, &run.TargetBranch,
                &run.TasksDir, &run.Parallelism, &run.Status, &run.StartedAt,
                &run.CompletedAt, &run.Error, &run.ConfigJSON)

    if err == sql.ErrNoRows {
        return nil, nil
    }
    return &run, err
}

func (db *DB) GetRunsByStatus(ctx context.Context, status RunStatus) ([]*Run, error) {
    rows, err := db.conn.QueryContext(ctx, `
        SELECT id, feature_branch, repo_path, target_branch, tasks_dir,
               parallelism, status, started_at, completed_at, error, config_json
        FROM runs WHERE status = ?
    `, status)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var runs []*Run
    for rows.Next() {
        var run Run
        if err := rows.Scan(&run.ID, &run.FeatureBranch, &run.RepoPath, &run.TargetBranch,
            &run.TasksDir, &run.Parallelism, &run.Status, &run.StartedAt,
            &run.CompletedAt, &run.Error, &run.ConfigJSON); err != nil {
            return nil, err
        }
        runs = append(runs, &run)
    }
    return runs, rows.Err()
}

func (db *DB) UpdateRunStatus(ctx context.Context, id string, status RunStatus, errMsg *string) error {
    var completedAt *time.Time
    if status == RunStatusCompleted || status == RunStatusFailed || status == RunStatusCancelled {
        now := time.Now()
        completedAt = &now
    }

    _, err := db.conn.ExecContext(ctx, `
        UPDATE runs SET status = ?, completed_at = ?, error = ? WHERE id = ?
    `, status, completedAt, errMsg, id)

    return err
}
```

```go
// internal/db/units.go

func (db *DB) CreateUnit(ctx context.Context, unit *UnitRecord) error {
    _, err := db.conn.ExecContext(ctx, `
        INSERT INTO units (id, run_id, unit_id, status)
        VALUES (?, ?, ?, ?)
    `, unit.ID, unit.RunID, unit.UnitID, unit.Status)
    return err
}

func (db *DB) GetUnitsForRun(ctx context.Context, runID string) ([]*UnitRecord, error) {
    rows, err := db.conn.QueryContext(ctx, `
        SELECT id, run_id, unit_id, status, branch, worktree_path,
               started_at, completed_at, error
        FROM units WHERE run_id = ?
    `, runID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var units []*UnitRecord
    for rows.Next() {
        var u UnitRecord
        if err := rows.Scan(&u.ID, &u.RunID, &u.UnitID, &u.Status, &u.Branch,
            &u.WorktreePath, &u.StartedAt, &u.CompletedAt, &u.Error); err != nil {
            return nil, err
        }
        units = append(units, &u)
    }
    return units, rows.Err()
}

func (db *DB) UpdateUnitStatus(ctx context.Context, id, status string) error {
    _, err := db.conn.ExecContext(ctx, `
        UPDATE units SET status = ? WHERE id = ?
    `, status, id)
    return err
}
```

```go
// internal/db/events.go

func (db *DB) InsertEvent(ctx context.Context, event *EventRecord) error {
    _, err := db.conn.ExecContext(ctx, `
        INSERT INTO events (run_id, sequence, event_type, unit_id, payload_json)
        VALUES (?, ?, ?, ?, ?)
    `, event.RunID, event.Sequence, event.EventType, event.UnitID, event.PayloadJSON)
    return err
}

func (db *DB) GetEventsForRun(ctx context.Context, runID string, fromSequence int) ([]*EventRecord, error) {
    rows, err := db.conn.QueryContext(ctx, `
        SELECT id, run_id, sequence, event_type, unit_id, payload_json, created_at
        FROM events WHERE run_id = ? AND sequence > ?
        ORDER BY sequence ASC
    `, runID, fromSequence)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var events []*EventRecord
    for rows.Next() {
        var e EventRecord
        if err := rows.Scan(&e.ID, &e.RunID, &e.Sequence, &e.EventType,
            &e.UnitID, &e.PayloadJSON, &e.CreatedAt); err != nil {
            return nil, err
        }
        events = append(events, &e)
    }
    return events, rows.Err()
}
```

---

## 7. Event System Updates

### 7.1 Sequence Numbers

Events need sequence numbers for replay during reconnection:

```go
// internal/events/bus.go

type Event struct {
    Sequence int         `json:"sequence"`
    Time     time.Time   `json:"time"`
    Type     EventType   `json:"type"`
    Unit     string      `json:"unit,omitempty"`
    Task     *int        `json:"task,omitempty"`
    PR       *int        `json:"pr,omitempty"`
    Payload  any         `json:"payload,omitempty"`
    Error    string      `json:"error,omitempty"`
}

type Bus struct {
    handlers []Handler
    ch       chan Event
    done     chan struct{}

    mu       sync.Mutex
    sequence int
}

func (b *Bus) Emit(e Event) {
    b.mu.Lock()
    b.sequence++
    e.Sequence = b.sequence
    b.mu.Unlock()

    e.Time = time.Now()
    select {
    case b.ch <- e:
    default:
        log.Printf("WARN: event buffer full, dropping %s", e.Type)
    }
}
```

---

## 8. Configuration

### 8.1 Daemon Configuration

```yaml
# ~/.choo/config.yaml

daemon:
  # Unix socket path
  socket_path: ~/.choo/daemon.sock

  # PID file path
  pid_file: ~/.choo/daemon.pid

  # SQLite database path
  db_path: ~/.choo/choo.db

  # Maximum concurrent jobs
  max_jobs: 10

  # Graceful shutdown timeout (seconds)
  shutdown_timeout: 30
```

### 8.2 Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CHOO_SOCKET` | Daemon Unix socket path | `~/.choo/daemon.sock` |
| `CHOO_DB` | SQLite database path | `~/.choo/choo.db` |
| `CHOO_MAX_JOBS` | Maximum concurrent jobs | `10` |
| `CHOO_USE_DAEMON` | Default to daemon mode | `true` |

---

## 9. Implementation Phases

### Phase 1: SQLite Database Layer

**Files to create:**
- `internal/db/db.go` - Connection management, migrations
- `internal/db/runs.go` - Run CRUD operations
- `internal/db/units.go` - Unit CRUD operations
- `internal/db/events.go` - Event log operations
- `internal/db/types.go` - Database types

**Deliverables:**
- SQLite database with schema
- CRUD operations for runs, units, events
- Migration system

### Phase 2: Event System Updates

**Files to modify:**
- `internal/events/bus.go` - Add sequence numbers
- `internal/events/types.go` - Update Event struct

**Deliverables:**
- Events have sequence numbers for replay
- Event bus supports persistence hooks

### Phase 3: gRPC Interface

**Files to create:**
- `proto/choo/v1/daemon.proto` - Service definition
- `pkg/api/v1/` - Generated protobuf code (via buf)

**Deliverables:**
- gRPC service definition
- Generated Go client/server code

### Phase 4: Daemon Core

**Files to create:**
- `internal/daemon/daemon.go` - Main daemon process
- `internal/daemon/jobmanager.go` - Manages concurrent jobs
- `internal/daemon/grpc.go` - gRPC service implementation
- `internal/daemon/resume.go` - Resume jobs after restart

**Deliverables:**
- Daemon process with lifecycle management
- Job manager with concurrency control
- gRPC server implementation

### Phase 5: CLI Client Refactor

**Files to create:**
- `internal/client/client.go` - gRPC client wrapper
- `internal/cli/daemon.go` - `choo daemon start|stop|status`
- `internal/cli/watch.go` - `choo watch <job-id>`
- `internal/cli/jobs.go` - `choo jobs [--status=running]`

**Files to modify:**
- `internal/cli/run.go` - Use gRPC client when `--use-daemon`

**Deliverables:**
- CLI commands for daemon management
- `choo run` uses daemon by default
- Event streaming via `choo watch`

### Phase 6: Resume Logic

**Files to modify:**
- `internal/daemon/resume.go` - Full implementation
- `internal/orchestrator/orchestrator.go` - Add job ID, SQLite hooks

**Deliverables:**
- Daemon resumes `running` jobs on startup
- Unit states reconstructed from database
- Worktree validation and recovery

### Phase 7: Migration Path

**Tasks:**
1. Add `--use-daemon` flag (default: false initially)
2. Test daemon mode alongside inline mode
3. Flip default to daemon mode
4. Deprecate inline mode in future version

---

## 10. Files Summary

### Files to Create

| File | Purpose |
|------|---------|
| `internal/db/db.go` | SQLite connection, migrations |
| `internal/db/runs.go` | Run CRUD |
| `internal/db/units.go` | Unit CRUD |
| `internal/db/events.go` | Event log operations |
| `internal/db/types.go` | Database types |
| `proto/choo/v1/daemon.proto` | gRPC service definition |
| `internal/daemon/daemon.go` | Daemon lifecycle |
| `internal/daemon/jobmanager.go` | Multi-job coordination |
| `internal/daemon/grpc.go` | gRPC implementation |
| `internal/daemon/resume.go` | Resume on restart |
| `internal/client/client.go` | gRPC client |
| `internal/cli/daemon.go` | Daemon CLI commands |
| `internal/cli/watch.go` | Watch job events |
| `internal/cli/jobs.go` | List jobs |

### Files to Modify

| File | Changes |
|------|---------|
| `internal/cli/run.go` | Add `--use-daemon` flag, gRPC client mode |
| `internal/orchestrator/orchestrator.go` | Add job ID, SQLite persistence hooks |
| `internal/events/bus.go` | Add sequence numbers |
| `internal/events/types.go` | Update Event struct with Sequence |

---

## 11. Dependencies

New Go dependencies:

| Package | Purpose |
|---------|---------|
| `google.golang.org/grpc` | gRPC framework |
| `google.golang.org/protobuf` | Protocol Buffers |
| `modernc.org/sqlite` | Pure Go SQLite (no CGO) |
| `github.com/oklog/ulid/v2` | Sortable unique IDs |

---

## 12. Acceptance Criteria

- [ ] `choo daemon start` starts daemon and writes PID file
- [ ] `choo daemon status` shows daemon info (PID, socket, active jobs)
- [ ] `choo daemon stop` gracefully shuts down with running jobs
- [ ] `choo run` connects to daemon via gRPC by default
- [ ] `choo run --use-daemon=false` runs inline (backwards compatible)
- [ ] `choo jobs` lists all jobs with status
- [ ] `choo watch <job-id>` attaches to job event stream
- [ ] `choo stop <job-id>` stops a running job
- [ ] Multiple concurrent jobs run in isolation
- [ ] Daemon resumes `running` jobs on restart
- [ ] Events are persisted to SQLite
- [ ] Event replay works for reconnecting clients
- [ ] Unit states are persisted and resumed correctly
- [ ] Graceful shutdown waits for safe points
- [ ] PID file is cleaned up on shutdown
- [ ] Socket file is created with correct permissions

---

## 13. Verification

1. **Unit tests**: Each new package (`db`, `daemon`, `client`) has tests
2. **Integration test**: Start daemon, run job via CLI, verify SQLite state
3. **Resume test**: Start job, kill daemon (SIGKILL), restart, verify resume
4. **Concurrent jobs test**: Start 2 jobs on different branches, verify isolation
5. **Reconnection test**: Watch job, disconnect, reconnect, verify event replay
6. **Manual testing**:
   - `choo daemon start` in one terminal
   - `choo run specs/tasks` in another terminal
   - `choo jobs` shows running job
   - `choo watch <job-id>` reconnects to event stream
   - Kill daemon, restart, verify job resumes
   - `choo daemon stop` waits for jobs then exits

---

## 14. Migration Path

1. **Phase 1**: Add SQLite layer (non-breaking, dual-write with frontmatter)
2. **Phase 2**: Add gRPC interface and daemon
3. **Phase 3**: Add `--use-daemon` flag to `choo run` (default: false)
4. **Phase 4**: Flip default to daemon mode (`--use-daemon=true`)
5. **Phase 5**: Deprecate inline mode with warning
6. **Phase 6**: Remove inline mode

---

## 15. Open Questions

1. **Database location**: Should `~/.choo/choo.db` be per-repo or global?
   - Global enables cross-repo job listing
   - Per-repo isolates state but complicates job discovery

2. **Daemon auto-start**: Should `choo run` auto-start daemon if not running?
   - Pro: Better UX, no manual daemon management
   - Con: Implicit process creation may surprise users

3. **Event retention**: How long should events be retained in SQLite?
   - Options: Forever, N days, N events per run
   - Impact: Database size vs debugging capability

4. **Worktree validation**: On resume, if worktree exists but is stale?
   - Option A: Delete and recreate
   - Option B: Mark unit as failed, require manual intervention
   - Option C: Attempt to detect and recover

5. **Job limits**: Per-repo or global max jobs?
   - Global simpler, per-repo more flexible
