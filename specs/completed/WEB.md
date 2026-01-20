# WEB - Web Server Daemon for Real-time Orchestrator Monitoring

## Overview

The Web package provides a real-time monitoring interface for the choo orchestrator through HTTP and Server-Sent Events (SSE). It runs as an independent daemon (`choo web`) that receives events from `choo run` via Unix socket, maintains an in-memory state store, and broadcasts updates to connected browsers.

The architecture separates concerns: `choo run` focuses on orchestration while `choo web` handles visualization. This allows the web UI to start before, during, or after orchestration runs. The daemon preserves the final state after `choo run` exits, allowing post-run analysis.

```
+-------------------------------------+    Unix Socket      +---------------------------------+
|  choo run                           |  --------------->   |  choo web (daemon)              |
|                                     |  ~/.choo/web.sock   |                                 |
|  Event Bus                          |                     |  +-------------------------+    |
|     |                               |  JSON lines:        |  | State Store             |    |
|     +---> Orchestrator              |  - orch.started     |  | (graph + unit states)   |    |
|     |       |                       |  - unit.*           |  +-------------------------+    |
|     |       v                       |  - task.*           |           |                     |
|     |    Scheduler ---> Workers     |  - pr.*             |           v                     |
|     |                               |  - orch.completed   |  +-------------------------+    |
|     +---> SocketPusher (new)        |                     |  | SSE Hub                 |    |
|          (subscribes, writes)       |                     |  | (broadcasts to browsers)|    |
+-------------------------------------+                     |  +-------------------------+    |
                                                            |           |                     |
                                                            |           v                     |
                                                            |  HTTP :8080 ---> Browser        |
                                                            |  +-------------------------+    |
                                                            |  | D3.js Graph + UI        |    |
                                                            |  +-------------------------+    |
                                                            +---------------------------------+
```

## Requirements

### Functional Requirements

1. Create Unix socket at `~/.choo/web.sock` (or `$XDG_RUNTIME_DIR/choo/web.sock` if set)
2. Accept connections from `choo run` and read newline-delimited JSON events
3. Parse and validate incoming event messages
4. Maintain in-memory state store with orchestration status, unit states, and dependency graph
5. Start HTTP server on configurable port (default :8080)
6. Serve embedded static files (HTML, JS, CSS) at `/`
7. Expose REST API at `/api/state` for current state snapshot
8. Expose REST API at `/api/graph` for dependency graph structure
9. Provide SSE stream at `/api/events` for real-time event delivery to browsers
10. Support multiple concurrent browser connections with fan-out
11. Preserve final state when orchestrator disconnects
12. Clean up socket file on exit
13. Handle graceful shutdown on SIGINT/SIGTERM

### Performance Requirements

| Metric | Target |
|--------|--------|
| Socket read latency | <1ms per event |
| SSE broadcast latency | <10ms from socket to browser |
| State snapshot response | <50ms |
| Max concurrent browsers | 100 connections |
| Memory per browser connection | <10KB |
| Static file response (gzip) | <100ms |

### Constraints

- Go stdlib only: `net` for sockets, `net/http` for web, no external dependencies
- Static files embedded via `//go:embed` for single binary deployment
- Unix socket for IPC (no Windows support in MVP)
- Single orchestrator connection at a time
- Browser must support EventSource API for SSE

## Design

### Module Structure

```
internal/web/
+-- server.go       # HTTP server, lifecycle management, main entry point
+-- socket.go       # Unix socket listener, event reading from choo run
+-- handlers.go     # HTTP handlers: /, /api/state, /api/graph, /api/events
+-- sse.go          # SSE hub for managing browser connections
+-- store.go        # In-memory state store (graph + unit states)
+-- embed.go        # //go:embed directives for static files
+-- types.go        # Shared types (Event, State, etc.)

internal/cli/
+-- web.go          # New choo web CLI command
```

### Core Types

