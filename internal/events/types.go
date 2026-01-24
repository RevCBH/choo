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
	UnitQueued            EventType = "unit.queued"
	UnitStarted           EventType = "unit.started"
	UnitCompleted         EventType = "unit.completed" // Terminal: tasks done and merged to feature branch
	UnitMerged            EventType = "unit.merged"    // Emitted when unit is merged to feature branch (same as completed)
	UnitFailed            EventType = "unit.failed"
	UnitBlocked           EventType = "unit.blocked"
	UnitDependencyMissing EventType = "unit.dependency.missing"
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

// PR lifecycle events (deprecated: local merge workflow replaces PRs for unit branches)
const (
	PRCreated           EventType = "pr.created"            // Deprecated
	PRReviewPending     EventType = "pr.review.pending"     // Deprecated
	PRReviewInProgress  EventType = "pr.review.in_progress" // Deprecated
	PRReviewApproved    EventType = "pr.review.approved"    // Deprecated
	PRFeedbackReceived  EventType = "pr.feedback.received"  // Deprecated
	PRFeedbackAddressed EventType = "pr.feedback.addressed" // Deprecated
	PRMergeQueued       EventType = "pr.merge.queued"       // Deprecated
	PRConflict          EventType = "pr.conflict"           // Deprecated
	PRMerged            EventType = "pr.merged"             // Deprecated
	PRFailed            EventType = "pr.failed"             // Deprecated
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

// Code review events (advisory, never block merge)
const (
	// CodeReviewStarted is emitted when code review begins for a unit
	// Payload: unit (string)
	CodeReviewStarted EventType = "codereview.started"

	// CodeReviewPassed is emitted when review finds no issues
	// Payload: summary (string)
	CodeReviewPassed EventType = "codereview.passed"

	// CodeReviewIssuesFound is emitted when review discovers issues
	// Payload: count (int), issues ([]ReviewIssue)
	CodeReviewIssuesFound EventType = "codereview.issues_found"

	// CodeReviewFixAttempt is emitted when a fix iteration begins
	// Payload: iteration (int), max_iterations (int)
	CodeReviewFixAttempt EventType = "codereview.fix_attempt"

	// CodeReviewFixApplied is emitted when fix changes are successfully committed
	// Payload: iteration (int)
	CodeReviewFixApplied EventType = "codereview.fix_applied"

	// CodeReviewFailed is emitted when review fails to run (non-blocking)
	// Payload: error (string)
	CodeReviewFailed EventType = "codereview.failed"
)

// PRD events
const (
	PRDDiscovered    EventType = "prd.discovered"
	PRDSelected      EventType = "prd.selected"
	PRDUpdated       EventType = "prd.updated"
	PRDBodyChanged   EventType = "prd.body.changed"
	PRDDriftDetected EventType = "prd.drift.detected"
)

// Spec validation/normalization events
const (
	SpecValidationStarted    EventType = "spec.validation.started"
	SpecValidationCompleted  EventType = "spec.validation.completed"
	SpecValidationFailed     EventType = "spec.validation.failed"
	SpecNormalizationApplied EventType = "spec.normalization.applied"
	SpecRepairApplied        EventType = "spec.repair.applied"
	SpecRepairFailed         EventType = "spec.repair.failed"
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
