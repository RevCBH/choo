package daemon

import (
	"context"
	"time"

	"github.com/RevCBH/choo/internal/daemon/db"
)

// jobManagerAdapter adapts jobManagerImpl to implement the JobManager interface
// required by GRPCServer.
type jobManagerAdapter struct {
	impl *jobManagerImpl
	db   *db.DB
}

// newJobManagerAdapter creates a new adapter for the job manager.
func newJobManagerAdapter(impl *jobManagerImpl, database *db.DB) *jobManagerAdapter {
	return &jobManagerAdapter{
		impl: impl,
		db:   database,
	}
}

// Start creates and starts a new job, returning the job ID.
func (a *jobManagerAdapter) Start(ctx context.Context, cancel context.CancelFunc, cfg JobConfig) (string, error) {
	return a.impl.Start(ctx, cancel, cfg)
}

// Stop gracefully stops a running job.
func (a *jobManagerAdapter) Stop(ctx context.Context, jobID string, force bool) error {
	// The underlying implementation doesn't support force, just call Stop
	return a.impl.Stop(jobID)
}

// GetJob returns the current state of a job.
func (a *jobManagerAdapter) GetJob(jobID string) (*JobState, error) {
	// First check in-memory jobs
	job, exists := a.impl.Get(jobID)
	if exists {
		return &JobState{
			ID:            job.ID,
			Status:        "running",
			StartedAt:     job.StartedAt,
			UnitsTotal:    0, // Would need orchestrator integration
			UnitsComplete: 0,
			UnitsFailed:   0,
		}, nil
	}

	// Fall back to database
	run, err := a.db.GetRun(jobID)
	if err != nil {
		return nil, err
	}

	var startedAt time.Time
	if run.StartedAt != nil {
		startedAt = *run.StartedAt
	}

	state := &JobState{
		ID:        run.ID,
		Status:    string(run.Status),
		StartedAt: startedAt,
	}

	if run.CompletedAt != nil {
		state.CompletedAt = run.CompletedAt
	}
	if run.Error != nil {
		state.Error = run.Error
	}

	return state, nil
}

// ListJobs returns all jobs, optionally filtered by status.
func (a *jobManagerAdapter) ListJobs(statusFilter []string) ([]*JobSummary, error) {
	// If status filter is provided, use ListRunsByStatus for each status
	// Otherwise list incomplete runs as a reasonable default
	var runs []*db.Run
	var err error

	if len(statusFilter) > 0 {
		for _, s := range statusFilter {
			statusRuns, err := a.db.ListRunsByStatus(db.RunStatus(s))
			if err != nil {
				return nil, err
			}
			runs = append(runs, statusRuns...)
		}
	} else {
		// List incomplete runs by default (running + pending)
		runs, err = a.db.ListIncompleteRuns()
		if err != nil {
			return nil, err
		}
	}

	var summaries []*JobSummary
	for _, run := range runs {
		summary := &JobSummary{
			JobID:         run.ID,
			FeatureBranch: run.FeatureBranch,
			Status:        string(run.Status),
			StartedAt:     run.StartedAt,
			UnitsComplete: 0, // Would need unit tracking
			UnitsTotal:    0,
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// Subscribe returns a channel of events for a job starting from sequence.
func (a *jobManagerAdapter) Subscribe(jobID string, fromSeq int) (<-chan Event, func()) {
	// Use SubscribeFrom if available, otherwise fall back to Subscribe
	eventCh, unsub, err := a.impl.SubscribeFrom(jobID, fromSeq)
	if err != nil {
		// Return closed channel on error
		ch := make(chan Event)
		close(ch)
		return ch, func() {}
	}

	// Convert events.Event to Event
	// Note: events.Event doesn't have a Sequence field, so we track it ourselves
	outCh := make(chan Event, 100)
	go func() {
		defer close(outCh)
		seq := fromSeq
		for e := range eventCh {
			outCh <- Event{
				Sequence:  seq,
				EventType: string(e.Type),
				UnitID:    e.Unit,
				Timestamp: e.Time,
			}
			seq++
		}
	}()

	return outCh, unsub
}

// ActiveJobCount returns the number of currently running jobs.
func (a *jobManagerAdapter) ActiveJobCount() int {
	return a.impl.ActiveCount()
}
