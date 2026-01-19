package scheduler

import "testing"

func TestUnitStatus_IsTerminal(t *testing.T) {
	terminalStatuses := []UnitStatus{
		StatusComplete,
		StatusFailed,
		StatusBlocked,
	}

	for _, status := range terminalStatuses {
		if !status.IsTerminal() {
			t.Errorf("Expected %s to be terminal", status)
		}
	}
}

func TestUnitStatus_IsTerminal_NonTerminal(t *testing.T) {
	nonTerminalStatuses := []UnitStatus{
		StatusPending,
		StatusReady,
		StatusInProgress,
		StatusPROpen,
		StatusInReview,
		StatusMerging,
	}

	for _, status := range nonTerminalStatuses {
		if status.IsTerminal() {
			t.Errorf("Expected %s to not be terminal", status)
		}
	}
}

func TestUnitStatus_IsActive(t *testing.T) {
	activeStatuses := []UnitStatus{
		StatusInProgress,
		StatusPROpen,
		StatusInReview,
		StatusMerging,
	}

	for _, status := range activeStatuses {
		if !status.IsActive() {
			t.Errorf("Expected %s to be active", status)
		}
	}
}

func TestUnitStatus_IsActive_Inactive(t *testing.T) {
	inactiveStatuses := []UnitStatus{
		StatusPending,
		StatusReady,
		StatusComplete,
		StatusFailed,
		StatusBlocked,
	}

	for _, status := range inactiveStatuses {
		if status.IsActive() {
			t.Errorf("Expected %s to not be active", status)
		}
	}
}

func TestCanTransition_Valid(t *testing.T) {
	for from, validTargets := range ValidTransitions {
		for _, to := range validTargets {
			if !CanTransition(from, to) {
				t.Errorf("Expected transition from %s to %s to be valid", from, to)
			}
		}
	}
}

func TestCanTransition_Invalid(t *testing.T) {
	if CanTransition(StatusPending, StatusComplete) {
		t.Error("Expected transition from pending to complete to be invalid")
	}
}

func TestCanTransition_Terminal(t *testing.T) {
	terminalStatuses := []UnitStatus{
		StatusComplete,
		StatusFailed,
		StatusBlocked,
	}

	allStatuses := []UnitStatus{
		StatusPending,
		StatusReady,
		StatusInProgress,
		StatusPROpen,
		StatusInReview,
		StatusMerging,
		StatusComplete,
		StatusFailed,
		StatusBlocked,
	}

	for _, from := range terminalStatuses {
		for _, to := range allStatuses {
			if CanTransition(from, to) {
				t.Errorf("Expected transition from terminal status %s to %s to be invalid", from, to)
			}
		}
	}
}

func TestNewUnitState(t *testing.T) {
	unitID := "test-unit"
	state := NewUnitState(unitID)

	if state.UnitID != unitID {
		t.Errorf("Expected UnitID to be %s, got %s", unitID, state.UnitID)
	}

	if state.Status != StatusPending {
		t.Errorf("Expected Status to be %s, got %s", StatusPending, state.Status)
	}

	if state.StartedAt != nil {
		t.Error("Expected StartedAt to be nil")
	}

	if state.CompletedAt != nil {
		t.Error("Expected CompletedAt to be nil")
	}

	if state.Error != nil {
		t.Error("Expected Error to be nil")
	}

	if state.BlockedBy != nil {
		t.Error("Expected BlockedBy to be nil")
	}
}