```go
// internal/web/types.go

// Event represents a message received from the orchestrator
type Event struct {
    Type    string          `json:"type"`
    Time    time.Time       `json:"time"`
    Unit    string          `json:"unit,omitempty"`
    Task    *int            `json:"task,omitempty"`
    PR      *int            `json:"pr,omitempty"`
    Payload json.RawMessage `json:"payload,omitempty"`
    Error   string          `json:"error,omitempty"`
}

// OrchestratorPayload is the payload for orch.started events
type OrchestratorPayload struct {
    UnitCount   int        `json:"unit_count"`
    Parallelism int        `json:"parallelism"`
    Graph       *GraphData `json:"graph"`
}
```

```go
// internal/web/store.go

// Store maintains the current orchestration state
type Store struct {
    mu          sync.RWMutex
    connected   bool              // true when orchestrator is connected
    status      string            // "waiting", "running", "completed", "failed"
    startedAt   time.Time
    parallelism int
    graph       *GraphData
    units       map[string]*UnitState
}

// UnitState tracks the status of a single unit
type UnitState struct {
    ID          string    `json:"id"`
    Status      string    `json:"status"` // "pending", "ready", "in_progress", "complete", "failed", "blocked"
    CurrentTask int       `json:"currentTask"`
    TotalTasks  int       `json:"totalTasks"`
    Error       string    `json:"error,omitempty"`
    StartedAt   time.Time `json:"startedAt,omitempty"`
}

// GraphData represents the dependency graph
type GraphData struct {
    Nodes  []GraphNode `json:"nodes"`
    Edges  []GraphEdge `json:"edges"`
    Levels [][]string  `json:"levels"`
}

// GraphNode represents a unit in the graph
type GraphNode struct {
    ID    string `json:"id"`
    Level int    `json:"level"`
}

// GraphEdge represents a dependency between units
type GraphEdge struct {
    From string `json:"from"`
    To   string `json:"to"`
}

// StateSnapshot is the response for GET /api/state
type StateSnapshot struct {
    Connected   bool         `json:"connected"`
    Status      string       `json:"status"`
    StartedAt   *time.Time   `json:"startedAt,omitempty"`
    Parallelism int          `json:"parallelism,omitempty"`
    Units       []*UnitState `json:"units"`
    Summary     StateSummary `json:"summary"`
}

// StateSummary provides aggregate counts
type StateSummary struct {
    Total      int `json:"total"`
    Pending    int `json:"pending"`
    InProgress int `json:"inProgress"`
    Complete   int `json:"complete"`
    Failed     int `json:"failed"`
    Blocked    int `json:"blocked"`
}
```

```go
// internal/web/sse.go

// Hub manages SSE client connections
type Hub struct {
    mu      sync.RWMutex
    clients map[*Client]struct{}

    // Channels for client management
    register   chan *Client
    unregister chan *Client
    broadcast  chan *Event
}

// Client represents a connected browser
type Client struct {
    id     string
    events chan *Event
    done   chan struct{}
}
```

```go
// internal/web/server.go

// Server is the main web server
type Server struct {
    addr   string
    socket string

    store *Store
    hub   *Hub

    httpServer   *http.Server
    socketServer *SocketServer

    shutdown chan struct{}
}

// Config holds server configuration
type Config struct {
    // Addr is the HTTP listen address (default ":8080")
    Addr string

    // SocketPath is the Unix socket path (default ~/.choo/web.sock)
    SocketPath string
}
```

```go
// internal/web/socket.go

// SocketServer listens for orchestrator connections
type SocketServer struct {
    path     string
    listener net.Listener
    store    *Store
    hub      *Hub
    done     chan struct{}
}
```

### API Surface

```go
// internal/web/server.go

// New creates a new web server with the given configuration
func New(cfg Config) (*Server, error)

// Start begins listening on HTTP and Unix socket
func (s *Server) Start() error

// Stop performs graceful shutdown
func (s *Server) Stop(ctx context.Context) error

// Addr returns the HTTP listen address
func (s *Server) Addr() string
```

