# WEB-PUSHER - Event Pusher for Web UI Integration

## Overview

The WEB-PUSHER component bridges the orchestrator's event bus to the web UI. When `choo run --web` is invoked, a SocketPusher subscribes to the event bus and writes events as JSON lines to a Unix socket where `choo web` is listening.

This design enables real-time visibility into orchestration progress through the web dashboard without coupling the core orchestrator to web-specific concerns. The pusher acts as a passive observer: it subscribes to events, serializes them, and writes to the socket. If the web server is not running, the pusher handles the connection failure gracefully without disrupting orchestration.

```
+-----------------------------------------+    Unix Socket      +---------------------------------+
|  choo run                               |  --------------->   |  choo web (daemon)              |
|                                         |  ~/.choo/web.sock   |                                 |
|  Event Bus                              |                     |                                 |
|     |                                   |  JSON lines:        |                                 |
|     +---> Orchestrator                  |  - orch.started     |                                 |
|     |                                   |  - unit.*           |                                 |
|     +---> SocketPusher (THIS SPEC)      |  - task.*           |                                 |
|          (subscribes, writes)           |  - pr.*             |                                 |
+-----------------------------------------+  - orch.completed   +---------------------------------+
```

## Requirements

### Functional Requirements

1. Connect to Unix socket at `~/.choo/web.sock` (or `$XDG_RUNTIME_DIR/choo/web.sock` if set)
2. Subscribe to ALL event types from the event bus
3. Serialize events as JSON and write one per line (newline-delimited JSON)
4. Include full dependency graph structure in `orch.started` payload
5. Handle connection failures gracefully without blocking the event bus
6. Reconnect automatically if connection is lost during operation
7. Close socket cleanly when orchestrator exits or context is cancelled
8. Support non-blocking writes to prevent slow socket from blocking event processing

### Performance Requirements

| Metric | Target |
|--------|--------|
| Event serialization latency | < 1ms |
| Write timeout | 100ms per event |
| Reconnection backoff | Exponential, max 5 seconds |
| Buffer capacity | 100 events before dropping |

### Constraints

- Must not block the event bus if socket write is slow
- Must handle missing socket file gracefully (web server not running)
- Must work on macOS and Linux
- Depends on events package for event types and bus subscription

## Design

### Module Structure

```
internal/web/
+-- pusher.go       # SocketPusher implementation
+-- pusher_test.go  # Unit tests
```

### Core Types

```go
// internal/web/pusher.go

// SocketPusher subscribes to the event bus and writes events to the web socket
type SocketPusher struct {
    // socketPath is the Unix socket path to connect to
    socketPath string

    // bus is the event bus to subscribe to
    bus *events.Bus

    // conn is the current socket connection (nil if not connected)
    conn net.Conn

    // mu protects conn access
    mu sync.RWMutex

    // eventCh buffers events for non-blocking writes
    eventCh chan events.Event

    // done signals shutdown
    done chan struct{}

    // wg tracks goroutines for clean shutdown
    wg sync.WaitGroup

    // graph is the dependency graph to include in orch.started
    graph *GraphPayload
}

// PusherConfig holds configuration for the SocketPusher
type PusherConfig struct {
    // SocketPath overrides the default socket path
    // If empty, uses ~/.choo/web.sock or $XDG_RUNTIME_DIR/choo/web.sock
    SocketPath string

    // BufferSize is the event buffer capacity (default: 100)
    BufferSize int

    // WriteTimeout is the max time to wait for a write (default: 100ms)
    WriteTimeout time.Duration

    // ReconnectBackoff is the initial reconnection delay (default: 100ms)
    ReconnectBackoff time.Duration

    // MaxReconnectBackoff is the maximum reconnection delay (default: 5s)
    MaxReconnectBackoff time.Duration
}

// GraphPayload represents the dependency graph for JSON serialization
type GraphPayload struct {
    Nodes  []NodePayload `json:"nodes"`
    Edges  []EdgePayload `json:"edges"`
    Levels [][]string    `json:"levels"`
}

// NodePayload represents a single node in the graph
type NodePayload struct {
    ID    string `json:"id"`
    Level int    `json:"level"`
}

// EdgePayload represents a dependency edge
type EdgePayload struct {
    From string `json:"from"`
    To   string `json:"to"`
}

// OrchStartedPayload is the payload for orch.started events
type OrchStartedPayload struct {
    UnitCount   int           `json:"unit_count"`
    Parallelism int           `json:"parallelism"`
    Graph       *GraphPayload `json:"graph"`
}

// WireEvent is the JSON structure written to the socket
type WireEvent struct {
    Type    string    `json:"type"`
    Time    time.Time `json:"time"`
    Unit    string    `json:"unit,omitempty"`
    Task    *int      `json:"task,omitempty"`
    PR      *int      `json:"pr,omitempty"`
    Payload any       `json:"payload,omitempty"`
    Error   string    `json:"error,omitempty"`
}
```

