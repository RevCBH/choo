# DAEMON-GRPC — gRPC Interface and Protocol Buffer Definitions for Daemon Communication

## Overview

DAEMON-GRPC defines the remote procedure call interface that enables CLI tools and external clients to communicate with the Charlotte daemon process. The daemon runs as a long-lived background service managing job orchestration, and gRPC provides the transport layer for job control, status queries, and real-time event streaming.

This component serves as the primary integration point between user-facing tools and the daemon's internal job management system. By using gRPC with Protocol Buffers, clients benefit from strongly-typed contracts, efficient binary serialization, and native support for bidirectional streaming required by the event watch functionality.

```
┌─────────────────┐     gRPC/HTTP2      ┌──────────────────────────────┐
│   CLI Client    │◄──────────────────► │         Daemon               │
│   (choo run)    │                     │  ┌────────────────────────┐  │
└─────────────────┘                     │  │     GRPCServer         │  │
                                        │  │  - StartJob            │  │
┌─────────────────┐     gRPC/HTTP2      │  │  - StopJob             │  │
│   Web UI        │◄──────────────────► │  │  - GetJobStatus        │  │
│   (optional)    │                     │  │  - ListJobs            │  │
└─────────────────┘                     │  │  - WatchJob (stream)   │  │
                                        │  │  - Shutdown            │  │
                                        │  │  - Health              │  │
                                        │  └───────────┬────────────┘  │
                                        │              │               │
                                        │  ┌───────────▼────────────┐  │
                                        │  │     JobManager         │  │
                                        │  └────────────────────────┘  │
                                        └──────────────────────────────┘
```

## Requirements

### Functional Requirements

1. **Job Lifecycle Management**: Clients must be able to start, stop, and query the status of orchestration jobs through the gRPC interface.

2. **Real-time Event Streaming**: The WatchJob RPC must provide server-side streaming of job events, supporting resume from a specific sequence number for reconnection scenarios.

3. **Job Enumeration**: Clients must be able to list all jobs with optional filtering by status (running, completed, failed).

4. **Daemon Lifecycle Control**: The interface must support graceful shutdown with configurable wait behavior for in-progress jobs.

5. **Health Checking**: A health endpoint must report daemon status, active job count, and version information for monitoring and service discovery.

6. **Concurrent Client Support**: Multiple clients must be able to connect simultaneously and receive independent event streams.

7. **Error Propagation**: All errors must be returned using standard gRPC status codes with descriptive messages.

### Performance Requirements

| Metric | Target |
|--------|--------|
| StartJob latency | < 50ms to return job ID |
| GetJobStatus latency | < 10ms |
| Event stream latency | < 100ms from internal event to client delivery |
| Concurrent streams | Support 100+ simultaneous WatchJob streams |
| Connection overhead | < 5MB memory per connected client |

### Constraints

- Requires Go 1.21+ for generics support in server implementation
- Protocol Buffers v3 syntax required for optional field support
- Unix domain sockets preferred for local daemon communication (lower overhead than TCP)
- Must maintain backward compatibility with existing protobuf message definitions

## Design

### Module Structure

```
proto/choo/v1/
└── daemon.proto           # Service and message definitions

pkg/api/v1/
├── daemon.pb.go           # Generated message types
└── daemon_grpc.pb.go      # Generated service stubs

internal/daemon/
├── grpc.go                # GRPCServer implementation
├── grpc_test.go           # Server unit tests
└── server.go              # Main daemon server (listener setup)
```

### Core Types

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
```

#### Request/Response Messages

```protobuf
// StartJob creates and starts a new job
message StartJobRequest {
    string tasks_dir = 1;       // Path to directory containing task YAML files
    string target_branch = 2;   // Base branch for PRs (e.g., "main")
    string feature_branch = 3;  // Optional: for feature mode, omit for PR mode
    int32 parallelism = 4;      // Max concurrent units (0 = default from config)
    string repo_path = 5;       // Absolute path to git repository
}

message StartJobResponse {
    string job_id = 1;          // Unique identifier for the created job
    string status = 2;          // Initial status, typically "running"
}

// StopJob gracefully stops a running job
message StopJobRequest {
    string job_id = 1;
    bool force = 2;             // If true, kill immediately without waiting for cleanup
}

message StopJobResponse {
    bool success = 1;
    string message = 2;         // Human-readable result description
}

// GetJobStatus returns current status of a job
message GetJobStatusRequest {
    string job_id = 1;
}

message GetJobStatusResponse {
    string job_id = 1;
    string status = 2;                          // "pending", "running", "completed", "failed"
    google.protobuf.Timestamp started_at = 3;
    google.protobuf.Timestamp completed_at = 4; // Zero if still running
    string error = 5;                           // Error message if failed
    repeated UnitStatus units = 6;              // Status of each execution unit
}

