---
task: 5
status: complete
backpressure: "go test ./internal/web/... -run TestHandler"
depends_on: [1, 2, 3]
---

# Implement HTTP Handlers

**Parent spec**: `/specs/WEB.md`
**Task**: #5 of 7 in implementation plan

## Objective

Implement HTTP handlers for the web API: static file serving, state snapshot, graph data, and SSE event stream.

## Dependencies

### Task Dependencies (within this unit)
- #1 (types.go) - Event, StateSnapshot, GraphData types
- #2 (store.go) - Store for state queries
- #3 (sse.go) - Hub for SSE streaming

### Package Dependencies
- `encoding/json`
- `fmt`
- `io/fs`
- `net/http`

## Deliverables

### Files to Create

```
internal/web/
├── handlers.go       # CREATE: HTTP handler functions
├── handlers_test.go  # CREATE: Handler tests
└── embed.go          # CREATE: Static file embedding
```

Also create placeholder static files:

```
internal/web/static/
├── index.html    # CREATE: Placeholder HTML
├── style.css     # CREATE: Placeholder CSS
└── app.js        # CREATE: Placeholder JS
```

### Functions to Implement

```go
// handlers.go
package web

import (
    "encoding/json"
    "fmt"
    "io/fs"
    "net/http"
)

// IndexHandler serves the embedded HTML UI.
// Serves index.html for "/" and static files for other paths.
func IndexHandler(staticFS fs.FS) http.Handler

// StateHandler returns the current state snapshot as JSON.
// GET /api/state
func StateHandler(store *Store) http.HandlerFunc

// GraphHandler returns the dependency graph as JSON.
// GET /api/graph
// Returns empty graph if orchestrator not connected.
func GraphHandler(store *Store) http.HandlerFunc

// EventsHandler provides the SSE event stream.
// GET /api/events
// Sets appropriate headers and streams events to browser.
func EventsHandler(hub *Hub) http.HandlerFunc
```

```go
// embed.go
package web

import "embed"

//go:embed static/*
var staticFS embed.FS
```

### Tests to Implement

```go
// handlers_test.go

func TestIndexHandler_ServesHTML(t *testing.T)
// - GET / returns index.html content
// - Content-Type is text/html

func TestIndexHandler_ServesStaticFiles(t *testing.T)
// - GET /style.css returns CSS
// - GET /app.js returns JS

func TestStateHandler_ReturnsJSON(t *testing.T)
// - GET /api/state returns JSON
// - Content-Type is application/json

func TestStateHandler_WaitingState(t *testing.T)
// - New store returns status="waiting"
// - connected=false
// - empty units array

func TestStateHandler_RunningState(t *testing.T)
// - After orch.started event
// - status="running"
// - units populated from graph

func TestGraphHandler_NoGraph(t *testing.T)
// - Before orch.started
// - Returns empty graph structure

func TestGraphHandler_WithGraph(t *testing.T)
// - After orch.started with graph
// - Returns nodes and edges

func TestEventsHandler_SetsHeaders(t *testing.T)
// - Content-Type is text/event-stream
// - Cache-Control is no-cache
// - Connection is keep-alive

func TestEventsHandler_StreamsEvents(t *testing.T)
// - Connect to /api/events
// - Broadcast event via hub
// - Event received in SSE format

func TestEventsHandler_SSEFormat(t *testing.T)
// - Event formatted as "event: type\ndata: json\n\n"
```

## Backpressure

### Validation Command

```bash
go test ./internal/web/... -run TestHandler -v
```

### Must Pass
- All TestHandler* tests pass
- Static files serve correctly
- JSON responses are valid

### CI Compatibility
- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### Static File Serving

Use `fs.Sub` to strip the "static" prefix from embedded files:

```go
func IndexHandler(staticFS fs.FS) http.Handler {
    subFS, _ := fs.Sub(staticFS, "static")
    return http.FileServer(http.FS(subFS))
}
```

### State Handler

```go
func StateHandler(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        snapshot := store.Snapshot()
        json.NewEncoder(w).Encode(snapshot)
    }
}
```

### Graph Handler

```go
func GraphHandler(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        graph := store.Graph()
        if graph == nil {
            graph = &GraphData{
                Nodes:  []GraphNode{},
                Edges:  []GraphEdge{},
                Levels: [][]string{},
            }
        }
        json.NewEncoder(w).Encode(graph)
    }
}
```

### SSE Events Handler

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

        client := NewClient(generateID())
        hub.Register(client)
        defer hub.Unregister(client)

        ctx := r.Context()
        for {
            select {
            case <-ctx.Done():
                return
            case event, ok := <-client.events:
                if !ok {
                    return
                }
                data, _ := json.Marshal(event)
                fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
                flusher.Flush()
            }
        }
    }
}
```

### Placeholder Static Files

Create minimal placeholders that will be replaced later:

**index.html:**
```html
<!DOCTYPE html>
<html>
<head><title>Choo Monitor</title></head>
<body>
<h1>Choo Orchestrator Monitor</h1>
<p>Web UI placeholder - coming soon</p>
</body>
</html>
```

**style.css:**
```css
/* Placeholder CSS */
body { font-family: sans-serif; }
```

**app.js:**
```javascript
// Placeholder JS
console.log('Choo web UI');
```

## NOT In Scope

- Full web UI implementation (future task)
- Server wiring (task #6)
- CLI command (task #7)
