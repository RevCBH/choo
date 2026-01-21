# Task 02: Daemon Core

```yaml
task: 02-daemon
unit: daemon
depends_on: [01-pidfile]
backpressure: "go test ./internal/daemon/... -run TestDaemon -v"
```

## Objective

Implement the core `Daemon` struct with lifecycle management (start, stop, health check).

## Requirements

1. Create `internal/daemon/daemon.go` with:

   ```go
   type Config struct {
       DataDir     string // ~/.choo
       Port        int    // default 8099
       Host        string // default "127.0.0.1"
       IdleTimeout time.Duration // shutdown after no activity
   }

   type Daemon struct {
       cfg     Config
       store   *history.Store
       pidFile *PIDFile
       server  *http.Server
       runs    map[string]*RunState // active runs
       mu      sync.RWMutex
   }

   type RunState struct {
       ID        string
       RepoPath  string
       StartedAt time.Time
       Seq       int64 // next sequence number
   }

   // Constructor
   func New(cfg Config) (*Daemon, error)

   // Lifecycle
   func (d *Daemon) Start() error  // Acquire PID, open store, start HTTP
   func (d *Daemon) Stop() error   // Graceful shutdown
   func (d *Daemon) Wait() error   // Block until shutdown

   // Health
   func (d *Daemon) IsHealthy() bool
   func (d *Daemon) Addr() string  // Returns "host:port"

   // Run management (called by HTTP handlers)
   func (d *Daemon) RegisterRun(cfg history.RunConfig) (*history.Run, error)
   func (d *Daemon) RecordEvent(runID string, event history.EventRecord) (*history.StoredEvent, error)
   func (d *Daemon) CompleteRun(runID string, result history.RunResult) (*history.Run, error)
   func (d *Daemon) GetActiveRuns() []*RunState
   ```

2. Start sequence:
   - Create data directory if needed
   - Acquire PID file
   - Open history store (with migrations)
   - Start HTTP server
   - Log startup message with address

3. Shutdown sequence:
   - Stop accepting new connections
   - Wait for active requests (with timeout)
   - Close history store
   - Release PID file
   - Log shutdown message

4. Run state tracking:
   - Track active runs in memory
   - Sequence numbers per run
   - Clean up run state when completed

## Acceptance Criteria

- [ ] Daemon starts and stops cleanly
- [ ] PID file prevents duplicate instances
- [ ] History store is initialized with migrations
- [ ] HTTP server binds to configured address
- [ ] Active runs are tracked in memory
- [ ] Graceful shutdown completes within timeout

## Files to Create/Modify

- `internal/daemon/daemon.go` (create)
- `internal/daemon/daemon_test.go` (create)