message UnitStatus {
    string unit_id = 1;
    string status = 2;          // "pending", "running", "completed", "failed"
    int32 tasks_complete = 3;
    int32 tasks_total = 4;
    int32 pr_number = 5;        // GitHub PR number, 0 if not yet created
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
    string feature_branch = 2;              // Empty for PR mode jobs
    string status = 3;
    google.protobuf.Timestamp started_at = 4;
    int32 units_complete = 5;
    int32 units_total = 6;
}

// WatchJob streams events for a running job
message WatchJobRequest {
    string job_id = 1;
    int32 from_sequence = 2;    // Resume from sequence number (0 = beginning)
}

message JobEvent {
    int32 sequence = 1;         // Monotonically increasing per job
    string event_type = 2;      // "unit_started", "task_completed", "unit_failed", etc.
    string unit_id = 3;         // Which unit this event relates to (empty for job-level)
    string payload_json = 4;    // Event-specific data as JSON
    google.protobuf.Timestamp timestamp = 5;
}

// Shutdown gracefully shuts down the daemon
message ShutdownRequest {
    bool wait_for_jobs = 1;     // If true, wait for running jobs to complete
    int32 timeout_seconds = 2;  // Max wait time before force shutdown (0 = no timeout)
}

message ShutdownResponse {
    bool success = 1;
    int32 jobs_stopped = 2;     // Number of jobs that were interrupted
}

// Health check
message HealthRequest {}

message HealthResponse {
    bool healthy = 1;
    int32 active_jobs = 2;
    string version = 3;         // Daemon version string
}
```

### API Surface

```go
// internal/daemon/grpc.go

package daemon

