---
task: 3
status: complete
backpressure: "go test ./internal/daemon/... -run TestGRPC_Job"
depends_on: [1, 2]
---

# Job Lifecycle RPCs

**Parent spec**: `specs/DAEMON-GRPC.md`
**Task**: #3 of 7 in implementation plan

## Objective

Implement the four job lifecycle RPC methods: StartJob, StopJob, GetJobStatus, and ListJobs.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Proto definitions for request/response types
- Task #2: GRPCServer struct and JobManager interface

### Package Dependencies
- `github.com/RevCBH/choo/pkg/api/v1` - Generated protobuf types
- `google.golang.org/grpc/codes` - gRPC status codes
- `google.golang.org/grpc/status` - gRPC error handling

## Deliverables

### Files to Create/Modify

```
internal/daemon/
├── grpc.go         # MODIFY: Add RPC method implementations
└── grpc_test.go    # CREATE: Unit tests for job RPCs
```

### Functions to Implement

```go
// internal/daemon/grpc.go

// StartJob creates and starts a new orchestration job
func (s *GRPCServer) StartJob(ctx context.Context, req *apiv1.StartJobRequest) (*apiv1.StartJobResponse, error) {
    // Check if server is shutting down
    if s.isShuttingDown() {
        return nil, status.Errorf(codes.Unavailable, "daemon is shutting down")
    }

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

    // Create job context for cancellation
    jobCtx, cancel := context.WithCancel(context.Background())

    jobID, err := s.jobManager.Start(jobCtx, JobConfig{
        TasksDir:      req.TasksDir,
        TargetBranch:  req.TargetBranch,
        FeatureBranch: req.FeatureBranch,
        Parallelism:   int(req.Parallelism),
        RepoPath:      req.RepoPath,
    })
    if err != nil {
        cancel() // Clean up context
        return nil, status.Errorf(codes.Internal, "failed to start job: %v", err)
    }

    // Track job for shutdown coordination
    s.trackJob(jobID, cancel)

    return &apiv1.StartJobResponse{
        JobId:  jobID,
        Status: "running",
    }, nil
}

// StopJob gracefully stops a running job
func (s *GRPCServer) StopJob(ctx context.Context, req *apiv1.StopJobRequest) (*apiv1.StopJobResponse, error) {
    // Validate required fields
    if req.JobId == "" {
        return nil, status.Errorf(codes.InvalidArgument, "job_id is required")
    }

    // Check if job exists
    job, err := s.jobManager.GetJob(req.JobId)
    if err != nil {
        return nil, status.Errorf(codes.NotFound, "job not found: %s", req.JobId)
    }

    // Check if job is already stopped
    if job.Status == "completed" || job.Status == "failed" || job.Status == "cancelled" {
        return nil, status.Errorf(codes.FailedPrecondition, "job already stopped: %s", job.Status)
    }

    // Stop the job
    if err := s.jobManager.Stop(ctx, req.JobId, req.Force); err != nil {
        return nil, status.Errorf(codes.Internal, "failed to stop job: %v", err)
    }

    // Untrack job
    s.untrackJob(req.JobId)

    message := "job stopped gracefully"
    if req.Force {
        message = "job force killed"
    }

    return &apiv1.StopJobResponse{
        Success: true,
        Message: message,
    }, nil
}

// GetJobStatus returns the current status of a job
func (s *GRPCServer) GetJobStatus(ctx context.Context, req *apiv1.GetJobStatusRequest) (*apiv1.GetJobStatusResponse, error) {
    // Validate required fields
    if req.JobId == "" {
        return nil, status.Errorf(codes.InvalidArgument, "job_id is required")
    }

    // Get job state
    job, err := s.jobManager.GetJob(req.JobId)
    if err != nil {
        return nil, status.Errorf(codes.NotFound, "job not found: %s", req.JobId)
    }

    return jobStateToProto(job), nil
}

// ListJobs returns all jobs matching the optional status filter
func (s *GRPCServer) ListJobs(ctx context.Context, req *apiv1.ListJobsRequest) (*apiv1.ListJobsResponse, error) {
    jobs, err := s.jobManager.ListJobs(req.StatusFilter)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to list jobs: %v", err)
    }

    resp := &apiv1.ListJobsResponse{}
    for _, j := range jobs {
        resp.Jobs = append(resp.Jobs, jobSummaryToProto(j))
    }

    return resp, nil
}
```

