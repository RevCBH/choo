# HISTORY-API — HTTP API for Historical Run Data

## Overview

The History API provides HTTP endpoints served by the choo daemon for querying and managing historical orchestration run data. It acts as the bridge between the orchestrator CLI (which generates events) and the web UI (which displays them), with SQLite providing persistent storage.

The API serves two distinct purposes: read endpoints supply the web UI with run lists, event timelines, and dependency graph data, while write endpoints allow CLI processes to stream events and manage run lifecycle. All endpoints are scoped to localhost for security, with payload data automatically redacted of sensitive information.

```
┌─────────────────────────────────────────────────────────────────┐
│                         choo daemon                              │
├─────────────────────────────────────────────────────────────────┤
│  ┌───────────────────┐    ┌───────────────────┐                │
│  │   History API     │    │    SQLite Store   │                │
│  │   (handlers.go)   │───▶│   (runs, events,  │                │
│  │                   │    │    graphs tables) │                │
│  └─────────┬─────────┘    └───────────────────┘                │
│            │                                                     │
│  ┌─────────┴─────────────────────────────────┐                 │
│  │              HTTP Routes                   │                 │
│  ├───────────────────┬───────────────────────┤                 │
│  │   Read (Web UI)   │   Write (CLI)         │                 │
│  │  GET /history/*   │  POST /runs/*         │                 │
│  └───────────────────┴───────────────────────┘                 │
└─────────────────────────────────────────────────────────────────┘
         ▲                           ▲
         │                           │
    ┌────┴────┐                ┌─────┴─────┐
    │ Web UI  │                │   CLI     │
    │(browser)│                │(choo run) │
    └─────────┘                └───────────┘
```

## Requirements

### Functional Requirements

1. **Run Listing**: List runs filtered by repository path with pagination support
2. **Run Details**: Retrieve complete details for a specific run by ID
3. **Event Retrieval**: Query events for a run with filtering by type and unit
4. **Graph Data**: Return dependency graph nodes, edges, and levels for visualization
5. **Run Creation**: Accept new run registration from CLI with initial metadata
6. **Event Streaming**: Receive and persist events from CLI processes in order
7. **Run Completion**: Mark runs as completed, failed, or stopped with final stats
8. **Resume Tracking**: Support `run.stopped` and `run.resumed` event types for continuation
9. **Repo Scoping**: All queries scoped by canonical repository path

### Performance Requirements

| Metric | Target |
|--------|--------|
| Run list response time | < 50ms for 100 runs |
| Event query response time | < 100ms for 1000 events |
| Event write throughput | > 100 events/second |
| Concurrent CLI connections | Support 10+ parallel runs |

### Constraints

- API binds to localhost only (127.0.0.1)
- No authentication required (local-only access assumed)
- Payload data redacted of sensitive information before storage
- Depends on SQLite store for persistence
- Events must maintain sequence ordering per run

## Design

### Shared Types

All shared types (Run, RunStatus, EventRecord, StoredEvent, GraphData, ListOptions, EventListOptions, RunList, EventList, APIError) are defined in [HISTORY-TYPES.md](./HISTORY-TYPES.md). This spec references those canonical definitions.

### Module Structure

```
internal/web/
├── server.go       # HTTP server setup, router configuration
├── handlers.go     # History API request handlers
├── types.go        # Request/response type definitions (implements HISTORY-TYPES.md)
├── middleware.go   # Logging, error handling middleware
└── static/         # Frontend assets served at /
```

### API-Specific Types

All shared types (Run, RunList, StoredEvent, EventList, GraphData, APIError) are defined in [HISTORY-TYPES.md](./HISTORY-TYPES.md). Below are the HTTP-specific request types.