import (
    "context"

    apiv1 "github.com/charlotte/pkg/api/v1"
    "github.com/charlotte/internal/db"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// GRPCServer implements the DaemonService gRPC interface
type GRPCServer struct {
    apiv1.UnimplementedDaemonServiceServer

    jobManager *JobManager
    db         *db.DB
}

// NewGRPCServer creates a new gRPC server instance
func NewGRPCServer(jm *JobManager, db *db.DB) *GRPCServer

// StartJob creates and starts a new orchestration job
func (s *GRPCServer) StartJob(ctx context.Context, req *apiv1.StartJobRequest) (*apiv1.StartJobResponse, error)

// StopJob gracefully stops a running job
func (s *GRPCServer) StopJob(ctx context.Context, req *apiv1.StopJobRequest) (*apiv1.StopJobResponse, error)

// GetJobStatus returns the current status of a job
func (s *GRPCServer) GetJobStatus(ctx context.Context, req *apiv1.GetJobStatusRequest) (*apiv1.GetJobStatusResponse, error)

// ListJobs returns all jobs matching the optional status filter
func (s *GRPCServer) ListJobs(ctx context.Context, req *apiv1.ListJobsRequest) (*apiv1.ListJobsResponse, error)

// WatchJob streams events for a job until completion or client disconnect
func (s *GRPCServer) WatchJob(req *apiv1.WatchJobRequest, stream apiv1.DaemonService_WatchJobServer) error

// Shutdown initiates graceful daemon shutdown
func (s *GRPCServer) Shutdown(ctx context.Context, req *apiv1.ShutdownRequest) (*apiv1.ShutdownResponse, error)

// Health returns daemon health status
func (s *GRPCServer) Health(ctx context.Context, req *apiv1.HealthRequest) (*apiv1.HealthResponse, error)
```

### Server Implementation Details

```go
func (s *GRPCServer) StartJob(ctx context.Context, req *apiv1.StartJobRequest) (*apiv1.StartJobResponse, error) {
    // Validate required fields
    if req.TasksDir == "" {
        return nil, status.Errorf(codes.InvalidArgument, "tasks_dir is required")
    }
    if req.TargetBranch == "" {
        return nil, status.Errorf(codes.InvalidArgument, "target_branch is required")
    }
    if req.RepoPath == "" {
        return nil, status.Errorf(codes.InvalidArgument, "repo_path is required")
    }

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
    // Validate job exists
    if _, err := s.jobManager.GetJob(req.JobId); err != nil {
        return status.Errorf(codes.NotFound, "job not found: %s", req.JobId)
    }

    // Subscribe to job events starting from requested sequence
    events, unsub := s.jobManager.Subscribe(req.JobId, int(req.FromSequence))
    defer unsub()

    for event := range events {
        if err := stream.Send(eventToProto(event)); err != nil {
            // Client disconnected or stream error
            return err
        }
    }

    // Channel closed means job completed
    return nil
}
```

### gRPC Status Code Mapping

| Condition | gRPC Code | Example |
|-----------|-----------|---------|
| Job not found | `NotFound` | GetJobStatus with invalid ID |
| Missing required field | `InvalidArgument` | StartJob without tasks_dir |
| Job already stopped | `FailedPrecondition` | StopJob on completed job |
| Internal error | `Internal` | Database failure |
| Shutdown in progress | `Unavailable` | StartJob during shutdown |

## Implementation Notes

### Unix Domain Socket Setup

The daemon should listen on a Unix domain socket for local communication to minimize latency and avoid TCP overhead:

```go
func (d *Daemon) Start() error {
    // Socket path from daemon config (~/.choo/daemon.sock)
    socketPath := d.cfg.SocketPath

    // Remove stale socket file
    os.Remove(socketPath)

    listener, err := net.Listen("unix", socketPath)
    if err != nil {
        return fmt.Errorf("failed to listen: %w", err)
    }

    // Set permissions so only current user can connect
    os.Chmod(socketPath, 0600)

    server := grpc.NewServer()
    apiv1.RegisterDaemonServiceServer(server, d.grpcServer)

    return server.Serve(listener)
}
```

### Event Subscription Pattern

The JobManager must support multiple concurrent subscribers with replay capability:

```go
type JobManager struct {
    mu          sync.RWMutex
    jobs        map[string]*Job
    subscribers map[string][]chan Event
}

// Subscribe returns a channel of events and an unsubscribe function
func (jm *JobManager) Subscribe(jobID string, fromSeq int) (<-chan Event, func()) {
    jm.mu.Lock()
    defer jm.mu.Unlock()

    ch := make(chan Event, 100) // Buffer to prevent blocking
    jm.subscribers[jobID] = append(jm.subscribers[jobID], ch)

    // Replay historical events from database
    go func() {
        events, _ := jm.db.GetEventsFrom(jobID, fromSeq)
        for _, e := range events {
            ch <- e
        }
    }()

    return ch, func() {
        jm.mu.Lock()
        defer jm.mu.Unlock()
        // Remove channel from subscribers
        // Close channel after removal
    }
}
```

### Graceful Shutdown Sequence

1. Stop accepting new gRPC connections
2. Cancel context for all in-progress StartJob calls
3. If `wait_for_jobs=true`, wait for running jobs (up to timeout)
4. If `wait_for_jobs=false` or timeout exceeded, cancel all jobs
5. Close all event streams (triggers client reconnect logic)
6. Close database connections
7. Return ShutdownResponse

## Testing Strategy

### Unit Tests

```go
func TestStartJob_ValidatesRequiredFields(t *testing.T) {
    jm := NewMockJobManager()
    server := NewGRPCServer(jm, nil)

    tests := []struct {
        name    string
        req     *apiv1.StartJobRequest
        wantErr codes.Code
    }{
        {
            name:    "missing tasks_dir",
            req:     &apiv1.StartJobRequest{TargetBranch: "main", RepoPath: "/repo"},
            wantErr: codes.InvalidArgument,
        },
        {
            name:    "missing target_branch",
            req:     &apiv1.StartJobRequest{TasksDir: "/tasks", RepoPath: "/repo"},
            wantErr: codes.InvalidArgument,
        },
        {
            name:    "missing repo_path",
            req:     &apiv1.StartJobRequest{TasksDir: "/tasks", TargetBranch: "main"},
            wantErr: codes.InvalidArgument,
        },
        {
            name: "valid request",
            req: &apiv1.StartJobRequest{
                TasksDir:     "/tasks",
                TargetBranch: "main",
                RepoPath:     "/repo",
            },
            wantErr: codes.OK,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := server.StartJob(context.Background(), tt.req)
            if tt.wantErr == codes.OK {
                require.NoError(t, err)
            } else {
                require.Equal(t, tt.wantErr, status.Code(err))
            }
        })
    }
}

func TestWatchJob_ReplaysPastEvents(t *testing.T) {
    db := setupTestDB(t)
    jm := NewJobManager(db)
    server := NewGRPCServer(jm, db)

    // Create job and emit some events
    jobID := jm.CreateTestJob()
    jm.EmitEvent(jobID, Event{Sequence: 1, Type: "unit_started"})
    jm.EmitEvent(jobID, Event{Sequence: 2, Type: "task_completed"})
    jm.EmitEvent(jobID, Event{Sequence: 3, Type: "unit_completed"})

    // Watch from sequence 2 should replay events 2 and 3
    stream := &mockWatchStream{events: make(chan *apiv1.JobEvent, 10)}
    go server.WatchJob(&apiv1.WatchJobRequest{
        JobId:        jobID,
        FromSequence: 2,
    }, stream)

    received := collectEvents(stream, 2, time.Second)
    require.Len(t, received, 2)
    require.Equal(t, int32(2), received[0].Sequence)
    require.Equal(t, int32(3), received[1].Sequence)
}

