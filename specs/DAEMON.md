# DAEMON - Long-running Daemon Process for History and Web UI

## Overview

The Daemon package provides a long-running background process that owns all writes to the history database and serves the web UI. It acts as the single point of coordination for orchestrator processes, receiving event streams via HTTP, persisting them to SQLite with WAL mode, and broadcasting to connected browsers.

**Note:** The daemon replaces and subsumes the existing web server concept from `specs/completed/WEB.md`. The prior web server provided real-time visualization for a single run; the daemon extends this to support historical run storage, cross-run queries, and persistence across CLI invocations.

The architecture separates concerns: orchestrator processes focus on executing tasks while the daemon handles persistence and visualization. This single-writer pattern eliminates database contention and enables concurrent read access from the web UI. The daemon starts automatically when running `choo run` (unless `--no-daemon` is specified) and persists across multiple orchestration runs.

```
                          Choo Daemon (single process)
                         +-----------------------------+
                         |     HTTP Server (web UI)    |
                         |              |              |
                         |       History Store         |
                         |      (sole DB writer)       |
                         |              |              |
Orchestrator --events--> |         SQLite DB           |
  Process                |        (WAL mode)           |
    |                    +-----------------------------+
    |                                  |
    +-------[HTTP]---------------------+
                                       |
                                    Browser
                              [History View + Graph]
```

## Requirements

### Functional Requirements

1. Run as a single daemon process with PID file enforcement
2. Own all writes to the SQLite history database (single writer pattern)
3. Configure SQLite with WAL mode for concurrent read access
4. Accept HTTP connections from orchestrator processes for event streaming
5. Persist events to the history database with sequence ordering
6. Serve the web UI on a configurable port (default: 8080)
7. Provide REST API for history queries (runs, events, graphs)
8. Broadcast real-time events to connected browsers via SSE
9. Support multiple repositories concurrently (repo-scoped history)
10. Start automatically when `choo run` executes (if configured)
11. Stop gracefully on `choo daemon stop` or SIGTERM
12. Clean up stale PID files on startup
13. Support `--no-daemon` flag for fully isolated execution
14. Support `--no-history` flag for daemon without history recording

### Performance Requirements

| Metric | Target |
|--------|--------|
| Event write latency | <5ms per event |
| API response time | <50ms for state/graph endpoints |
| Concurrent orchestrators | 10+ simultaneous connections |
| Concurrent browsers | 100+ SSE connections |
| Database write throughput | 1000 events/sec sustained |
| Memory usage | <100MB base + 10KB per connection |

### Constraints

- SQLite with WAL mode (no external database dependencies)
- Go stdlib plus `database/sql` with `modernc.org/sqlite` driver
- Single PID file at `~/.choo/daemon.pid`
- Database at `~/.choo/history.db`
- Bind to localhost only by default (security)
- HTTP-based IPC (no Unix sockets for daemon communication)

## Design

### Shared Types

All shared types (Run, RunStatus, EventRecord, StoredEvent, GraphData, ListOptions, etc.) are defined in [HISTORY-TYPES.md](./HISTORY-TYPES.md). This spec references those types rather than redefining them.

### Module Structure

```
internal/daemon/
+-- daemon.go    # Daemon process lifecycle, HTTP server, event receiver
+-- client.go    # Client for CLI to communicate with daemon
+-- pidfile.go   # PID file management for single-instance enforcement

internal/history/
+-- store.go     # SQLite operations with WAL mode
+-- types.go     # Data types (imports from HISTORY-TYPES.md definitions)
+-- migrate.go   # Schema creation and migration
+-- handler.go   # Event handler that sends to daemon via HTTP
```

### Daemon-Specific Types

