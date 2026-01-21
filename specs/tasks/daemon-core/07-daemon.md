---
task: 7
status: pending
backpressure: "go test ./internal/daemon/... -run TestDaemon"
depends_on: [1, 2, 4, 6]
---

# Daemon Lifecycle

**Parent spec**: `specs/DAEMON-CORE.md`
**Task**: #7 of 7 in implementation plan

## Objective

Wire together all daemon components into the main Daemon struct with startup and shutdown lifecycle management.

## Dependencies

### Task Dependencies (within this unit)
- Task #1 (Config)
- Task #2 (PID file utilities)
- Task #4 (JobManager)
- Task #6 (Resume logic)

### Package Dependencies
- `net` - for Unix socket listener
- `os` - for socket cleanup
- `sync` - for WaitGroup
- `context` - for cancellation
- `log` - for daemon logging
- `google.golang.org/grpc` - for gRPC server
- `github.com/charlotte/pkg/api/v1` - for service registration

## Deliverables

### Files to Create/Modify

```
internal/daemon/
└── daemon.go    # CREATE: Main daemon struct and lifecycle
```

### Types to Implement

```go
// Daemon is the main daemon process coordinator.
type Daemon struct {
    cfg        *Config
    db         *db.DB
    jobManager *JobManager
    grpcServer *grpc.Server
    listener   net.Listener
    pidFile    *PIDFile

    shutdownCh chan struct{}
    wg         sync.WaitGroup
}
```

### Functions to Implement

```go
// New creates a new daemon instance.
func New(cfg *Config) (*Daemon, error) {
    // 1. Validate config
    // 2. Ensure directories exist
    // 3. Open database connection
    // 4. Create JobManager
    // 5. Create PIDFile manager
    // 6. Return initialized Daemon
}

// Start begins the daemon, resumes jobs, and listens for connections.
// Blocks until shutdown is triggered.
func (d *Daemon) Start(ctx context.Context) error {
    // 1. Acquire PID file (fails if daemon already running)
    // 2. Resume any interrupted jobs
    // 3. Create Unix socket listener (remove stale socket first)
    // 4. Create and register gRPC server
    // 5. Start gRPC server in goroutine
    // 6. Log startup message
    // 7. Wait for shutdown signal
    // 8. Run graceful shutdown
}

// Shutdown initiates graceful shutdown.
func (d *Daemon) Shutdown() {
    // Close shutdown channel to signal Start() to exit
}

// gracefulShutdown performs ordered shutdown of daemon components.
func (d *Daemon) gracefulShutdown(ctx context.Context) error {
    // 1. Stop accepting new gRPC connections
    // 2. Signal all jobs to stop at safe points
    // 3. Wait for jobs with timeout (30s)
    // 4. Force kill jobs if timeout exceeded
    // 5. Close database connection
    // 6. Release PID file
    // 7. Remove socket file
}

// setupSocket creates the Unix domain socket listener.
func (d *Daemon) setupSocket() (net.Listener, error) {
    // Remove stale socket file
    // Create Unix listener
    // Set permissions (0600 - user only)
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestDaemon
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestDaemon_New` | Creates daemon with valid config |
| `TestDaemon_New_InvalidConfig` | Returns error for invalid config |
| `TestDaemon_Start_CreatesPIDFile` | PID file created on start |
| `TestDaemon_Start_AlreadyRunning` | Returns error if PID file exists |
| `TestDaemon_Start_CreatesSocket` | Unix socket created |
| `TestDaemon_Start_ResumesJobs` | Resume called on startup |
| `TestDaemon_Shutdown_GracefulStop` | Jobs stopped gracefully |
| `TestDaemon_Shutdown_CleanupFiles` | PID and socket files removed |
| `TestDaemon_Shutdown_Timeout` | Force kill after timeout |

### Test Fixtures

```go
func TestDaemon_New(t *testing.T) {
    tmpDir := t.TempDir()
    cfg := &Config{
        SocketPath: filepath.Join(tmpDir, "daemon.sock"),
        PIDFile:    filepath.Join(tmpDir, "daemon.pid"),
        DBPath:     filepath.Join(tmpDir, "test.db"),
        MaxJobs:    5,
    }

    d, err := New(cfg)
    require.NoError(t, err)
    require.NotNil(t, d)

    assert.NotNil(t, d.jobManager)
    assert.NotNil(t, d.db)
}

func TestDaemon_Start_CreatesPIDFile(t *testing.T) {
    tmpDir := t.TempDir()
    cfg := testConfig(tmpDir)

    d, err := New(cfg)
    require.NoError(t, err)

    ctx, cancel := context.WithCancel(context.Background())

    // Start in goroutine since it blocks
    errCh := make(chan error, 1)
    go func() {
        errCh <- d.Start(ctx)
    }()

    // Wait for startup
    time.Sleep(100 * time.Millisecond)

    // Verify PID file exists
    _, err = os.Stat(cfg.PIDFile)
    assert.NoError(t, err)

    // Trigger shutdown
    cancel()
    d.Shutdown()

    // Wait for clean exit
    select {
    case err := <-errCh:
        assert.NoError(t, err)
    case <-time.After(5 * time.Second):
        t.Fatal("timeout waiting for shutdown")
    }
}

func TestDaemon_Shutdown_CleanupFiles(t *testing.T) {
    tmpDir := t.TempDir()
    cfg := testConfig(tmpDir)

    d, err := New(cfg)
    require.NoError(t, err)

    ctx, cancel := context.WithCancel(context.Background())

    go d.Start(ctx)
    time.Sleep(100 * time.Millisecond)

    // Files should exist
    assert.FileExists(t, cfg.PIDFile)
    assert.FileExists(t, cfg.SocketPath)

    cancel()
    d.Shutdown()
    time.Sleep(100 * time.Millisecond)

    // Files should be cleaned up
    assert.NoFileExists(t, cfg.PIDFile)
    assert.NoFileExists(t, cfg.SocketPath)
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `defer` for cleanup to ensure PID and socket removal
- Shutdown timeout should be configurable (default 30s per spec)
- Log startup/shutdown messages for operational visibility
- Socket file permissions 0600 ensure only owner can connect
- Resume errors are logged but don't prevent daemon startup

## NOT In Scope

- gRPC service implementation (handled by DAEMON-GRPC)
- CLI integration (handled by DAEMON-CLI)
- Signal handling (SIGTERM/SIGINT) in main package