```go
// Request Types

// ListRunsParams contains query parameters for listing runs
// Uses limit/offset pagination per HISTORY-TYPES.md
type ListRunsParams struct {
    Repo   string `json:"repo"`   // Required: canonical repo path
    Limit  int    `json:"limit"`  // Max results (default 50, max 100)
    Offset int    `json:"offset"` // Pagination offset
    Status string `json:"status"` // Filter: running/completed/failed/stopped
}

// ListEventsParams contains query parameters for event listing
// Uses limit/offset pagination per HISTORY-TYPES.md
type ListEventsParams struct {
    Type   string `json:"type"`   // Filter by event type prefix (e.g., "unit")
    Unit   string `json:"unit"`   // Filter by unit name
    Limit  int    `json:"limit"`  // Max results (default 100, max 1000)
    Offset int    `json:"offset"` // Pagination offset
}

// CreateRunRequest is sent by CLI to start a new run
// Maps to RunConfig in HISTORY-TYPES.md
type CreateRunRequest struct {
    ID           string `json:"id"`            // "run_20250120_143052_a1b2"
    RepoPath     string `json:"repo_path"`     // Canonical repo path
    Parallelism  int    `json:"parallelism"`   // Max concurrent workers
    TotalUnits   int    `json:"total_units"`   // Total units to process
    TasksDir     string `json:"tasks_dir"`     // Path to tasks directory
    DryRun       bool   `json:"dry_run"`       // Whether this is a dry run
}

// CreateEventRequest is sent by CLI to record an event
// Maps to EventRecord in HISTORY-TYPES.md
type CreateEventRequest struct {
    Seq     int             `json:"seq"`              // Sequence number for ordering
    Time    time.Time       `json:"time"`             // Event timestamp
    Type    string          `json:"type"`             // Event type (see HISTORY-TYPES.md)
    Unit    string          `json:"unit,omitempty"`   // Unit name (if applicable)
    Task    *int            `json:"task,omitempty"`   // Task number (if applicable)
    PR      *int            `json:"pr,omitempty"`     // PR number (if applicable)
    Payload json.RawMessage `json:"payload,omitempty"` // Additional data (will be redacted)
    Error   string          `json:"error,omitempty"`  // Error message (if applicable)
}

// CompleteRunRequest is sent by CLI to finalize a run
// Maps to RunResult in HISTORY-TYPES.md
type CompleteRunRequest struct {
    Status         string  `json:"status"`          // completed/failed/stopped
    CompletedUnits int     `json:"completed_units"` // Final count
    FailedUnits    int     `json:"failed_units"`    // Final count
    BlockedUnits   int     `json:"blocked_units"`   // Final count
    Error          *string `json:"error,omitempty"` // Error message if failed
}
```

### Event Types

Event type constants are defined in [HISTORY-TYPES.md](./HISTORY-TYPES.md). Key types include:

- **Run lifecycle:** `run.started`, `run.stopped`, `run.resumed`, `run.completed`, `run.failed`
- **Unit lifecycle:** `unit.queued`, `unit.started`, `unit.completed`, `unit.failed`, `unit.blocked`, `unit.skipped`
- **Task events:** `task.started`, `task.completed`, `task.failed`
- **PR events:** `pr.created`, `pr.merged`, `pr.failed`

### API Surface

```go
// Handler holds dependencies for HTTP handlers
type Handler struct {
    store *store.Store
}

// NewHandler creates a new API handler
func NewHandler(store *store.Store) *Handler

// Read endpoints (for Web UI)
func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request)
func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request)
func (h *Handler) GetRunEvents(w http.ResponseWriter, r *http.Request)
func (h *Handler) GetRunGraph(w http.ResponseWriter, r *http.Request)

// Write endpoints (for CLI)
func (h *Handler) CreateRun(w http.ResponseWriter, r *http.Request)
func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request)
func (h *Handler) CompleteRun(w http.ResponseWriter, r *http.Request)

// Router setup
func (h *Handler) RegisterRoutes(mux *http.ServeMux)
```

### API Endpoints

#### GET /api/history/runs

List runs for a repository with pagination.

**Request:**
```
GET /api/history/runs?repo=/Users/dev/myproject&limit=20&offset=0&status=running
```

**Response (200 OK):**
```json
{
  "runs": [
    {
      "id": "run_20250120_143052_a1b2",
      "repo_path": "/Users/dev/myproject",
      "started_at": "2025-01-20T14:30:52Z",
      "completed_at": null,
      "status": "running",
      "parallelism": 4,
      "total_units": 12,
      "completed_units": 5,
      "failed_units": 0,
      "blocked_units": 0,
      "error": null,
      "tasks_dir": "/Users/dev/myproject/.choo/tasks",
      "dry_run": false
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0,
  "has_more": false
}
```

**Error Response (400 Bad Request):**
```json
{
  "error": "repo parameter is required",
  "code": "MISSING_PARAM"
}
```

#### GET /api/history/runs/{id}

Get details for a specific run.

**Request:**
```
GET /api/history/runs/run_20250120_143052_a1b2
```

