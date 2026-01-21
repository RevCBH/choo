package web

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"os"
	"path/filepath"
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

// NewSocketServer creates a Unix socket server.
// Does not start listening - call Start() for that.
func NewSocketServer(path string, store *Store, hub *Hub) *SocketServer {
	return &SocketServer{
		path:  path,
		store: store,
		hub:   hub,
		done:  make(chan struct{}),
	}
}

// Start begins listening for orchestrator connections.
// Removes any stale socket file before listening.
// Runs accept loop in a goroutine.
func (s *SocketServer) Start() error {
	// Create parent directory if needed
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Remove stale socket file if it exists
	os.Remove(s.path)

	// Start listening
	listener, err := net.Listen("unix", s.path)
	if err != nil {
		return err
	}
	s.listener = listener

	// Run accept loop in background
	go s.acceptLoop()

	return nil
}

// Stop closes the socket and cleans up.
// Removes the socket file.
func (s *SocketServer) Stop() error {
	close(s.done)

	if s.listener != nil {
		s.listener.Close()
	}

	// Remove socket file
	os.Remove(s.path)

	return nil
}

// Path returns the socket path.
func (s *SocketServer) Path() string {
	return s.path
}

// acceptLoop accepts connections one at a time.
func (s *SocketServer) acceptLoop() {
	for {
		select {
		case <-s.done:
			return
		default:
			// Continue accepting connections
		}

		conn, err := s.listener.Accept()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-s.done:
				return
			default:
				log.Printf("accept error: %v", err)
				continue
			}
		}

		// Handle connection synchronously (one at a time)
		s.handleConnection(conn)
	}
}

// handleConnection processes a single orchestrator connection.
// Reads JSON events line by line, updates store, broadcasts to hub.
// Sets store connected=true on connect, connected=false on disconnect.
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

// defaultSocketPath returns the default socket path.
// Uses $XDG_RUNTIME_DIR/choo/web.sock if set,
// otherwise ~/.choo/web.sock
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