func TestStopJob_ForceFlagBehavior(t *testing.T) {
    jm := NewMockJobManager()
    server := NewGRPCServer(jm, nil)

    jobID := jm.CreateRunningJob()

    // Graceful stop
    resp, err := server.StopJob(context.Background(), &apiv1.StopJobRequest{
        JobId: jobID,
        Force: false,
    })
    require.NoError(t, err)
    require.True(t, resp.Success)
    require.True(t, jm.WasGracefullyStopped(jobID))

    // Force stop
    jobID2 := jm.CreateRunningJob()
    resp, err = server.StopJob(context.Background(), &apiv1.StopJobRequest{
        JobId: jobID2,
        Force: true,
    })
    require.NoError(t, err)
    require.True(t, resp.Success)
    require.True(t, jm.WasForceKilled(jobID2))
}
```

### Integration Tests

- **Full job lifecycle**: Start job via gRPC, watch events, verify completion
- **Concurrent clients**: Multiple CLI instances watching same job
- **Reconnection**: Client disconnects and resumes from last sequence
- **Shutdown during job**: Verify jobs are properly cancelled or waited for
- **Socket permissions**: Non-owner cannot connect to daemon socket

### Manual Testing

- [ ] Start daemon, verify socket created with correct permissions
- [ ] Run `choo run` with valid tasks, verify job starts and events stream
- [ ] Kill CLI process, reconnect, verify event replay works
- [ ] Run `choo status` while job running, verify accurate unit status
- [ ] Run `choo stop <job-id>`, verify graceful shutdown
- [ ] Run `choo stop --force <job-id>`, verify immediate termination
- [ ] Send SIGTERM to daemon, verify jobs complete before exit (with wait_for_jobs)
- [ ] Query health endpoint, verify accurate job count and version

## Design Decisions

### Why gRPC over REST?

gRPC was chosen for several reasons:

1. **Native streaming support**: WatchJob requires server-side streaming which gRPC handles natively. REST would require WebSockets or long-polling, adding complexity.

2. **Strongly-typed contracts**: Protocol Buffers provide compile-time type safety for both Go server and potential future clients (web UI, other languages).

3. **Efficient binary protocol**: Job events can be high-frequency; protobuf encoding is significantly smaller and faster than JSON.

4. **Code generation**: Generated client/server stubs eliminate hand-written serialization code and reduce bugs.

Trade-off: gRPC is harder to debug with standard tools like curl. This is mitigated by the Health endpoint and structured logging.

### Why Unix Domain Sockets?

For local daemon communication, Unix sockets offer:

1. **Lower latency**: No TCP handshake or network stack overhead
2. **File-based permissions**: OS-level access control via socket file permissions
3. **No port conflicts**: Avoids binding to TCP ports that may be in use

Trade-off: Cannot support remote daemon connections. If remote access is needed in the future, we can add a TCP listener behind authentication.

### Why JSON in payload_json?

The `payload_json` field in JobEvent uses JSON string rather than protobuf Any:

1. **Flexibility**: Event payloads vary significantly by event type (task output, error details, PR URLs)
2. **Debugging**: JSON is human-readable in logs and database
3. **Schema evolution**: New event types can add fields without protobuf schema changes

Trade-off: Loses type safety for event payloads. Mitigated by defining payload structures in Go and using json.Marshal/Unmarshal.

## Future Enhancements

- **TLS support**: Add mTLS for secure remote daemon communication
- **Authentication**: Token-based auth for multi-user environments
- **Rate limiting**: Prevent runaway clients from overwhelming daemon
- **Metrics endpoint**: Prometheus-compatible metrics for observability
- **Reflection API**: Enable grpcurl and other tools for debugging
- **Batch operations**: StopAllJobs, GetMultipleJobStatus for efficiency
- **Event filtering**: Allow WatchJob to filter by event type or unit

## References

- [gRPC Go Documentation](https://grpc.io/docs/languages/go/)
- [Protocol Buffers Language Guide](https://protobuf.dev/programming-guides/proto3/)
- [gRPC Status Codes](https://grpc.io/docs/guides/status-codes/)
- Internal: `docs/architecture.md` - Overall system design
- Internal: `internal/daemon/job_manager.go` - JobManager interface
