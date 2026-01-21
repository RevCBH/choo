package scheduler

import "time"

// UnitStatus represents the unit's lifecycle state
type UnitStatus string

const (
	StatusPending    UnitStatus = "pending"
	StatusReady      UnitStatus = "ready"
	StatusInProgress UnitStatus = "in_progress"
	StatusComplete   UnitStatus = "complete" // Terminal: all tasks done and merged to feature branch
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
// Simplified flow: pending -> ready -> in_progress -> complete (merged to feature branch)
var ValidTransitions = map[UnitStatus][]UnitStatus{
	StatusPending:    {StatusReady, StatusBlocked},
	StatusReady:      {StatusInProgress, StatusBlocked},
	StatusInProgress: {StatusComplete, StatusFailed},
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
	return s == StatusInProgress
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
