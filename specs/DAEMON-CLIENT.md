# DAEMON-CLIENT — gRPC Client Wrapper for CLI Communication with Daemon

## Overview

The daemon client provides a Go wrapper around the gRPC interface for CLI tools to communicate with the Charlotte daemon. It handles connection management over Unix sockets, translates between Go types and protobuf messages, and provides streaming support for real-time job event watching.

## Requirements

### Functional Requirements

1. **Connection Management**: Establish and maintain gRPC connections to the daemon via Unix socket
2. **Job Lifecycle**: Start, stop, and monitor orchestration jobs
3. **Event Streaming**: Watch job events in real-time with sequence-based resumption
4. **Health Checking**: Query daemon health status and version information
5. **Graceful Shutdown**: Request daemon shutdown with configurable job completion behavior

### Performance Requirements

1. **Connection Latency**: Initial connection establishment under 100ms on local socket
2. **Streaming Throughput**: Handle 1000+ events/second without backpressure
3. **Resource Efficiency**: Single connection per client instance, reused across calls

### Constraints

1. Unix socket path fixed at `~/.choo/daemon.sock`
2. Insecure transport (local socket security model)
3. Context-based cancellation for all operations
4. Protobuf v1 API compatibility

## Design

### Module Structure

```
internal/client/
├── client.go       # Main client implementation
├── types.go        # Client-side type definitions
└── convert.go      # Protobuf conversion utilities
```

### Core Types

```go
// Client wraps gRPC connection and service stub
type Client struct {
    conn   *grpc.ClientConn
    daemon apiv1.DaemonServiceClient
}

// JobConfig contains parameters for starting a new job
type JobConfig struct {
    TasksDir      string    // Directory containing task definitions
    TargetBranch  string    // Base branch for PRs
    FeatureBranch string    // Branch name for work
    Parallelism   int       // Max concurrent units
    RepoPath      string    // Repository root path
}

// JobSummary provides high-level job information for listings
type JobSummary struct {
    JobID         string
    FeatureBranch string
    Status        string
    StartedAt     time.Time
    UnitsComplete int
    UnitsTotal    int
}

// JobStatus provides detailed job state including all units
type JobStatus struct {
    JobID       string
    Status      string
    StartedAt   time.Time
    CompletedAt *time.Time
    Error       string
    Units       []UnitStatus
}

// UnitStatus tracks individual unit progress
type UnitStatus struct {
    UnitID        string
    Status        string
    TasksComplete int
    TasksTotal    int
    PRNumber      int
}

// HealthInfo contains daemon health check response
type HealthInfo struct {
    Healthy    bool
    ActiveJobs int
    Version    string
}
```

### API Surface

```go
// New creates a client connected to the daemon socket
func New(socketPath string) (*Client, error)

// StartJob initiates a new orchestration job
func (c *Client) StartJob(ctx context.Context, cfg JobConfig) (string, error)

// WatchJob streams job events, calling handler for each event.
// The fromSeq parameter specifies the sequence number to start from (0 = beginning).
// This enables reconnection scenarios where the client resumes from a specific point.
func (c *Client) WatchJob(ctx context.Context, jobID string, fromSeq int, handler func(events.Event)) error

// StopJob cancels a running job, optionally forcing immediate termination
func (c *Client) StopJob(ctx context.Context, jobID string, force bool) error

// ListJobs returns job summaries, optionally filtered by status
func (c *Client) ListJobs(ctx context.Context, statusFilter []string) ([]*JobSummary, error)

// GetJobStatus returns detailed status for a specific job
func (c *Client) GetJobStatus(ctx context.Context, jobID string) (*JobStatus, error)

// Health checks daemon health and returns version info
func (c *Client) Health(ctx context.Context) (*HealthInfo, error)

// Shutdown requests daemon termination with optional job completion wait
func (c *Client) Shutdown(ctx context.Context, waitForJobs bool, timeout int) error

// Close releases the underlying gRPC connection
func (c *Client) Close() error
```

## Implementation Notes

### Connection Establishment

```go
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
```

The `unix://` scheme prefix instructs gRPC to use Unix domain socket transport. Insecure credentials are appropriate since the socket is protected by filesystem permissions.

### Event Streaming Pattern

```go
func (c *Client) WatchJob(ctx context.Context, jobID string, fromSeq int, handler func(events.Event)) error {
    stream, err := c.daemon.WatchJob(ctx, &apiv1.WatchJobRequest{
        JobId:        jobID,
        FromSequence: int32(fromSeq),
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
```

The streaming loop handles three cases:
- `io.EOF`: Job completed, clean exit
- Other error: Connection lost or job failed
- Success: Convert protobuf event and invoke handler

### Protobuf Conversion

Conversion functions translate between client types and protobuf messages:

