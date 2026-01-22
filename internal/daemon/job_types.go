package daemon

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/orchestrator"
)

// ManagedJob represents an active job with its orchestrator.
// It tracks the running orchestrator instance and provides
// cancellation and event access.
type ManagedJob struct {
	ID           string
	Orchestrator orchestratorRunner
	Cancel       context.CancelFunc
	Events       *events.Bus
	StartedAt    time.Time
	Config       JobConfig
}

type orchestratorRunner interface {
	Run(ctx context.Context) (*orchestrator.Result, error)
}

// Validate checks the JobConfig for required fields.
func (c *JobConfig) Validate() error {
	// RepoPath must be non-empty and absolute
	if c.RepoPath == "" {
		return fmt.Errorf("RepoPath is required")
	}
	if !filepath.IsAbs(c.RepoPath) {
		return fmt.Errorf("RepoPath must be absolute, got %s", c.RepoPath)
	}

	// TasksDir must be non-empty
	if c.TasksDir == "" {
		return fmt.Errorf("TasksDir is required")
	}

	// TargetBranch must be non-empty
	if c.TargetBranch == "" {
		return fmt.Errorf("TargetBranch is required")
	}

	return nil
}

// IsRunning returns true if the job is still executing.
func (j *ManagedJob) IsRunning() bool {
	// Check if orchestrator is still running
	// This may involve checking context cancellation
	if j.Orchestrator == nil {
		return false
	}
	// Since we don't have direct access to orchestrator's context,
	// we can use a simple heuristic: if Cancel is not nil, job is considered running
	// The actual running state will be managed by the JobManager
	return j.Cancel != nil
}

// State returns a snapshot of the job's current state.
func (j *ManagedJob) State() JobState {
	// Build state from orchestrator status
	state := JobState{
		ID:        j.ID,
		StartedAt: j.StartedAt,
		Status:    "running",
	}

	// If the job is not running, update the status
	if !j.IsRunning() {
		state.Status = "completed"
	}

	return state
}
