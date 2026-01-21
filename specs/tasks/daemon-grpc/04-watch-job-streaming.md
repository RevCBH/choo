---
task: 4
status: complete
backpressure: "go test ./internal/daemon/... -run TestGRPC_Watch"
depends_on: [1, 2, 3]
---

# WatchJob Event Streaming

**Parent spec**: `specs/DAEMON-GRPC.md`
**Task**: #4 of 7 in implementation plan

## Objective

Implement the WatchJob server-streaming RPC with event subscription and historical event replay capability.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Proto definitions for WatchJobRequest and JobEvent
- Task #2: GRPCServer struct and JobManager.Subscribe interface
- Task #3: Job lifecycle RPCs provide context for job existence checks

### Package Dependencies
- `github.com/RevCBH/choo/pkg/api/v1` - Generated protobuf types
- `google.golang.org/grpc/codes` - gRPC status codes
- `google.golang.org/grpc/status` - gRPC error handling

## Deliverables

### Files to Create/Modify

```
internal/daemon/
├── grpc.go         # MODIFY: Add WatchJob implementation
└── grpc_test.go    # MODIFY: Add WatchJob tests
```

### Functions to Implement

```go
// internal/daemon/grpc.go

// WatchJob streams events for a job until completion or client disconnect.
// Supports resuming from a specific sequence number for reconnection scenarios.
func (s *GRPCServer) WatchJob(req *apiv1.WatchJobRequest, stream apiv1.DaemonService_WatchJobServer) error {
    // Validate required fields
    if req.JobId == "" {
        return status.Errorf(codes.InvalidArgument, "job_id is required")
    }

    // Validate job exists
    job, err := s.jobManager.GetJob(req.JobId)
    if err != nil {
        return status.Errorf(codes.NotFound, "job not found: %s", req.JobId)
    }

    // If job is already complete and no replay requested, return immediately
    if isTerminalStatus(job.Status) && req.FromSequence == 0 {
        return nil
    }

    // Subscribe to job events starting from requested sequence
    events, unsub := s.jobManager.Subscribe(req.JobId, int(req.FromSequence))
    defer unsub()

    // Stream events until channel closes (job complete) or client disconnects
    for {
        select {
        case event, ok := <-events:
            if !ok {
                // Channel closed - job completed
                return nil
            }
            if err := stream.Send(eventToProto(event)); err != nil {
                // Client disconnected or stream error
                return err
            }

        case <-stream.Context().Done():
            // Client disconnected
            return stream.Context().Err()

        case <-s.shutdownCh:
            // Server shutting down
            return status.Errorf(codes.Unavailable, "daemon is shutting down")
        }
    }
}

// isTerminalStatus returns true if the job status indicates completion
func isTerminalStatus(status string) bool {
    switch status {
    case "completed", "failed", "cancelled":
        return true
    default:
        return false
    }
}
```

### Test Implementation

