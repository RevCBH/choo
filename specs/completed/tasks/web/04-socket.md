---
task: 4
status: complete
backpressure: "go test ./internal/web/... -run TestSocket"
depends_on: [1, 2, 3]
---

# Implement Unix Socket Server

**Parent spec**: `/specs/WEB.md`
**Task**: #4 of 7 in implementation plan

## Objective

Implement the Unix socket server that listens for orchestrator connections, reads JSON events, updates the store, and broadcasts to SSE clients.

## Dependencies

### Task Dependencies (within this unit)
- #1 (types.go) - Event type
- #2 (store.go) - Store for state updates
- #3 (sse.go) - Hub for broadcasting

### Package Dependencies
- `bufio`
- `encoding/json`
- `fmt`
- `log`
- `net`
- `os`
- `path/filepath`

## Deliverables

### Files to Create

```
internal/web/
├── socket.go       # CREATE: Unix socket server
└── socket_test.go  # CREATE: Socket server tests
```

### Types to Implement

```go
package web

import (
    "net"
)

// SocketServer listens for orchestrator connections on a Unix socket.
// Only one orchestrator connection is handled at a time.
type SocketServer struct {
    path     string
    listener net.Listener
    store    *Store
    hub      *Hub
    done     chan struct{}
}
```

### Functions to Implement

```go
// NewSocketServer creates a Unix socket server.
// Does not start listening - call Start() for that.
func NewSocketServer(path string, store *Store, hub *Hub) *SocketServer

// Start begins listening for orchestrator connections.
// Removes any stale socket file before listening.
// Runs accept loop in a goroutine.
func (s *SocketServer) Start() error

// Stop closes the socket and cleans up.
// Removes the socket file.
func (s *SocketServer) Stop() error

// Path returns the socket path.
func (s *SocketServer) Path() string

// defaultSocketPath returns the default socket path.
// Uses $XDG_RUNTIME_DIR/choo/web.sock if set,
// otherwise ~/.choo/web.sock
func defaultSocketPath() string

// handleConnection processes a single orchestrator connection.
// Reads JSON events line by line, updates store, broadcasts to hub.
// Sets store connected=true on connect, connected=false on disconnect.
func (s *SocketServer) handleConnection(conn net.Conn)
```

### Tests to Implement

```go
// socket_test.go

func TestSocketServer_NewSocketServer(t *testing.T)
// - Creates server with given path
// - Path() returns correct path

func TestSocketServer_StartStop(t *testing.T)
// - Start creates socket file
// - Stop removes socket file

func TestSocketServer_RemovesStaleSocket(t *testing.T)
// - Create stale socket file
// - Start removes it and creates new socket

func TestSocketServer_AcceptsConnection(t *testing.T)
// - Start server
// - Connect to socket
// - Connection succeeds

func TestSocketServer_ParsesEvents(t *testing.T)
// - Connect to socket
// - Write JSON event line
// - Store receives event

func TestSocketServer_BroadcastsEvents(t *testing.T)
// - Register SSE client
// - Connect to socket
// - Write event
// - SSE client receives event

func TestSocketServer_SetsConnected(t *testing.T)
// - Connect sets store.connected=true
// - Disconnect sets store.connected=false

func TestSocketServer_HandlesMalformedJSON(t *testing.T)
// - Send malformed JSON
// - Does not crash
// - Continues processing subsequent valid events

func TestSocketServer_DefaultSocketPath(t *testing.T)
// - With XDG_RUNTIME_DIR set, uses that
// - Without XDG_RUNTIME_DIR, uses ~/.choo/web.sock

func TestSocketServer_LargeEvents(t *testing.T)
// - Send event with large payload (near 1MB)
// - Event is parsed correctly
```

## Backpressure

### Validation Command

```bash
go test ./internal/web/... -run TestSocket -v
```

### Must Pass
- All TestSocket* tests pass
- Socket file is created on Start
- Socket file is removed on Stop

### CI Compatibility
- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Remove stale socket file before `net.Listen("unix", path)` to handle unclean shutdown
- Create parent directories with `os.MkdirAll(dir, 0700)` before creating socket
- Use `bufio.Scanner` with custom buffer for reading large events:
  ```go
  scanner := bufio.NewScanner(conn)
  scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 1MB max line
  ```
- Set `store.SetConnected(true)` at start of `handleConnection`
- Use `defer store.SetConnected(false)` for cleanup on disconnect
- Log malformed JSON errors but continue processing
- Only one connection at a time (subsequent connections wait)

### Socket Path Resolution

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

### Connection Handler Pattern

```go
func (s *SocketServer) handleConnection(conn net.Conn) {
    defer conn.Close()

    s.store.SetConnected(true)
    defer s.store.SetConnected(false)

    scanner := bufio.NewScanner(conn)
    scanner.Buffer(make([]byte, 64*1024), 1024*1024)

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

## NOT In Scope

- HTTP handlers (task #5)
- Server lifecycle integration (task #6)
- CLI command (task #7)
