package daemon

import (
	"context"
	"sync"
	"time"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
	"github.com/RevCBH/choo/internal/daemon/db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCServer implements the DaemonService gRPC interface
type GRPCServer struct {
	apiv1.UnimplementedDaemonServiceServer

	db         *db.DB
	jobManager JobManager
	version    string

	// Shutdown coordination
	mu             sync.RWMutex
	shuttingDown   bool
	shutdownCh     chan struct{}
	activeJobs     map[string]context.CancelFunc
	onShutdown     func() // Callback to signal daemon shutdown
}

// JobManager defines the interface for job lifecycle management
// This abstraction allows the gRPC layer to remain decoupled from
// the actual job execution implementation
type JobManager interface {
	// Start creates and starts a new job, returning the job ID
	Start(ctx context.Context, cfg JobConfig) (string, error)

	// Stop gracefully stops a running job
	Stop(ctx context.Context, jobID string, force bool) error

	// GetJob returns the current state of a job
	GetJob(jobID string) (*JobState, error)

	// ListJobs returns all jobs, optionally filtered by status
	ListJobs(statusFilter []string) ([]*JobSummary, error)

	// Subscribe returns a channel of events for a job starting from sequence
	Subscribe(jobID string, fromSeq int) (<-chan Event, func())

	// ActiveJobCount returns the number of currently running jobs
	ActiveJobCount() int
}

// JobConfig contains configuration for starting a new job
type JobConfig struct {
	RepoPath      string // Absolute path to git repository
	TasksDir      string // Directory containing task definitions
	TargetBranch  string // Base branch for PRs (e.g., "main")
	FeatureBranch string // Optional: for feature mode
	DryRun        bool   // If true, don't create PRs or merge
	Concurrency   int    // Max parallel units (0 = default)
}

// JobState represents the full state of a job
type JobState struct {
	ID            string
	Status        string     // "running", "completed", "failed", "cancelled"
	StartedAt     time.Time
	CompletedAt   *time.Time
	Error         *string
	UnitsTotal    int
	UnitsComplete int
	UnitsFailed   int
	Units         []*UnitState // Detailed unit states (optional, for backwards compat)
}

// UnitState represents the state of a unit within a job
type UnitState struct {
	UnitID        string
	Status        string
	TasksComplete int
	TasksTotal    int
	PRNumber      int
}

// JobSummary is a condensed view of a job for listing
type JobSummary struct {
	JobID         string
	FeatureBranch string
	Status        string
	StartedAt     *time.Time
	UnitsComplete int
	UnitsTotal    int
}

// Event represents a job event for streaming
type Event struct {
	Sequence    int
	EventType   string
	UnitID      string
	PayloadJSON string
	Timestamp   time.Time
}

// NewGRPCServer creates a new gRPC server instance.
// The onShutdown callback is called when a shutdown request is received via gRPC.
func NewGRPCServer(db *db.DB, jm JobManager, version string, onShutdown func()) *GRPCServer {
	return &GRPCServer{
		db:         db,
		jobManager: jm,
		version:    version,
		shutdownCh: make(chan struct{}),
		activeJobs: make(map[string]context.CancelFunc),
		onShutdown: onShutdown,
	}
}

// isShuttingDown returns true if the server is in shutdown mode
func (s *GRPCServer) isShuttingDown() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shuttingDown
}

// setShuttingDown marks the server as shutting down
func (s *GRPCServer) setShuttingDown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shuttingDown = true
	close(s.shutdownCh)
}

// trackJob registers a running job for shutdown coordination
func (s *GRPCServer) trackJob(jobID string, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeJobs[jobID] = cancel
}

// untrackJob removes a job from shutdown tracking
func (s *GRPCServer) untrackJob(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activeJobs, jobID)
}

// StartJob creates and starts a new orchestration job
func (s *GRPCServer) StartJob(ctx context.Context, req *apiv1.StartJobRequest) (*apiv1.StartJobResponse, error) {
	// Check if server is shutting down
	if s.isShuttingDown() {
		return nil, status.Errorf(codes.Unavailable, "daemon is shutting down")
	}

	// Validate required fields
	if req.TasksDir == "" {
		return nil, status.Errorf(codes.InvalidArgument, "tasks_dir is required")
	}
	if req.TargetBranch == "" {
		return nil, status.Errorf(codes.InvalidArgument, "target_branch is required")
	}
	if req.RepoPath == "" {
		return nil, status.Errorf(codes.InvalidArgument, "repo_path is required")
	}

	// Create job context for cancellation
	jobCtx, cancel := context.WithCancel(context.Background())

	jobID, err := s.jobManager.Start(jobCtx, JobConfig{
		RepoPath:      req.RepoPath,
		TasksDir:      req.TasksDir,
		TargetBranch:  req.TargetBranch,
		FeatureBranch: req.FeatureBranch,
		DryRun:        false, // TODO: add to proto
		Concurrency:   int(req.Parallelism),
	})
	if err != nil {
		cancel() // Clean up context
		return nil, status.Errorf(codes.Internal, "failed to start job: %v", err)
	}

	// Track job for shutdown coordination
	s.trackJob(jobID, cancel)

	return &apiv1.StartJobResponse{
		JobId:  jobID,
		Status: "running",
	}, nil
}

