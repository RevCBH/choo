---
task: 6
status: pending
backpressure: "go test ./internal/web/... -run TestServer"
depends_on: [1, 2, 3, 4, 5]
---

# Implement Main Server

**Parent spec**: `/specs/WEB.md`
**Task**: #6 of 7 in implementation plan

## Objective

Implement the main Server that wires together the store, hub, socket server, and HTTP server with proper lifecycle management.

## Dependencies

### Task Dependencies (within this unit)
- #1 (types.go) - Config type
- #2 (store.go) - Store
- #3 (sse.go) - Hub
- #4 (socket.go) - SocketServer
- #5 (handlers.go) - HTTP handlers, staticFS

### Package Dependencies
- `context`
- `fmt`
- `net/http`

## Deliverables

### Files to Create

```
internal/web/
├── server.go       # CREATE: Main server implementation
└── server_test.go  # CREATE: Server tests
```

### Types to Implement

```go
package web

import (
    "context"
    "net/http"
)

// Server is the main web server that coordinates all components.
type Server struct {
    addr   string
    socket string

    store *Store
    hub   *Hub

    httpServer   *http.Server
    socketServer *SocketServer

    shutdown chan struct{}
}
```

### Functions to Implement

```go
// New creates a new web server with the given configuration.
// Initializes store, hub, socket server, and HTTP server.
// Does not start any servers - call Start() for that.
func New(cfg Config) (*Server, error)

// Start begins listening on HTTP and Unix socket.
// - Starts the socket server
// - Starts the SSE hub event loop
// - Starts the HTTP server
// Non-blocking - servers run in goroutines.
func (s *Server) Start() error

// Stop performs graceful shutdown.
// - Stops socket server (closes listener, removes socket file)
// - Shuts down HTTP server with context timeout
// - Stops SSE hub
func (s *Server) Stop(ctx context.Context) error

// Addr returns the HTTP listen address.
func (s *Server) Addr() string

// SocketPath returns the Unix socket path.
func (s *Server) SocketPath() string
```

### Tests to Implement

```go
// server_test.go

func TestServer_New(t *testing.T)
// - Creates server with config
// - Store is initialized
// - Hub is initialized

func TestServer_NewWithDefaults(t *testing.T)
// - Empty config uses defaults
// - Addr defaults to ":8080"
// - SocketPath uses defaultSocketPath()

func TestServer_StartStop(t *testing.T)
// - Start creates socket and listens
// - HTTP server accepts connections
// - Stop cleans up socket file

func TestServer_HTTPRoutes(t *testing.T)
// - GET / serves static files
// - GET /api/state returns JSON
// - GET /api/graph returns JSON
// - GET /api/events streams SSE

func TestServer_GracefulShutdown(t *testing.T)
// - With active connections
// - Stop waits for in-flight requests
// - Connections are closed cleanly

func TestServer_SocketToSSE(t *testing.T)
// - Connect orchestrator to socket
// - Connect browser to SSE
// - Send event via socket
// - Browser receives event via SSE
```

## Backpressure

### Validation Command

```bash
go test ./internal/web/... -run TestServer -v
```

### Must Pass
- All TestServer* tests pass
- HTTP server responds on configured addr
- Socket accepts connections
- SSE streams events

### CI Compatibility
- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### Server Creation

```go
func New(cfg Config) (*Server, error) {
    if cfg.Addr == "" {
        cfg.Addr = ":8080"
    }
    if cfg.SocketPath == "" {
        cfg.SocketPath = defaultSocketPath()
    }

    store := NewStore()
    hub := NewHub()
    socketServer := NewSocketServer(cfg.SocketPath, store, hub)

    mux := http.NewServeMux()
    mux.Handle("/", IndexHandler(staticFS))
    mux.HandleFunc("/api/state", StateHandler(store))
    mux.HandleFunc("/api/graph", GraphHandler(store))
    mux.HandleFunc("/api/events", EventsHandler(hub))

    httpServer := &http.Server{
        Addr:    cfg.Addr,
        Handler: mux,
    }

    return &Server{
        addr:         cfg.Addr,
        socket:       cfg.SocketPath,
        store:        store,
        hub:          hub,
        httpServer:   httpServer,
        socketServer: socketServer,
        shutdown:     make(chan struct{}),
    }, nil
}
```

### Server Lifecycle

```go
func (s *Server) Start() error {
    // Start socket server
    if err := s.socketServer.Start(); err != nil {
        return fmt.Errorf("socket server: %w", err)
    }

    // Start SSE hub
    go s.hub.Run()

    // Start HTTP server
    go func() {
        if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
            // Log error but don't crash
        }
    }()

    return nil
}

func (s *Server) Stop(ctx context.Context) error {
    // Stop accepting new socket connections
    s.socketServer.Stop()

    // Stop SSE hub
    s.hub.Stop()

    // Shutdown HTTP server with timeout
    if err := s.httpServer.Shutdown(ctx); err != nil {
        return fmt.Errorf("HTTP shutdown: %w", err)
    }

    return nil
}
```

### Testing with Ephemeral Ports

For testing, use port 0 to get an ephemeral port:

```go
func TestServer_StartStop(t *testing.T) {
    tmpDir := t.TempDir()
    sockPath := filepath.Join(tmpDir, "test.sock")

    srv, err := New(Config{
        Addr:       "127.0.0.1:0", // Ephemeral port
        SocketPath: sockPath,
    })
    // ... test code
}
```

## NOT In Scope

- CLI command wiring (task #7)
- Signal handling (handled in CLI)
- Production logging configuration