```go
// internal/daemon/daemon.go

// Daemon is the long-running process that owns history writes and serves web UI
type Daemon struct {
    cfg        Config
    store      *history.Store
    httpServer *http.Server
    sseHub     *web.Hub
    pidFile    *PIDFile

    mu         sync.RWMutex
    activeRuns map[string]*RunState // runID -> state
}

// Config holds daemon configuration
type Config struct {
    // Port is the HTTP listen port (default: 8080)
    Port int

    // DBPath is the SQLite database path (default: ~/.choo/history.db)
    DBPath string

    // PIDPath is the PID file path (default: ~/.choo/daemon.pid)
    PIDPath string

    // LogPath is the log file path (default: ~/.choo/daemon.log)
    LogPath string
}

// RunState tracks an active orchestration run
type RunState struct {
    RunID     string
    RepoPath  string
    StartedAt time.Time
    Seq       int64 // next sequence number
}
```

```go
// internal/daemon/client.go

// Client communicates with the daemon from CLI processes
type Client struct {
    baseURL    string
    httpClient *http.Client
}

// RunConfig and RunResult are defined in HISTORY-TYPES.md
```

```go
// internal/daemon/pidfile.go

// PIDFile manages daemon single-instance enforcement
type PIDFile struct {
    path string
    file *os.File
}
```

```go
// internal/history/types.go
//
// All shared types (Run, RunStatus, EventRecord, StoredEvent, GraphData,
// GraphNode, GraphEdge, ListOptions, EventListOptions, RunList, EventList)
// are defined in HISTORY-TYPES.md and implemented in this file.
```

```go
// internal/history/store.go

// Store manages SQLite operations for history persistence
type Store struct {
    db *sql.DB
}
```

```go
// internal/history/handler.go

// Handler sends events to the daemon instead of writing directly
type Handler struct {
    client *daemon.Client
    runID  string
    seq    atomic.Int64
}
```

### API Surface

```go
// internal/daemon/daemon.go

// New creates a new daemon with the given configuration
func New(cfg Config) (*Daemon, error)

// Start begins the daemon process (blocks until Stop is called)
func (d *Daemon) Start() error

// Stop performs graceful shutdown
func (d *Daemon) Stop(ctx context.Context) error

// ServeHTTP handles all HTTP requests (implements http.Handler)
func (d *Daemon) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

```go
// internal/daemon/client.go

// NewClient creates a client for daemon communication
func NewClient(baseURL string) *Client

// DefaultClient creates a client with default localhost URL
func DefaultClient() *Client

// IsRunning checks if the daemon is responding
func (c *Client) IsRunning() bool

// StartRun registers a new orchestration run
func (c *Client) StartRun(cfg RunConfig) (string, error)

// SendEvent sends an event to be persisted
func (c *Client) SendEvent(runID string, event EventRecord) error

// SendEvents sends multiple events in a batch
func (c *Client) SendEvents(runID string, events []EventRecord) error

// AppendResumeMarker adds a resume marker to an existing run
func (c *Client) AppendResumeMarker(runID string) error

// CompleteRun marks a run as finished
func (c *Client) CompleteRun(runID string, result RunResult) error
```

```go
// internal/daemon/pidfile.go

// NewPIDFile creates a PID file manager
func NewPIDFile(path string) *PIDFile

// Acquire locks the PID file and writes current PID
// Returns error if daemon already running
func (p *PIDFile) Acquire() error

// Release removes the PID file
func (p *PIDFile) Release() error

// ReadPID returns the PID from an existing file (0 if not exists)
func ReadPID(path string) (int, error)

// IsStale checks if a PID file refers to a dead process
func IsStale(path string) bool
```

```go
// internal/history/store.go

// NewStore opens the SQLite database with WAL mode
func NewStore(dbPath string) (*Store, error)

// Close closes the database connection
func (s *Store) Close() error

// CreateRun inserts a new run record
func (s *Store) CreateRun(cfg RunConfig) (string, error)

// InsertEvent inserts an event with sequence number
func (s *Store) InsertEvent(runID string, seq int, event EventRecord) error

// InsertEvents inserts multiple events in a transaction
func (s *Store) InsertEvents(runID string, events []SequencedEvent) error

