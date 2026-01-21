---
task: 6
status: pending
backpressure: "go test ./internal/daemon/... -run TestServer_Socket"
depends_on: [1, 2]
---

# Unix Domain Socket Server

**Parent spec**: `specs/DAEMON-GRPC.md`
**Task**: #6 of 7 in implementation plan

## Objective

Implement the daemon server with Unix domain socket listener, including socket file management and gRPC server setup.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Proto definitions for RegisterDaemonServiceServer
- Task #2: GRPCServer struct for service implementation

### Package Dependencies
- `github.com/RevCBH/choo/pkg/api/v1` - Generated protobuf types
- `google.golang.org/grpc` - gRPC server framework
- `net` - Unix socket listener
- `os` - File operations

## Deliverables

### Files to Create/Modify

```
internal/daemon/
├── server.go       # CREATE: Server struct with socket management
└── server_test.go  # CREATE: Socket and server tests
```

### Types to Implement

```go
// internal/daemon/server.go

package daemon

import (
    "context"
    "fmt"
    "net"
    "os"
    "path/filepath"
    "sync"

    apiv1 "github.com/RevCBH/choo/pkg/api/v1"
    "github.com/RevCBH/choo/internal/daemon/db"
    "google.golang.org/grpc"
)

const (
    // DefaultSocketPath is the default location for the daemon socket
    DefaultSocketPath = "/tmp/charlotte.sock"
)

// Server manages the gRPC server and Unix socket
type Server struct {
    socketPath string
    grpcServer *grpc.Server
    grpcImpl   *GRPCServer
    listener   net.Listener

    mu      sync.Mutex
    running bool
}

// ServerConfig holds configuration for the daemon server
type ServerConfig struct {
    // SocketPath overrides the default socket path
    SocketPath string

    // Version string reported in health checks
    Version string
}

// NewServer creates a new daemon server
func NewServer(cfg ServerConfig, database *db.DB, jobMgr JobManager) *Server {
    socketPath := cfg.SocketPath
    if socketPath == "" {
        socketPath = DefaultSocketPath
    }

    grpcImpl := NewGRPCServer(database, jobMgr, cfg.Version)
    grpcServer := grpc.NewServer()
    apiv1.RegisterDaemonServiceServer(grpcServer, grpcImpl)

    return &Server{
        socketPath: socketPath,
        grpcServer: grpcServer,
        grpcImpl:   grpcImpl,
    }
}

// SocketPath returns the socket path this server listens on
func (s *Server) SocketPath() string {
    return s.socketPath
}
```

### Functions to Implement

```go
// Start begins listening on the Unix socket and serving gRPC requests.
// This method blocks until Stop is called or an error occurs.
func (s *Server) Start() error {
    s.mu.Lock()
    if s.running {
        s.mu.Unlock()
        return fmt.Errorf("server already running")
    }
    s.running = true
    s.mu.Unlock()

    // Remove stale socket file if it exists
    if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("failed to remove stale socket: %w", err)
    }

    // Ensure socket directory exists
    socketDir := filepath.Dir(s.socketPath)
    if err := os.MkdirAll(socketDir, 0755); err != nil {
        return fmt.Errorf("failed to create socket directory: %w", err)
    }

    // Create Unix socket listener
    listener, err := net.Listen("unix", s.socketPath)
    if err != nil {
        return fmt.Errorf("failed to listen on socket: %w", err)
    }
    s.listener = listener

    // Set socket permissions (only owner can connect)
    if err := os.Chmod(s.socketPath, 0600); err != nil {
        listener.Close()
        return fmt.Errorf("failed to set socket permissions: %w", err)
    }

    // Serve gRPC requests (blocks until stopped)
    return s.grpcServer.Serve(listener)
}

// Stop gracefully stops the server.
// If ctx has a deadline, it will force stop after the deadline.
func (s *Server) Stop(ctx context.Context) error {
    s.mu.Lock()
    if !s.running {
        s.mu.Unlock()
        return nil
    }
    s.mu.Unlock()

    // Create channel to signal graceful stop completion
    done := make(chan struct{})
    go func() {
        s.grpcServer.GracefulStop()
        close(done)
    }()

    // Wait for graceful stop or context deadline
    select {
    case <-done:
        // Graceful stop completed
    case <-ctx.Done():
        // Force stop
        s.grpcServer.Stop()
    }

    // Clean up socket file
    if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("failed to remove socket file: %w", err)
    }

    s.mu.Lock()
    s.running = false
    s.mu.Unlock()

    return nil
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.running
}

// GRPCServer returns the underlying GRPCServer for testing
func (s *Server) GRPCServer() *GRPCServer {
    return s.grpcImpl
}
```

### Test Implementation

