---
task: 7
status: pending
backpressure: "go test ./internal/daemon/... -run TestGRPC_Integration"
depends_on: [1, 2, 3, 4, 5, 6]
---

# Integration Tests

**Parent spec**: `specs/DAEMON-GRPC.md`
**Task**: #7 of 7 in implementation plan

## Objective

Create end-to-end integration tests that verify the complete gRPC flow with real client connections through Unix sockets.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Proto definitions and generated client stubs
- Task #2: GRPCServer struct
- Task #3: Job lifecycle RPCs
- Task #4: WatchJob streaming
- Task #5: Daemon lifecycle RPCs
- Task #6: Unix socket server

### Package Dependencies
- `github.com/RevCBH/choo/pkg/api/v1` - Generated protobuf types and client
- `google.golang.org/grpc` - gRPC client
- `google.golang.org/grpc/credentials/insecure` - No TLS for tests

## Deliverables

### Files to Create/Modify

```
internal/daemon/
└── integration_test.go    # CREATE: End-to-end integration tests
```

### Test Implementation

```go
// internal/daemon/integration_test.go

package daemon

import (
    "context"
    "path/filepath"
    "sync"
    "testing"
    "time"

    apiv1 "github.com/RevCBH/choo/pkg/api/v1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/status"
)

// testServer creates a server with mocked dependencies for integration testing
func setupIntegrationTest(t *testing.T) (*Server, apiv1.DaemonServiceClient, func()) {
    t.Helper()

    socketPath := filepath.Join(t.TempDir(), "integration.sock")
    jm := newMockJobManager()

    server := NewServer(ServerConfig{
        SocketPath: socketPath,
        Version:    "v1.0.0-test",
    }, nil, jm)

    // Start server
    go server.Start()

    // Wait for server to be ready
    require.Eventually(t, func() bool {
        return server.IsRunning()
    }, time.Second, 10*time.Millisecond)

    // Create client connection
    conn, err := grpc.Dial(
        "unix://"+socketPath,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    require.NoError(t, err)

    client := apiv1.NewDaemonServiceClient(conn)

    cleanup := func() {
        conn.Close()
        ctx, cancel := context.WithTimeout(context.Background(), time.Second)
        defer cancel()
        server.Stop(ctx)
    }

    return server, client, cleanup
}

func TestGRPC_IntegrationFullJobLifecycle(t *testing.T) {
    _, client, cleanup := setupIntegrationTest(t)
    defer cleanup()

    ctx := context.Background()

    // Start a job
    startResp, err := client.StartJob(ctx, &apiv1.StartJobRequest{
        TasksDir:     "/path/to/tasks",
        TargetBranch: "main",
        RepoPath:     "/path/to/repo",
        Parallelism:  4,
    })
    require.NoError(t, err)
    assert.NotEmpty(t, startResp.JobId)
    assert.Equal(t, "running", startResp.Status)

    jobID := startResp.JobId

    // Get job status
    statusResp, err := client.GetJobStatus(ctx, &apiv1.GetJobStatusRequest{
        JobId: jobID,
    })
    require.NoError(t, err)
    assert.Equal(t, jobID, statusResp.JobId)
    assert.Equal(t, "running", statusResp.Status)

    // List jobs
    listResp, err := client.ListJobs(ctx, &apiv1.ListJobsRequest{})
    require.NoError(t, err)
    assert.Len(t, listResp.Jobs, 1)
    assert.Equal(t, jobID, listResp.Jobs[0].JobId)

    // Stop job
    stopResp, err := client.StopJob(ctx, &apiv1.StopJobRequest{
        JobId: jobID,
        Force: false,
    })
    require.NoError(t, err)
    assert.True(t, stopResp.Success)
}

func TestGRPC_IntegrationConcurrentClients(t *testing.T) {
    _, client, cleanup := setupIntegrationTest(t)
    defer cleanup()

    const numClients = 10
    var wg sync.WaitGroup
    errors := make(chan error, numClients)

    for i := 0; i < numClients; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            ctx := context.Background()

            // Each client calls Health
            resp, err := client.Health(ctx, &apiv1.HealthRequest{})
            if err != nil {
                errors <- err
                return
            }
            if !resp.Healthy {
                errors <- fmt.Errorf("unhealthy response")
            }
        }()
    }

    wg.Wait()
    close(errors)

    for err := range errors {
        t.Errorf("concurrent client error: %v", err)
    }
}

func TestGRPC_IntegrationWatchJobStream(t *testing.T) {
    server, client, cleanup := setupIntegrationTest(t)
    defer cleanup()

    // Access the mock job manager through the server
    jm := server.GRPCServer().jobManager.(*mockJobManager)

    // Create a job
    ctx := context.Background()
    startResp, err := client.StartJob(ctx, &apiv1.StartJobRequest{
        TasksDir:     "/tasks",
        TargetBranch: "main",
        RepoPath:     "/repo",
    })
    require.NoError(t, err)
    jobID := startResp.JobId

    // Setup event channel for streaming
    eventsCh := make(chan Event, 3)
    jm.subscribeFunc = func(id string, fromSeq int) (<-chan Event, func()) {
        return eventsCh, func() { close(eventsCh) }
    }

    // Start watching in background
    watchCtx, watchCancel := context.WithCancel(ctx)
    defer watchCancel()

    stream, err := client.WatchJob(watchCtx, &apiv1.WatchJobRequest{
        JobId:        jobID,
        FromSequence: 0,
    })
    require.NoError(t, err)

    // Send events
    eventsCh <- Event{Sequence: 1, EventType: "unit_started", UnitID: "unit-1"}
    eventsCh <- Event{Sequence: 2, EventType: "task_completed", UnitID: "unit-1"}
    eventsCh <- Event{Sequence: 3, EventType: "unit_completed", UnitID: "unit-1"}
    close(eventsCh)

    // Receive events
    var received []*apiv1.JobEvent
    for {
        event, err := stream.Recv()
        if err != nil {
            break
        }
        received = append(received, event)
    }

    assert.Len(t, received, 3)
    assert.Equal(t, int32(1), received[0].Sequence)
    assert.Equal(t, "unit_started", received[0].EventType)
}

func TestGRPC_IntegrationMultipleWatchers(t *testing.T) {
    server, client, cleanup := setupIntegrationTest(t)
    defer cleanup()

    jm := server.GRPCServer().jobManager.(*mockJobManager)

    ctx := context.Background()
    startResp, err := client.StartJob(ctx, &apiv1.StartJobRequest{
        TasksDir:     "/tasks",
        TargetBranch: "main",
        RepoPath:     "/repo",
    })
    require.NoError(t, err)
    jobID := startResp.JobId

    // Create shared event channel
    var subscribeCount int
    var mu sync.Mutex
    jm.subscribeFunc = func(id string, fromSeq int) (<-chan Event, func()) {
        mu.Lock()
        subscribeCount++
        mu.Unlock()
        ch := make(chan Event, 1)
        ch <- Event{Sequence: 1, EventType: "test"}
        close(ch)
        return ch, func() {}
    }

    // Start multiple watchers
    const numWatchers = 5
    var wg sync.WaitGroup

    for i := 0; i < numWatchers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            watchCtx, cancel := context.WithTimeout(ctx, time.Second)
            defer cancel()

            stream, err := client.WatchJob(watchCtx, &apiv1.WatchJobRequest{
                JobId: jobID,
            })
            if err != nil {
                return
            }

            // Read at least one event
            _, _ = stream.Recv()
        }()
    }

    wg.Wait()

    mu.Lock()
    assert.Equal(t, numWatchers, subscribeCount)
    mu.Unlock()
}

func TestGRPC_IntegrationReconnect(t *testing.T) {
    _, client, cleanup := setupIntegrationTest(t)
    defer cleanup()

    ctx := context.Background()

    // First connection
    resp1, err := client.Health(ctx, &apiv1.HealthRequest{})
    require.NoError(t, err)
    assert.True(t, resp1.Healthy)

    // Simulate reconnect by making another call
    // (In a real scenario, this might be after network hiccup)
    resp2, err := client.Health(ctx, &apiv1.HealthRequest{})
    require.NoError(t, err)
    assert.True(t, resp2.Healthy)
}

func TestGRPC_IntegrationShutdownDuringWatch(t *testing.T) {
    server, client, cleanup := setupIntegrationTest(t)
    // Don't defer cleanup - we're testing shutdown

    jm := server.GRPCServer().jobManager.(*mockJobManager)

    ctx := context.Background()
    startResp, err := client.StartJob(ctx, &apiv1.StartJobRequest{
        TasksDir:     "/tasks",
        TargetBranch: "main",
        RepoPath:     "/repo",
    })
    require.NoError(t, err)
    jobID := startResp.JobId

    // Setup blocking event channel
    blockingCh := make(chan Event)
    jm.subscribeFunc = func(id string, fromSeq int) (<-chan Event, func()) {
        return blockingCh, func() { close(blockingCh) }
    }

    // Start watching
    watchCtx, watchCancel := context.WithCancel(ctx)
    defer watchCancel()

    stream, err := client.WatchJob(watchCtx, &apiv1.WatchJobRequest{
        JobId: jobID,
    })
    require.NoError(t, err)

    // Start reading in background
    errCh := make(chan error, 1)
    go func() {
        _, err := stream.Recv()
        errCh <- err
    }()

    // Shutdown server
    time.Sleep(50 * time.Millisecond)
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
    defer shutdownCancel()
    server.Stop(shutdownCtx)

    // Watch should receive an error
    select {
    case err := <-errCh:
        assert.Error(t, err)
    case <-time.After(2 * time.Second):
        t.Fatal("WatchJob did not return after server shutdown")
    }
}

func TestGRPC_IntegrationErrorCodes(t *testing.T) {
    _, client, cleanup := setupIntegrationTest(t)
    defer cleanup()

    ctx := context.Background()

    tests := []struct {
        name     string
        call     func() error
        wantCode codes.Code
    }{
        {
            name: "GetJobStatus not found",
            call: func() error {
                _, err := client.GetJobStatus(ctx, &apiv1.GetJobStatusRequest{
                    JobId: "nonexistent",
                })
                return err
            },
            wantCode: codes.NotFound,
        },
        {
            name: "StartJob missing required field",
            call: func() error {
                _, err := client.StartJob(ctx, &apiv1.StartJobRequest{
                    TargetBranch: "main",
                    RepoPath:     "/repo",
                    // Missing TasksDir
                })
                return err
            },
            wantCode: codes.InvalidArgument,
        },
        {
            name: "StopJob not found",
            call: func() error {
                _, err := client.StopJob(ctx, &apiv1.StopJobRequest{
                    JobId: "nonexistent",
                })
                return err
            },
            wantCode: codes.NotFound,
        },
        {
            name: "WatchJob not found",
            call: func() error {
                stream, err := client.WatchJob(ctx, &apiv1.WatchJobRequest{
                    JobId: "nonexistent",
                })
                if err != nil {
                    return err
                }
                _, err = stream.Recv()
                return err
            },
            wantCode: codes.NotFound,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.call()
            require.Error(t, err)
            assert.Equal(t, tt.wantCode, status.Code(err))
        })
    }
}

func TestGRPC_IntegrationListJobsFilter(t *testing.T) {
    server, client, cleanup := setupIntegrationTest(t)
    defer cleanup()

    jm := server.GRPCServer().jobManager.(*mockJobManager)
    ctx := context.Background()

    // Create jobs with different statuses via mock
    jm.addJob("job-running-1", "running")
    jm.addJob("job-running-2", "running")
    jm.addJob("job-completed", "completed")
    jm.addJob("job-failed", "failed")

    // List all
    allResp, err := client.ListJobs(ctx, &apiv1.ListJobsRequest{})
    require.NoError(t, err)
    assert.Len(t, allResp.Jobs, 4)

    // Filter by running
    runningResp, err := client.ListJobs(ctx, &apiv1.ListJobsRequest{
        StatusFilter: []string{"running"},
    })
    require.NoError(t, err)
    assert.Len(t, runningResp.Jobs, 2)

    // Filter by completed and failed
    terminalResp, err := client.ListJobs(ctx, &apiv1.ListJobsRequest{
        StatusFilter: []string{"completed", "failed"},
    })
    require.NoError(t, err)
    assert.Len(t, terminalResp.Jobs, 2)
}

func TestGRPC_IntegrationHealthAfterShutdownRequest(t *testing.T) {
    _, client, cleanup := setupIntegrationTest(t)
    defer cleanup()

    ctx := context.Background()

    // Health before shutdown
    resp1, err := client.Health(ctx, &apiv1.HealthRequest{})
    require.NoError(t, err)
    assert.True(t, resp1.Healthy)

    // Request shutdown via RPC
    _, err = client.Shutdown(ctx, &apiv1.ShutdownRequest{
        WaitForJobs: false,
    })
    require.NoError(t, err)

    // Health after shutdown request shows unhealthy
    resp2, err := client.Health(ctx, &apiv1.HealthRequest{})
    require.NoError(t, err)
    assert.False(t, resp2.Healthy)
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestGRPC_Integration
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGRPC_IntegrationFullJobLifecycle` | Start, status, list, stop all work |
| `TestGRPC_IntegrationConcurrentClients` | Multiple clients work simultaneously |
| `TestGRPC_IntegrationWatchJobStream` | Events stream correctly |
| `TestGRPC_IntegrationMultipleWatchers` | Multiple watchers on same job |
| `TestGRPC_IntegrationReconnect` | Reconnection works |
| `TestGRPC_IntegrationShutdownDuringWatch` | Watch terminates on shutdown |
| `TestGRPC_IntegrationErrorCodes` | Correct gRPC status codes |
| `TestGRPC_IntegrationListJobsFilter` | Status filtering works |
| `TestGRPC_IntegrationHealthAfterShutdownRequest` | Health reports unhealthy |

## NOT In Scope

- Performance benchmarks (future enhancement)
- Load testing (manual testing)
- Real JobManager integration (orchestrator responsibility)
- Database integration tests (DAEMON-DB responsibility)