// AppendResumeMarker inserts a run.resumed event
func (s *Store) AppendResumeMarker(runID string, seq int) error

// CompleteRun updates run status and final counts
func (s *Store) CompleteRun(runID string, result RunResult) error

// GetRun retrieves a single run by ID
func (s *Store) GetRun(runID string) (*Run, error)

// ListRuns returns runs for a repository (newest first)
func (s *Store) ListRuns(repoPath string, opts ListOptions) (*RunList, error)

// GetRunEvents retrieves events for a run
func (s *Store) GetRunEvents(runID string, opts EventListOptions) (*EventList, error)

// GetGraph retrieves the dependency graph for a run
func (s *Store) GetGraph(runID string) (*GraphData, error)

// StoreGraph persists the dependency graph for a run
func (s *Store) StoreGraph(runID string, graph *GraphData) error

// Cleanup removes runs older than retention period
func (s *Store) Cleanup(olderThan time.Duration) (int, error)
```

```go
// internal/history/handler.go

// NewHandler creates an event handler that sends to daemon
func NewHandler(client *daemon.Client, runID string) *Handler

// Handle processes an event and sends to daemon
func (h *Handler) Handle(e events.Event)
```

### HTTP API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check (returns 200 if daemon running) |
| `/api/runs` | POST | Start a new run (from orchestrator) |
| `/api/runs/{id}/events` | POST | Send events for a run |
| `/api/runs/{id}/resume` | POST | Append resume marker |
| `/api/runs/{id}/complete` | POST | Mark run complete |
| `/api/history/runs` | GET | List runs (query: repo, limit, offset) |
| `/api/history/runs/{id}` | GET | Get run details |
| `/api/history/runs/{id}/events` | GET | Get run events (query: type, unit, limit, offset) |
| `/api/history/runs/{id}/graph` | GET | Get dependency graph |
| `/api/state` | GET | Current live state (for web UI) |
| `/api/events` | GET | SSE stream for real-time events |
| `/` | GET | Serve embedded web UI |

### Request/Response Formats

**POST /api/runs**
```json
// Request
{
    "repo_path": "/Users/dev/myproject",
    "tasks_dir": "specs/tasks",
    "parallelism": 4,
    "total_units": 12,
    "dry_run": false,
    "graph": {
        "nodes": [{"id": "unit-a", "level": 0}],
        "edges": [{"from": "unit-a", "to": "unit-b"}],
        "levels": [["unit-a"], ["unit-b"]]
    }
}

// Response
{
    "run_id": "run_20250120_143052_a1b2"
}
```

**POST /api/runs/{id}/events**
```json
// Request
{
    "events": [
        {
            "time": "2025-01-20T14:30:52Z",
            "type": "unit.started",
            "unit": "app-shell"
        },
        {
            "time": "2025-01-20T14:30:53Z",
            "type": "task.started",
            "unit": "app-shell",
            "task": 1
        }
    ]
}

// Response
{
    "accepted": 2
}
```

**POST /api/runs/{id}/complete**
```json
// Request
{
    "status": "completed",
    "completed_units": 10,
    "failed_units": 2,
    "blocked_units": 0,
    "error": ""
}

