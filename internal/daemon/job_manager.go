package daemon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/RevCBH/choo/internal/daemon/db"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/orchestrator"
	"github.com/oklog/ulid/v2"
)

// jobManagerImpl is the concrete implementation of the JobManager interface.
// It tracks and coordinates active jobs.
type jobManagerImpl struct {
	db      *db.DB
	maxJobs int

	mu   sync.RWMutex
	jobs map[string]*ManagedJob

	eventBus *events.Bus // Global daemon event bus
}

// NewJobManager creates a new job manager.
func NewJobManager(database *db.DB, maxJobs int) *jobManagerImpl {
	return &jobManagerImpl{
		db:       database,
		maxJobs:  maxJobs,
		jobs:     make(map[string]*ManagedJob),
		eventBus: events.NewBus(1000), // Global event bus for daemon-level events
	}
}

// Start creates and starts a new job, returning the job ID.
func (jm *jobManagerImpl) Start(ctx context.Context, cfg JobConfig) (string, error) {
	// 1. Validate configuration
	if err := cfg.Validate(); err != nil {
		return "", fmt.Errorf("invalid job config: %w", err)
	}

	// 2. Lock for write
	jm.mu.Lock()
	defer jm.mu.Unlock()

	// 3. Enforce capacity limit
	if len(jm.jobs) >= jm.maxJobs {
		return "", fmt.Errorf("max jobs (%d) reached, cannot start new job", jm.maxJobs)
	}

	// 4. Generate unique job ID using ULID
	jobID := ulid.Make().String()

	// 5. Create run record in SQLite
	run := &db.Run{
		ID:            jobID,
		FeatureBranch: cfg.FeatureBranch,
		RepoPath:      cfg.RepoPath,
		TargetBranch:  cfg.TargetBranch,
		TasksDir:      cfg.TasksDir,
		Parallelism:   cfg.Concurrency,
		Status:        db.RunStatusRunning,
	}

	if err := jm.db.CreateRun(run); err != nil {
		return "", fmt.Errorf("failed to create run record: %w", err)
	}

	// 6. Create isolated event bus for this job
	jobEventBus := events.NewBus(1000)

	// 7. Create orchestrator with job-specific config
	orchConfig := orchestrator.Config{
		Parallelism:  cfg.Concurrency,
		TargetBranch: cfg.TargetBranch,
		TasksDir:     cfg.TasksDir,
		RepoRoot:     cfg.RepoPath,
		DryRun:       cfg.DryRun,
	}

	orchDeps := orchestrator.Dependencies{
		Bus: jobEventBus,
		// Note: Other dependencies (Git, GitHub, Escalator) would be injected here
		// in production, but are nil for now as they're not required for the spec
	}

	orch := orchestrator.New(orchConfig, orchDeps)

	// Create cancellable context for the job
	jobCtx, cancel := context.WithCancel(ctx)

	// 8. Register ManagedJob in map
	job := &ManagedJob{
		ID:           jobID,
		Orchestrator: orch,
		Cancel:       cancel,
		Events:       jobEventBus,
		StartedAt:    time.Now(),
		Config:       cfg,
	}
	jm.jobs[jobID] = job

	// 9. Start orchestrator in goroutine with cleanup on completion
	go func() {
		defer jm.cleanup(jobID)

		// Run the orchestrator
		_, err := orch.Run(jobCtx)

		// Update database status based on result
		var status db.RunStatus
		var errMsg *string
		if err != nil {
			if err == context.Canceled {
				status = db.RunStatusCancelled
			} else {
				status = db.RunStatusFailed
				errStr := err.Error()
				errMsg = &errStr
			}
		} else {
			status = db.RunStatusCompleted
		}

		if updateErr := jm.db.UpdateRunStatus(jobID, status, errMsg); updateErr != nil {
			// Log error but don't fail - job has already completed
			fmt.Printf("failed to update run status: %v\n", updateErr)
		}

		// Close the job's event bus
		jobEventBus.Close()
	}()

	// 10. Return job ID
	return jobID, nil
}

// Stop cancels a running job.
func (jm *jobManagerImpl) Stop(jobID string) error {
	jm.mu.RLock()
	job, exists := jm.jobs[jobID]
	jm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Call Cancel function
	if job.Cancel != nil {
		job.Cancel()
	}

	// Update status in database
	return jm.db.UpdateRunStatus(jobID, db.RunStatusCancelled, nil)
}

// StopAll cancels all running jobs.
func (jm *jobManagerImpl) StopAll() {
	jm.mu.RLock()
	jobs := make([]*ManagedJob, 0, len(jm.jobs))
	for _, job := range jm.jobs {
		jobs = append(jobs, job)
	}
	jm.mu.RUnlock()

	// Cancel all jobs
	for _, job := range jobs {
		if job.Cancel != nil {
			job.Cancel()
		}
	}
}

// Get returns a managed job by ID.
func (jm *jobManagerImpl) Get(jobID string) (*ManagedJob, bool) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	job, exists := jm.jobs[jobID]
	return job, exists
}

// List returns all active job IDs.
func (jm *jobManagerImpl) List() []string {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	ids := make([]string, 0, len(jm.jobs))
	for id := range jm.jobs {
		ids = append(ids, id)
	}
	return ids
}

// ActiveCount returns the number of currently running jobs.
func (jm *jobManagerImpl) ActiveCount() int {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	return len(jm.jobs)
}

// cleanup removes a completed job from tracking.
// Called when orchestrator goroutine exits.
func (jm *jobManagerImpl) cleanup(jobID string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	delete(jm.jobs, jobID)
}