```go
// internal/web/store.go

// NewStore creates an empty state store
func NewStore() *Store

// HandleEvent processes an event and updates state
func (s *Store) HandleEvent(e *Event)

// Snapshot returns the current state
func (s *Store) Snapshot() *StateSnapshot

// Graph returns the dependency graph
func (s *Store) Graph() *GraphData

// SetConnected updates the connection status
func (s *Store) SetConnected(connected bool)

// Reset clears all state for a new run
func (s *Store) Reset()
```

```go
// internal/web/sse.go

// NewHub creates a new SSE hub
func NewHub() *Hub

// Run starts the hub's event loop
func (h *Hub) Run()

// Register adds a client to receive events
func (h *Hub) Register(c *Client)

// Unregister removes a client
func (h *Hub) Unregister(c *Client)

// Broadcast sends an event to all connected clients
func (h *Hub) Broadcast(e *Event)

// Count returns the number of connected clients
func (h *Hub) Count() int
```

```go
// internal/web/socket.go

// NewSocketServer creates a Unix socket server
func NewSocketServer(path string, store *Store, hub *Hub) *SocketServer

// Start begins listening for orchestrator connections
func (s *SocketServer) Start() error

// Stop closes the socket and cleans up
func (s *SocketServer) Stop() error
```

```go
// internal/web/handlers.go

// IndexHandler serves the embedded HTML UI
func IndexHandler(fs fs.FS) http.HandlerFunc

// StateHandler returns the current state snapshot
func StateHandler(store *Store) http.HandlerFunc

// GraphHandler returns the dependency graph
func GraphHandler(store *Store) http.HandlerFunc

// EventsHandler provides the SSE event stream
func EventsHandler(hub *Hub) http.HandlerFunc
```

```go
// internal/cli/web.go

// NewWebCmd creates the web command
func NewWebCmd(app *App) *cobra.Command
```

### HTTP API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Serve embedded HTML UI |
| `/api/state` | GET | Current orchestration state snapshot |
| `/api/graph` | GET | Dependency graph structure |
| `/api/events` | GET | SSE stream for real-time events |

### Socket Protocol

JSON lines over Unix socket at `~/.choo/web.sock` (or `$XDG_RUNTIME_DIR/choo/web.sock` if set).

**Connection flow:**
1. `choo web` creates socket and listens
2. `choo run` connects when it starts
3. `choo run` writes JSON events, one per line
4. `choo web` reads and updates state store
5. Connection closes when `choo run` exits

**Message format (newline-delimited JSON):**
```json
{"type":"orch.started","time":"2024-01-15T10:00:00Z","payload":{"unit_count":12,"parallelism":4,"graph":{...}}}
{"type":"unit.started","unit":"app-shell","time":"2024-01-15T10:00:05Z"}
{"type":"task.started","unit":"app-shell","task":1,"time":"2024-01-15T10:00:06Z"}
{"type":"unit.completed","unit":"app-shell","time":"2024-01-15T10:05:00Z"}
{"type":"orch.completed","time":"2024-01-15T10:30:00Z"}
```

**Event types:**
| Event | Description |
|-------|-------------|
| `orch.started` | Orchestration begins, includes graph and config |
| `orch.completed` | All units finished successfully |
| `orch.failed` | Orchestration stopped due to failure |
| `unit.queued` | Unit ready to start (dependencies met) |
| `unit.started` | Unit execution began |
| `unit.completed` | Unit finished successfully |
| `unit.failed` | Unit failed |
| `unit.blocked` | Unit cannot proceed (dependency failed) |
| `task.started` | Task execution began |
| `task.completed` | Task finished successfully |
| `pr.created` | Pull request created |
| `pr.merged` | Pull request merged |

### State Update Mapping