**Response (200 OK):**
```json
{
  "id": "run_20250120_143052_a1b2",
  "repo_path": "/Users/dev/myproject",
  "started_at": "2025-01-20T14:30:52Z",
  "completed_at": "2025-01-20T14:45:30Z",
  "status": "completed",
  "parallelism": 4,
  "total_units": 12,
  "completed_units": 12,
  "failed_units": 0,
  "blocked_units": 0,
  "error": null,
  "tasks_dir": "/Users/dev/myproject/.choo/tasks",
  "dry_run": false
}
```

**Error Response (404 Not Found):**
```json
{
  "error": "run not found",
  "code": "NOT_FOUND"
}
```

#### GET /api/history/runs/{id}/events

Get events for a run with optional filtering.

**Request:**
```
GET /api/history/runs/run_20250120_143052_a1b2/events?type=unit.started&unit=auth-service&limit=50
```

**Response (200 OK):**
```json
{
  "events": [
    {
      "id": 42,
      "run_id": "run_20250120_143052_a1b2",
      "seq": 5,
      "time": "2025-01-20T14:31:15Z",
      "type": "unit.started",
      "unit": "auth-service",
      "task": null,
      "pr": null,
      "payload": {"worktree": "/tmp/choo-work/auth-service"},
      "error": ""
    },
    {
      "id": 48,
      "run_id": "run_20250120_143052_a1b2",
      "seq": 11,
      "time": "2025-01-20T14:32:45Z",
      "type": "unit.complete",
      "unit": "auth-service",
      "task": null,
      "pr": 123,
      "payload": {"duration_ms": 90000},
      "error": ""
    }
  ],
  "total": 2,
  "limit": 50,
  "offset": 0,
  "has_more": false
}
```

#### GET /api/history/runs/{id}/graph

Get dependency graph data for visualization.

**Request:**
```
GET /api/history/runs/run_20250120_143052_a1b2/graph
```

**Response (200 OK):**
```json
{
  "run_id": "run_20250120_143052_a1b2",
  "nodes": [
    {"id": "auth-service", "label": "auth-service", "status": "completed"},
    {"id": "api-gateway", "label": "api-gateway", "status": "completed"},
    {"id": "user-service", "label": "user-service", "status": "running"}
  ],
  "edges": [
    {"from": "auth-service", "to": "api-gateway"},
    {"from": "auth-service", "to": "user-service"}
  ],
  "levels": [
    ["auth-service"],
    ["api-gateway", "user-service"]
  ]
}
```

**Error Response (404 Not Found):**
```json
{
  "error": "graph not found for run",
  "code": "NOT_FOUND"
}
```

#### POST /api/runs

Create a new run (called by CLI at start).

**Request:**
```json
{
  "id": "run_20250120_143052_a1b2",
  "repo_path": "/Users/dev/myproject",
  "parallelism": 4,
  "total_units": 12,
  "tasks_dir": "/Users/dev/myproject/.choo/tasks",
  "dry_run": false
}
```

**Response (201 Created):**
```json
{
  "id": "run_20250120_143052_a1b2",
  "repo_path": "/Users/dev/myproject",
  "started_at": "2025-01-20T14:30:52Z",
  "completed_at": null,
  "status": "running",
  "parallelism": 4,
  "total_units": 12,
  "completed_units": 0,
  "failed_units": 0,
  "blocked_units": 0,
  "error": null,
  "tasks_dir": "/Users/dev/myproject/.choo/tasks",
  "dry_run": false
}
```

**Error Response (409 Conflict):**
```json
{
  "error": "run already exists",
  "code": "ALREADY_EXISTS"
}
```

#### POST /api/runs/{id}/events

Record an event for a run (called by CLI during execution).

**Request:**
```json
{
  "seq": 5,
  "time": "2025-01-20T14:31:15Z",
  "type": "unit.started",
  "unit": "auth-service",
  "payload": {"worktree": "/tmp/choo-work/auth-service"}
}
```

**Response (201 Created):**
```json
{
  "id": 42,
  "run_id": "run_20250120_143052_a1b2",
  "seq": 5,
  "time": "2025-01-20T14:31:15Z",
  "type": "unit.started",
  "unit": "auth-service",
  "task": null,
  "pr": null,
  "payload": {"worktree": "/tmp/choo-work/auth-service"},
  "error": ""
}
```