// Response
{
    "ok": true
}
```

**GET /api/history/runs?repo=/Users/dev/myproject&limit=20&offset=0**
```json
{
    "runs": [
        {
            "id": "run_20250120_143052_a1b2",
            "repo_path": "/Users/dev/myproject",
            "started_at": "2025-01-20T14:30:52Z",
            "completed_at": "2025-01-20T15:00:00Z",
            "status": "completed",
            "parallelism": 4,
            "total_units": 12,
            "completed_units": 10,
            "failed_units": 2,
            "blocked_units": 0
        }
    ],
    "total": 45
}
```

## Implementation Notes

### PID File Management

The daemon enforces single-instance via a PID file with advisory locking.

```go
func (p *PIDFile) Acquire() error {
    // Check for stale PID file
    if IsStale(p.path) {
        os.Remove(p.path)
    }

    // Create/open file with exclusive access
    file, err := os.OpenFile(p.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
    if err != nil {
        if os.IsExist(err) {
            return fmt.Errorf("daemon already running (PID file exists: %s)", p.path)
        }
        return fmt.Errorf("create PID file: %w", err)
    }

    // Write current PID
    fmt.Fprintf(file, "%d\n", os.Getpid())
    p.file = file
    return nil
}

func IsStale(path string) bool {
    pid, err := ReadPID(path)
    if err != nil || pid == 0 {
        return true
    }

    // Check if process exists
    proc, err := os.FindProcess(pid)
    if err != nil {
        return true
    }

    // On Unix, sending signal 0 checks if process exists
    err = proc.Signal(syscall.Signal(0))
    return err != nil
}
```

### SQLite WAL Mode Configuration

The store initializes SQLite with WAL mode for concurrent reads.

```go
func NewStore(dbPath string) (*Store, error) {
    // Ensure directory exists
    dir := filepath.Dir(dbPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return nil, fmt.Errorf("create db directory: %w", err)
    }

    // Open database
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return nil, fmt.Errorf("open database: %w", err)
    }

    // Configure for WAL mode and better concurrency
    pragmas := []string{
        "PRAGMA journal_mode=WAL",
        "PRAGMA busy_timeout=5000",
        "PRAGMA synchronous=NORMAL",
        "PRAGMA cache_size=-64000", // 64MB cache
        "PRAGMA foreign_keys=ON",
    }

    for _, pragma := range pragmas {
        if _, err := db.Exec(pragma); err != nil {
            db.Close()
            return nil, fmt.Errorf("set pragma %s: %w", pragma, err)
        }
    }

    store := &Store{db: db}

    // Run migrations
    if err := store.migrate(); err != nil {
        db.Close()
        return nil, fmt.Errorf("migrate: %w", err)
    }

    return store, nil
}
```

### Run ID Generation

Run IDs are human-readable and sortable.

```go
func generateRunID() string {
    now := time.Now()
    suffix := make([]byte, 4)
    rand.Read(suffix)
    return fmt.Sprintf("run_%s_%s",
        now.Format("20060102_150405"),
        hex.EncodeToString(suffix)[:4],
    )
}
```

### Event Batching

The handler batches events to reduce HTTP overhead.

```go
type Handler struct {
    client    *daemon.Client
    runID     string
    seq       atomic.Int64
    batch     []EventRecord
    batchMu   sync.Mutex
    flushChan chan struct{}
}

func (h *Handler) Handle(e events.Event) {
    record := EventRecord{
        Time:    e.Time,
        Type:    string(e.Type),
        Unit:    e.Unit,
        Task:    e.Task,
        PR:      e.PR,
        Payload: marshalPayload(e.Payload),
        Error:   e.Error,
    }

    h.batchMu.Lock()
    h.batch = append(h.batch, record)
    shouldFlush := len(h.batch) >= 10
    h.batchMu.Unlock()

    if shouldFlush {
        select {
        case h.flushChan <- struct{}{}:
        default:
        }
    }
}