### API Surface

```go
// NewSocketPusher creates a pusher that writes events to the web socket
func NewSocketPusher(bus *events.Bus, cfg PusherConfig) *SocketPusher

// SetGraph sets the dependency graph to include in orch.started events
// Must be called before Start if graph should be included
func (p *SocketPusher) SetGraph(graph *scheduler.Graph, parallelism int)

// Start begins listening for events and writing to the socket
// Connects to the socket and subscribes to the event bus
// Returns error if initial connection fails and web server is required
func (p *SocketPusher) Start(ctx context.Context) error

// Close stops the pusher and closes the socket connection
// Blocks until all pending events are flushed or context times out
func (p *SocketPusher) Close() error

// Connected returns true if currently connected to the socket
func (p *SocketPusher) Connected() bool
```

```go
// DefaultSocketPath returns the default socket path based on environment
func DefaultSocketPath() string
```

### Event Flow

```
Event Bus                    SocketPusher                    Socket
    |                             |                             |
    | Emit(event)                 |                             |
    |-------------------------->  |                             |
    |                             |                             |
    |                     [subscribe handler]                   |
    |                             |                             |
    |                       event -> eventCh                    |
    |                             |                             |
    |                     [writer goroutine]                    |
    |                             |                             |
    |                       <- eventCh                          |
    |                             |                             |
    |                     json.Marshal(event)                   |
    |                             |                             |
    |                             | conn.Write(json + "\n")     |
    |                             |-------------------------->  |
    |                             |                             |
```

### Socket Path Resolution

```go
// DefaultSocketPath returns the socket path in order of preference:
// 1. $XDG_RUNTIME_DIR/choo/web.sock (if XDG_RUNTIME_DIR is set)
// 2. ~/.choo/web.sock (fallback)
func DefaultSocketPath() string {
    if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
        return filepath.Join(xdg, "choo", "web.sock")
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".choo", "web.sock")
}
```

### Message Format

Events are written as newline-delimited JSON (JSON Lines format):

```json
{"type":"orch.started","time":"2025-01-18T10:30:00Z","payload":{"unit_count":12,"parallelism":4,"graph":{"nodes":[{"id":"project-setup","level":0}],"edges":[{"from":"project-setup","to":"app-shell"}],"levels":[["project-setup"],["app-shell","config"]]}}}
{"type":"unit.started","unit":"app-shell","time":"2025-01-18T10:30:01Z"}
{"type":"task.started","unit":"app-shell","task":1,"time":"2025-01-18T10:30:02Z"}
{"type":"task.completed","unit":"app-shell","task":1,"time":"2025-01-18T10:30:15Z"}
{"type":"unit.completed","unit":"app-shell","time":"2025-01-18T10:31:00Z"}
{"type":"orch.completed","time":"2025-01-18T10:35:00Z"}
```

## Implementation Notes

### Non-Blocking Event Handling

The pusher must never block the event bus. Events are queued in a buffered channel and written by a dedicated goroutine:

```go
func (p *SocketPusher) handleEvent(e events.Event) {
    select {
    case p.eventCh <- e:
        // Queued successfully
    default:
        // Buffer full, drop event (log warning)
        log.Printf("WARN: web pusher buffer full, dropping event %s", e.Type)
    }
}

func (p *SocketPusher) writerLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-p.done:
            return
        case e := <-p.eventCh:
            p.writeEvent(e)
        }
    }
}
```

### Connection Management

The pusher maintains a single connection and handles reconnection with exponential backoff:

```go
func (p *SocketPusher) connect() error {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.conn != nil {
        return nil // Already connected
    }

    conn, err := net.DialTimeout("unix", p.socketPath, 5*time.Second)
    if err != nil {
        return fmt.Errorf("failed to connect to web socket: %w", err)
    }

    p.conn = conn
    return nil
}

func (p *SocketPusher) reconnectLoop(ctx context.Context) {
    backoff := p.cfg.ReconnectBackoff

    for {
        select {
        case <-ctx.Done():
            return
        case <-p.done:
            return
        default:
        }

        if err := p.connect(); err != nil {
            log.Printf("web pusher reconnect failed: %v, retrying in %v", err, backoff)
            time.Sleep(backoff)
            backoff = min(backoff*2, p.cfg.MaxReconnectBackoff)
            continue
        }

        // Connected, reset backoff
        backoff = p.cfg.ReconnectBackoff
        return
    }
}
```

### Graph Serialization

The dependency graph is converted to a JSON-friendly format when `SetGraph` is called:

```go
func (p *SocketPusher) SetGraph(graph *scheduler.Graph, parallelism int) {
    levels := graph.GetLevels()

    // Build node list with levels
    var nodes []NodePayload
    levelMap := make(map[string]int)
    for levelIdx, levelNodes := range levels {
        for _, nodeID := range levelNodes {
            nodes = append(nodes, NodePayload{
                ID:    nodeID,
                Level: levelIdx,
            })
            levelMap[nodeID] = levelIdx
        }
    }

    // Build edge list
    var edges []EdgePayload
    for _, node := range nodes {
        deps := graph.GetDependencies(node.ID)
        for _, dep := range deps {
            edges = append(edges, EdgePayload{
                From: dep,
                To:   node.ID,
            })
        }
    }

    p.graph = &GraphPayload{
        Nodes:  nodes,
        Edges:  edges,
        Levels: levels,
    }
}
```

### Write Timeout Handling

Writes use a deadline to prevent blocking on a slow or unresponsive socket:

```go
func (p *SocketPusher) writeEvent(e events.Event) error {
    p.mu.RLock()
    conn := p.conn
    p.mu.RUnlock()

    if conn == nil {
        return fmt.Errorf("not connected")
    }

    // Convert to wire format
    wire := WireEvent{
        Type:    string(e.Type),
        Time:    e.Time,
        Unit:    e.Unit,
        Task:    e.Task,
        PR:      e.PR,
        Payload: e.Payload,
        Error:   e.Error,
    }

    // Enrich orch.started with graph
    if e.Type == events.OrchStarted && p.graph != nil {
        if payload, ok := e.Payload.(map[string]any); ok {
            payload["graph"] = p.graph
            wire.Payload = payload
        }
    }

    data, err := json.Marshal(wire)
    if err != nil {
        return fmt.Errorf("failed to marshal event: %w", err)
    }

    // Set write deadline
    conn.SetWriteDeadline(time.Now().Add(p.cfg.WriteTimeout))

    // Write JSON line
    if _, err := conn.Write(append(data, '\n')); err != nil {
        // Connection lost, trigger reconnect
        p.mu.Lock()
        p.conn.Close()
        p.conn = nil
        p.mu.Unlock()
        go p.reconnectLoop(context.Background())
        return fmt.Errorf("write failed: %w", err)
    }

    return nil
}
```

### Graceful Shutdown

On shutdown, the pusher drains pending events before closing:

```go
func (p *SocketPusher) Close() error {
    close(p.done)

    // Drain remaining events with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    for {
        select {
        case <-ctx.Done():
            // Timeout, force close
            break
        case e := <-p.eventCh:
            p.writeEvent(e)
        default:
            // Buffer empty
            break
        }
    }

    // Close connection
    p.mu.Lock()
    if p.conn != nil {
        p.conn.Close()
        p.conn = nil
    }
    p.mu.Unlock()

    // Wait for goroutines
    p.wg.Wait()

    return nil
}
```

## CLI Integration

The `--web` flag in `run.go` creates and starts the SocketPusher:

```go
// In NewRunCmd, add the --web flag
cmd.Flags().BoolVar(&opts.Web, "web", false, "Connect to web UI via socket")

// In RunOrchestrator, create pusher when --web is set
if opts.Web {
    pusher := web.NewSocketPusher(eventBus, web.PusherConfig{})

    // Set graph after discovery
    pusher.SetGraph(graph, opts.Parallelism)

    if err := pusher.Start(ctx); err != nil {
        // Log warning but continue - web UI is optional
        log.Printf("WARN: failed to connect to web UI: %v", err)
    } else {
        defer pusher.Close()
        fmt.Println("Orchestrator connected to web UI")
    }
}
```

## Testing Strategy

### Unit Tests

