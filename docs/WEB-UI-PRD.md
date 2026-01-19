# Choo Web UI - Product Requirements Document

## Document Info

| Field   | Value      |
| ------- | ---------- |
| Status  | Draft      |
| Author  | Claude     |
| Created | 2026-01-19 |
| Target  | v0.3       |

---

## 1. Overview

### 1.1 Goal

Add a real-time web UI to monitor the orchestrator while it's running. The UI will display a dependency graph of units, show current task progress, and expose errors.

### 1.2 Architecture

`choo web` runs as an independent daemon. `choo run` pushes events via Unix socket.

```
┌─────────────────────────────────────┐    Unix Socket      ┌─────────────────────────────────┐
│  choo run                           │  ────────────────►  │  choo web (daemon)              │
│                                     │  ~/.choo/web.sock   │                                 │
│  Event Bus                          │                     │  ┌─────────────────────────┐   │
│     │                               │  JSON lines:        │  │ State Store             │   │
│     ├──► Orchestrator               │  - orch.started     │  │ (graph + unit states)   │   │
│     │       │                       │  - unit.*           │  └─────────────────────────┘   │
│     │       ▼                       │  - task.*           │           │                    │
│     │    Scheduler ──► Workers      │  - pr.*             │           ▼                    │
│     │                               │  - orch.completed   │  ┌─────────────────────────┐   │
│     └──► SocketPusher (new)         │                     │  │ SSE Hub                 │   │
│          (subscribes, writes)       │                     │  │ (broadcasts to browsers)│   │
└─────────────────────────────────────┘                     │  └─────────────────────────┘   │
                                                            │           │                    │
                                                            │           ▼                    │
                                                            │  HTTP :8080 ──► Browser        │
                                                            │  ┌─────────────────────────┐   │
                                                            │  │ D3.js Graph + UI        │   │
                                                            │  └─────────────────────────┘   │
                                                            └─────────────────────────────────┘
```

### 1.3 Benefits

- `choo web` can start before orchestrator (shows "waiting")
- `choo web` survives after orchestrator exits (shows final state)
- Multiple terminals can connect to same web UI
- Unix socket is fast with no port conflicts
- Automatic cleanup when processes die

### 1.4 Key Decisions

- **Unix socket for IPC** - `~/.choo/web.sock`, JSON lines protocol
- **HTTP for browser** - SSE for real-time updates
- **Go stdlib only** - `net` for sockets, `net/http` for web
- **Embedded static files** via `//go:embed`

---

## 2. Socket Protocol

JSON lines over Unix socket at `~/.choo/web.sock` (or `$XDG_RUNTIME_DIR/choo/web.sock` if set).

### 2.1 Connection Flow

1. `choo web` creates socket and listens
2. `choo run` connects when it starts
3. `choo run` writes JSON events, one per line
4. `choo web` reads and updates state store
5. Connection closes when `choo run` exits

### 2.2 Message Format

Newline-delimited JSON:

```json
{"type":"orch.started","time":"...","payload":{"unit_count":12,"parallelism":4,"graph":{...}}}
{"type":"unit.started","unit":"app-shell","time":"..."}
{"type":"task.started","unit":"app-shell","task":1,"time":"..."}
{"type":"unit.completed","unit":"app-shell","time":"..."}
{"type":"orch.completed","time":"..."}
```

The first message (`orch.started`) includes the graph structure in its payload.

---

## 3. HTTP API

### 3.1 Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Serve embedded HTML UI |
| `/api/state` | GET | Current orchestration state snapshot |
| `/api/graph` | GET | Dependency graph structure |
| `/api/events` | GET | SSE stream for real-time events |

### 3.2 State Snapshot (`GET /api/state`)

```json
{
  "connected": true,
  "status": "running",
  "startedAt": "2024-01-15T10:00:00Z",
  "parallelism": 4,
  "units": [
    {
      "id": "app-shell",
      "status": "in_progress",
      "currentTask": 2,
      "totalTasks": 6,
      "error": null
    }
  ],
  "summary": {"total": 12, "pending": 5, "inProgress": 3, "complete": 1, "failed": 0, "blocked": 1}
}
```

When no orchestrator is connected:

```json
{"connected": false, "status": "waiting", "units": [], "summary": {}}
```

### 3.3 Graph Structure (`GET /api/graph`)

```json
{
  "nodes": [{"id": "project-setup", "level": 0}],
  "edges": [{"from": "project-setup", "to": "app-shell"}],
  "levels": [["project-setup"], ["app-shell", "config"]]
}
```

### 3.4 SSE Events (`GET /api/events`)

```
event: unit.started
data: {"type":"unit.started","unit":"app-shell","time":"..."}

event: orch.completed
data: {"type":"orch.completed","time":"..."}
```

---

## 4. State Store Design

### 4.1 Data Structures

```go
type Store struct {
    mu          sync.RWMutex
    connected   bool
    status      string // "waiting", "running", "completed", "failed"
    startedAt   time.Time
    parallelism int
    graph       *GraphData
    units       map[string]*UnitState
}

type UnitState struct {
    ID          string
    Status      string
    CurrentTask int
    TotalTasks  int
    Error       string
    StartedAt   time.Time
}
```