### Test Implementation

```go
// internal/daemon/grpc_test.go

package daemon

import (
    "context"
    "errors"
    "testing"
    "time"

    apiv1 "github.com/RevCBH/choo/pkg/api/v1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// mockJobManager implements JobManager for testing
type mockJobManager struct {
    jobs          map[string]*JobState
    startErr      error
    stopErr       error
    stoppedJobs   map[string]bool
    forceStopped  map[string]bool
}

func newMockJobManager() *mockJobManager {
    return &mockJobManager{
        jobs:         make(map[string]*JobState),
        stoppedJobs:  make(map[string]bool),
        forceStopped: make(map[string]bool),
    }
}

func (m *mockJobManager) Start(ctx context.Context, cfg JobConfig) (string, error) {
    if m.startErr != nil {
        return "", m.startErr
    }
    jobID := "job-" + time.Now().Format("20060102150405")
    now := time.Now()
    m.jobs[jobID] = &JobState{
        ID:        jobID,
        Status:    "running",
        StartedAt: &now,
    }
    return jobID, nil
}

func (m *mockJobManager) Stop(ctx context.Context, jobID string, force bool) error {
    if m.stopErr != nil {
        return m.stopErr
    }
    m.stoppedJobs[jobID] = true
    if force {
        m.forceStopped[jobID] = true
    }
    if job, ok := m.jobs[jobID]; ok {
        job.Status = "cancelled"
    }
    return nil
}

func (m *mockJobManager) GetJob(jobID string) (*JobState, error) {
    job, ok := m.jobs[jobID]
    if !ok {
        return nil, errors.New("job not found")
    }
    return job, nil
}

func (m *mockJobManager) ListJobs(statusFilter []string) ([]*JobSummary, error) {
    var result []*JobSummary
    for _, job := range m.jobs {
        if len(statusFilter) > 0 {
            found := false
            for _, s := range statusFilter {
                if job.Status == s {
                    found = true
                    break
                }
            }
            if !found {
                continue
            }
        }
        result = append(result, &JobSummary{
            JobID:     job.ID,
            Status:    job.Status,
            StartedAt: job.StartedAt,
        })
    }
    return result, nil
}

func (m *mockJobManager) Subscribe(jobID string, fromSeq int) (<-chan Event, func()) {
    ch := make(chan Event)
    return ch, func() { close(ch) }
}

func (m *mockJobManager) ActiveJobCount() int {
    count := 0
    for _, job := range m.jobs {
        if job.Status == "running" {
            count++
        }
    }
    return count
}

func (m *mockJobManager) addJob(id string, status string) {
    now := time.Now()
    m.jobs[id] = &JobState{
        ID:        id,
        Status:    status,
        StartedAt: &now,
    }
}

func TestGRPC_JobStartJob_ValidatesRequiredFields(t *testing.T) {
    jm := newMockJobManager()
    server := NewGRPCServer(nil, jm, "v1.0.0")

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
                require.Error(t, err)
                assert.Equal(t, tt.wantErr, status.Code(err))
            }
        })
    }
}

func TestGRPC_JobStartJob_RejectsWhenShuttingDown(t *testing.T) {
    jm := newMockJobManager()
    server := NewGRPCServer(nil, jm, "v1.0.0")
    server.setShuttingDown()

    req := &apiv1.StartJobRequest{
        TasksDir:     "/tasks",
        TargetBranch: "main",
        RepoPath:     "/repo",
    }

    _, err := server.StartJob(context.Background(), req)
    require.Error(t, err)
    assert.Equal(t, codes.Unavailable, status.Code(err))
}

func TestGRPC_JobStopJob_GracefulStop(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-123", "running")
    server := NewGRPCServer(nil, jm, "v1.0.0")

    resp, err := server.StopJob(context.Background(), &apiv1.StopJobRequest{
        JobId: "job-123",
        Force: false,
    })

    require.NoError(t, err)
    assert.True(t, resp.Success)
    assert.True(t, jm.stoppedJobs["job-123"])
    assert.False(t, jm.forceStopped["job-123"])
}

func TestGRPC_JobStopJob_ForceStop(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-456", "running")
    server := NewGRPCServer(nil, jm, "v1.0.0")

    resp, err := server.StopJob(context.Background(), &apiv1.StopJobRequest{
        JobId: "job-456",
        Force: true,
    })

    require.NoError(t, err)
    assert.True(t, resp.Success)
    assert.True(t, jm.forceStopped["job-456"])
}

func TestGRPC_JobStopJob_NotFound(t *testing.T) {
    jm := newMockJobManager()
    server := NewGRPCServer(nil, jm, "v1.0.0")

    _, err := server.StopJob(context.Background(), &apiv1.StopJobRequest{
        JobId: "nonexistent",
    })

    require.Error(t, err)
    assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestGRPC_JobStopJob_AlreadyStopped(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-done", "completed")
    server := NewGRPCServer(nil, jm, "v1.0.0")

    _, err := server.StopJob(context.Background(), &apiv1.StopJobRequest{
        JobId: "job-done",
    })

    require.Error(t, err)
    assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestGRPC_JobGetJobStatus(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-status", "running")
    server := NewGRPCServer(nil, jm, "v1.0.0")

    resp, err := server.GetJobStatus(context.Background(), &apiv1.GetJobStatusRequest{
        JobId: "job-status",
    })

    require.NoError(t, err)
    assert.Equal(t, "job-status", resp.JobId)
    assert.Equal(t, "running", resp.Status)
}

func TestGRPC_JobListJobs_NoFilter(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-1", "running")
    jm.addJob("job-2", "completed")
    server := NewGRPCServer(nil, jm, "v1.0.0")

    resp, err := server.ListJobs(context.Background(), &apiv1.ListJobsRequest{})

    require.NoError(t, err)
    assert.Len(t, resp.Jobs, 2)
}

func TestGRPC_JobListJobs_WithFilter(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-1", "running")
    jm.addJob("job-2", "completed")
    jm.addJob("job-3", "running")
    server := NewGRPCServer(nil, jm, "v1.0.0")

    resp, err := server.ListJobs(context.Background(), &apiv1.ListJobsRequest{
        StatusFilter: []string{"running"},
    })

    require.NoError(t, err)
    assert.Len(t, resp.Jobs, 2)
    for _, job := range resp.Jobs {
        assert.Equal(t, "running", job.Status)
    }
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestGRPC_Job
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGRPC_JobStartJob_ValidatesRequiredFields` | All validation cases pass |
| `TestGRPC_JobStartJob_RejectsWhenShuttingDown` | Returns Unavailable during shutdown |
| `TestGRPC_JobStopJob_GracefulStop` | Graceful stop recorded |
| `TestGRPC_JobStopJob_ForceStop` | Force stop flag propagated |
| `TestGRPC_JobStopJob_NotFound` | Returns NotFound for missing job |
| `TestGRPC_JobStopJob_AlreadyStopped` | Returns FailedPrecondition |
| `TestGRPC_JobGetJobStatus` | Returns correct job state |
| `TestGRPC_JobListJobs_NoFilter` | Returns all jobs |
| `TestGRPC_JobListJobs_WithFilter` | Filters by status |

## NOT In Scope

- WatchJob streaming (task #4)
- Shutdown/Health RPCs (task #5)
- Socket server setup (task #6)
- Real JobManager implementation