**Error Response (404 Not Found):**
```json
{
  "error": "run not found",
  "code": "NOT_FOUND"
}
```

#### POST /api/runs/{id}/complete

Mark a run as complete (called by CLI at end).

**Request:**
```json
{
  "status": "completed",
  "completed_units": 12,
  "failed_units": 0,
  "blocked_units": 0
}
```

**Response (200 OK):**
```json
{
  "id": "run_20250120_143052_a1b2",
  "repo_path": "/Users/dev/myproject",
  "started_at": "2025-01-20T14:30:52Z",
  "completed_at": "2025-01-20T14:45:30Z",
  "status": "completed",
  "parallelism": 4,
  "total_units": 12,
  "completed_units": 12,
  "failed_units": 0,
  "blocked_units": 0,
  "error": null,
  "tasks_dir": "/Users/dev/myproject/.choo/tasks",
  "dry_run": false
}
```

**Request with failure:**
```json
{
  "status": "failed",
  "completed_units": 8,
  "failed_units": 2,
  "blocked_units": 2,
  "error": "Unit api-gateway failed: merge conflict"
}
```

### Handler Implementation

```go
package web

import (
    "encoding/json"
    "net/http"
    "strconv"
    "strings"

    "github.com/example/choo/internal/store"
)

type Handler struct {
    store *store.Store
}

func NewHandler(s *store.Store) *Handler {
    return &Handler{store: s}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
    // Read endpoints (Web UI)
    mux.HandleFunc("GET /api/history/runs", h.ListRuns)
    mux.HandleFunc("GET /api/history/runs/{id}", h.GetRun)
    mux.HandleFunc("GET /api/history/runs/{id}/events", h.GetRunEvents)
    mux.HandleFunc("GET /api/history/runs/{id}/graph", h.GetRunGraph)

    // Write endpoints (CLI)
    mux.HandleFunc("POST /api/runs", h.CreateRun)
    mux.HandleFunc("POST /api/runs/{id}/events", h.CreateEvent)
    mux.HandleFunc("POST /api/runs/{id}/complete", h.CompleteRun)
}

func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
    repo := r.URL.Query().Get("repo")
    if repo == "" {
        writeError(w, http.StatusBadRequest, "repo parameter is required", "MISSING_PARAM")
        return
    }

    limit := parseIntOrDefault(r.URL.Query().Get("limit"), 50)
    if limit > 100 {
        limit = 100
    }
    offset := parseIntOrDefault(r.URL.Query().Get("offset"), 0)
    status := r.URL.Query().Get("status")

    opts := store.ListOptions{
        Limit:  limit,
        Offset: offset,
        Status: status,
    }

    result, err := h.store.ListRuns(repo, opts)
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL")
        return
    }

    writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")

    run, err := h.store.GetRun(id)
    if err == store.ErrNotFound {
        writeError(w, http.StatusNotFound, "run not found", "NOT_FOUND")
        return
    }
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL")
        return
    }

    writeJSON(w, http.StatusOK, run)
}

func (h *Handler) GetRunEvents(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")

    // Verify run exists
    _, err := h.store.GetRun(id)
    if err == store.ErrNotFound {
        writeError(w, http.StatusNotFound, "run not found", "NOT_FOUND")
        return
    }

    limit := parseIntOrDefault(r.URL.Query().Get("limit"), 100)
    if limit > 1000 {
        limit = 1000
    }

    opts := store.EventListOptions{
        Type:   r.URL.Query().Get("type"),
        Unit:   r.URL.Query().Get("unit"),
        Limit:  limit,
        Offset: parseIntOrDefault(r.URL.Query().Get("offset"), 0),
    }

    result, err := h.store.GetRunEvents(id, opts)
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL")
        return
    }

    writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetRunGraph(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")

    graph, err := h.store.GetGraph(id)
    if err == store.ErrNotFound {
        writeError(w, http.StatusNotFound, "graph not found for run", "NOT_FOUND")
        return
    }
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL")
        return
    }

    writeJSON(w, http.StatusOK, graph)
}

func (h *Handler) CreateRun(w http.ResponseWriter, r *http.Request) {
    var req CreateRunRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON", "INVALID_JSON")
        return
    }

    if req.ID == "" || req.RepoPath == "" {
        writeError(w, http.StatusBadRequest, "id and repo_path are required", "MISSING_PARAM")
        return
    }

    run, err := h.store.CreateRun(store.CreateRunParams{
        ID:          req.ID,
        RepoPath:    req.RepoPath,
        Parallelism: req.Parallelism,
        TotalUnits:  req.TotalUnits,
        TasksDir:    req.TasksDir,
        DryRun:      req.DryRun,
    })
    if err == store.ErrAlreadyExists {
        writeError(w, http.StatusConflict, "run already exists", "ALREADY_EXISTS")
        return
    }
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL")
        return
    }

    writeJSON(w, http.StatusCreated, run)
}

func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")

    var req CreateEventRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON", "INVALID_JSON")
        return
    }

    // Redact sensitive data from payload
    redactedPayload := redactPayload(req.Payload)

    event, err := h.store.CreateEvent(id, store.CreateEventParams{
        Seq:     req.Seq,
        Time:    req.Time,
        Type:    req.Type,
        Unit:    req.Unit,
        Task:    req.Task,
        PR:      req.PR,
        Payload: redactedPayload,
        Error:   req.Error,
    })
    if err == store.ErrNotFound {
        writeError(w, http.StatusNotFound, "run not found", "NOT_FOUND")
        return
    }
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL")
        return
    }

    writeJSON(w, http.StatusCreated, event)
}

func (h *Handler) CompleteRun(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")

    var req CompleteRunRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON", "INVALID_JSON")
        return
    }

    if req.Status == "" {
        writeError(w, http.StatusBadRequest, "status is required", "MISSING_PARAM")
        return
    }

    run, err := h.store.CompleteRun(id, store.CompleteRunParams{
        Status:         req.Status,
        CompletedUnits: req.CompletedUnits,
        FailedUnits:    req.FailedUnits,
        BlockedUnits:   req.BlockedUnits,
        Error:          req.Error,
    })
    if err == store.ErrNotFound {
        writeError(w, http.StatusNotFound, "run not found", "NOT_FOUND")
        return
    }
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL")
        return
    }

    writeJSON(w, http.StatusOK, run)
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
    writeJSON(w, status, APIError{
        Error: message,
        Code:  code,
    })
}

func parseIntOrDefault(s string, def int) int {
    if s == "" {
        return def
    }
    v, err := strconv.Atoi(s)
    if err != nil {
        return def
    }
    return v
}

// redactPayload removes sensitive fields from event payloads
func redactPayload(payload json.RawMessage) json.RawMessage {
    if payload == nil {
        return nil
    }

    var data map[string]interface{}
    if err := json.Unmarshal(payload, &data); err != nil {
        return payload // Return as-is if not a JSON object
    }

    sensitiveKeys := []string{
        "token", "api_key", "apiKey", "secret", "password",
        "credentials", "auth", "authorization",
    }

    for _, key := range sensitiveKeys {
        for k := range data {
            if strings.EqualFold(k, key) {
                data[k] = "[REDACTED]"
            }
        }
    }

    redacted, _ := json.Marshal(data)
    return redacted
}
```

