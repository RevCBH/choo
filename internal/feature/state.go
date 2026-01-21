package feature

import "time"

// FeatureStatus represents the current state of a feature workflow
type FeatureStatus string

const (
	StatusNotStarted      FeatureStatus = "not_started"
	StatusGeneratingSpecs FeatureStatus = "generating_specs"
	StatusReviewingSpecs  FeatureStatus = "reviewing_specs"
	StatusReviewBlocked   FeatureStatus = "review_blocked"
	StatusValidatingSpecs FeatureStatus = "validating_specs"
	StatusGeneratingTasks FeatureStatus = "generating_tasks"
	StatusSpecsCommitted  FeatureStatus = "specs_committed"

	// Additional statuses for backward compatibility with existing workflow code
	StatusPending        FeatureStatus = "pending"
	StatusUpdatingSpecs  FeatureStatus = "updating_specs"
	StatusInProgress     FeatureStatus = "in_progress"
	StatusUnitsComplete  FeatureStatus = "units_complete"
	StatusPROpen         FeatureStatus = "pr_open"
	StatusComplete       FeatureStatus = "complete"
	StatusFailed         FeatureStatus = "failed"
)

// FeatureState holds the complete state of a feature workflow
type FeatureState struct {
	PRDID            string        `yaml:"prd_id"`
	Status           FeatureStatus `yaml:"feature_status"`
	Branch           string        `yaml:"branch"`
	StartedAt        time.Time     `yaml:"started_at"`
	ReviewIterations int           `yaml:"review_iterations"`
	MaxReviewIter    int           `yaml:"max_review_iter"`
	LastFeedback     string        `yaml:"last_feedback,omitempty"`
	SpecCount        int           `yaml:"spec_count,omitempty"`
	TaskCount        int           `yaml:"task_count,omitempty"`
}

// ValidTransitions defines allowed state transitions
var ValidTransitions = map[FeatureStatus][]FeatureStatus{
	StatusNotStarted:      {StatusGeneratingSpecs},
	StatusGeneratingSpecs: {StatusReviewingSpecs, StatusReviewBlocked, StatusFailed},
	StatusReviewingSpecs:  {StatusUpdatingSpecs, StatusValidatingSpecs, StatusReviewBlocked, StatusFailed},
	StatusReviewBlocked:   {StatusReviewingSpecs, StatusValidatingSpecs, StatusGeneratingTasks, StatusFailed},
	StatusValidatingSpecs: {StatusGeneratingTasks, StatusFailed},
	StatusGeneratingTasks: {StatusSpecsCommitted, StatusFailed},

	// Additional transitions for backward compatibility with existing workflow code
	StatusPending:        {StatusGeneratingSpecs, StatusFailed},
	StatusUpdatingSpecs:  {StatusReviewingSpecs, StatusFailed},
	StatusSpecsCommitted: {StatusInProgress, StatusFailed},
	StatusInProgress:     {StatusUnitsComplete, StatusFailed},
	StatusUnitsComplete:  {StatusPROpen, StatusFailed},
	StatusPROpen:         {StatusComplete, StatusFailed},
	StatusComplete:       {},
	StatusFailed:         {},
}

// CanTransition checks if transitioning from one state to another is valid
func CanTransition(from, to FeatureStatus) bool {
	allowedTransitions, exists := ValidTransitions[from]
	if !exists {
		return false
	}

	for _, allowed := range allowedTransitions {
		if allowed == to {
			return true
		}
	}

	return false
}

// IsTerminal returns true if the status is a terminal state
func (s FeatureStatus) IsTerminal() bool {
	// Check if status has no valid outgoing transitions
	transitions, exists := ValidTransitions[s]
	if !exists {
		return false
	}
	return len(transitions) == 0
}

// IsBlocked returns true if the workflow is in a blocked state
func (s FeatureStatus) IsBlocked() bool {
	// Only StatusReviewBlocked is a blocked state
	return s == StatusReviewBlocked
}

// String returns the string representation of the status
func (s FeatureStatus) String() string {
	return string(s)
}

// CanResume returns true if the status supports resume
func (s FeatureStatus) CanResume() bool {
	// Only StatusReviewBlocked supports resume (returns to reviewing)
	return s == StatusReviewBlocked
}