func (h *Handler) flushLoop() {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-h.flushChan:
        case <-ticker.C:
        }

        h.batchMu.Lock()
        if len(h.batch) == 0 {
            h.batchMu.Unlock()
            continue
        }
        batch := h.batch
        h.batch = nil
        h.batchMu.Unlock()

        if err := h.client.SendEvents(h.runID, batch); err != nil {
            log.Printf("WARN: failed to send events: %v", err)
        }
    }
}
```

### Resume Marker Handling

When a run resumes, events continue with new sequence numbers.

```go
func (s *Store) AppendResumeMarker(runID string, seq int) error {
    event := EventRecord{
        Time: time.Now(),
        Type: "run.resumed",
    }

    payload, _ := json.Marshal(map[string]int{"resume_seq": seq})

    _, err := s.db.Exec(`
        INSERT INTO events (run_id, seq, time, type, payload)
        VALUES (?, ?, ?, ?, ?)
    `, runID, seq, event.Time, event.Type, payload)

    return err
}
```

### Graceful Shutdown

The daemon waits for in-flight requests during shutdown.

```go
func (d *Daemon) Stop(ctx context.Context) error {
    // Signal active runs that daemon is stopping
    d.mu.RLock()
    for runID := range d.activeRuns {
        log.Printf("Notifying run %s of daemon shutdown", runID)
    }
    d.mu.RUnlock()

    // Shutdown HTTP server with timeout
    shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    if err := d.httpServer.Shutdown(shutdownCtx); err != nil {
        log.Printf("HTTP shutdown error: %v", err)
    }

    // Close database
    if err := d.store.Close(); err != nil {
        log.Printf("Database close error: %v", err)
    }

    // Release PID file
    d.pidFile.Release()

    return nil
}
```

### Configuration Loading

Daemon config is loaded from the standard config file with environment overrides.

```go
// DaemonConfig holds daemon-specific configuration
type DaemonConfig struct {
    Port      int  `yaml:"port"`
    AutoStart bool `yaml:"auto_start"`
}

// HistoryConfig holds history-specific configuration
type HistoryConfig struct {
    Enabled       bool `yaml:"enabled"`
    RetentionDays int  `yaml:"retention_days"`
}
```

## Testing Strategy

### Unit Tests

```go
// internal/daemon/pidfile_test.go

func TestPIDFile_Acquire(t *testing.T) {
    tmpDir := t.TempDir()
    pidPath := filepath.Join(tmpDir, "test.pid")

    pf := NewPIDFile(pidPath)
    if err := pf.Acquire(); err != nil {
        t.Fatalf("failed to acquire: %v", err)
    }
    defer pf.Release()

    // Verify PID written
    pid, err := ReadPID(pidPath)
    if err != nil {
        t.Fatalf("failed to read PID: %v", err)
    }
    if pid != os.Getpid() {
        t.Errorf("expected PID %d, got %d", os.Getpid(), pid)
    }

    // Second acquire should fail
    pf2 := NewPIDFile(pidPath)
    if err := pf2.Acquire(); err == nil {
        t.Error("expected error on second acquire")
    }
}

func TestPIDFile_StaleDetection(t *testing.T) {
    tmpDir := t.TempDir()
    pidPath := filepath.Join(tmpDir, "test.pid")

    // Write a PID that doesn't exist
    os.WriteFile(pidPath, []byte("99999999\n"), 0644)

    if !IsStale(pidPath) {
        t.Error("expected stale PID to be detected")
    }
}
```

```go
// internal/history/store_test.go

func TestStore_CreateAndGetRun(t *testing.T) {
    store := newTestStore(t)
    defer store.Close()

    cfg := RunConfig{
        RepoPath:    "/test/repo",
        TasksDir:    "specs/tasks",
        Parallelism: 4,
        TotalUnits:  10,
    }

    runID, err := store.CreateRun(cfg)
    if err != nil {
        t.Fatalf("CreateRun failed: %v", err)
    }

    run, err := store.GetRun(runID)
    if err != nil {
        t.Fatalf("GetRun failed: %v", err)
    }

    if run.RepoPath != cfg.RepoPath {
        t.Errorf("expected repo %s, got %s", cfg.RepoPath, run.RepoPath)
    }
    if run.Status != "running" {
        t.Errorf("expected status running, got %s", run.Status)
    }
}

