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

// NewHub creates a new SSE hub with initialized channels.
// Call Run() to start the event loop.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *Event),
		done:       make(chan struct{}),
	}
}

// Run starts the hub's event loop.
// Processes register, unregister, and broadcast operations.
// Blocks until Stop() is called - run in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			// Close all clients and return
			h.mu.Lock()
			for client := range h.clients {
				close(client.events)
			}
			h.clients = make(map[*Client]struct{})
			h.mu.Unlock()
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

// Stop signals the hub to stop processing.
// Closes all client connections.
func (h *Hub) Stop() {
	close(h.done)
}

// Register adds a client to receive events.
// Non-blocking - sends to register channel.
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Unregister removes a client.
// Non-blocking - sends to unregister channel.
func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

// Broadcast sends an event to all connected clients.
// Non-blocking - sends to broadcast channel.
// If a client's buffer is full, the event is dropped for that client.
func (h *Hub) Broadcast(e *Event) {
	h.broadcast <- e
}

// Count returns the number of connected clients.
// Thread-safe.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// NewClient creates a new client with the given ID.
// The events channel is buffered (256 events).
func NewClient(id string) *Client {
	return &Client{
		id:     id,
		events: make(chan *Event, 256),
		done:   make(chan struct{}),
	}
}