```go
// internal/daemon/server_test.go

package daemon

import (
    "context"
    "net"
    "os"
    "path/filepath"
    "testing"
    "time"

    apiv1 "github.com/RevCBH/choo/pkg/api/v1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func TestServer_SocketCreation(t *testing.T) {
    socketPath := filepath.Join(t.TempDir(), "test.sock")
    jm := newMockJobManager()

    server := NewServer(ServerConfig{
        SocketPath: socketPath,
        Version:    "v1.0.0",
    }, nil, jm)

    // Start server in background
    errCh := make(chan error, 1)
    go func() {
        errCh <- server.Start()
    }()

    // Wait for socket to be created
    require.Eventually(t, func() bool {
        _, err := os.Stat(socketPath)
        return err == nil
    }, time.Second, 10*time.Millisecond)

    // Verify socket exists
    info, err := os.Stat(socketPath)
    require.NoError(t, err)
    assert.Equal(t, os.ModeSocket, info.Mode()&os.ModeSocket)

    // Stop server
    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    require.NoError(t, server.Stop(ctx))
}

func TestServer_SocketPermissions(t *testing.T) {
    socketPath := filepath.Join(t.TempDir(), "test.sock")
    jm := newMockJobManager()

    server := NewServer(ServerConfig{
        SocketPath: socketPath,
    }, nil, jm)

    go server.Start()

    require.Eventually(t, func() bool {
        _, err := os.Stat(socketPath)
        return err == nil
    }, time.Second, 10*time.Millisecond)

    // Check permissions are 0600
    info, err := os.Stat(socketPath)
    require.NoError(t, err)
    assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    server.Stop(ctx)
}

func TestServer_SocketRemovesStaleFile(t *testing.T) {
    socketPath := filepath.Join(t.TempDir(), "test.sock")

    // Create a stale socket file
    f, err := os.Create(socketPath)
    require.NoError(t, err)
    f.Close()

    jm := newMockJobManager()
    server := NewServer(ServerConfig{
        SocketPath: socketPath,
    }, nil, jm)

    go server.Start()

    require.Eventually(t, func() bool {
        info, err := os.Stat(socketPath)
        if err != nil {
            return false
        }
        // Should be a socket now, not a regular file
        return info.Mode()&os.ModeSocket != 0
    }, time.Second, 10*time.Millisecond)

    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    server.Stop(ctx)
}

func TestServer_SocketCleanuponStop(t *testing.T) {
    socketPath := filepath.Join(t.TempDir(), "test.sock")
    jm := newMockJobManager()

    server := NewServer(ServerConfig{
        SocketPath: socketPath,
    }, nil, jm)

    go server.Start()

    require.Eventually(t, func() bool {
        _, err := os.Stat(socketPath)
        return err == nil
    }, time.Second, 10*time.Millisecond)

    // Stop server
    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    require.NoError(t, server.Stop(ctx))

    // Socket should be removed
    _, err := os.Stat(socketPath)
    assert.True(t, os.IsNotExist(err))
}

func TestServer_SocketGRPCConnection(t *testing.T) {
    socketPath := filepath.Join(t.TempDir(), "test.sock")
    jm := newMockJobManager()

    server := NewServer(ServerConfig{
        SocketPath: socketPath,
        Version:    "v1.2.3",
    }, nil, jm)

    go server.Start()

    require.Eventually(t, func() bool {
        _, err := os.Stat(socketPath)
        return err == nil
    }, time.Second, 10*time.Millisecond)

    // Connect via gRPC
    conn, err := grpc.Dial(
        "unix://"+socketPath,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    require.NoError(t, err)
    defer conn.Close()

    // Call Health RPC
    client := apiv1.NewDaemonServiceClient(conn)
    resp, err := client.Health(context.Background(), &apiv1.HealthRequest{})
    require.NoError(t, err)
    assert.True(t, resp.Healthy)
    assert.Equal(t, "v1.2.3", resp.Version)

    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    server.Stop(ctx)
}

func TestServer_SocketDoubleStart(t *testing.T) {
    socketPath := filepath.Join(t.TempDir(), "test.sock")
    jm := newMockJobManager()

    server := NewServer(ServerConfig{
        SocketPath: socketPath,
    }, nil, jm)

    go server.Start()

    require.Eventually(t, func() bool {
        return server.IsRunning()
    }, time.Second, 10*time.Millisecond)

    // Second start should fail
    err := server.Start()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "already running")

    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    server.Stop(ctx)
}

func TestServer_SocketGracefulStop(t *testing.T) {
    socketPath := filepath.Join(t.TempDir(), "test.sock")
    jm := newMockJobManager()

    server := NewServer(ServerConfig{
        SocketPath: socketPath,
    }, nil, jm)

    go server.Start()

    require.Eventually(t, func() bool {
        return server.IsRunning()
    }, time.Second, 10*time.Millisecond)

    // Graceful stop with long timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    err := server.Stop(ctx)
    require.NoError(t, err)

    assert.False(t, server.IsRunning())
}

func TestServer_SocketForceStop(t *testing.T) {
    socketPath := filepath.Join(t.TempDir(), "test.sock")
    jm := newMockJobManager()

    server := NewServer(ServerConfig{
        SocketPath: socketPath,
    }, nil, jm)

    go server.Start()

    require.Eventually(t, func() bool {
        return server.IsRunning()
    }, time.Second, 10*time.Millisecond)

    // Force stop with immediate timeout
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel immediately to force stop

    err := server.Stop(ctx)
    require.NoError(t, err)
    assert.False(t, server.IsRunning())
}

func TestServer_SocketDefaultPath(t *testing.T) {
    jm := newMockJobManager()

    server := NewServer(ServerConfig{
        // SocketPath not specified
    }, nil, jm)

    assert.Equal(t, DefaultSocketPath, server.SocketPath())
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestServer_Socket
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestServer_SocketCreation` | Socket file created |
| `TestServer_SocketPermissions` | Permissions are 0600 |
| `TestServer_SocketRemovesStaleFile` | Stale socket replaced |
| `TestServer_SocketCleanuponStop` | Socket removed on stop |
| `TestServer_SocketGRPCConnection` | Client can connect and call Health |
| `TestServer_SocketDoubleStart` | Second start returns error |
| `TestServer_SocketGracefulStop` | Clean shutdown |
| `TestServer_SocketForceStop` | Force shutdown works |
| `TestServer_SocketDefaultPath` | Uses /tmp/charlotte.sock by default |

## NOT In Scope

- TCP listener support (future enhancement)
- TLS configuration (future enhancement)
- gRPC reflection API (future enhancement)
- Process signal handling (CLI responsibility)
