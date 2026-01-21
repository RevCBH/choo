# Task 05: Server-Sent Events Broadcast

```yaml
task: 05-sse
unit: daemon
depends_on: [04-routes]
backpressure: "go test ./internal/daemon/... -run TestSSE -v"
```

## Objective

Implement Server-Sent Events (SSE) for real-time event broadcasting to web UI clients.

## Requirements

1. Create `internal/daemon/sse.go` with:

   ```go
   type SSEBroadcaster struct {
       clients map[string]map[chan SSEEvent]bool // runID -> set of client channels
       mu      sync.RWMutex
   }

   type SSEEvent struct {
       Type string          // "event", "state", "complete"
       Data json.RawMessage // Event or state payload
   }

   func NewSSEBroadcaster() *SSEBroadcaster

   // Subscribe adds a client for a specific run
   func (b *SSEBroadcaster) Subscribe(runID string) (<-chan SSEEvent, func())

   // Broadcast sends an event to all clients watching a run
   func (b *SSEBroadcaster) Broadcast(runID string, event SSEEvent)

   // BroadcastAll sends an event to all connected clients
   func (b *SSEBroadcaster) BroadcastAll(event SSEEvent)

   // ClientCount returns number of connected clients
   func (b *SSEBroadcaster) ClientCount(runID string) int
   ```

2. Add SSE endpoint to routes:

   ```go
   mux.HandleFunc("GET /api/runs/{id}/stream", d.handleSSEStream)
   mux.HandleFunc("GET /api/stream", d.handleSSEStreamAll) // All runs
   ```

3. SSE handler implementation:
   - Set headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`
   - Keep connection open with periodic heartbeats (every 30s)
   - Handle client disconnect (context cancellation)
   - Send initial state on connect (current run status)
   - Forward events from broadcaster

4. Integration with daemon:
   - Create broadcaster in `Daemon.Start()`
   - Call `Broadcast` when events are recorded
   - Call `Broadcast` when runs complete
   - Clean up broadcaster on shutdown

5. SSE message format:
   ```
   event: event
   data: {"type":"unit.started","unit":"auth-service",...}

   event: state
   data: {"id":"run_123","status":"running","completed_units":5,...}

   event: complete
   data: {"id":"run_123","status":"completed",...}
   ```

## Acceptance Criteria

- [ ] SSE connections work from browser
- [ ] Events are broadcast to connected clients
- [ ] Multiple clients can watch the same run
- [ ] Client disconnect is handled gracefully
- [ ] Heartbeats keep connections alive
- [ ] Initial state is sent on connect

## Files to Create/Modify

- `internal/daemon/sse.go` (create)
- `internal/daemon/sse_test.go` (create)
- `internal/daemon/routes.go` (modify - add SSE endpoints)
- `internal/daemon/daemon.go` (modify - integrate broadcaster)
