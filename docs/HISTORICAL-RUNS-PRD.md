# Historical Runs System for Houston

## Overview

Add a historical run tracking system to Houston, enabling users to view past orchestration runs and their complete event histories through the web UI. Uses SQLite for persistence with WAL mode for concurrent access, captures full event logs, and provides a drill-down UI. History is scoped per repository but managed by a single daemon process.

## Architecture

```
                          Choo Daemon (single process)
                         ┌─────────────────────────────┐
                         │     HTTP Server (web UI)    │
                         │              │              │
                         │       History Store         │
                         │      (sole DB writer)       │
                         │              │              │
Orchestrator ──events──▶ │         SQLite DB           │
  Process                │        (WAL mode)           │
    │                    └─────────────────────────────┘
    │                                  │
    └───────[IPC/HTTP]─────────────────┘
                                       │
                                    Browser
                              [History View + Graph]
```

### Key Design Decisions

1. **Single Writer Pattern**: The choo daemon is the sole writer to the SQLite database. Orchestrator processes send events to the daemon via IPC/HTTP rather than writing directly.

2. **WAL Mode**: SQLite is configured with Write-Ahead Logging for better concurrent read performance while the daemon writes.

3. **Daemon-Managed Web Server**: The daemon also serves the web UI, eliminating the need for separate `choo web` invocations.

4. **Per-Repo History**: History is scoped to individual repositories, but a single daemon instance manages all repos.

## Database Schema

Location: `~/.choo/history.db` (single database for all repos)

```sql
-- Enable WAL mode on connection
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;

CREATE TABLE runs (
    id TEXT PRIMARY KEY,              -- "run_20250120_143052_a1b2"
    repo_path TEXT NOT NULL,          -- canonical repo path for scoping
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    status TEXT NOT NULL DEFAULT 'running',  -- running/completed/failed/stopped
    parallelism INTEGER NOT NULL,
    total_units INTEGER NOT NULL,
    completed_units INTEGER DEFAULT 0,
    failed_units INTEGER DEFAULT 0,
    blocked_units INTEGER DEFAULT 0,
    error TEXT,
    tasks_dir TEXT,
    dry_run BOOLEAN DEFAULT FALSE
);

CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    seq INTEGER NOT NULL,             -- sequence for ordering
    time DATETIME NOT NULL,
    type TEXT NOT NULL,               -- "unit.started", "run.stopped", "run.resumed", etc.
    unit TEXT,
    task INTEGER,
    pr INTEGER,
    payload TEXT,                     -- JSON (redacted for sensitive data)
    error TEXT
);

CREATE TABLE graphs (
    run_id TEXT PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE,
    nodes TEXT NOT NULL,              -- JSON array
    edges TEXT NOT NULL,
    levels TEXT NOT NULL
);

CREATE INDEX idx_events_run_seq ON events(run_id, seq);
CREATE INDEX idx_runs_started_at ON runs(started_at DESC);
CREATE INDEX idx_runs_repo ON runs(repo_path, started_at DESC);
```

## Daemon Design

### Daemon Lifecycle

The choo daemon is a long-running process that:
- Owns all writes to the history database
- Serves the web UI on a configurable port
- Accepts event streams from orchestrator processes
- Handles multiple repositories concurrently

### CLI Commands for Daemon Management

```bash
# Start daemon (if not running)
choo daemon start [--port 8080]

# Stop daemon gracefully
choo daemon stop

# Restart daemon
choo daemon restart

# Check daemon status
choo daemon status

# View daemon logs
choo daemon logs [--follow]
```

### Automatic Daemon Management

When running `choo run`, the CLI:
1. Checks if daemon is running
2. Starts daemon automatically if not running (unless `--no-daemon` specified)
3. Connects to daemon for event streaming
4. Daemon persists events and serves web UI

```bash
# Normal execution (daemon auto-started if needed)
choo run specs/tasks

# Fully isolated execution (no daemon, no history)
choo run --no-daemon specs/tasks

# Run with history but no web UI served
choo run --no-history specs/tasks
```

### Daemon Configuration

In `.choo.yaml` or `~/.config/choo/config.yaml`:
```yaml
daemon:
  port: 8080
  auto_start: true    # Start daemon automatically on `choo run`

history:
  enabled: true       # Set to false to disable history globally
  retention_days: 90  # Auto-cleanup runs older than this
```

## Implementation

### 1. Daemon Package (`internal/daemon/`)

**New files:**
- `daemon.go` - Daemon process management, event receiver, web server
- `client.go` - Client for CLI to communicate with daemon
- `pidfile.go` - PID file management for single-instance enforcement