```go
func protoToJobSummaries(protos []*apiv1.JobSummary) []*JobSummary {
    summaries := make([]*JobSummary, len(protos))
    for i, p := range protos {
        summaries[i] = &JobSummary{
            JobID:         p.JobId,
            FeatureBranch: p.FeatureBranch,
            Status:        p.Status,
            StartedAt:     p.StartedAt.AsTime(),
            UnitsComplete: int(p.UnitsComplete),
            UnitsTotal:    int(p.UnitsTotal),
        }
    }
    return summaries
}

func protoToJobStatus(resp *apiv1.GetJobStatusResponse) *JobStatus {
    status := &JobStatus{
        JobID:     resp.JobId,
        Status:    resp.Status,
        StartedAt: resp.StartedAt.AsTime(),
        Error:     resp.Error,
        Units:     make([]UnitStatus, len(resp.Units)),
    }
    if resp.CompletedAt != nil {
        t := resp.CompletedAt.AsTime()
        status.CompletedAt = &t
    }
    for i, u := range resp.Units {
        status.Units[i] = UnitStatus{
            UnitID:        u.UnitId,
            Status:        u.Status,
            TasksComplete: int(u.TasksComplete),
            TasksTotal:    int(u.TasksTotal),
            PRNumber:      int(u.PrNumber),
        }
    }
    return status
}
```

### Error Handling

All methods propagate gRPC errors directly. Callers should check for:
- `codes.Unavailable`: Daemon not running
- `codes.NotFound`: Job ID does not exist
- `codes.InvalidArgument`: Malformed request parameters

## Testing Strategy

### Unit Tests

1. **Type Conversion**: Verify `protoToX` functions correctly map all fields
2. **Error Wrapping**: Confirm connection errors include context
3. **Nil Handling**: Test optional fields like `CompletedAt` with nil values

```go
func TestProtoToJobStatus_NilCompletedAt(t *testing.T) {
    resp := &apiv1.GetJobStatusResponse{
        JobId:       "job-123",
        Status:      "running",
        CompletedAt: nil,
    }
    status := protoToJobStatus(resp)
    assert.Nil(t, status.CompletedAt)
}
```

### Integration Tests

1. **Connection Lifecycle**: Connect, make calls, close, verify cleanup
2. **Event Streaming**: Start job, watch events, verify sequence and content
3. **Concurrent Calls**: Multiple goroutines sharing single client instance
4. **Reconnection**: Simulate daemon restart, verify recovery behavior

```go
func TestWatchJob_StreamsAllEvents(t *testing.T) {
    client := setupTestClient(t)
    defer client.Close()

    jobID, _ := client.StartJob(ctx, testConfig)

    var events []events.Event
    err := client.WatchJob(ctx, jobID, func(e events.Event) {
        events = append(events, e)
    })

    require.NoError(t, err)
    assert.True(t, len(events) > 0)
    assert.Equal(t, "job.started", events[0].Type)
}
```

### Manual Testing

1. Start daemon manually, verify client connects
2. Run job through CLI, observe event output
3. Kill daemon during watch, verify error handling
4. Test with various parallelism levels

## Design Decisions

### 1. Thin Wrapper Over gRPC

**Decision**: Client methods directly call gRPC with minimal abstraction.

**Rationale**: The daemon API is already well-designed. Adding layers would obscure errors and complicate debugging. Direct mapping keeps the client predictable.

### 2. Callback-Based Event Handling

**Decision**: `WatchJob` accepts a handler function rather than returning a channel.

**Rationale**: Callbacks allow the client to control backpressure naturally. Channel-based designs require buffering decisions and risk deadlocks if consumers are slow.

### 3. Insecure Transport

**Decision**: Use `insecure.NewCredentials()` for Unix socket connections.

**Rationale**: Unix sockets are protected by filesystem permissions. TLS would add complexity without security benefit for local IPC.

### 4. No Automatic Reconnection

**Decision**: Connection failures surface as errors rather than triggering automatic reconnection.

**Rationale**: CLI commands are typically short-lived. Reconnection logic belongs in the calling code if needed, allowing appropriate UX decisions.

## Future Enhancements

1. **Connection Pooling**: For long-running processes, maintain connection health with keepalives
2. **Retry Middleware**: Add configurable retry policies for transient failures
3. **Metrics Integration**: Expose connection and call metrics for observability
4. **Streaming Resumption**: Track sequence numbers to resume event streams after reconnection
5. **Dial Options**: Allow callers to customize gRPC dial options (timeouts, interceptors)

## References

- `api/v1/daemon.proto` — gRPC service definition
- `internal/daemon/server.go` — Server-side implementation
- `cmd/choo/` — CLI commands using this client
- gRPC Go documentation: https://grpc.io/docs/languages/go/
