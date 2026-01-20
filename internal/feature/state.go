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
	StatusGeneratingSpecs: {StatusReviewingSpecs, StatusReviewBlocked},
	StatusReviewingSpecs:  {StatusValidatingSpecs, StatusReviewBlocked},
	StatusReviewBlocked:   {StatusReviewingSpecs, StatusValidatingSpecs, StatusGeneratingTasks},
	StatusValidatingSpecs: {StatusGeneratingTasks},
	StatusGeneratingTasks: {StatusSpecsCommitted},
}

// CanTransition checks if transitioning from one state to another is valid
func CanTransition(from, to FeatureStatus) bool {
	allowedStates, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, allowed := range allowedStates {
		if allowed == to {
			return true
		}
	}
	return false
}

// IsTerminal returns true if the status is a terminal state
func (s FeatureStatus) IsTerminal() bool {
	return s == StatusSpecsCommitted
}

// IsBlocked returns true if the workflow is in a blocked state
func (s FeatureStatus) IsBlocked() bool {
	return s == StatusReviewBlocked
}

// String returns the string representation of the status
func (s FeatureStatus) String() string {
	return string(s)
}
