package discovery

import (
	"fmt"
	"time"
)

// Unit represents a discovered unit of work with its tasks
type Unit struct {
	// Parsed from directory structure
	ID   string // directory name, e.g., "app-shell"
	Path string // absolute path to unit directory

	// Parsed from IMPLEMENTATION_PLAN.md frontmatter
	DependsOn []string // other unit IDs this unit depends on
	Provider  string   // provider override from frontmatter (empty = use default)

	// MetadataSource indicates where unit metadata was parsed from
	MetadataSource MetadataSource

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
// Simplified flow: pending -> in_progress -> complete (merged to feature branch)
type UnitStatus string

const (
	UnitStatusPending    UnitStatus = "pending"
	UnitStatusInProgress UnitStatus = "in_progress"
	UnitStatusComplete   UnitStatus = "complete" // Terminal: all tasks done and merged to feature branch
	UnitStatusFailed     UnitStatus = "failed"
	UnitStatusBlocked    UnitStatus = "blocked"
)

// Task represents a single task within a unit
type Task struct {
	// Parsed from frontmatter
	Number       int        // task field from frontmatter
	Status       TaskStatus // status field from frontmatter
	Backpressure string     // backpressure field from frontmatter
	DependsOn    []int      // depends_on field (task numbers within unit)

	// MetadataSource indicates where task metadata was parsed from
	MetadataSource MetadataSource

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

// Discovery represents the complete discovery state for all units
type Discovery struct {
	Units []*Unit
}

// parseUnitStatus converts a string to UnitStatus with validation
func parseUnitStatus(s string) (UnitStatus, error) {
	if s == "" {
		return UnitStatusPending, nil
	}

	status := UnitStatus(s)
	switch status {
	case UnitStatusPending, UnitStatusInProgress, UnitStatusComplete, UnitStatusFailed, UnitStatusBlocked:
		return status, nil
	// Backwards compatibility: treat old PR-related states as in_progress
	// so they will be re-executed with the new local merge workflow
	case "pr_open", "in_review", "merging":
		return UnitStatusInProgress, nil
	default:
		return "", fmt.Errorf("invalid unit status: %q", s)
	}
}

// parseTaskStatus converts a string to TaskStatus with validation
func parseTaskStatus(s string) (TaskStatus, error) {
	if s == "" {
		return TaskStatusPending, nil
	}

	status := TaskStatus(s)
	switch status {
	case TaskStatusPending, TaskStatusInProgress, TaskStatusComplete, TaskStatusFailed:
		return status, nil
	default:
		return "", fmt.Errorf("invalid task status: %q", s)
	}
}