**Key types:**
```go
type Daemon struct {
    store      *history.Store
    httpServer *http.Server
    eventChan  chan EventMessage
}

func (d *Daemon) Start(port int) error
func (d *Daemon) Stop() error
func (d *Daemon) HandleEventStream(w http.ResponseWriter, r *http.Request)

type Client struct {
    baseURL string
}

func (c *Client) IsRunning() bool
func (c *Client) StartRun(cfg RunConfig) (string, error)
func (c *Client) SendEvent(runID string, event EventRecord) error
func (c *Client) CompleteRun(runID string, result RunResult) error
```

### 2. Storage Package (`internal/history/`)

**New files:**
- `store.go` - SQLite operations with WAL mode
- `types.go` - Data types (Run, EventRecord, StoredEvent, GraphData, ListOptions)
- `migrate.go` - Schema creation with version management
- `handler.go` - Event handler that sends to daemon (not direct DB writes)

**Key types:**
```go
type Store struct {
    db *sql.DB
}

func NewStore(dbPath string) (*Store, error)  // Enables WAL mode
func (s *Store) CreateRun(cfg RunConfig) (string, error)
func (s *Store) InsertEvent(runID string, event EventRecord) error
func (s *Store) AppendResumeMarker(runID string) error  // For resume tracking
func (s *Store) CompleteRun(runID string, result RunResult) error
func (s *Store) ListRuns(repoPath string, opts ListOptions) (*RunList, error)
func (s *Store) GetRunEvents(runID string, opts EventListOptions) (*EventList, error)
func (s *Store) GetGraph(runID string) (*GraphData, error)

// Handler sends events to daemon instead of writing directly
type Handler struct {
    client *daemon.Client
    runID  string
    seq    int64  // atomic counter
}

func (h *Handler) Handle(e events.Event)  // implements events.Handler
```

### 3. CLI Integration

**Daemon commands (`internal/cli/daemon.go`):**
```go
func DaemonStartCmd() *cobra.Command
func DaemonStopCmd() *cobra.Command
func DaemonStatusCmd() *cobra.Command
func DaemonLogsCmd() *cobra.Command
```

**Run command changes (`internal/cli/run.go`):**
```go
func RunOrchestrator() {
    // Check for --no-daemon flag
    if noDaemon {
        // Run without history, fully isolated
        runIsolated()
        return
    }

    // Ensure daemon is running
    client := daemon.NewClient()
    if !client.IsRunning() {
        if err := daemon.Start(defaultPort); err != nil {
            log.Fatal("Failed to start daemon:", err)
        }
    }

    // Check for --no-history flag
    if noHistory {
        // Run without history tracking
        runWithoutHistory(client)
        return
    }

    // Normal execution with history
    repoPath := getCanonicalRepoPath()
    runID, err := client.StartRun(history.RunConfig{
        RepoPath: repoPath,
        ...
    })

    historyHandler := history.NewHandler(client, runID)
    eventBus.Subscribe(historyHandler.Handle)

    // ... run orchestrator ...

    client.CompleteRun(runID, history.RunResult{...})
}
```

**Resume behavior (`internal/cli/resume.go`):**
```go
func ResumeRun(originalRunID string) {
    client := daemon.NewClient()

    // Append resume marker to existing run (not a new run)
    client.AppendResumeMarker(originalRunID)

    // Continue with same runID, events append with new sequence numbers
    historyHandler := history.NewHandler(client, originalRunID)
    // ...
}
```

### 4. Web API (served by Daemon)

**Endpoints:**
- `GET /api/history/runs?repo={path}` - List runs for a repo (paginated)
- `GET /api/history/runs/{id}` - Get run details
- `GET /api/history/runs/{id}/events` - Get events (filterable by type, unit)
- `GET /api/history/runs/{id}/graph` - Get graph data
- `POST /api/runs` - Start a new run (from CLI)
- `POST /api/runs/{id}/events` - Stream events (from CLI)
- `POST /api/runs/{id}/complete` - Mark run complete (from CLI)

### 5. Frontend (`internal/web/static/`)

**Navigation:** Add Live/History tabs to switch views

**history.js (new):**
```javascript
export class HistoryView {
    async loadRuns(repoPath, page)  // Fetch runs for current repo
    async selectRun(runId)          // Load run details + graph + events
    renderRunDetail()               // Show graph (reuse D3) + event timeline
    renderResumeMarkers()           // Visual indicators for stop/resume points
}
```

**index.html changes:**
```html
<nav class="main-nav">
    <button id="nav-live">Live</button>
    <button id="nav-history">History</button>
</nav>
<div id="live-view">...</div>
<div id="history-view" class="hidden">
    <div class="history-list"><!-- Run table --></div>
    <div class="history-detail"><!-- Graph + events --></div>
</div>
```

