package daemon

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
	"github.com/RevCBH/choo/internal/daemon/db"
	"google.golang.org/grpc"
)

const (
	// DefaultSocketPath is the default location for the daemon socket
	DefaultSocketPath = "/tmp/charlotte.sock"
)

// Server manages the gRPC server and Unix socket
type Server struct {
	socketPath string
	grpcServer *grpc.Server
	grpcImpl   *GRPCServer
	listener   net.Listener

	mu      sync.Mutex
	running bool
}

// ServerConfig holds configuration for the daemon server
type ServerConfig struct {
	// SocketPath overrides the default socket path
	SocketPath string

	// Version string reported in health checks
	Version string
}

// NewServer creates a new daemon server
func NewServer(cfg ServerConfig, database *db.DB, jobMgr JobManager) *Server {
	socketPath := cfg.SocketPath
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}

	grpcImpl := NewGRPCServer(database, jobMgr, cfg.Version, nil)
	grpcServer := grpc.NewServer()
	apiv1.RegisterDaemonServiceServer(grpcServer, grpcImpl)

	return &Server{
		socketPath: socketPath,
		grpcServer: grpcServer,
		grpcImpl:   grpcImpl,
	}
}

// SocketPath returns the socket path this server listens on
func (s *Server) SocketPath() string {
	return s.socketPath
}

// Start begins listening on the Unix socket and serving gRPC requests.
// This method blocks until Stop is called or an error occurs.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.mu.Unlock()

	// Remove stale socket file if it exists
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove stale socket: %w", err)
	}

	// Ensure socket directory exists
	socketDir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	s.listener = listener

	// Set socket permissions (only owner can connect)
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	// Mark as running now that we're ready to serve
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	// Serve gRPC requests (blocks until stopped)
	return s.grpcServer.Serve(listener)
}

// Stop gracefully stops the server.
// If ctx has a deadline, it will force stop after the deadline.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// Create channel to signal graceful stop completion
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	// Wait for graceful stop or context deadline
	select {
	case <-done:
		// Graceful stop completed
	case <-ctx.Done():
		// Force stop
		s.grpcServer.Stop()
	}

	// Clean up socket file
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove socket file: %w", err)
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	return nil
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// GRPCServer returns the underlying GRPCServer for testing
func (s *Server) GRPCServer() *GRPCServer {
	return s.grpcImpl
}