```go
// internal/web/pusher_test.go

func TestSocketPusher_WriteEvent(t *testing.T) {
    // Create a test socket pair
    server, client := net.Pipe()
    defer server.Close()
    defer client.Close()

    pusher := &SocketPusher{
        conn:    client,
        eventCh: make(chan events.Event, 10),
        done:    make(chan struct{}),
        cfg: PusherConfig{
            WriteTimeout: 100 * time.Millisecond,
        },
    }

    // Write an event
    e := events.NewEvent(events.UnitStarted, "test-unit")
    e.Time = time.Date(2025, 1, 18, 10, 30, 0, 0, time.UTC)

    err := pusher.writeEvent(e)
    assert.NoError(t, err)

    // Read from server side
    buf := make([]byte, 1024)
    n, err := server.Read(buf)
    assert.NoError(t, err)

    var wire WireEvent
    err = json.Unmarshal(buf[:n-1], &wire) // -1 for newline
    assert.NoError(t, err)
    assert.Equal(t, "unit.started", wire.Type)
    assert.Equal(t, "test-unit", wire.Unit)
}

func TestSocketPusher_NonBlocking(t *testing.T) {
    pusher := &SocketPusher{
        eventCh: make(chan events.Event, 1), // Small buffer
        done:    make(chan struct{}),
    }

    // Fill buffer
    e := events.NewEvent(events.UnitStarted, "test")
    pusher.handleEvent(e)

    // This should not block - event should be dropped
    start := time.Now()
    pusher.handleEvent(e)
    pusher.handleEvent(e)
    elapsed := time.Since(start)

    assert.Less(t, elapsed, 10*time.Millisecond, "handleEvent should not block")
}

func TestSocketPusher_GraphPayload(t *testing.T) {
    units := []*discovery.Unit{
        {ID: "project-setup", DependsOn: []string{}},
        {ID: "app-shell", DependsOn: []string{"project-setup"}},
        {ID: "config", DependsOn: []string{"project-setup"}},
    }

    graph, err := scheduler.NewGraph(units)
    assert.NoError(t, err)

    pusher := &SocketPusher{}
    pusher.SetGraph(graph, 4)

    assert.NotNil(t, pusher.graph)
    assert.Len(t, pusher.graph.Nodes, 3)
    assert.Len(t, pusher.graph.Edges, 2)
    assert.Len(t, pusher.graph.Levels, 2)

    // Verify levels
    assert.Equal(t, []string{"project-setup"}, pusher.graph.Levels[0])
    assert.Contains(t, pusher.graph.Levels[1], "app-shell")
    assert.Contains(t, pusher.graph.Levels[1], "config")
}

func TestSocketPusher_ReconnectBackoff(t *testing.T) {
    pusher := &SocketPusher{
        socketPath: "/nonexistent/socket.sock",
        cfg: PusherConfig{
            ReconnectBackoff:    10 * time.Millisecond,
            MaxReconnectBackoff: 50 * time.Millisecond,
        },
        done: make(chan struct{}),
    }

    // Track reconnect attempts
    attempts := 0
    start := time.Now()

    ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel()

    // Run reconnect in background
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            default:
                pusher.connect()
                attempts++
                time.Sleep(pusher.cfg.ReconnectBackoff)
            }
        }
    }()

    <-ctx.Done()
    elapsed := time.Since(start)

    // With exponential backoff, should have fewer attempts than linear
    // 200ms with 10ms backoff linear = 20 attempts
    // With exponential (10, 20, 40, 50, 50...) = ~5 attempts
    assert.Less(t, attempts, 15, "should use exponential backoff")
    assert.GreaterOrEqual(t, elapsed, 150*time.Millisecond)
}

func TestDefaultSocketPath(t *testing.T) {
    // Test with XDG_RUNTIME_DIR set
    t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
    path := DefaultSocketPath()
    assert.Equal(t, "/run/user/1000/choo/web.sock", path)

    // Test fallback to home directory
    t.Setenv("XDG_RUNTIME_DIR", "")
    home, _ := os.UserHomeDir()
    path = DefaultSocketPath()
    assert.Equal(t, filepath.Join(home, ".choo", "web.sock"), path)
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Full event flow | Start mock socket server, create pusher, emit events, verify JSON received |
| Connection loss | Connect, close server, emit events, verify reconnection attempts |
| Graceful shutdown | Connect, emit events, close pusher, verify all events flushed |
| Web server not running | Start pusher without server, verify graceful handling |

### Manual Testing

- [ ] `choo web` creates socket at expected path
- [ ] `choo run --web` connects to socket on startup
- [ ] Events appear in web UI in real-time
- [ ] `orch.started` includes complete graph structure
- [ ] Stopping `choo web` doesn't crash `choo run --web`
- [ ] Restarting `choo web` reconnects automatically
- [ ] `choo run --web` without web server shows warning and continues

## Design Decisions

### Why Unix Socket over TCP?

Unix sockets provide several advantages for local IPC:
- No port conflicts or firewall issues
- Filesystem-based access control (permissions)
- Slightly lower latency than TCP loopback
- Natural single-writer semantics (one `choo run` per socket)

The trade-off is platform limitation (Unix-like only), but the target audience uses macOS/Linux.

### Why JSON Lines over WebSocket?

JSON Lines is simpler to implement and debug:
- No framing protocol needed (newline is the delimiter)
- Each line is independently parseable
- Easy to test with `nc` or `socat`
- Logs can be tailed directly

WebSocket would require additional dependencies and complexity for minimal benefit in this local IPC scenario.

### Why Buffered Channel for Non-Blocking?

The event bus handlers run synchronously. A slow socket write would block all other handlers (like logging). The buffered channel decouples event reception from socket writes, allowing the handler to return immediately.

The trade-off is potential event loss under extreme load, but with 100 events buffer capacity, this is unlikely in practice.

### Why Graceful Degradation?

The web UI is optional - orchestration should work without it. If the socket doesn't exist or the connection fails, the pusher logs a warning but doesn't stop the orchestrator. This prevents the web integration from becoming a reliability risk.

## Future Enhancements

1. **Bidirectional communication**: Allow web UI to send commands back (pause, cancel, etc.)
2. **Multiple clients**: Support multiple web UI connections (fan-out to all)
3. **Event filtering**: Allow clients to subscribe to specific event types
4. **Compression**: Compress event stream for lower bandwidth
5. **TLS support**: Encrypted socket for remote connections

## References

- [EVENTS spec](completed/EVENTS.md) - Event types and bus API
- [ORCHESTRATOR spec](completed/ORCHESTRATOR.md) - Event emission points
- [CLI spec](completed/CLI.md) - Command structure and flags
- [SCHEDULER spec](completed/SCHEDULER.md) - Graph structure and levels
- [JSON Lines specification](https://jsonlines.org/)
- [Unix domain sockets](https://man7.org/linux/man-pages/man7/unix.7.html)