```go
// internal/daemon/grpc_test.go (additions)

// mockWatchStream implements DaemonService_WatchJobServer for testing
type mockWatchStream struct {
    apiv1.DaemonService_WatchJobServer
    events    []*apiv1.JobEvent
    ctx       context.Context
    cancel    context.CancelFunc
    sendErr   error
    mu        sync.Mutex
}

func newMockWatchStream() *mockWatchStream {
    ctx, cancel := context.WithCancel(context.Background())
    return &mockWatchStream{
        ctx:    ctx,
        cancel: cancel,
    }
}

func (m *mockWatchStream) Send(event *apiv1.JobEvent) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.sendErr != nil {
        return m.sendErr
    }
    m.events = append(m.events, event)
    return nil
}

func (m *mockWatchStream) Context() context.Context {
    return m.ctx
}

func (m *mockWatchStream) getEvents() []*apiv1.JobEvent {
    m.mu.Lock()
    defer m.mu.Unlock()
    result := make([]*apiv1.JobEvent, len(m.events))
    copy(result, m.events)
    return result
}

// Enhanced mockJobManager with Subscribe support
func (m *mockJobManager) SetupEventStream(jobID string, events []Event) chan Event {
    ch := make(chan Event, len(events))
    for _, e := range events {
        ch <- e
    }
    close(ch)
    m.eventChannels[jobID] = ch
    return ch
}

func TestGRPC_WatchJob_ValidatesJobID(t *testing.T) {
    jm := newMockJobManager()
    server := NewGRPCServer(nil, jm, "v1.0.0")
    stream := newMockWatchStream()

    err := server.WatchJob(&apiv1.WatchJobRequest{
        JobId: "",
    }, stream)

    require.Error(t, err)
    assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGRPC_WatchJob_JobNotFound(t *testing.T) {
    jm := newMockJobManager()
    server := NewGRPCServer(nil, jm, "v1.0.0")
    stream := newMockWatchStream()

    err := server.WatchJob(&apiv1.WatchJobRequest{
        JobId: "nonexistent",
    }, stream)

    require.Error(t, err)
    assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestGRPC_WatchJob_StreamsEvents(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-stream", "running")

    // Setup event channel that will close after sending events
    eventsCh := make(chan Event, 3)
    eventsCh <- Event{Sequence: 1, EventType: "unit_started", UnitID: "unit-1"}
    eventsCh <- Event{Sequence: 2, EventType: "task_completed", UnitID: "unit-1"}
    eventsCh <- Event{Sequence: 3, EventType: "unit_completed", UnitID: "unit-1"}
    close(eventsCh)

    jm.subscribeFunc = func(jobID string, fromSeq int) (<-chan Event, func()) {
        return eventsCh, func() {}
    }

    server := NewGRPCServer(nil, jm, "v1.0.0")
    stream := newMockWatchStream()

    err := server.WatchJob(&apiv1.WatchJobRequest{
        JobId:        "job-stream",
        FromSequence: 0,
    }, stream)

    require.NoError(t, err)
    events := stream.getEvents()
    assert.Len(t, events, 3)
    assert.Equal(t, int32(1), events[0].Sequence)
    assert.Equal(t, "unit_started", events[0].EventType)
    assert.Equal(t, int32(2), events[1].Sequence)
    assert.Equal(t, int32(3), events[2].Sequence)
}

func TestGRPC_WatchJob_ReplayFromSequence(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-replay", "running")

    // Create channel with events starting from sequence 3
    eventsCh := make(chan Event, 2)
    eventsCh <- Event{Sequence: 3, EventType: "task_completed"}
    eventsCh <- Event{Sequence: 4, EventType: "unit_completed"}
    close(eventsCh)

    var capturedFromSeq int
    jm.subscribeFunc = func(jobID string, fromSeq int) (<-chan Event, func()) {
        capturedFromSeq = fromSeq
        return eventsCh, func() {}
    }

    server := NewGRPCServer(nil, jm, "v1.0.0")
    stream := newMockWatchStream()

    err := server.WatchJob(&apiv1.WatchJobRequest{
        JobId:        "job-replay",
        FromSequence: 2,
    }, stream)

    require.NoError(t, err)
    assert.Equal(t, 2, capturedFromSeq)
    events := stream.getEvents()
    assert.Len(t, events, 2)
    assert.Equal(t, int32(3), events[0].Sequence)
}

func TestGRPC_WatchJob_ClientDisconnect(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-disconnect", "running")

    // Create a channel that blocks
    blockingCh := make(chan Event)
    jm.subscribeFunc = func(jobID string, fromSeq int) (<-chan Event, func()) {
        return blockingCh, func() { close(blockingCh) }
    }

    server := NewGRPCServer(nil, jm, "v1.0.0")
    stream := newMockWatchStream()

    // Start watching in goroutine
    errCh := make(chan error, 1)
    go func() {
        errCh <- server.WatchJob(&apiv1.WatchJobRequest{
            JobId: "job-disconnect",
        }, stream)
    }()

    // Simulate client disconnect
    time.Sleep(10 * time.Millisecond)
    stream.cancel()

    // Should return context error
    select {
    case err := <-errCh:
        assert.Error(t, err)
    case <-time.After(time.Second):
        t.Fatal("WatchJob did not return after client disconnect")
    }
}

func TestGRPC_WatchJob_ServerShutdown(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-shutdown", "running")

    blockingCh := make(chan Event)
    jm.subscribeFunc = func(jobID string, fromSeq int) (<-chan Event, func()) {
        return blockingCh, func() { close(blockingCh) }
    }

    server := NewGRPCServer(nil, jm, "v1.0.0")
    stream := newMockWatchStream()

    errCh := make(chan error, 1)
    go func() {
        errCh <- server.WatchJob(&apiv1.WatchJobRequest{
            JobId: "job-shutdown",
        }, stream)
    }()

    // Trigger shutdown
    time.Sleep(10 * time.Millisecond)
    server.setShuttingDown()

    select {
    case err := <-errCh:
        require.Error(t, err)
        assert.Equal(t, codes.Unavailable, status.Code(err))
    case <-time.After(time.Second):
        t.Fatal("WatchJob did not return after server shutdown")
    }
}

func TestGRPC_WatchJob_SendError(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-send-err", "running")

    eventsCh := make(chan Event, 1)
    eventsCh <- Event{Sequence: 1, EventType: "test"}

    jm.subscribeFunc = func(jobID string, fromSeq int) (<-chan Event, func()) {
        return eventsCh, func() { close(eventsCh) }
    }

    server := NewGRPCServer(nil, jm, "v1.0.0")
    stream := newMockWatchStream()
    stream.sendErr = errors.New("connection reset")

    err := server.WatchJob(&apiv1.WatchJobRequest{
        JobId: "job-send-err",
    }, stream)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "connection reset")
}

func TestGRPC_WatchJob_CompletedJobNoReplay(t *testing.T) {
    jm := newMockJobManager()
    jm.addJob("job-done", "completed")
    server := NewGRPCServer(nil, jm, "v1.0.0")
    stream := newMockWatchStream()

    err := server.WatchJob(&apiv1.WatchJobRequest{
        JobId:        "job-done",
        FromSequence: 0,
    }, stream)

    // Should return immediately with no events
    require.NoError(t, err)
    assert.Empty(t, stream.getEvents())
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestGRPC_Watch
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGRPC_WatchJob_ValidatesJobID` | Returns InvalidArgument for empty job_id |
| `TestGRPC_WatchJob_JobNotFound` | Returns NotFound for missing job |
| `TestGRPC_WatchJob_StreamsEvents` | All events sent to stream |
| `TestGRPC_WatchJob_ReplayFromSequence` | fromSeq passed to Subscribe |
| `TestGRPC_WatchJob_ClientDisconnect` | Returns when context cancelled |
| `TestGRPC_WatchJob_ServerShutdown` | Returns Unavailable during shutdown |
| `TestGRPC_WatchJob_SendError` | Propagates stream send errors |
| `TestGRPC_WatchJob_CompletedJobNoReplay` | Returns immediately for completed job |

## NOT In Scope

- Shutdown/Health RPCs (task #5)
- Socket server setup (task #6)
- Event replay from database (JobManager responsibility)
- Event buffering strategy (JobManager responsibility)
