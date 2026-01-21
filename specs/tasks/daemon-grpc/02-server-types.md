---
task: 2
status: pending
backpressure: "go build ./internal/daemon/..."
depends_on: [1]
---

# gRPC Server Types

**Parent spec**: `specs/DAEMON-GRPC.md`
**Task**: #2 of 7 in implementation plan

## Objective

Define the GRPCServer struct with its dependencies and constructor function.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Proto definitions must be generated for apiv1 types

### Package Dependencies
- `github.com/RevCBH/choo/pkg/api/v1` - Generated protobuf types
- `github.com/RevCBH/choo/internal/daemon/db` - Database layer (from DAEMON-DB)
- `google.golang.org/grpc` - gRPC framework
- `google.golang.org/grpc/codes` - gRPC status codes
- `google.golang.org/grpc/status` - gRPC error handling

## Deliverables

### Files to Create/Modify

```
internal/daemon/
├── grpc.go            # CREATE: GRPCServer struct and constructor
└── grpc_helpers.go    # CREATE: Helper functions and type conversions
```

### Types to Implement

```go
// internal/daemon/grpc.go

package daemon

import (
    "sync"
    "context"

    apiv1 "github.com/RevCBH/choo/pkg/api/v1"
    "github.com/RevCBH/choo/internal/daemon/db"
)

// GRPCServer implements the DaemonService gRPC interface
type GRPCServer struct {
    apiv1.UnimplementedDaemonServiceServer

    db         *db.DB
    jobManager JobManager
    version    string

    // Shutdown coordination
    mu              sync.RWMutex
    shuttingDown    bool
    shutdownCh      chan struct{}
    activeJobs      map[string]context.CancelFunc
}

// JobManager defines the interface for job lifecycle management
// This abstraction allows the gRPC layer to remain decoupled from
// the actual job execution implementation
type JobManager interface {
    // Start creates and starts a new job, returning the job ID
    Start(ctx context.Context, cfg JobConfig) (string, error)

    // Stop gracefully stops a running job
    Stop(ctx context.Context, jobID string, force bool) error

    // GetJob returns the current state of a job
    GetJob(jobID string) (*JobState, error)

    // ListJobs returns all jobs, optionally filtered by status
    ListJobs(statusFilter []string) ([]*JobSummary, error)

    // Subscribe returns a channel of events for a job starting from sequence
    Subscribe(jobID string, fromSeq int) (<-chan Event, func())

    // ActiveJobCount returns the number of currently running jobs
    ActiveJobCount() int
}

// JobConfig contains configuration for starting a new job
type JobConfig struct {
    TasksDir      string
    TargetBranch  string
    FeatureBranch string
    Parallelism   int
    RepoPath      string
}

// JobState represents the full state of a job
type JobState struct {
    ID          string
    Status      string
    StartedAt   *time.Time
    CompletedAt *time.Time
    Error       *string
    Units       []*UnitState
}

// UnitState represents the state of a unit within a job
type UnitState struct {
    UnitID        string
    Status        string
    TasksComplete int
    TasksTotal    int
    PRNumber      int
}

// JobSummary is a condensed view of a job for listing
type JobSummary struct {
    JobID         string
    FeatureBranch string
    Status        string
    StartedAt     *time.Time
    UnitsComplete int
    UnitsTotal    int
}

// Event represents a job event for streaming
type Event struct {
    Sequence    int
    EventType   string
    UnitID      string
    PayloadJSON string
    Timestamp   time.Time
}
```

### Functions to Implement

```go
// NewGRPCServer creates a new gRPC server instance
func NewGRPCServer(db *db.DB, jm JobManager, version string) *GRPCServer {
    return &GRPCServer{
        db:         db,
        jobManager: jm,
        version:    version,
        shutdownCh: make(chan struct{}),
        activeJobs: make(map[string]context.CancelFunc),
    }
}

// isShuttingDown returns true if the server is in shutdown mode
func (s *GRPCServer) isShuttingDown() bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.shuttingDown
}

// setShuttingDown marks the server as shutting down
func (s *GRPCServer) setShuttingDown() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.shuttingDown = true
    close(s.shutdownCh)
}

// trackJob registers a running job for shutdown coordination
func (s *GRPCServer) trackJob(jobID string, cancel context.CancelFunc) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.activeJobs[jobID] = cancel
}

// untrackJob removes a job from shutdown tracking
func (s *GRPCServer) untrackJob(jobID string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.activeJobs, jobID)
}
```

### Helper Functions

```go
// internal/daemon/grpc_helpers.go

package daemon

import (
    "time"

    apiv1 "github.com/RevCBH/choo/pkg/api/v1"
    "google.golang.org/protobuf/types/known/timestamppb"
)

// timeToProto converts a Go time.Time pointer to protobuf Timestamp
func timeToProto(t *time.Time) *timestamppb.Timestamp {
    if t == nil || t.IsZero() {
        return nil
    }
    return timestamppb.New(*t)
}

// unitStateToProto converts internal UnitState to protobuf UnitStatus
func unitStateToProto(u *UnitState) *apiv1.UnitStatus {
    return &apiv1.UnitStatus{
        UnitId:        u.UnitID,
        Status:        u.Status,
        TasksComplete: int32(u.TasksComplete),
        TasksTotal:    int32(u.TasksTotal),
        PrNumber:      int32(u.PRNumber),
    }
}

// jobStateToProto converts internal JobState to protobuf GetJobStatusResponse
func jobStateToProto(j *JobState) *apiv1.GetJobStatusResponse {
    resp := &apiv1.GetJobStatusResponse{
        JobId:       j.ID,
        Status:      j.Status,
        StartedAt:   timeToProto(j.StartedAt),
        CompletedAt: timeToProto(j.CompletedAt),
    }
    if j.Error != nil {
        resp.Error = *j.Error
    }
    for _, u := range j.Units {
        resp.Units = append(resp.Units, unitStateToProto(u))
    }
    return resp
}

// jobSummaryToProto converts internal JobSummary to protobuf JobSummary
func jobSummaryToProto(j *JobSummary) *apiv1.JobSummary {
    return &apiv1.JobSummary{
        JobId:         j.JobID,
        FeatureBranch: j.FeatureBranch,
        Status:        j.Status,
        StartedAt:     timeToProto(j.StartedAt),
        UnitsComplete: int32(j.UnitsComplete),
        UnitsTotal:    int32(j.UnitsTotal),
    }
}

// eventToProto converts internal Event to protobuf JobEvent
func eventToProto(e Event) *apiv1.JobEvent {
    return &apiv1.JobEvent{
        Sequence:    int32(e.Sequence),
        EventType:   e.EventType,
        UnitId:      e.UnitID,
        PayloadJson: e.PayloadJSON,
        Timestamp:   timestamppb.New(e.Timestamp),
    }
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
| Package compiles | No build errors |
| GRPCServer embeds UnimplementedDaemonServiceServer | Compile-time interface satisfaction |
| Constructor returns valid instance | `NewGRPCServer(db, jm, "v1")` compiles |

## NOT In Scope

- RPC method implementations (tasks #3-#5)
- Socket server setup (task #6)
- Integration tests (task #7)
- JobManager concrete implementation (outside this spec)