// StopJob gracefully stops a running job
func (s *GRPCServer) StopJob(ctx context.Context, req *apiv1.StopJobRequest) (*apiv1.StopJobResponse, error) {
	// Validate required fields
	if req.JobId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "job_id is required")
	}

	// Check if job exists
	job, err := s.jobManager.GetJob(req.JobId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %s", req.JobId)
	}

	// Check if job is already stopped
	if job.Status == "completed" || job.Status == "failed" || job.Status == "cancelled" {
		return nil, status.Errorf(codes.FailedPrecondition, "job already stopped: %s", job.Status)
	}

	// Stop the job
	if err := s.jobManager.Stop(ctx, req.JobId, req.Force); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to stop job: %v", err)
	}

	// Untrack job
	s.untrackJob(req.JobId)

	message := "job stopped gracefully"
	if req.Force {
		message = "job force killed"
	}

	return &apiv1.StopJobResponse{
		Success: true,
		Message: message,
	}, nil
}

// GetJobStatus returns the current status of a job
func (s *GRPCServer) GetJobStatus(ctx context.Context, req *apiv1.GetJobStatusRequest) (*apiv1.GetJobStatusResponse, error) {
	// Validate required fields
	if req.JobId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "job_id is required")
	}

	// Get job state
	job, err := s.jobManager.GetJob(req.JobId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %s", req.JobId)
	}

	return jobStateToProto(job), nil
}

// ListJobs returns all jobs matching the optional status filter
func (s *GRPCServer) ListJobs(ctx context.Context, req *apiv1.ListJobsRequest) (*apiv1.ListJobsResponse, error) {
	jobs, err := s.jobManager.ListJobs(req.StatusFilter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list jobs: %v", err)
	}

	resp := &apiv1.ListJobsResponse{}
	for _, j := range jobs {
		resp.Jobs = append(resp.Jobs, jobSummaryToProto(j))
	}

	return resp, nil
}

// WatchJob streams events for a job until completion or client disconnect.
// Supports resuming from a specific sequence number for reconnection scenarios.
func (s *GRPCServer) WatchJob(req *apiv1.WatchJobRequest, stream apiv1.DaemonService_WatchJobServer) error {
	// Validate required fields
	if req.JobId == "" {
		return status.Errorf(codes.InvalidArgument, "job_id is required")
	}

	// Validate job exists
	job, err := s.jobManager.GetJob(req.JobId)
	if err != nil {
		return status.Errorf(codes.NotFound, "job not found: %s", req.JobId)
	}

	// If job is already complete and no replay requested, return immediately
	if isTerminalStatus(job.Status) && req.FromSequence == 0 {
		return nil
	}

	// Subscribe to job events starting from requested sequence
	events, unsub := s.jobManager.Subscribe(req.JobId, int(req.FromSequence))
	defer unsub()

	// Stream events until channel closes (job complete) or client disconnects
	for {
		select {
		case event, ok := <-events:
			if !ok {
				// Channel closed - job completed
				return nil
			}
			if err := stream.Send(eventToProto(event)); err != nil {
				// Client disconnected or stream error
				return err
			}

		case <-stream.Context().Done():
			// Client disconnected
			return stream.Context().Err()

		case <-s.shutdownCh:
			// Server shutting down
			return status.Errorf(codes.Unavailable, "daemon is shutting down")
		}
	}
}

// isTerminalStatus returns true if the job status indicates completion
func isTerminalStatus(status string) bool {
	switch status {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

// Shutdown initiates graceful daemon shutdown.
// If wait_for_jobs is true, waits for running jobs up to timeout_seconds.
func (s *GRPCServer) Shutdown(ctx context.Context, req *apiv1.ShutdownRequest) (*apiv1.ShutdownResponse, error) {
	s.mu.Lock()

	// Check if already shutting down
	if s.shuttingDown {
		s.mu.Unlock()
		return nil, status.Errorf(codes.FailedPrecondition, "shutdown already in progress")
	}

	// Mark as shutting down
	s.shuttingDown = true
	close(s.shutdownCh)

	// Copy active jobs map for iteration
	activeJobs := make(map[string]context.CancelFunc)
	for k, v := range s.activeJobs {
		activeJobs[k] = v
	}
	s.mu.Unlock()

	jobsStopped := 0

	if req.WaitForJobs && len(activeJobs) > 0 {
		// Wait for jobs with timeout
		timeout := time.Duration(req.TimeoutSeconds) * time.Second
		if timeout == 0 {
			timeout = 30 * time.Second // default timeout
		}

		done := make(chan struct{})
		go func() {
			// Wait for all jobs to complete naturally
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for range ticker.C {
				if s.jobManager.ActiveJobCount() == 0 {
					close(done)
					return
				}
			}
		}()

		select {
		case <-done:
			// All jobs completed gracefully - no action needed
			_ = 0 // explicit no-op to satisfy linter
		case <-time.After(timeout):
			// Timeout - force stop remaining jobs
			for jobID, cancel := range activeJobs {
				cancel()
				jobsStopped++
				_ = s.jobManager.Stop(ctx, jobID, true)
			}
		}
	} else if !req.WaitForJobs {
		// Force stop all jobs immediately
		for jobID, cancel := range activeJobs {
			cancel()
			jobsStopped++
			_ = s.jobManager.Stop(ctx, jobID, true)
		}
	}

	// Signal the daemon to shutdown
	if s.onShutdown != nil {
		go s.onShutdown()
	}

	return &apiv1.ShutdownResponse{
		Success:     true,
		JobsStopped: int32(jobsStopped),
	}, nil
}

// Health returns daemon health status for monitoring and service discovery.
func (s *GRPCServer) Health(ctx context.Context, req *apiv1.HealthRequest) (*apiv1.HealthResponse, error) {
	s.mu.RLock()
	shuttingDown := s.shuttingDown
	s.mu.RUnlock()

	return &apiv1.HealthResponse{
		Healthy:    !shuttingDown,
		ActiveJobs: int32(s.jobManager.ActiveJobCount()),
		Version:    s.version,
	}, nil
}