| Event | State Update |
|-------|--------------|
| `orch.started` | set connected=true, status="running", store graph |
| `unit.queued` | set unit status to "ready" |
| `unit.started` | set unit status to "in_progress", set startedAt |
| `task.started` | increment currentTask |
| `unit.completed` | set unit status to "complete" |
| `unit.failed` | set unit status to "failed", store error |
| `unit.blocked` | set unit status to "blocked" |
| `orch.completed` | set status="completed" |
| `orch.failed` | set status="failed" |
| Socket disconnect | set connected=false (keep last state visible) |

### Response Formats

**GET /api/state (running):**
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
    },
    {
      "id": "config",
      "status": "pending",
      "currentTask": 0,
      "totalTasks": 3,
      "error": null
    }
  ],
  "summary": {
    "total": 12,
    "pending": 5,
    "inProgress": 3,
    "complete": 1,
    "failed": 0,
    "blocked": 1
  }
}
```

**GET /api/state (waiting):**
```json
{
  "connected": false,
  "status": "waiting",
  "units": [],
  "summary": {
    "total": 0,
    "pending": 0,
    "inProgress": 0,
    "complete": 0,
    "failed": 0,
    "blocked": 0
  }
}
```

**GET /api/graph:**
```json
{
  "nodes": [
    {"id": "project-setup", "level": 0},
    {"id": "app-shell", "level": 1},
    {"id": "config", "level": 1}
  ],
  "edges": [
    {"from": "project-setup", "to": "app-shell"},
    {"from": "project-setup", "to": "config"}
  ],
  "levels": [
    ["project-setup"],
    ["app-shell", "config"]
  ]
}
```

**GET /api/events (SSE stream):**
```
event: unit.started
data: {"type":"unit.started","unit":"app-shell","time":"2024-01-15T10:00:05Z"}

event: task.started
data: {"type":"task.started","unit":"app-shell","task":1,"time":"2024-01-15T10:00:06Z"}

event: orch.completed
data: {"type":"orch.completed","time":"2024-01-15T10:30:00Z"}

```

### Static File Embedding

```go
// internal/web/embed.go

package web

import "embed"

//go:embed static/*
var staticFS embed.FS
```

The static directory contains:
```
internal/web/static/
+-- index.html      # Main HTML page
+-- style.css       # Styles for the UI
+-- app.js          # JavaScript for D3.js graph and SSE handling
```

## Implementation Notes

### Socket Path Resolution

The socket path is determined by environment variables with fallback:

```go
func defaultSocketPath() string {
    if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
        dir := filepath.Join(xdg, "choo")
        os.MkdirAll(dir, 0700)
        return filepath.Join(dir, "web.sock")
    }

    home, _ := os.UserHomeDir()
    dir := filepath.Join(home, ".choo")
    os.MkdirAll(dir, 0700)
    return filepath.Join(dir, "web.sock")
}
```

### Socket Cleanup

The socket file must be cleaned up on exit, including abnormal termination:

```go
func (s *SocketServer) Start() error {
    // Remove stale socket file
    os.Remove(s.path)

    listener, err := net.Listen("unix", s.path)
    if err != nil {
        return fmt.Errorf("listen on %s: %w", s.path, err)
    }
    s.listener = listener

    go s.acceptLoop()
    return nil
}