## Implementation Notes

### Security Considerations

1. **Localhost Binding**: The server binds exclusively to `127.0.0.1` to prevent remote access
2. **Payload Redaction**: All event payloads are scrubbed of sensitive fields before storage
3. **No Authentication**: Acceptable for local-only access; add auth if exposing to network
4. **Input Validation**: All inputs validated before database operations

### Event Ordering

Events use a `seq` field to maintain order within a run. The CLI is responsible for assigning monotonically increasing sequence numbers. The API does not enforce uniqueness but stores events in order of receipt.

```go
// CLI assigns sequence numbers
seq := atomic.AddInt64(&runSeq, 1)
client.CreateEvent(runID, CreateEventRequest{
    Seq:  int(seq),
    Time: time.Now(),
    Type: EventUnitStarted,
    Unit: unitName,
})
```

### Resume Event Handling

The `run.stopped` and `run.resumed` events mark pause/resume points:

```go
// When run is stopped
client.CreateEvent(runID, CreateEventRequest{
    Seq:     seq,
    Time:    time.Now(),
    Type:    EventRunStopped,
    Payload: json.RawMessage(`{"reason": "user_interrupt"}`),
})

// When run is resumed
client.CreateEvent(runID, CreateEventRequest{
    Seq:     seq,
    Time:    time.Now(),
    Type:    EventRunResumed,
    Payload: json.RawMessage(`{"resumed_from_seq": 42}`),
})
```

