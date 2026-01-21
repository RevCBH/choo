---
task: 5
status: pending
backpressure: "go test ./internal/daemon/... -run TestGRPC_Lifecycle"
depends_on: [1, 2]
---

# Daemon Lifecycle RPCs

**Parent spec**: `specs/DAEMON-GRPC.md`
**Task**: #5 of 7 in implementation plan

## Objective

Implement the Shutdown and Health RPC methods for daemon lifecycle management and monitoring.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Proto definitions for request/response types
- Task #2: GRPCServer struct with shutdown coordination fields

### Package Dependencies
- `github.com/RevCBH/choo/pkg/api/v1` - Generated protobuf types
- `google.golang.org/grpc/codes` - gRPC status codes
- `google.golang.org/grpc/status` - gRPC error handling

## Deliverables

### Files to Create/Modify

```
internal/daemon/
├── grpc.go         # MODIFY: Add Shutdown and Health implementations
└── grpc_test.go    # MODIFY: Add lifecycle tests
```

### Functions to Implement

```go
// internal/daemon/grpc.go

import (
    "time"
)

// Shutdown initiates graceful daemon shutdown.
// If wait_for_jobs is true, waits for running jobs up to timeout_seconds.
func (s *GRPCServer) Shutdown(ctx context.Context, req *apiv1.ShutdownRequest) (*apiv1.ShutdownResponse, error) {
    s.mu.Lock()

    // Check if already shutting down
    if s.shuttingDown {
        s.mu.Unlock()
        return nil, status.Errorf(codes.FailedPrecondition, "shutdown already in progress")
    }

    // Mark as shutting down
    s.shuttingDown = true
    close(s.shutdownCh)

    // Copy active jobs map for iteration
    activeJobs := make(map[string]context.CancelFunc)
    for k, v := range s.activeJobs {
        activeJobs[k] = v
    }
    s.mu.Unlock()

    jobsStopped := 0

    if req.WaitForJobs && len(activeJobs) > 0 {
        // Wait for jobs with timeout
        timeout := time.Duration(req.TimeoutSeconds) * time.Second
        if timeout == 0 {
            timeout = 30 * time.Second // default timeout
        }

        done := make(chan struct{})
        go func() {
            // Wait for all jobs to complete naturally
            ticker := time.NewTicker(100 * time.Millisecond)
            defer ticker.Stop()
            for {
                select {
                case <-ticker.C:
                    if s.jobManager.ActiveJobCount() == 0 {
                        close(done)
                        return
                    }
                }
            }
        }()

        select {
        case <-done:
            // All jobs completed gracefully
        case <-time.After(timeout):
            // Timeout - force stop remaining jobs
            for jobID, cancel := range activeJobs {
                cancel()
                jobsStopped++
                _ = s.jobManager.Stop(ctx, jobID, true)
            }
        }
    } else if !req.WaitForJobs {
        // Force stop all jobs immediately
        for jobID, cancel := range activeJobs {
            cancel()
            jobsStopped++
            _ = s.jobManager.Stop(ctx, jobID, true)
        }
    }

    return &apiv1.ShutdownResponse{
        Success:     true,
        JobsStopped: int32(jobsStopped),
    }, nil
}

// Health returns daemon health status for monitoring and service discovery.
func (s *GRPCServer) Health(ctx context.Context, req *apiv1.HealthRequest) (*apiv1.HealthResponse, error) {
    s.mu.RLock()
    shuttingDown := s.shuttingDown
    s.mu.RUnlock()

    return &apiv1.HealthResponse{
        Healthy:    !shuttingDown,
        ActiveJobs: int32(s.jobManager.ActiveJobCount()),
        Version:    s.version,
    }, nil
}
```

### Test Implementation