func (s *SocketServer) Stop() error {
    close(s.done)
    if s.listener != nil {
        s.listener.Close()
    }
    os.Remove(s.path)
    return nil
}
```

### Event Reading from Socket

Events are read line by line using a scanner:

```go
func (s *SocketServer) handleConnection(conn net.Conn) {
    defer conn.Close()

    s.store.SetConnected(true)
    defer s.store.SetConnected(false)

    scanner := bufio.NewScanner(conn)
    scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 1MB max line

    for scanner.Scan() {
        var event Event
        if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
            log.Printf("invalid event JSON: %v", err)
            continue
        }

        s.store.HandleEvent(&event)
        s.hub.Broadcast(&event)
    }

    if err := scanner.Err(); err != nil {
        log.Printf("socket read error: %v", err)
    }
}
```

### SSE Client Management

The hub manages client connections with proper cleanup:

```go
func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.mu.Lock()
            h.clients[client] = struct{}{}
            h.mu.Unlock()

        case client := <-h.unregister:
            h.mu.Lock()
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.events)
            }
            h.mu.Unlock()

        case event := <-h.broadcast:
            h.mu.RLock()
            for client := range h.clients {
                select {
                case client.events <- event:
                default:
                    // Client buffer full, skip this event
                }
            }
            h.mu.RUnlock()
        }
    }
}
```

### SSE Response Format

The events handler sets correct headers and flushes after each event:

```go
func EventsHandler(hub *Hub) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        flusher, ok := w.(http.Flusher)
        if !ok {
            http.Error(w, "SSE not supported", http.StatusInternalServerError)
            return
        }

        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")
        w.Header().Set("Access-Control-Allow-Origin", "*")

        client := &Client{
            id:     uuid.NewString(),
            events: make(chan *Event, 256),
            done:   make(chan struct{}),
        }

        hub.Register(client)
        defer hub.Unregister(client)

        ctx := r.Context()
        for {
            select {
            case <-ctx.Done():
                return
            case event := <-client.events:
                data, _ := json.Marshal(event)
                fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
                flusher.Flush()
            }
        }
    }
}
```

### Graceful Shutdown

The server handles shutdown signals and waits for in-flight requests:

```go
func (s *Server) Start() error {
    // Start socket server
    if err := s.socketServer.Start(); err != nil {
        return err
    }

    // Start SSE hub
    go s.hub.Run()

    // Start HTTP server
    go func() {
        if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
            log.Printf("HTTP server error: %v", err)
        }
    }()

    return nil
}

func (s *Server) Stop(ctx context.Context) error {
    // Stop accepting new socket connections
    s.socketServer.Stop()

    // Shutdown HTTP server with timeout
    if err := s.httpServer.Shutdown(ctx); err != nil {
        return fmt.Errorf("HTTP shutdown: %w", err)
    }

    return nil
}
```

### Thread Safety

The store uses a read-write mutex to allow concurrent reads:

```go
func (s *Store) HandleEvent(e *Event) {
    s.mu.Lock()
    defer s.mu.Unlock()

    switch e.Type {
    case "orch.started":
        s.status = "running"
        s.startedAt = e.Time
        // Parse payload for graph and config
        var payload OrchestratorPayload
        json.Unmarshal(e.Payload, &payload)
        s.graph = payload.Graph
        s.parallelism = payload.Parallelism
        // Initialize unit states
        for _, node := range payload.Graph.Nodes {
            s.units[node.ID] = &UnitState{
                ID:     node.ID,
                Status: "pending",
            }
        }

    case "unit.started":
        if unit, ok := s.units[e.Unit]; ok {
            unit.Status = "in_progress"
            unit.StartedAt = e.Time
        }

    case "task.started":
        if unit, ok := s.units[e.Unit]; ok {
            if e.Task != nil {
                unit.CurrentTask = *e.Task
            }
        }

    case "unit.completed":
        if unit, ok := s.units[e.Unit]; ok {
            unit.Status = "complete"
        }

    case "unit.failed":
        if unit, ok := s.units[e.Unit]; ok {
            unit.Status = "failed"
            unit.Error = e.Error
        }

    case "orch.completed":
        s.status = "completed"

    case "orch.failed":
        s.status = "failed"
    }
}

