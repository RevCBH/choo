package client

import "time"

// JobConfig contains parameters for starting a new job
type JobConfig struct {
	TasksDir      string // Directory containing task definitions
	TargetBranch  string // Base branch for PRs
	FeatureBranch string // Branch name for work
	Parallelism   int    // Max concurrent units
	RepoPath      string // Repository root path
}

// JobSummary provides high-level job information for listings
type JobSummary struct {
	JobID         string
	FeatureBranch string
	Status        string
	StartedAt     time.Time
	UnitsComplete int
	UnitsTotal    int
}

// JobStatus provides detailed job state including all units
type JobStatus struct {
	JobID       string
	Status      string
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       string
	Units       []UnitStatus
}

// UnitStatus tracks individual unit progress
type UnitStatus struct {
	UnitID        string
	Status        string
	TasksComplete int
	TasksTotal    int
	PRNumber      int
}

// HealthInfo contains daemon health check response
type HealthInfo struct {
	Healthy    bool
	ActiveJobs int
	Version    string
}
