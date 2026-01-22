package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
	"github.com/RevCBH/choo/internal/daemon/db"
	"github.com/RevCBH/choo/internal/web"
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
	webServer  *web.Server

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
	adapter := newJobManagerAdapter(d.jobManager, d.db)
	grpcImpl := NewGRPCServer(d.db, adapter, "dev", d.Shutdown) // TODO: pass actual version
	apiv1.RegisterDaemonServiceServer(d.grpcServer, grpcImpl)

	// Wire up job completion callback to clean up gRPC tracking
	d.jobManager.OnJobComplete = grpcImpl.UntrackJob

	// 5. Start gRPC server in goroutine
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		if err := d.grpcServer.Serve(d.listener); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// 6. Start web server (using job manager's Store for shared state)
	webCfg := web.Config{
		Addr:       d.cfg.WebAddr,
		SocketPath: d.cfg.WebSocketPath,
	}
	// Use job manager's Store so state is shared regardless of startup order
	webSrv, err := web.NewWithStore(webCfg, d.jobManager.Store())
	if err != nil {
		log.Printf("Warning: failed to create web server: %v", err)
	} else {
		if err := webSrv.Start(); err != nil {
			log.Printf("Warning: failed to start web server: %v", err)
		} else {
			d.webServer = webSrv
			log.Printf("Web server listening on http://localhost%s", d.cfg.WebAddr)

			// Wire up SSE hub for broadcasting events to web clients
			d.jobManager.SetWebHub(webSrv.Hub())
		}
	}

	// 7. Log startup message
	log.Printf("Daemon started on %s (PID: %d)", d.cfg.SocketPath, os.Getpid())

	// 8. Wait for shutdown signal
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
// The order is critical for prompt shutdown:
// 1. Cancel all jobs FIRST (proactive interruption)
// 2. Wait briefly for jobs to start cleanup
// 3. Stop gRPC server (streams complete quickly since jobs are cancelled)
// 4. Stop web server
// 5. Final cleanup
func (d *Daemon) gracefulShutdown(ctx context.Context) error {
	log.Println("Starting graceful shutdown...")

	// 1. IMMEDIATELY cancel all running jobs (proactive interruption)
	// This must happen BEFORE stopping gRPC so that WatchJob streams can complete
	activeJobs := d.jobManager.ActiveCount()
	if activeJobs > 0 {
		log.Printf("Cancelling %d running job(s)...", activeJobs)
		d.jobManager.StopAll()
	}

	// 2. Wait for jobs to finish with a short timeout (10 seconds)
	// Jobs should respond to cancellation quickly
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
		log.Println("All jobs stopped")
	case <-time.After(10 * time.Second):
		remaining := d.jobManager.ActiveCount()
		if remaining > 0 {
			log.Printf("Job shutdown timeout, %d job(s) still running - continuing shutdown", remaining)
		}
	case <-ctx.Done():
		log.Printf("Shutdown context cancelled, %d job(s) may not have stopped cleanly", d.jobManager.ActiveCount())
	}

	// 3. Stop gRPC server (should be quick now that jobs are cancelled)
	if d.grpcServer != nil {
		stopped := make(chan struct{})
		go func() {
			d.grpcServer.GracefulStop()
			close(stopped)
		}()

		// Wait for graceful stop or timeout
		select {
		case <-stopped:
			log.Println("gRPC server stopped")
		case <-time.After(3 * time.Second):
			log.Println("gRPC server graceful stop timed out, forcing stop")
			d.grpcServer.Stop()
		}
	}

	// 4. Stop web server
	if d.webServer != nil {
		log.Println("Stopping web server...")
		webCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := d.webServer.Stop(webCtx); err != nil {
			log.Printf("Error stopping web server: %v", err)
		}
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