func (s *Store) Snapshot() *StateSnapshot {
    s.mu.RLock()
    defer s.mu.RUnlock()

    units := make([]*UnitState, 0, len(s.units))
    summary := StateSummary{Total: len(s.units)}

    for _, u := range s.units {
        units = append(units, u)
        switch u.Status {
        case "pending":
            summary.Pending++
        case "in_progress":
            summary.InProgress++
        case "complete":
            summary.Complete++
        case "failed":
            summary.Failed++
        case "blocked":
            summary.Blocked++
        }
    }

    snapshot := &StateSnapshot{
        Connected:   s.connected,
        Status:      s.status,
        Parallelism: s.parallelism,
        Units:       units,
        Summary:     summary,
    }

    if !s.startedAt.IsZero() {
        snapshot.StartedAt = &s.startedAt
    }

    return snapshot
}
```

## Testing Strategy

### Unit Tests

```go
// internal/web/store_test.go

func TestStore_HandleOrchStarted(t *testing.T) {
    store := NewStore()

    graph := &GraphData{
        Nodes: []GraphNode{
            {ID: "unit-a", Level: 0},
            {ID: "unit-b", Level: 1},
        },
        Edges: []GraphEdge{{From: "unit-a", To: "unit-b"}},
    }
    payload, _ := json.Marshal(OrchestratorPayload{
        UnitCount:   2,
        Parallelism: 4,
        Graph:       graph,
    })

    event := &Event{
        Type:    "orch.started",
        Time:    time.Now(),
        Payload: payload,
    }

    store.HandleEvent(event)

    snapshot := store.Snapshot()
    if snapshot.Status != "running" {
        t.Errorf("expected status running, got %s", snapshot.Status)
    }
    if snapshot.Parallelism != 4 {
        t.Errorf("expected parallelism 4, got %d", snapshot.Parallelism)
    }
    if len(snapshot.Units) != 2 {
        t.Errorf("expected 2 units, got %d", len(snapshot.Units))
    }
}

func TestStore_HandleUnitLifecycle(t *testing.T) {
    store := NewStore()

    // Initialize with orch.started
    payload, _ := json.Marshal(OrchestratorPayload{
        Graph: &GraphData{
            Nodes: []GraphNode{{ID: "app-shell", Level: 0}},
        },
    })
    store.HandleEvent(&Event{Type: "orch.started", Payload: payload})

    // Start unit
    store.HandleEvent(&Event{Type: "unit.started", Unit: "app-shell", Time: time.Now()})

    snapshot := store.Snapshot()
    var unit *UnitState
    for _, u := range snapshot.Units {
        if u.ID == "app-shell" {
            unit = u
            break
        }
    }

    if unit == nil {
        t.Fatal("unit not found")
    }
    if unit.Status != "in_progress" {
        t.Errorf("expected in_progress, got %s", unit.Status)
    }

    // Complete unit
    store.HandleEvent(&Event{Type: "unit.completed", Unit: "app-shell"})

    snapshot = store.Snapshot()
    for _, u := range snapshot.Units {
        if u.ID == "app-shell" {
            if u.Status != "complete" {
                t.Errorf("expected complete, got %s", u.Status)
            }
        }
    }
}

func TestStore_SummaryCalculation(t *testing.T) {
    store := NewStore()

    payload, _ := json.Marshal(OrchestratorPayload{
        Graph: &GraphData{
            Nodes: []GraphNode{
                {ID: "a", Level: 0},
                {ID: "b", Level: 0},
                {ID: "c", Level: 0},
                {ID: "d", Level: 0},
            },
        },
    })
    store.HandleEvent(&Event{Type: "orch.started", Payload: payload})
    store.HandleEvent(&Event{Type: "unit.started", Unit: "a"})
    store.HandleEvent(&Event{Type: "unit.completed", Unit: "a"})
    store.HandleEvent(&Event{Type: "unit.started", Unit: "b"})
    store.HandleEvent(&Event{Type: "unit.failed", Unit: "b", Error: "test error"})
    store.HandleEvent(&Event{Type: "unit.started", Unit: "c"})

    snapshot := store.Snapshot()

    if snapshot.Summary.Total != 4 {
        t.Errorf("expected total 4, got %d", snapshot.Summary.Total)
    }
    if snapshot.Summary.Complete != 1 {
        t.Errorf("expected complete 1, got %d", snapshot.Summary.Complete)
    }
    if snapshot.Summary.Failed != 1 {
        t.Errorf("expected failed 1, got %d", snapshot.Summary.Failed)
    }
    if snapshot.Summary.InProgress != 1 {
        t.Errorf("expected in_progress 1, got %d", snapshot.Summary.InProgress)
    }
    if snapshot.Summary.Pending != 1 {
        t.Errorf("expected pending 1, got %d", snapshot.Summary.Pending)
    }
}
```

```go
// internal/web/sse_test.go