### 4.2 Event Handling

| Event | State Update |
|-------|--------------|
| `orch.started` | set connected=true, status="running", store graph |
| `unit.queued` | set unit status to "ready" |
| `unit.started` | set unit status to "in_progress" |
| `task.started` | update currentTask |
| `unit.completed` | set unit status to "complete" |
| `unit.failed` | set unit status to "failed", store error |
| `unit.blocked` | set unit status to "blocked" |
| `orch.completed` | set status="completed" |
| `orch.failed` | set status="failed" |
| Socket disconnect | set connected=false (keep last state visible) |

---

## 5. Graph Visualization

### 5.1 Layout

Level-based horizontal layout:
- X-axis: dependency level (0 = no deps, 1 = depends on level 0, etc.)
- Y-axis: distribute nodes within each level vertically
- Edges: bezier curves between connected nodes

### 5.2 Node Colors

| Status | Color |
|--------|-------|
| pending | gray |
| ready | yellow |
| in_progress | blue (animated pulse) |
| pr_open/in_review/merging | purple |
| complete | green |
| failed | red |
| blocked | orange |

### 5.3 Interactivity

- Click node: show detail panel with tasks and errors
- Hover: highlight dependencies

---

## 6. Error Display

1. **Visual indicators**: Failed nodes turn red, blocked nodes turn orange
2. **Toast notifications**: Failed events trigger a toast at top of screen
3. **Detail panel**: Click failed node to see full error message
4. **Event log**: Scrollable log at bottom showing recent events

---

## 7. Files to Create

| File | Purpose |
|------|---------|
| `internal/web/server.go` | HTTP server for browser, lifecycle management |
| `internal/web/socket.go` | Unix socket listener for receiving events from `choo run` |
| `internal/web/handlers.go` | HTTP handlers: `/`, `/api/state`, `/api/graph`, `/api/events` |
| `internal/web/sse.go` | SSE hub for browser client connections |
| `internal/web/store.go` | In-memory state store (graph + unit states) |
| `internal/web/pusher.go` | Client that subscribes to event bus and writes to socket |
| `internal/web/embed.go` | `//go:embed` directives for static files |
| `internal/web/static/index.html` | Main HTML page |
| `internal/web/static/style.css` | CSS styling |
| `internal/web/static/app.js` | Main JS: state management, SSE handling |
| `internal/web/static/graph.js` | D3.js DAG visualization |
| `internal/cli/web.go` | New `choo web` CLI command |

---

## 8. Files to Modify

| File | Change |
|------|--------|
| `internal/cli/cli.go` | Register new `web` command |
| `internal/cli/run.go` | Add `--web` flag; create SocketPusher when set |

---

## 9. Implementation Phases

### Phase 1: Web Server Core

1. Create `internal/web/server.go` - HTTP server with graceful shutdown
2. Create `internal/web/store.go` - in-memory state store
3. Create `internal/web/socket.go` - Unix socket listener
4. Create `internal/web/sse.go` - SSE hub for browser connections
5. Create `internal/web/handlers.go` - HTTP handlers
6. Create `internal/web/embed.go` - embed directives
7. Create `internal/cli/web.go` - `choo web` command

### Phase 2: Socket Pusher

1. Create `internal/web/pusher.go` - subscribes to event bus, writes to socket
2. Modify `internal/cli/run.go` - add `--web` flag, create pusher when set
3. Include graph structure with `orch.started` event

### Phase 3: Frontend

1. Create `index.html` - layout with header, graph area, detail panel, event log
2. Create `style.css` - node styling, responsive layout, animations
3. Create `app.js` - state management, SSE connection, UI updates
4. Create `graph.js` - D3.js DAG rendering

### Phase 4: Polish

1. Add "waiting for orchestrator" state in UI
2. Add connection status indicator
3. Add event log with error highlighting
4. Handle SSE reconnection

---

## 10. Usage

```bash
# Terminal 1: Start web server
choo web
# Listening on http://localhost:8080
# Waiting for orchestrator connection...

# Terminal 2: Run orchestrator with web integration
choo run --web
# Orchestrator connected to web UI

# Open browser to http://localhost:8080
```

---

## 11. Acceptance Criteria

- [ ] `choo web` creates Unix socket and starts HTTP server on :8080
- [ ] `choo run --web` connects to socket and pushes events
- [ ] Browser displays real-time dependency graph
- [ ] Nodes update color/state as units progress
- [ ] Click node shows detail panel with task list and errors
- [ ] Failed units display error message in UI
- [ ] Web UI shows "waiting" state before orchestrator connects
- [ ] Web UI preserves final state after orchestrator exits
- [ ] SSE reconnects automatically on disconnect
- [ ] Socket cleaned up on `choo web` exit

---

## 12. Verification

1. Start `choo web` - verify socket created, shows "waiting" in browser
2. Run `choo run --web` with sample specs
3. Verify graph displays with correct dependency structure
4. Verify nodes update in real-time as tasks progress
5. Trigger a failure and verify error is displayed in UI
6. Stop orchestrator - verify web UI still shows final state
7. Verify socket is cleaned up on `choo web` exit
