package daemon

import (
	"context"
	"sync"
	"time"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
	"github.com/RevCBH/choo/internal/daemon/db"
)

// GRPCServer implements the DaemonService gRPC interface
type GRPCServer struct {
	apiv1.UnimplementedDaemonServiceServer

	db         *db.DB
	jobManager JobManager
	version    string

	// Shutdown coordination
	mu           sync.RWMutex
	shuttingDown bool
	shutdownCh   chan struct{}
	activeJobs   map[string]context.CancelFunc
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
	TasksDir      string
	TargetBranch  string
	FeatureBranch string
	Parallelism   int
	RepoPath      string
}

// JobState represents the full state of a job
type JobState struct {
	ID          string
	Status      string
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       *string
	Units       []*UnitState
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

// NewGRPCServer creates a new gRPC server instance
func NewGRPCServer(db *db.DB, jm JobManager, version string) *GRPCServer {
	return &GRPCServer{
		db:         db,
		jobManager: jm,
		version:    version,
		shutdownCh: make(chan struct{}),
		activeJobs: make(map[string]context.CancelFunc),
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
