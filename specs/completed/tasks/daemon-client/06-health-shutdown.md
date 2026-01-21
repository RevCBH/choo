---
task: 6
status: complete
backpressure: "go test ./internal/client/... -run TestHealth"
depends_on: [3]
---

# Health and Shutdown

**Parent spec**: `specs/DAEMON-CLIENT.md`
**Task**: #6 of 6 in implementation plan

## Objective

Implement the Health and Shutdown methods for daemon health checking and graceful termination.

## Dependencies

### Task Dependencies (within this unit)
- Task #3 must be complete (provides connection for gRPC calls)

### Package Dependencies
- `context` - context-based cancellation
- `internal/api/v1` - generated protobuf client

## Deliverables

### Files to Create/Modify

```
internal/client/
├── client.go        # MODIFY: Add Health and Shutdown methods
└── client_test.go   # MODIFY: Add health/shutdown tests
```

### Types to Implement

None (HealthInfo defined in Task #1).

### Functions to Implement

```go
// client.go

// Health checks daemon health and returns version info.
// This is a lightweight call suitable for polling.
func (c *Client) Health(ctx context.Context) (*HealthInfo, error) {
    // Build HealthRequest (empty message)
    // Call daemon.Health via gRPC
    // Convert response using protoToHealthInfo
    // Return health info or wrapped error
}

// Shutdown requests daemon termination.
// If waitForJobs is true, the daemon waits for active jobs to complete
// before shutting down, up to the specified timeout in seconds.
// If waitForJobs is false, active jobs are cancelled immediately.
func (c *Client) Shutdown(ctx context.Context, waitForJobs bool, timeout int) error {
    // Build ShutdownRequest with waitForJobs and timeout
    // Call daemon.Shutdown via gRPC
    // Return nil on success or wrapped error
}
```

## Usage Patterns

Health check for daemon status:
```go
client, _ := client.New(socketPath)
defer client.Close()

health, err := client.Health(ctx)
if err != nil {
    // Daemon not running or unreachable
}
fmt.Printf("Daemon v%s: %d active jobs\n", health.Version, health.ActiveJobs)
```

Graceful shutdown with timeout:
```go
// Wait up to 30 seconds for jobs to complete
err := client.Shutdown(ctx, true, 30)
```

Immediate shutdown:
```go
// Cancel all jobs immediately
err := client.Shutdown(ctx, false, 0)
```

## Error Handling

Callers should check for specific gRPC error codes:
- `codes.Unavailable`: Daemon not running
- `codes.DeadlineExceeded`: Shutdown timeout elapsed

## Backpressure

### Validation Command

```bash
go test ./internal/client/... -run TestHealth
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestHealth_Success` | Returns HealthInfo with all fields |
| `TestHealth_Unavailable` | Returns error when daemon down |
| `TestShutdown_WaitForJobs` | waitForJobs flag passed correctly |
| `TestShutdown_Immediate` | Force shutdown with no wait |
| `TestShutdown_Timeout` | Timeout value passed correctly |

## NOT In Scope

- Health polling loop (caller responsibility)
- Retry on transient failures
- Daemon restart functionality