func TestStore_InsertAndGetEvents(t *testing.T) {
    store := newTestStore(t)
    defer store.Close()

    runID, _ := store.CreateRun(RunConfig{RepoPath: "/test"})

    events := []SequencedEvent{
        {Seq: 1, Event: EventRecord{Type: "unit.started", Unit: "app"}},
        {Seq: 2, Event: EventRecord{Type: "task.started", Unit: "app", Task: ptr(1)}},
        {Seq: 3, Event: EventRecord{Type: "task.completed", Unit: "app", Task: ptr(1)}},
    }

    if err := store.InsertEvents(runID, events); err != nil {
        t.Fatalf("InsertEvents failed: %v", err)
    }

    list, err := store.GetRunEvents(runID, EventListOptions{})
    if err != nil {
        t.Fatalf("GetRunEvents failed: %v", err)
    }

    if len(list.Events) != 3 {
        t.Errorf("expected 3 events, got %d", len(list.Events))
    }
    if list.Events[0].Seq != 1 {
        t.Errorf("expected seq 1, got %d", list.Events[0].Seq)
    }
}

func TestStore_WALMode(t *testing.T) {
    store := newTestStore(t)
    defer store.Close()

    var mode string
    err := store.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
    if err != nil {
        t.Fatalf("failed to get journal_mode: %v", err)
    }

    if mode != "wal" {
        t.Errorf("expected WAL mode, got %s", mode)
    }
}

func newTestStore(t *testing.T) *Store {
    t.Helper()
    dbPath := filepath.Join(t.TempDir(), "test.db")
    store, err := NewStore(dbPath)
    if err != nil {
        t.Fatalf("failed to create store: %v", err)
    }
    return store
}
```

```go
// internal/daemon/client_test.go

func TestClient_IsRunning(t *testing.T) {
    // Start test server
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    defer srv.Close()

    client := NewClient(srv.URL)
    if !client.IsRunning() {
        t.Error("expected IsRunning to return true")
    }

    srv.Close()
    if client.IsRunning() {
        t.Error("expected IsRunning to return false after server close")
    }
}

func TestClient_StartRun(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/api/runs" || r.Method != "POST" {
            t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
        }

        var cfg RunConfig
        json.NewDecoder(r.Body).Decode(&cfg)

        if cfg.RepoPath != "/test/repo" {
            t.Errorf("expected repo /test/repo, got %s", cfg.RepoPath)
        }

        json.NewEncoder(w).Encode(map[string]string{"run_id": "run_123"})
    }))
    defer srv.Close()

    client := NewClient(srv.URL)
    runID, err := client.StartRun(RunConfig{RepoPath: "/test/repo"})
    if err != nil {
        t.Fatalf("StartRun failed: %v", err)
    }
    if runID != "run_123" {
        t.Errorf("expected run_123, got %s", runID)
    }
}
```

```go
// internal/daemon/daemon_test.go

