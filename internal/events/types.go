package events

import (
	"fmt"
	"strings"
	"time"
)

// Event represents a single occurrence in the orchestrator lifecycle
type Event struct {
	// Time is when the event occurred (set by bus on emit)
	Time time.Time `json:"time"`

	// Type identifies what happened
	Type EventType `json:"type"`

	// Unit is the unit ID this event relates to (empty for orchestrator events)
	Unit string `json:"unit,omitempty"`

	// Task is the task number within the unit (nil if not task-related)
	Task *int `json:"task,omitempty"`

	// PR is the pull request number (nil if not PR-related)
	PR *int `json:"pr,omitempty"`

	// Payload contains event-specific data (type varies by event)
	Payload any `json:"payload,omitempty"`

	// Error contains error message if this is a failure event
	Error string `json:"error,omitempty"`
}

// EventType is a string constant identifying the event category
type EventType string

// Orchestrator lifecycle events
const (
	OrchStarted   EventType = "orch.started"
	OrchCompleted EventType = "orch.completed"
	OrchFailed    EventType = "orch.failed"

	// Dry-run events (no actual execution)
	OrchDryRunStarted   EventType = "orch.dryrun.started"
	OrchDryRunCompleted EventType = "orch.dryrun.completed"
)

// Unit lifecycle events
const (
	UnitQueued    EventType = "unit.queued"
	UnitStarted   EventType = "unit.started"
	UnitCompleted EventType = "unit.completed"
	UnitFailed    EventType = "unit.failed"
	UnitBlocked   EventType = "unit.blocked"
)

// Task lifecycle events
const (
	TaskStarted        EventType = "task.started"
	TaskClaudeInvoke   EventType = "task.claude.invoke"
	TaskClaudeDone     EventType = "task.claude.done"
	TaskBackpressure   EventType = "task.backpressure"
	TaskValidationOK   EventType = "task.validation.ok"
	TaskValidationFail EventType = "task.validation.fail"
	TaskCommitted      EventType = "task.committed"
	TaskCompleted      EventType = "task.completed"
	TaskRetry          EventType = "task.retry"
	TaskFailed         EventType = "task.failed"
)

// PR lifecycle events
const (
	PRCreated           EventType = "pr.created"
	PRReviewPending     EventType = "pr.review.pending"
	PRReviewInProgress  EventType = "pr.review.in_progress"
	PRReviewApproved    EventType = "pr.review.approved"
	PRFeedbackReceived  EventType = "pr.feedback.received"
	PRFeedbackAddressed EventType = "pr.feedback.addressed"
	PRMergeQueued       EventType = "pr.merge.queued"
	PRConflict          EventType = "pr.conflict"
	PRMerged            EventType = "pr.merged"
	PRFailed            EventType = "pr.failed"
)

// Git operation events
const (
	WorktreeCreated EventType = "worktree.created"
	WorktreeRemoved EventType = "worktree.removed"
	BranchPushed    EventType = "branch.pushed"
)

// Feature lifecycle events
const (
	FeatureStarted        EventType = "feature.started"
	FeatureSpecsGenerated EventType = "feature.specs.generated"
	FeatureSpecsReviewed  EventType = "feature.specs.reviewed"
	FeatureSpecsCommitted EventType = "feature.specs.committed"
	FeatureTasksGenerated EventType = "feature.tasks.generated"
	FeatureUnitsComplete  EventType = "feature.units.complete"
	FeaturePROpened       EventType = "feature.pr.opened"
	FeatureCompleted      EventType = "feature.completed"
	FeatureFailed         EventType = "feature.failed"
)

// PRD events
const (
	PRDDiscovered    EventType = "prd.discovered"
	PRDSelected      EventType = "prd.selected"
	PRDUpdated       EventType = "prd.updated"
	PRDBodyChanged   EventType = "prd.body.changed"
	PRDDriftDetected EventType = "prd.drift.detected"
)

// NewEvent creates an event with the given type and unit
func NewEvent(eventType EventType, unit string) Event {
	return Event{
		Type: eventType,
		Unit: unit,
	}
}

// WithTask returns a copy of the event with the task number set
func (e Event) WithTask(task int) Event {
	e.Task = &task
	return e
}

// WithPR returns a copy of the event with the PR number set
func (e Event) WithPR(pr int) Event {
	e.PR = &pr
	return e
}

// WithPayload returns a copy of the event with the payload set
func (e Event) WithPayload(payload any) Event {
	e.Payload = payload
	return e
}

// WithError returns a copy of the event with the error message set
func (e Event) WithError(err error) Event {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

// IsFailure returns true if this is a failure event type
func (e Event) IsFailure() bool {
	return strings.HasSuffix(string(e.Type), ".failed")
}

// String returns a human-readable representation of the event
func (e Event) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("[%s]", e.Type))

	if e.Unit != "" {
		parts = append(parts, e.Unit)
	}

	if e.Task != nil {
		parts = append(parts, fmt.Sprintf("task=#%d", *e.Task))
	}

	if e.PR != nil {
		parts = append(parts, fmt.Sprintf("pr=#%d", *e.PR))
	}

	return strings.Join(parts, " ")
}
