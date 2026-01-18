package discovery

import "time"

// Unit represents a discovered unit of work with its tasks
type Unit struct {
	// Parsed from directory structure
	ID   string // directory name, e.g., "app-shell"
	Path string // absolute path to unit directory

	// Parsed from IMPLEMENTATION_PLAN.md frontmatter
	DependsOn []string // other unit IDs this unit depends on

	// Orchestrator state (from frontmatter, updated at runtime)
	Status      UnitStatus
	Branch      string     // orch_branch from frontmatter
	Worktree    string     // orch_worktree from frontmatter
	PRNumber    int        // orch_pr_number from frontmatter
	StartedAt   *time.Time // orch_started_at from frontmatter
	CompletedAt *time.Time // orch_completed_at from frontmatter

	// Parsed from task files
	Tasks []*Task
}

// UnitStatus represents the lifecycle state of a unit
type UnitStatus string

const (
	UnitStatusPending    UnitStatus = "pending"
	UnitStatusInProgress UnitStatus = "in_progress"
	UnitStatusPROpen     UnitStatus = "pr_open"
	UnitStatusInReview   UnitStatus = "in_review"
	UnitStatusMerging    UnitStatus = "merging"
	UnitStatusComplete   UnitStatus = "complete"
	UnitStatusFailed     UnitStatus = "failed"
)

// Task represents a single task within a unit
type Task struct {
	// Parsed from frontmatter
	Number       int        // task field from frontmatter
	Status       TaskStatus // status field from frontmatter
	Backpressure string     // backpressure field from frontmatter
	DependsOn    []int      // depends_on field (task numbers within unit)

	// Parsed from file
	FilePath string // relative to unit dir, e.g., "01-nav-types.md"
	Title    string // extracted from first H1 heading
	Content  string // full markdown content (including frontmatter)
}

// TaskStatus represents the lifecycle state of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusComplete   TaskStatus = "complete"
	TaskStatusFailed     TaskStatus = "failed"
)