**UI components:**
- Run list: ID, status badge, unit counts, duration, resume count
- Run detail: D3 graph (reuse existing), event timeline with stop/resume markers
- Event timeline: Scrollable, color-coded by event type, resume boundaries highlighted

## Resume Behavior

When a run is resumed:
1. The same `run_id` is used (no new run created)
2. A `run.stopped` event is recorded when the original run stops
3. A `run.resumed` event is recorded when resumed
4. New events append with continuing sequence numbers
5. The UI shows visual markers at stop/resume boundaries

```sql
-- Events for a resumed run might look like:
-- seq 1-50: original run events
-- seq 51: run.stopped (timestamp, reason)
-- seq 52: run.resumed (timestamp)
-- seq 53+: continued execution events
```

## Configuration & Flags

### CLI Flags

| Flag | Description |
|------|-------------|
| `--no-daemon` | Run without daemon, fully isolated (no history, no web) |
| `--no-history` | Run with daemon but don't record history |
| `--port` | Port for daemon web server (default: 8080) |

### Configuration File

```yaml
# .choo.yaml (repo-level) or ~/.config/choo/config.yaml (global)
daemon:
  port: 8080
  auto_start: true

history:
  enabled: true
  retention_days: 90
```

## Files to Modify/Create

| File | Action |
|------|--------|
| `internal/daemon/daemon.go` | CREATE - Daemon process, web server, event receiver |
| `internal/daemon/client.go` | CREATE - Client for CLI-daemon communication |
| `internal/daemon/pidfile.go` | CREATE - PID file management |
| `internal/history/store.go` | CREATE - SQLite operations with WAL mode |
| `internal/history/types.go` | CREATE - Data types |
| `internal/history/migrate.go` | CREATE - Schema migration with versioning |
| `internal/history/handler.go` | CREATE - Event handler (sends to daemon) |
| `internal/cli/daemon.go` | CREATE - Daemon management commands |
| `internal/cli/run.go` | MODIFY - Add daemon integration, --no-daemon, --no-history |
| `internal/cli/resume.go` | MODIFY - Append to existing run |
| `internal/web/server.go` | MODIFY - Move to daemon, add history routes |
| `internal/web/handlers.go` | MODIFY - Add history API handlers |
| `internal/web/static/history.js` | CREATE - History view module |
| `internal/web/static/index.html` | MODIFY - Add navigation and history view |
| `internal/web/static/app.js` | MODIFY - Add view switching |
| `internal/web/static/style.css` | MODIFY - Add history styles |

## Implementation Order

1. **Daemon foundation**: Create `internal/daemon/` with basic daemon lifecycle
2. **Storage foundation**: Create `internal/history/` package with WAL-enabled store
3. **Daemon integration**: Wire up storage to daemon, add event receiver endpoints
4. **CLI daemon commands**: Implement `choo daemon start/stop/status`
5. **CLI run integration**: Add auto-start, --no-daemon, --no-history flags
6. **Resume support**: Implement append-to-run behavior for resumes
7. **Web API**: Add history endpoints to daemon
8. **Frontend**: Add history view with run list, detail, and resume markers

## Verification

1. **Unit tests**: Test storage operations, daemon client, WAL behavior
2. **Daemon tests**: Test daemon start/stop, concurrent access, crash recovery
3. **Integration test**:
   - Run `choo run` and verify daemon auto-starts
   - Verify events appear in SQLite via daemon
   - Stop and resume, verify events append correctly
4. **Manual test**:
   - Run `choo daemon start`
   - Run `choo run specs/tasks` in multiple repos
   - Open browser, verify history is scoped per repo
   - Stop a run, resume it, verify timeline shows stop/resume markers
5. **Isolation test**: Run with `--no-daemon`, verify no daemon started, no history
6. **Test pagination**: Create multiple runs, verify list pagination works
7. **Test filtering**: Filter events by type/unit in API

## Error Handling

1. **Daemon unavailable**: If daemon can't start, fail with clear error (unless `--no-daemon`)
2. **History write failures**: Log warning but don't fail the orchestration
3. **Schema migration**: Fail fast if migration fails, provide clear error message
4. **Stale daemon**: Detect and clean up stale PID files on startup

## Security Considerations

1. **Payload redaction**: Sensitive data (API keys, tokens) should be redacted before storage
2. **Local-only by default**: Daemon binds to localhost only unless explicitly configured
3. **No secrets in history**: Event payloads should use allow-lists for safe fields
