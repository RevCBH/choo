package feature

import "fmt"

// FeatureStatus represents the current state of a feature workflow
type FeatureStatus string

const (
	StatusPending         FeatureStatus = "pending"
	StatusGeneratingSpecs FeatureStatus = "generating_specs"
	StatusReviewingSpecs  FeatureStatus = "reviewing_specs"
	StatusUpdatingSpecs   FeatureStatus = "updating_specs"
	StatusReviewBlocked   FeatureStatus = "review_blocked"
	StatusValidatingSpecs FeatureStatus = "validating_specs"
	StatusGeneratingTasks FeatureStatus = "generating_tasks"
	StatusSpecsCommitted  FeatureStatus = "specs_committed"
	StatusInProgress      FeatureStatus = "in_progress"
	StatusUnitsComplete   FeatureStatus = "units_complete"
	StatusPROpen          FeatureStatus = "pr_open"
	StatusComplete        FeatureStatus = "complete"
	StatusFailed          FeatureStatus = "failed"
)

// ValidTransitions defines allowed state transitions
var ValidTransitions = map[FeatureStatus][]FeatureStatus{
	StatusPending:         {StatusGeneratingSpecs, StatusFailed},
	StatusGeneratingSpecs: {StatusReviewingSpecs, StatusFailed},
	StatusReviewingSpecs:  {StatusUpdatingSpecs, StatusReviewBlocked, StatusValidatingSpecs, StatusFailed},
	StatusUpdatingSpecs:   {StatusReviewingSpecs, StatusFailed},
	StatusReviewBlocked:   {StatusReviewingSpecs, StatusFailed},
	StatusValidatingSpecs: {StatusGeneratingTasks, StatusFailed},
	StatusGeneratingTasks: {StatusSpecsCommitted, StatusFailed},
	StatusSpecsCommitted:  {StatusInProgress, StatusFailed},
	StatusInProgress:      {StatusUnitsComplete, StatusFailed},
	StatusUnitsComplete:   {StatusPROpen, StatusFailed},
	StatusPROpen:          {StatusComplete, StatusFailed},
	StatusComplete:        {},
	StatusFailed:          {},
}

// CanTransition checks if a state transition is valid
func CanTransition(from, to FeatureStatus) bool {
	validTargets, exists := ValidTransitions[from]
	if !exists {
		return false
	}

	for _, validTarget := range validTargets {
		if validTarget == to {
			return true
		}
	}

	return false
}

// ParseFeatureStatus converts a string to FeatureStatus with validation
func ParseFeatureStatus(s string) (FeatureStatus, error) {
	// Empty string should parse to StatusPending (default state)
	if s == "" {
		return StatusPending, nil
	}

	status := FeatureStatus(s)

	// Validate that the status is a known status by checking if it exists in ValidTransitions
	_, exists := ValidTransitions[status]
	if !exists {
		return "", fmt.Errorf("invalid feature status: %s", s)
	}

	return status, nil
}

// IsTerminal returns true if the status is a terminal state
func (s FeatureStatus) IsTerminal() bool {
	transitions, exists := ValidTransitions[s]
	if !exists {
		return false
	}

	// Terminal states have no valid outgoing transitions
	return len(transitions) == 0
}

// CanResume returns true if the status supports resume
func (s FeatureStatus) CanResume() bool {
	// Only StatusReviewBlocked supports resume (returns to reviewing)
	return s == StatusReviewBlocked
}
