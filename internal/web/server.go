package web

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

// Server is the main web server that coordinates all components.
type Server struct {
	addr   string
	socket string

	store *Store
	hub   *Hub

	httpServer   *http.Server
	httpListener net.Listener
	socketServer *SocketServer

	shutdown chan struct{}
}

// New creates a new web server with the given configuration.
// Initializes store, hub, socket server, and HTTP server.
// Does not start any servers - call Start() for that.
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

// Start begins listening on HTTP and Unix socket.
// - Starts the socket server
// - Starts the SSE hub event loop
// - Starts the HTTP server
// Non-blocking - servers run in goroutines.
func (s *Server) Start() error {
	// Start socket server
	if err := s.socketServer.Start(); err != nil {
		return fmt.Errorf("socket server: %w", err)
	}

	// Start SSE hub
	go s.hub.Run()

	// Create HTTP listener
	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("HTTP listen: %w", err)
	}
	s.httpListener = listener

	// Update addr with actual address (important for ephemeral ports)
	s.addr = listener.Addr().String()

	// Start HTTP server
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Log error but don't crash - server already started
			_ = err // explicitly ignore
		}
	}()

	return nil
}

// Stop performs graceful shutdown.
// - Stops socket server (closes listener, removes socket file)
// - Shuts down HTTP server with context timeout
// - Stops SSE hub
func (s *Server) Stop(ctx context.Context) error {
	// Stop accepting new socket connections
	_ = s.socketServer.Stop()

	// Stop SSE hub
	s.hub.Stop()

	// Shutdown HTTP server with timeout
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP shutdown: %w", err)
	}

	return nil
}

// Addr returns the HTTP listen address.
func (s *Server) Addr() string {
	return s.addr
}

// SocketPath returns the Unix socket path.
func (s *Server) SocketPath() string {
	return s.socket
}
