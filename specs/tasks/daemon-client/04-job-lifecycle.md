---
task: 4
status: pending
backpressure: "go test ./internal/client/... -run TestJob"
depends_on: [2, 3]
---

# Job Lifecycle Methods

**Parent spec**: `specs/DAEMON-CLIENT.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement the job management methods: StartJob, StopJob, ListJobs, and GetJobStatus.

## Dependencies

### Task Dependencies (within this unit)
- Task #2 must be complete (provides protobuf conversion functions)
- Task #3 must be complete (provides connection for gRPC calls)

### Package Dependencies
- `context` - context-based cancellation
- `internal/api/v1` - generated protobuf client

## Deliverables

### Files to Create/Modify

```
internal/client/
├── client.go        # MODIFY: Add job lifecycle methods
└── client_test.go   # CREATE: Unit tests with mocked gRPC client
```

### Types to Implement

None (uses existing types).

### Functions to Implement

```go
// client.go

// StartJob initiates a new orchestration job with the given configuration.
// Returns the job ID on success.
func (c *Client) StartJob(ctx context.Context, cfg JobConfig) (string, error) {
    // Convert JobConfig to protobuf request using jobConfigToProto
    // Call daemon.StartJob via gRPC
    // Return job ID from response or wrapped error
}

// StopJob cancels a running job.
// If force is true, the job terminates immediately without cleanup.
// If force is false, the job completes current tasks before stopping.
func (c *Client) StopJob(ctx context.Context, jobID string, force bool) error {
    // Build StopJobRequest with jobID and force flag
    // Call daemon.StopJob via gRPC
    // Return nil on success or wrapped error
}

// ListJobs returns job summaries, optionally filtered by status.
// Pass an empty slice for statusFilter to list all jobs.
func (c *Client) ListJobs(ctx context.Context, statusFilter []string) ([]*JobSummary, error) {
    // Build ListJobsRequest with status filter
    // Call daemon.ListJobs via gRPC
    // Convert response using protoToJobSummaries
    // Return summaries or wrapped error
}

// GetJobStatus returns detailed status for a specific job.
// Returns an error if the job ID does not exist.
func (c *Client) GetJobStatus(ctx context.Context, jobID string) (*JobStatus, error) {
    // Build GetJobStatusRequest with jobID
    // Call daemon.GetJobStatus via gRPC
    // Convert response using protoToJobStatus
    // Return status or wrapped error
}
```

## Testing Strategy

Create a mock DaemonServiceClient interface for unit testing:

```go
// client_test.go

type mockDaemonClient struct {
    apiv1.DaemonServiceClient
    startJobFn    func(context.Context, *apiv1.StartJobRequest, ...grpc.CallOption) (*apiv1.StartJobResponse, error)
    stopJobFn     func(context.Context, *apiv1.StopJobRequest, ...grpc.CallOption) (*apiv1.StopJobResponse, error)
    listJobsFn    func(context.Context, *apiv1.ListJobsRequest, ...grpc.CallOption) (*apiv1.ListJobsResponse, error)
    getJobStatusFn func(context.Context, *apiv1.GetJobStatusRequest, ...grpc.CallOption) (*apiv1.GetJobStatusResponse, error)
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/client/... -run TestJob
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestStartJob_Success` | Returns job ID from response |
| `TestStartJob_Error` | Propagates gRPC error with context |
| `TestStopJob_Force` | Force flag correctly passed to request |
| `TestListJobs_WithFilter` | Status filter applied to request |
| `TestListJobs_Empty` | Returns empty slice, not nil |
| `TestGetJobStatus_NotFound` | Handles NotFound error gracefully |

## NOT In Scope

- WatchJob streaming (Task #5)
- Retry logic on transient failures
- Request validation (daemon validates)