func TestHub_ClientRegistration(t *testing.T) {
    hub := NewHub()
    go hub.Run()

    client := &Client{
        id:     "test-1",
        events: make(chan *Event, 10),
        done:   make(chan struct{}),
    }

    hub.Register(client)
    time.Sleep(10 * time.Millisecond)

    if hub.Count() != 1 {
        t.Errorf("expected 1 client, got %d", hub.Count())
    }

    hub.Unregister(client)
    time.Sleep(10 * time.Millisecond)

    if hub.Count() != 0 {
        t.Errorf("expected 0 clients, got %d", hub.Count())
    }
}

func TestHub_Broadcast(t *testing.T) {
    hub := NewHub()
    go hub.Run()

    client1 := &Client{id: "1", events: make(chan *Event, 10)}
    client2 := &Client{id: "2", events: make(chan *Event, 10)}

    hub.Register(client1)
    hub.Register(client2)
    time.Sleep(10 * time.Millisecond)

    event := &Event{Type: "unit.started", Unit: "test"}
    hub.Broadcast(event)

    select {
    case e := <-client1.events:
        if e.Unit != "test" {
            t.Errorf("client1: expected unit test, got %s", e.Unit)
        }
    case <-time.After(100 * time.Millisecond):
        t.Error("client1 did not receive event")
    }

    select {
    case e := <-client2.events:
        if e.Unit != "test" {
            t.Errorf("client2: expected unit test, got %s", e.Unit)
        }
    case <-time.After(100 * time.Millisecond):
        t.Error("client2 did not receive event")
    }
}
```

```go
// internal/web/handlers_test.go

func TestStateHandler(t *testing.T) {
    store := NewStore()
    store.SetConnected(true)

    req := httptest.NewRequest("GET", "/api/state", nil)
    rec := httptest.NewRecorder()

    handler := StateHandler(store)
    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", rec.Code)
    }

    var snapshot StateSnapshot
    if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
        t.Fatalf("failed to unmarshal: %v", err)
    }

    if !snapshot.Connected {
        t.Error("expected connected=true")
    }
}

func TestGraphHandler_NoGraph(t *testing.T) {
    store := NewStore()

    req := httptest.NewRequest("GET", "/api/graph", nil)
    rec := httptest.NewRecorder()

    handler := GraphHandler(store)
    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", rec.Code)
    }

    var graph GraphData
    if err := json.Unmarshal(rec.Body.Bytes(), &graph); err != nil {
        t.Fatalf("failed to unmarshal: %v", err)
    }

    if len(graph.Nodes) != 0 {
        t.Errorf("expected empty nodes, got %d", len(graph.Nodes))
    }
}
```

```go
// internal/web/socket_test.go

