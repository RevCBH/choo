package daemon

import "time"

// ContainerJobConfig extends job configuration with container-specific settings.
type ContainerJobConfig struct {
	// JobID is the unique identifier for this job
	JobID string

	// RepoPath is the local repository path (for reference)
	RepoPath string

	// GitURL is the repository URL to clone inside the container
	GitURL string

	// TasksDir is the path to specs/tasks/ relative to repo root
	TasksDir string

	// TargetBranch is the branch PRs will target
	TargetBranch string

	// Unit is the specific unit to run (optional)
	Unit string
}

// ContainerJobState tracks the container lifecycle for a job.
type ContainerJobState struct {
	// ContainerID is the container identifier from the runtime
	ContainerID string

	// ContainerName is the human-readable name: choo-<job-id>
	ContainerName string

	// Status is the current container lifecycle state
	Status ContainerStatus

	// ExitCode is the container exit code when stopped (nil if still running)
	ExitCode *int

	// StartedAt is when the container started running
	StartedAt *time.Time

	// StoppedAt is when the container stopped
	StoppedAt *time.Time

	// Error is the error message if the container failed
	Error string
}

// ContainerStatus represents container lifecycle states.
type ContainerStatus string

const (
	// ContainerStatusCreating indicates the container is being created
	ContainerStatusCreating ContainerStatus = "creating"

	// ContainerStatusRunning indicates the container is executing
	ContainerStatusRunning ContainerStatus = "running"

	// ContainerStatusStopped indicates the container exited normally
	ContainerStatusStopped ContainerStatus = "stopped"

	// ContainerStatusFailed indicates the container failed
	ContainerStatusFailed ContainerStatus = "failed"
)

// IsTerminal returns true if this status is a terminal state.
func (s ContainerStatus) IsTerminal() bool {
	return s == ContainerStatusStopped || s == ContainerStatusFailed
}

// String returns the string representation of the status.
func (s ContainerStatus) String() string {
	return string(s)
}
