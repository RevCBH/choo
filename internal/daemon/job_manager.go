package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/daemon/db"
	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
	"github.com/RevCBH/choo/internal/orchestrator"
	"github.com/RevCBH/choo/internal/web"
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

	// store maintains job state and is always updated regardless of web server status.
	// This allows late-attaching clients to query current state.
	store *web.Store

	// webHub is optional and set when the web server starts.
	// When set, events are broadcast to SSE clients.
	webHub *web.Hub

	// OnJobComplete is called when a job finishes (success, failure, or cancellation).
	// Used to notify external components (e.g., GRPCServer) for cleanup.
	OnJobComplete func(jobID string)
}

var newOrchestrator = func(cfg orchestrator.Config, deps orchestrator.Dependencies) orchestratorRunner {
	return orchestrator.New(cfg, deps)
}

// NewJobManager creates a new job manager.
func NewJobManager(database *db.DB, maxJobs int) *jobManagerImpl {
	return &jobManagerImpl{
		db:       database,
		maxJobs:  maxJobs,
		jobs:     make(map[string]*ManagedJob),
		eventBus: events.NewBus(1000), // Global event bus for daemon-level events
		store:    web.NewStore(),      // Always have a store for state tracking
	}
}

// Store returns the job state store.
// This store is always kept in sync with job events, regardless of web server status.
func (jm *jobManagerImpl) Store() *web.Store {
	return jm.store
}

// SetWebHub configures the SSE broadcast hub.
// When set, job events are broadcast to connected web clients.
// The Store is always updated regardless of whether a Hub is set.
func (jm *jobManagerImpl) SetWebHub(hub *web.Hub) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jm.webHub = hub
}

// Start creates and starts a new job, returning the job ID.
// The caller provides both the context and its cancel func. This allows the caller
// (typically the gRPC layer) to retain control over job cancellation while the
// job manager tracks and executes the job.
func (jm *jobManagerImpl) Start(ctx context.Context, cancel context.CancelFunc, cfg JobConfig) (string, error) {
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

	// 5. Delete any non-active runs for the same branch/repo to avoid UNIQUE constraint violations
	// This cleans up completed/failed/cancelled runs before creating a new one
	if _, err := jm.db.DeleteNonActiveRunByBranch(cfg.FeatureBranch, cfg.RepoPath); err != nil {
		return "", fmt.Errorf("failed to clean up old runs: %w", err)
	}

	// 6. Create run record in SQLite
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

	// 7. Load repository config for dependency creation
	repoCfg, err := config.LoadConfig(cfg.RepoPath)
	if err != nil {
		// Mark job as failed if config cannot be loaded
		if updateErr := jm.db.UpdateRunStatus(jobID, db.RunStatusFailed, ptrString(err.Error())); updateErr != nil {
			log.Printf("failed to update run status: %v", updateErr)
		}
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	// 8. Create Git WorktreeManager
	gitManager := git.NewWorktreeManager(cfg.RepoPath, nil)

	// 9. Create GitHub PRClient (may fail if no token available)
	var ghClient *github.PRClient
	pollInterval, _ := repoCfg.ReviewPollIntervalDuration()
	reviewTimeout, _ := repoCfg.ReviewTimeoutDuration()

	ghClient, err = github.NewPRClient(github.PRClientConfig{
		Owner:         repoCfg.GitHub.Owner,
		Repo:          repoCfg.GitHub.Repo,
		PollInterval:  pollInterval,
		ReviewTimeout: reviewTimeout,
	})
	if err != nil {
		// Log warning but continue - jobs with NoPR: true will work fine
		log.Printf("GitHub client not available (jobs requiring PR creation will fail): %v", err)
		ghClient = nil
	}

	// 10. Create Escalator (Terminal for daemon mode)
	esc := escalate.NewTerminal()

	// 11. Create orchestrator with job-specific config
	// Make TasksDir absolute if it's relative (daemon runs from different cwd)
	tasksDir := cfg.TasksDir
	if !filepath.IsAbs(tasksDir) {
		tasksDir = filepath.Join(cfg.RepoPath, tasksDir)
	}

	orchConfig := orchestrator.Config{
		Parallelism:   cfg.Concurrency,
		TargetBranch:  cfg.TargetBranch,
		FeatureBranch: cfg.FeatureBranch,
		FeatureMode:   cfg.FeatureBranch != "",
		TasksDir:      tasksDir,
		RepoRoot:      cfg.RepoPath,
		DryRun:        cfg.DryRun,
		WorktreeBase:  repoCfg.Worktree.BasePath,
		ClaudeCommand: repoCfg.Claude.Command,
	}

	orchDeps := orchestrator.Dependencies{
		Bus:       jobEventBus,
		Escalator: esc,
		Git:       gitManager,
		GitHub:    ghClient,
	}

	orch := newOrchestrator(orchConfig, orchDeps)

	// 12. Set up event forwarding to Store (always) and Hub (if available)
	// Mark store as connected when job starts
	jm.store.SetConnected(true)

	// Subscribe to job events - always update Store, broadcast to Hub if set
	jobEventBus.Subscribe(func(e events.Event) {
		webEvent := convertToWebEvent(e)
		jm.store.HandleEvent(webEvent)

		// Broadcast to SSE clients if Hub is configured
		jm.mu.RLock()
		hub := jm.webHub
		jm.mu.RUnlock()
		if hub != nil {
			hub.Broadcast(webEvent)
		}
	})

	// 13. Register ManagedJob in map (use the caller-provided cancel func)
	job := &ManagedJob{
		ID:           jobID,
		Orchestrator: orch,
		Cancel:       cancel,
		Events:       jobEventBus,
		StartedAt:    time.Now(),
		Config:       cfg,
	}
	jm.jobs[jobID] = job

	// 14. Start orchestrator in goroutine with cleanup on completion
	go func() {
		defer jm.cleanup(jobID)

		// Run the orchestrator with the caller-provided context
		_, err := orch.Run(ctx)

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

		// Mark store as disconnected when job ends
		jm.store.SetConnected(false)

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
	delete(jm.jobs, jobID)
	callback := jm.OnJobComplete
	jm.mu.Unlock()

	// Notify listener outside of lock to prevent deadlocks
	if callback != nil {
		callback(jobID)
	}
}

// ptrString returns a pointer to the given string.
func ptrString(s string) *string {
	return &s
}

// convertToWebEvent converts an events.Event to a web.Event for the web UI.
func convertToWebEvent(e events.Event) *web.Event {
	var payload json.RawMessage
	if e.Payload != nil {
		// Marshal the payload to JSON
		data, err := json.Marshal(e.Payload)
		if err == nil {
			payload = data
		}
	}

	return &web.Event{
		Type:    string(e.Type),
		Time:    e.Time,
		Unit:    e.Unit,
		Task:    e.Task,
		PR:      e.PR,
		Payload: payload,
		Error:   e.Error,
	}
}