func TestSocketServer_ParseEvent(t *testing.T) {
    store := NewStore()
    hub := NewHub()
    go hub.Run()

    // Create a pipe for testing
    serverConn, clientConn := net.Pipe()
    defer serverConn.Close()
    defer clientConn.Close()

    go func() {
        scanner := bufio.NewScanner(serverConn)
        for scanner.Scan() {
            var event Event
            if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
                continue
            }
            store.HandleEvent(&event)
        }
    }()

    // Send events from client
    events := []string{
        `{"type":"orch.started","time":"2024-01-15T10:00:00Z","payload":{"graph":{"nodes":[{"id":"test","level":0}]}}}`,
        `{"type":"unit.started","unit":"test","time":"2024-01-15T10:00:01Z"}`,
    }

    for _, e := range events {
        fmt.Fprintln(clientConn, e)
    }

    time.Sleep(50 * time.Millisecond)

    snapshot := store.Snapshot()
    if snapshot.Status != "running" {
        t.Errorf("expected running, got %s", snapshot.Status)
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Full event flow | Start server, connect mock orchestrator, send lifecycle events, verify state updates |
| SSE delivery | Connect browser clients, send events, verify all clients receive events |
| Reconnection | Connect orchestrator, disconnect, verify state preserved, reconnect |
| Concurrent browsers | Connect 50 clients, broadcast 100 events, verify all received |
| Graceful shutdown | Start server, connect clients, stop server, verify clean shutdown |

### Manual Testing

- [ ] `choo web` creates socket at expected path
- [ ] `choo web` starts HTTP server on :8080
- [ ] Browser shows "waiting" state before orchestrator connects
- [ ] Browser receives real-time updates during `choo run`
- [ ] Graph visualization renders correctly
- [ ] Unit status updates reflect in UI
- [ ] Browser preserves final state after orchestrator exits
- [ ] Multiple browser tabs receive same events
- [ ] Ctrl+C on `choo web` cleans up socket file
- [ ] Error events display failure information

## Design Decisions

### Why Unix Socket for IPC?

Unix sockets provide:
- Simple, reliable local IPC with no network overhead
- Automatic cleanup when processes exit
- File-based permissions for security
- Natural one-to-one connection model

Alternatives considered:
- TCP localhost: Adds unnecessary network stack overhead
- Shared memory: Complex synchronization, no natural event stream
- Named pipes: Platform-specific, less flexible

### Why SSE Instead of WebSockets?

SSE is simpler for one-way server-to-client streaming:
- Native browser support via EventSource API
- Automatic reconnection built into the spec
- Works over standard HTTP (easier debugging, proxying)
- No library dependencies

WebSockets would only add value for bidirectional communication, which the monitoring UI does not need.

### Why Embedded Static Files?

`//go:embed` provides:
- Single binary deployment (no separate static file directory)
- Version consistency (UI always matches server version)
- No runtime file system access required

Trade-off: UI changes require rebuilding the binary.

### Why Keep State After Disconnect?

Preserving the final state serves important use cases:
- Post-run analysis of what happened
- Debugging failures without re-running
- Multiple people viewing results at different times

The "waiting" state clearly indicates the orchestrator is not connected, while still showing the last known state.

### Why No Authentication?

For MVP, the web UI is:
- Local-only (localhost binding)
- Read-only (no sensitive operations)
- Development-focused (not production monitoring)

Future versions may add authentication for remote access.

## Future Enhancements

1. **Remote access**: Bind to non-localhost with authentication
2. **Historical data**: Persist run history to SQLite for historical analysis
3. **Custom port via flag**: `choo web --port 3000`
4. **WebSocket support**: For bidirectional features (cancel unit, retry task)
5. **Dark mode**: Toggle between light and dark UI themes
6. **Export functionality**: Download run results as JSON
7. **Notifications**: Browser notifications for completion/failure
8. **Metrics endpoint**: Prometheus-compatible `/metrics` endpoint

## References

- [EVENTS spec](completed/EVENTS.md) - Event types and bus architecture
- [CLI spec](completed/CLI.md) - Command structure and wiring
- [ORCHESTRATOR spec](completed/ORCHESTRATOR.md) - Event publishing from orchestrator
- [Server-Sent Events spec](https://html.spec.whatwg.org/multipage/server-sent-events.html)
- [Go embed directive](https://pkg.go.dev/embed)