### Error Handling

All endpoints return structured errors with codes for programmatic handling:

| HTTP Status | Code | Meaning |
|-------------|------|---------|
| 400 | MISSING_PARAM | Required parameter missing |
| 400 | INVALID_JSON | Request body not valid JSON |
| 404 | NOT_FOUND | Resource does not exist |
| 409 | ALREADY_EXISTS | Resource already exists |
| 500 | INTERNAL | Internal server error |

## Testing Strategy

### Unit Tests

```go
func TestListRunsRequiresRepo(t *testing.T) {
    h := NewHandler(store.NewMemoryStore())

    req := httptest.NewRequest("GET", "/api/history/runs", nil)
    w := httptest.NewRecorder()

    h.ListRuns(w, req)

    if w.Code != http.StatusBadRequest {
        t.Errorf("expected 400, got %d", w.Code)
    }

    var resp APIError
    json.NewDecoder(w.Body).Decode(&resp)

    if resp.Code != "MISSING_PARAM" {
        t.Errorf("expected MISSING_PARAM, got %s", resp.Code)
    }
}

func TestCreateRunAndRetrieve(t *testing.T) {
    s := store.NewMemoryStore()
    h := NewHandler(s)

    // Create run
    createReq := CreateRunRequest{
        ID:          "run_test_001",
        RepoPath:    "/test/repo",
        Parallelism: 4,
        TotalUnits:  10,
    }
    body, _ := json.Marshal(createReq)

    req := httptest.NewRequest("POST", "/api/runs", bytes.NewReader(body))
    w := httptest.NewRecorder()
    h.CreateRun(w, req)

    if w.Code != http.StatusCreated {
        t.Fatalf("create failed: %d", w.Code)
    }

    // Retrieve run
    req = httptest.NewRequest("GET", "/api/history/runs/run_test_001", nil)
    req.SetPathValue("id", "run_test_001")
    w = httptest.NewRecorder()
    h.GetRun(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("get failed: %d", w.Code)
    }

    var run Run
    json.NewDecoder(w.Body).Decode(&run)

    if run.ID != "run_test_001" {
        t.Errorf("expected run_test_001, got %s", run.ID)
    }
    if run.Status != "running" {
        t.Errorf("expected running, got %s", run.Status)
    }
}

func TestEventCreationAndFiltering(t *testing.T) {
    s := store.NewMemoryStore()
    h := NewHandler(s)

    // Create run first
    s.CreateRun(store.CreateRunParams{
        ID:       "run_test_002",
        RepoPath: "/test/repo",
    })

    // Create events
    events := []CreateEventRequest{
        {Seq: 1, Time: time.Now(), Type: "unit.started", Unit: "auth"},
        {Seq: 2, Time: time.Now(), Type: "unit.started", Unit: "api"},
        {Seq: 3, Time: time.Now(), Type: "unit.complete", Unit: "auth"},
    }

    for _, e := range events {
        body, _ := json.Marshal(e)
        req := httptest.NewRequest("POST", "/api/runs/run_test_002/events", bytes.NewReader(body))
        req.SetPathValue("id", "run_test_002")
        w := httptest.NewRecorder()
        h.CreateEvent(w, req)

        if w.Code != http.StatusCreated {
            t.Fatalf("event create failed: %d", w.Code)
        }
    }

    // Filter by type
    req := httptest.NewRequest("GET", "/api/history/runs/run_test_002/events?type=unit.started", nil)
    req.SetPathValue("id", "run_test_002")
    w := httptest.NewRecorder()
    h.GetRunEvents(w, req)

    var result EventList
    json.NewDecoder(w.Body).Decode(&result)

    if len(result.Events) != 2 {
        t.Errorf("expected 2 unit.started events, got %d", len(result.Events))
    }

    // Filter by unit
    req = httptest.NewRequest("GET", "/api/history/runs/run_test_002/events?unit=auth", nil)
    req.SetPathValue("id", "run_test_002")
    w = httptest.NewRecorder()
    h.GetRunEvents(w, req)

    json.NewDecoder(w.Body).Decode(&result)

    if len(result.Events) != 2 {
        t.Errorf("expected 2 auth events, got %d", len(result.Events))
    }
}

func TestPayloadRedaction(t *testing.T) {
    payload := json.RawMessage(`{"worktree": "/tmp/work", "token": "secret123", "API_KEY": "key456"}`)

    redacted := redactPayload(payload)

    var data map[string]interface{}
    json.Unmarshal(redacted, &data)

    if data["worktree"] != "/tmp/work" {
        t.Error("worktree should not be redacted")
    }
    if data["token"] != "[REDACTED]" {
        t.Error("token should be redacted")
    }
    if data["API_KEY"] != "[REDACTED]" {
        t.Error("API_KEY should be redacted")
    }
}

func TestCompleteRunUpdatesStatus(t *testing.T) {
    s := store.NewMemoryStore()
    h := NewHandler(s)

    s.CreateRun(store.CreateRunParams{
        ID:         "run_test_003",
        RepoPath:   "/test/repo",
        TotalUnits: 5,
    })

    completeReq := CompleteRunRequest{
        Status:         "completed",
        CompletedUnits: 5,
        FailedUnits:    0,
        BlockedUnits:   0,
    }
    body, _ := json.Marshal(completeReq)

    req := httptest.NewRequest("POST", "/api/runs/run_test_003/complete", bytes.NewReader(body))
    req.SetPathValue("id", "run_test_003")
    w := httptest.NewRecorder()
    h.CompleteRun(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("complete failed: %d", w.Code)
    }

    var run Run
    json.NewDecoder(w.Body).Decode(&run)

    if run.Status != "completed" {
        t.Errorf("expected completed, got %s", run.Status)
    }
    if run.CompletedAt == nil {
        t.Error("completed_at should be set")
    }
}
```

