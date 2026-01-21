---
task: 3
status: pending
backpressure: "go build ./internal/client/..."
depends_on: [1]
---

# Connection Management

**Parent spec**: `specs/DAEMON-CLIENT.md`
**Task**: #3 of 6 in implementation plan

## Objective

Implement the New() constructor and Close() method for gRPC connection lifecycle over Unix sockets.

## Dependencies

### Task Dependencies (within this unit)
- Task #1 must be complete (defines Client struct)

### Package Dependencies
- `google.golang.org/grpc` - gRPC dialing
- `google.golang.org/grpc/credentials/insecure` - insecure transport for Unix socket
- `internal/api/v1` - generated service client

## Deliverables

### Files to Create/Modify

```
internal/client/
└── client.go    # MODIFY: Add New() and Close() methods
```

### Types to Implement

None (Client struct defined in Task #1).

### Functions to Implement

```go
// client.go

// New creates a client connected to the daemon Unix socket.
// The socketPath should be the full path to the daemon socket
// (typically ~/.choo/daemon.sock).
//
// The connection uses insecure credentials since Unix sockets
// are protected by filesystem permissions.
func New(socketPath string) (*Client, error) {
    // Use grpc.Dial with "unix://" scheme prefix
    // Apply insecure.NewCredentials() for local socket
    // Create DaemonServiceClient from connection
    // Return wrapped Client or error with context
}

// Close releases the underlying gRPC connection.
// It is safe to call Close multiple times.
func (c *Client) Close() error {
    // Close the underlying connection
    // Return any error from Close()
}
```

## Implementation Notes

Connection establishment pattern:
```go
conn, err := grpc.Dial(
    "unix://"+socketPath,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
```

The `unix://` scheme prefix instructs gRPC to use Unix domain socket transport.

## Backpressure

### Validation Command

```bash
go build ./internal/client/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build | Package compiles without errors |
| New signature | Returns (*Client, error) |
| Close signature | Returns error |

## NOT In Scope

- Automatic reconnection (design decision: errors surface to caller)
- Connection pooling (future enhancement)
- Custom dial options (future enhancement)
- Integration tests with actual daemon (manual testing)
