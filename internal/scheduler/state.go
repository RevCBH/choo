package scheduler

import "time"

// UnitStatus represents the unit's lifecycle state
type UnitStatus string

const (
	StatusPending    UnitStatus = "pending"
	StatusReady      UnitStatus = "ready"
	StatusInProgress UnitStatus = "in_progress"
	StatusPROpen     UnitStatus = "pr_open"
	StatusInReview   UnitStatus = "in_review"
	StatusMerging    UnitStatus = "merging"
	StatusComplete   UnitStatus = "complete"
	StatusFailed     UnitStatus = "failed"
	StatusBlocked    UnitStatus = "blocked"
)

// UnitState tracks the current state of a unit
type UnitState struct {
	UnitID      string
	Status      UnitStatus
	BlockedBy   []string
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       error
}

// ValidTransitions defines allowed state transitions
var ValidTransitions = map[UnitStatus][]UnitStatus{
	StatusPending:    {StatusReady, StatusBlocked},
	StatusReady:      {StatusInProgress, StatusBlocked},
	StatusInProgress: {StatusPROpen, StatusComplete, StatusFailed},
	StatusPROpen:     {StatusInReview, StatusComplete, StatusFailed},
	StatusInReview:   {StatusMerging, StatusPROpen, StatusFailed},
	StatusMerging:    {StatusComplete, StatusFailed},
	StatusComplete:   {},
	StatusFailed:     {},
	StatusBlocked:    {},
}

// IsTerminal returns true if the status is a final state
func (s UnitStatus) IsTerminal() bool {
	return s == StatusComplete || s == StatusFailed || s == StatusBlocked
}

// IsActive returns true if the unit is consuming a parallelism slot
func (s UnitStatus) IsActive() bool {
	return s == StatusInProgress || s == StatusPROpen || s == StatusInReview || s == StatusMerging
}

// CanTransition checks if a transition from -> to is valid
func CanTransition(from, to UnitStatus) bool {
	validTargets, exists := ValidTransitions[from]
	if !exists {
		return false
	}
	for _, target := range validTargets {
		if target == to {
			return true
		}
	}
	return false
}

// NewUnitState creates initial state for a unit (status = pending)
func NewUnitState(unitID string) *UnitState {
	return &UnitState{
		UnitID: unitID,
		Status: StatusPending,
	}
}
