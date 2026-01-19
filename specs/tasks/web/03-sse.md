---
task: 3
status: pending
backpressure: "go test ./internal/web/... -run TestHub"
depends_on: [1]
---

# Implement SSE Hub

**Parent spec**: `/specs/WEB.md`
**Task**: #3 of 7 in implementation plan

## Objective

Implement the SSE (Server-Sent Events) hub that manages browser connections and broadcasts events to all connected clients.

## Dependencies

### Task Dependencies (within this unit)
- #1 (types.go) - Event type

### Package Dependencies
- `sync`

## Deliverables

### Files to Create

```
internal/web/
├── sse.go       # CREATE: SSE hub and client types
└── sse_test.go  # CREATE: SSE hub tests
```

### Types to Implement

```go
package web

import "sync"

// Hub manages SSE client connections and broadcasts events.
// It runs an event loop in a separate goroutine.
type Hub struct {
    mu      sync.RWMutex
    clients map[*Client]struct{}

    // Channels for client management
    register   chan *Client
    unregister chan *Client
    broadcast  chan *Event

    // done signals the Run loop to exit
    done chan struct{}
}

// Client represents a connected browser.
// Each browser connection gets its own Client instance.
type Client struct {
    id     string
    events chan *Event
    done   chan struct{}
}
```

### Functions to Implement

```go
// NewHub creates a new SSE hub with initialized channels.
// Call Run() to start the event loop.
func NewHub() *Hub

// Run starts the hub's event loop.
// Processes register, unregister, and broadcast operations.
// Blocks until Stop() is called - run in a goroutine.
func (h *Hub) Run()

// Stop signals the hub to stop processing.
// Closes all client connections.
func (h *Hub) Stop()

// Register adds a client to receive events.
// Non-blocking - sends to register channel.
func (h *Hub) Register(c *Client)

// Unregister removes a client.
// Non-blocking - sends to unregister channel.
func (h *Hub) Unregister(c *Client)

// Broadcast sends an event to all connected clients.
// Non-blocking - sends to broadcast channel.
// If a client's buffer is full, the event is dropped for that client.
func (h *Hub) Broadcast(e *Event)

// Count returns the number of connected clients.
// Thread-safe.
func (h *Hub) Count() int

// NewClient creates a new client with the given ID.
// The events channel is buffered (256 events).
func NewClient(id string) *Client
```

### Tests to Implement

```go
// sse_test.go

func TestHub_NewHub(t *testing.T)
// - NewHub returns hub with empty clients
// - Channels are initialized

func TestHub_ClientRegistration(t *testing.T)
// - Register adds client to hub
// - Count returns correct number
// - Unregister removes client
// - Count decrements after unregister

func TestHub_Broadcast(t *testing.T)
// - Broadcast sends event to all clients
// - Each client receives the event on its channel

func TestHub_BroadcastMultipleClients(t *testing.T)
// - With 3 clients registered
// - Broadcast sends to all 3
// - All 3 receive the same event

func TestHub_BroadcastDropsWhenFull(t *testing.T)
// - Client with full buffer (256 events)
// - Broadcast does not block
// - Event is dropped for full client

func TestHub_UnregisterClosesChannel(t *testing.T)
// - After unregister, client.events channel is closed

func TestHub_Stop(t *testing.T)
// - Stop terminates the Run loop
// - All clients are disconnected

func TestHub_ConcurrentOperations(t *testing.T)
// - Multiple goroutines register/unregister/broadcast
// - No data races
```

## Backpressure

### Validation Command

```bash
go test ./internal/web/... -run TestHub -v -race
```

### Must Pass
- All TestHub* tests pass
- No data races detected

### CI Compatibility
- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Hub uses channel-based architecture for thread-safe client management
- `Run()` blocks on a select over register/unregister/broadcast/done channels
- Client event channel is buffered (256) to prevent blocking broadcast
- When client buffer is full, use non-blocking send with `select { case client.events <- e: default: }`
- `Unregister` must close the client's events channel after removing from map
- Use `sync.RWMutex` in `Count()` for safe concurrent reads
- `Stop()` closes the done channel to signal Run() to exit

### Hub Event Loop Pattern

```go
func (h *Hub) Run() {
    for {
        select {
        case <-h.done:
            // Close all clients and return
            return
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
                    // Buffer full, drop event for this client
                }
            }
            h.mu.RUnlock()
        }
    }
}
```

## NOT In Scope

- HTTP handler for SSE endpoint (task #5)
- Socket handling (task #4)
- Server lifecycle (task #6)
