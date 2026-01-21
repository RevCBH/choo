package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/RevCBH/choo/internal/daemon/db"
	"google.golang.org/grpc"
)

// Daemon is the main daemon process coordinator.
type Daemon struct {
	cfg        *Config
	db         *db.DB
	jobManager *jobManagerImpl
	grpcServer *grpc.Server
	listener   net.Listener
	pidFile    *PIDFile

	shutdownCh chan struct{}
	wg         sync.WaitGroup
}

// New creates a new daemon instance.
func New(cfg *Config) (*Daemon, error) {
	// 1. Validate config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// 2. Ensure directories exist
	if err := cfg.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	// 3. Open database connection
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 4. Create JobManager
	jobManager := NewJobManager(database, cfg.MaxJobs)

	// 5. Create PIDFile manager
	pidFile := NewPIDFile(cfg.PIDFile)

	// 6. Return initialized Daemon
	return &Daemon{
		cfg:        cfg,
		db:         database,
		jobManager: jobManager,
		pidFile:    pidFile,
		shutdownCh: make(chan struct{}),
	}, nil
}

// Start begins the daemon, resumes jobs, and listens for connections.
// Blocks until shutdown is triggered.
func (d *Daemon) Start(ctx context.Context) error {
	// 1. Acquire PID file (fails if daemon already running)
	if err := d.pidFile.Acquire(); err != nil {
		return fmt.Errorf("failed to acquire PID file: %w", err)
	}

	// 2. Resume any interrupted jobs
	log.Println("Resuming interrupted jobs...")
	results := d.jobManager.ResumeJobs(ctx)
	for _, result := range results {
		if result.Success {
			log.Printf("Resumed job %s: %s", result.JobID, result.Reason)
		} else if !result.Skipped && result.Error != nil {
			log.Printf("Failed to resume job %s: %v", result.JobID, result.Error)
		}
	}

	// 3. Create Unix socket listener (remove stale socket first)
	listener, err := d.setupSocket()
	if err != nil {
		if releaseErr := d.pidFile.Release(); releaseErr != nil {
			log.Printf("Error releasing PID file during cleanup: %v", releaseErr)
		}
		return fmt.Errorf("failed to setup socket: %w", err)
	}
	d.listener = listener

	// 4. Create and register gRPC server
	d.grpcServer = grpc.NewServer()
	// Note: GRPCServer implementation is handled by DAEMON-GRPC spec
	// For now, we create a basic server without service registration
	// The full gRPC service will be wired up in a later task

	// 5. Start gRPC server in goroutine
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		if err := d.grpcServer.Serve(d.listener); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// 6. Log startup message
	log.Printf("Daemon started on %s (PID: %d)", d.cfg.SocketPath, os.Getpid())

	// 7. Wait for shutdown signal
	select {
	case <-ctx.Done():
		log.Println("Received context cancellation")
	case <-d.shutdownCh:
		log.Println("Received shutdown signal")
	}

	// 8. Run graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return d.gracefulShutdown(shutdownCtx)
}

// Shutdown initiates graceful shutdown.
func (d *Daemon) Shutdown() {
	// Close shutdown channel to signal Start() to exit
	select {
	case <-d.shutdownCh:
		// Already closed
	default:
		close(d.shutdownCh)
	}
}

// gracefulShutdown performs ordered shutdown of daemon components.
func (d *Daemon) gracefulShutdown(ctx context.Context) error {
	log.Println("Starting graceful shutdown...")

	// 1. Stop accepting new gRPC connections
	if d.grpcServer != nil {
		stopped := make(chan struct{})
		go func() {
			d.grpcServer.GracefulStop()
			close(stopped)
		}()

		// Wait for graceful stop or timeout
		select {
		case <-stopped:
			log.Println("gRPC server stopped gracefully")
		case <-time.After(5 * time.Second):
			log.Println("gRPC server graceful stop timed out, forcing stop")
			d.grpcServer.Stop()
		}
	}

	// 2. Signal all jobs to stop at safe points
	log.Println("Stopping all running jobs...")
	d.jobManager.StopAll()

	// 3. Wait for jobs with timeout (30s)
	jobsDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			if d.jobManager.ActiveCount() == 0 {
				close(jobsDone)
				return
			}
			select {
			case <-ticker.C:
				// Continue waiting
			case <-ctx.Done():
				return
			}
		}
	}()

	select {
	case <-jobsDone:
		log.Println("All jobs stopped gracefully")
	case <-ctx.Done():
		// 4. Force kill jobs if timeout exceeded
		log.Printf("Shutdown timeout exceeded, %d jobs still running", d.jobManager.ActiveCount())
		// Jobs are already cancelled via StopAll, so we just continue
	}

	// Wait for gRPC goroutine to finish
	d.wg.Wait()

	// 5. Close database connection
	if d.db != nil {
		if err := d.db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}

	// 6. Release PID file
	if d.pidFile != nil {
		if err := d.pidFile.Release(); err != nil {
			log.Printf("Error releasing PID file: %v", err)
		}
	}

	// 7. Remove socket file
	if d.cfg != nil && d.cfg.SocketPath != "" {
		if err := os.Remove(d.cfg.SocketPath); err != nil && !os.IsNotExist(err) {
			log.Printf("Error removing socket file: %v", err)
		}
	}

	log.Println("Daemon shutdown complete")
	return nil
}

// setupSocket creates the Unix domain socket listener.
func (d *Daemon) setupSocket() (net.Listener, error) {
	// Remove stale socket file
	if err := os.Remove(d.cfg.SocketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove stale socket: %w", err)
	}

	// Create Unix listener
	listener, err := net.Listen("unix", d.cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	// Set permissions (0600 - user only)
	if err := os.Chmod(d.cfg.SocketPath, 0600); err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to set socket permissions: %w", err)
	}

	return listener, nil
}
