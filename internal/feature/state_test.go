package feature

import "testing"

func TestFeatureStatus_String(t *testing.T) {
	if StatusNotStarted.String() != "not_started" {
		t.Errorf("StatusNotStarted.String() = %v, want 'not_started'", StatusNotStarted.String())
	}
}

func TestCanTransition_Valid(t *testing.T) {
	if !CanTransition(StatusNotStarted, StatusGeneratingSpecs) {
		t.Error("CanTransition(StatusNotStarted, StatusGeneratingSpecs) = false, want true")
	}
}

func TestCanTransition_Invalid(t *testing.T) {
	if CanTransition(StatusNotStarted, StatusSpecsCommitted) {
		t.Error("CanTransition(StatusNotStarted, StatusSpecsCommitted) = true, want false")
	}
}

func TestFeatureStatus_IsTerminal(t *testing.T) {
	if !StatusSpecsCommitted.IsTerminal() {
		t.Error("StatusSpecsCommitted.IsTerminal() = false, want true")
	}
	if StatusNotStarted.IsTerminal() {
		t.Error("StatusNotStarted.IsTerminal() = true, want false")
	}
}

func TestFeatureStatus_IsBlocked(t *testing.T) {
	if !StatusReviewBlocked.IsBlocked() {
		t.Error("StatusReviewBlocked.IsBlocked() = false, want true")
	}
	if StatusNotStarted.IsBlocked() {
		t.Error("StatusNotStarted.IsBlocked() = true, want false")
	}
}