func TestDaemon_HandleEventStream(t *testing.T) {
    cfg := Config{
        Port:    0, // random port
        DBPath:  filepath.Join(t.TempDir(), "test.db"),
        PIDPath: filepath.Join(t.TempDir(), "test.pid"),
    }

    daemon, err := New(cfg)
    if err != nil {
        t.Fatalf("failed to create daemon: %v", err)
    }

    srv := httptest.NewServer(daemon)
    defer srv.Close()

    client := NewClient(srv.URL)

    // Start a run
    runID, err := client.StartRun(RunConfig{
        RepoPath:   "/test",
        TotalUnits: 5,
    })
    if err != nil {
        t.Fatalf("StartRun failed: %v", err)
    }

    // Send events
    events := []EventRecord{
        {Type: "unit.started", Unit: "app"},
        {Type: "unit.completed", Unit: "app"},
    }
    if err := client.SendEvents(runID, events); err != nil {
        t.Fatalf("SendEvents failed: %v", err)
    }

    // Complete run
    if err := client.CompleteRun(runID, RunResult{
        Status:         "completed",
        CompletedUnits: 5,
    }); err != nil {
        t.Fatalf("CompleteRun failed: %v", err)
    }

    // Verify via history API
    run, err := daemon.store.GetRun(runID)
    if err != nil {
        t.Fatalf("GetRun failed: %v", err)
    }
    if run.Status != "completed" {
        t.Errorf("expected completed, got %s", run.Status)
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Full lifecycle | Start daemon, run orchestrator, verify events persisted |
| Multiple repos | Run orchestrators in different repos, verify scoped history |
| Resume flow | Start run, stop, resume, verify events append correctly |
| Concurrent orchestrators | Start 3 orchestrators simultaneously, verify no conflicts |
| Daemon restart | Start runs, restart daemon, verify runs continue |
| Stale PID cleanup | Kill daemon without cleanup, restart, verify recovery |

### Manual Testing

- [ ] `choo daemon start` creates PID file and starts HTTP server
- [ ] `choo daemon status` shows running state and port
- [ ] `choo daemon stop` removes PID file and stops gracefully
- [ ] `choo run` auto-starts daemon if not running
- [ ] `choo run --no-daemon` runs without daemon (no history)
- [ ] `choo run --no-history` runs with daemon but no events persisted
- [ ] Multiple `choo run` in same repo share history
- [ ] Different repos show separate history in web UI
- [ ] Resume markers appear in event timeline
- [ ] Browser receives real-time updates during run
- [ ] History persists across daemon restarts

## Design Decisions

### Why Single Writer Pattern?

SQLite performs best with a single writer. Multiple writers cause:
- Lock contention and busy timeouts
- Potential for database corruption under extreme load
- Complex retry logic in each writer

A single daemon as the sole writer:
- Eliminates contention entirely
- Simplifies error handling
- Enables efficient batching
- Provides a natural coordination point for real-time broadcasting

The trade-off is an extra hop for event persistence, but HTTP overhead is negligible compared to orchestrator execution time.

### Why HTTP Instead of Unix Sockets for Daemon IPC?

The existing web package uses Unix sockets for the simpler "fire and forget" streaming model. The daemon needs:
- Request/response semantics (start run, get run ID)
- Multiple concurrent orchestrators
- Health checks and status queries

HTTP provides:
- Well-understood request/response model
- Built-in connection pooling
- Easy debugging with curl
- Natural fit for the web UI server

### Why WAL Mode?

Write-Ahead Logging provides:
- Concurrent read access while writing
- Better write performance (no full file sync on each commit)
- Automatic recovery after crashes

The daemon is the sole writer, so WAL's limitation (single writer) is not an issue.

### Why Retain Runs Per-Repo?

Scoping history to repositories:
- Keeps history relevant to current work
- Avoids cross-project confusion
- Enables per-project retention policies
- Matches mental model of "runs in this project"

A single database with repo_path filtering is simpler than per-repo databases while maintaining logical separation.

### Why Auto-Start Daemon?

Requiring manual `choo daemon start` adds friction and confusion. Auto-starting:
- "Just works" for new users
- Eliminates "daemon not running" errors
- Mirrors common tools (Docker, databases with socket activation)

The `--no-daemon` escape hatch supports CI and isolated testing.

## Future Enhancements

1. **Remote access**: Bind to network interface with authentication for team visibility
2. **Retention policies**: Automatic cleanup of old runs based on age or count
3. **Export functionality**: Download run history as JSON/CSV
4. **Metrics endpoint**: Prometheus-compatible `/metrics` for monitoring
5. **WebSocket support**: Lower-latency alternative to SSE for real-time updates
6. **Run labels/tags**: User-defined metadata for filtering and organization
7. **Diff storage**: Store actual code changes alongside events for deeper analysis
8. **Notifications**: Webhook/Slack notifications when runs complete

## References

- [HISTORY-TYPES.md](./HISTORY-TYPES.md) - Canonical shared type definitions
- [Historical Runs PRD](/docs/HISTORICAL-RUNS-PRD.md) - Product requirements
- [WEB spec](/specs/completed/WEB.md) - Prior web server design (superseded by this daemon)
- [EVENTS spec](/specs/completed/EVENTS.md) - Event types and bus architecture
- [CLI spec](/specs/completed/CLI.md) - Command structure
- [SQLite WAL Mode](https://sqlite.org/wal.html) - WAL documentation