```go
// internal/daemon/grpc_test.go (additions)

func TestGRPC_LifecycleHealth_ReturnsStatus(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-1", "running")
    jm.addJob("job-2", "running")
    jm.addJob("job-3", "completed")

    server := NewGRPCServer(nil, jm, "v1.2.3")

    resp, err := server.Health(context.Background(), &apiv1.HealthRequest{})

    require.NoError(t, err)
    assert.True(t, resp.Healthy)
    assert.Equal(t, int32(2), resp.ActiveJobs) // Only running jobs
    assert.Equal(t, "v1.2.3", resp.Version)
}

func TestGRPC_LifecycleHealth_UnhealthyDuringShutdown(t *testing.T) {
    jm := newMockJobManager()
    server := NewGRPCServer(nil, jm, "v1.0.0")
    server.setShuttingDown()

    resp, err := server.Health(context.Background(), &apiv1.HealthRequest{})

    require.NoError(t, err)
    assert.False(t, resp.Healthy)
}

func TestGRPC_LifecycleShutdown_NoJobs(t *testing.T) {
    jm := newMockJobManager()
    server := NewGRPCServer(nil, jm, "v1.0.0")

    resp, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{
        WaitForJobs: false,
    })

    require.NoError(t, err)
    assert.True(t, resp.Success)
    assert.Equal(t, int32(0), resp.JobsStopped)
    assert.True(t, server.isShuttingDown())
}

func TestGRPC_LifecycleShutdown_ForceStopJobs(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-1", "running")
    jm.addJob("job-2", "running")

    server := NewGRPCServer(nil, jm, "v1.0.0")

    // Track the jobs so they show in activeJobs
    server.trackJob("job-1", func() {})
    server.trackJob("job-2", func() {})

    resp, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{
        WaitForJobs: false,
    })

    require.NoError(t, err)
    assert.True(t, resp.Success)
    assert.Equal(t, int32(2), resp.JobsStopped)
}

func TestGRPC_LifecycleShutdown_WaitForJobs(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-wait", "running")

    server := NewGRPCServer(nil, jm, "v1.0.0")
    server.trackJob("job-wait", func() {})

    // Simulate job completing during wait
    go func() {
        time.Sleep(50 * time.Millisecond)
        jm.jobs["job-wait"].Status = "completed"
    }()

    resp, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{
        WaitForJobs:    true,
        TimeoutSeconds: 5,
    })

    require.NoError(t, err)
    assert.True(t, resp.Success)
    assert.Equal(t, int32(0), resp.JobsStopped) // Completed naturally
}

func TestGRPC_LifecycleShutdown_WaitTimeout(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-slow", "running")

    server := NewGRPCServer(nil, jm, "v1.0.0")
    server.trackJob("job-slow", func() {})

    // Job never completes, timeout will trigger
    resp, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{
        WaitForJobs:    true,
        TimeoutSeconds: 1, // Short timeout
    })

    require.NoError(t, err)
    assert.True(t, resp.Success)
    assert.Equal(t, int32(1), resp.JobsStopped) // Force stopped after timeout
}

func TestGRPC_LifecycleShutdown_AlreadyShuttingDown(t *testing.T) {
    jm := newMockJobManager()
    server := NewGRPCServer(nil, jm, "v1.0.0")

    // First shutdown
    _, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{})
    require.NoError(t, err)

    // Second shutdown should fail
    _, err = server.Shutdown(context.Background(), &apiv1.ShutdownRequest{})
    require.Error(t, err)
    assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestGRPC_LifecycleShutdown_CancelsJobContexts(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-ctx", "running")

    server := NewGRPCServer(nil, jm, "v1.0.0")

    // Track job with a cancel function we can verify
    cancelled := false
    server.trackJob("job-ctx", func() { cancelled = true })

    _, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{
        WaitForJobs: false,
    })

    require.NoError(t, err)
    assert.True(t, cancelled, "job context should be cancelled")
}

func TestGRPC_LifecycleHealth_ZeroActiveJobs(t *testing.T) {
    jm := newMockJobManager()
    server := NewGRPCServer(nil, jm, "v1.0.0")

    resp, err := server.Health(context.Background(), &apiv1.HealthRequest{})

    require.NoError(t, err)
    assert.True(t, resp.Healthy)
    assert.Equal(t, int32(0), resp.ActiveJobs)
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestGRPC_Lifecycle
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGRPC_LifecycleHealth_ReturnsStatus` | Version, active count, healthy=true |
| `TestGRPC_LifecycleHealth_UnhealthyDuringShutdown` | healthy=false during shutdown |
| `TestGRPC_LifecycleShutdown_NoJobs` | Success with 0 jobs stopped |
| `TestGRPC_LifecycleShutdown_ForceStopJobs` | All tracked jobs force stopped |
| `TestGRPC_LifecycleShutdown_WaitForJobs` | Jobs complete naturally within timeout |
| `TestGRPC_LifecycleShutdown_WaitTimeout` | Jobs force stopped after timeout |
| `TestGRPC_LifecycleShutdown_AlreadyShuttingDown` | Returns FailedPrecondition |
| `TestGRPC_LifecycleShutdown_CancelsJobContexts` | Cancel funcs invoked |
| `TestGRPC_LifecycleHealth_ZeroActiveJobs` | Works with no jobs |

## NOT In Scope

- Socket server setup (task #6)
- gRPC server stop sequence (task #6)
- Database cleanup during shutdown (orchestrator responsibility)
- Process signal handling (CLI responsibility)