### Integration Tests

1. **Full Run Lifecycle**: Create run, stream events, complete, query results
2. **Pagination**: Verify limit/offset work correctly across large datasets
3. **Concurrent Writes**: Multiple CLI processes writing events simultaneously
4. **Resume Flow**: Stop run, create stopped event, resume, verify continuation
5. **Graph Retrieval**: Store graph data, verify JSON structure integrity

### Manual Testing

- [ ] Start CLI run, verify run appears in GET /api/history/runs
- [ ] Verify events appear in real-time during run execution
- [ ] Stop run with Ctrl+C, verify `run.stopped` event recorded
- [ ] Resume run, verify `run.resumed` event with correct sequence
- [ ] Query events filtered by type, verify correct results
- [ ] Query events filtered by unit, verify correct results
- [ ] Complete run, verify status and completed_at updated
- [ ] Verify sensitive data redacted in stored payloads

## Design Decisions

### Why Separate Read/Write Endpoint Paths?

Read endpoints use `/api/history/runs` while write endpoints use `/api/runs`. This separation:
- Makes intent clear from the URL pattern
- Allows different rate limiting or middleware per category
- Supports future scenarios where reads might come from replicas

### Why Sequence Numbers Instead of Timestamps?

Events use `seq` for ordering rather than relying solely on timestamps:
- Timestamps can have millisecond collisions
- Distributed clocks may drift
- Sequence numbers provide deterministic ordering
- Enables replay from a specific point during resume

### Why Redact at API Layer?

Payload redaction happens in the API handlers rather than the store:
- Keeps storage layer simple and unaware of data semantics
- Allows different redaction rules per endpoint if needed
- Makes testing redaction logic straightforward

### Why No WebSocket for Events?

The initial design uses polling rather than WebSocket push:
- Simpler implementation for MVP
- Works reliably with existing infrastructure
- Can add WebSocket in future enhancement without breaking clients

## Future Enhancements

1. **WebSocket Event Stream**: Real-time push of events to web UI
2. **Event Aggregation**: Pre-computed summaries for dashboard views
3. **Run Comparison**: Compare two runs side-by-side
4. **Export Formats**: Export run data as JSON, CSV for analysis
5. **Retention Policies**: Auto-cleanup of old runs based on age/count
6. **Search**: Full-text search across event payloads
7. **Metrics Endpoint**: Prometheus-compatible metrics for monitoring

## References

- [HISTORY-TYPES.md](./HISTORY-TYPES.md) - Canonical shared type definitions
- [Historical Runs PRD](/docs/HISTORICAL-RUNS-PRD.md) - Product requirements
- [HISTORY-STORE.md](./HISTORY-STORE.md) - SQLite store implementation
- [HISTORY-UI.md](./HISTORY-UI.md) - Frontend that consumes this API
