package cli

import (
	"strings"
	"testing"

	"github.com/anthropics/choo/internal/discovery"
)

func TestResumeCmd_InheritsRunFlags(t *testing.T) {
	app := New()
	runCmd := NewRunCmd(app)
	resumeCmd := NewResumeCmd(app)

	// Check that resume command has the same flags as run command
	expectedFlags := []string{
		"parallelism",
		"target",
		"dry-run",
		"no-pr",
		"unit",
		"skip-review",
	}

	for _, flagName := range expectedFlags {
		runFlag := runCmd.Flags().Lookup(flagName)
		resumeFlag := resumeCmd.Flags().Lookup(flagName)

		if runFlag == nil {
			t.Fatalf("run command missing expected flag: %s", flagName)
		}
		if resumeFlag == nil {
			t.Fatalf("resume command missing expected flag: %s", flagName)
		}

		// Check that defaults match
		if runFlag.DefValue != resumeFlag.DefValue {
			t.Errorf("Flag %s has different defaults: run=%s, resume=%s",
				flagName, runFlag.DefValue, resumeFlag.DefValue)
		}
	}
}

func TestResumeOrchestrator_NoState(t *testing.T) {
	// Create an empty discovery (no units)
	disc := &discovery.Discovery{
		Units: []*discovery.Unit{},
	}

	err := validateResumeState(disc)
	if err == nil {
		t.Error("Expected error when no previous state, got nil")
	}
	if !strings.Contains(err.Error(), "no previous orchestration state found") {
		t.Errorf("Expected error message about no state, got: %v", err)
	}
}

func TestResumeOrchestrator_AllComplete(t *testing.T) {
	// Create discovery with all complete units
	disc := &discovery.Discovery{
		Units: []*discovery.Unit{
			{
				ID:     "unit1",
				Status: discovery.UnitStatusComplete,
				Tasks: []*discovery.Task{
					{
						Number: 1,
						Status: discovery.TaskStatusComplete,
					},
				},
			},
		},
	}

	err := validateResumeState(disc)
	if err == nil {
		t.Error("Expected error when all units complete, got nil")
	}
	if !strings.Contains(err.Error(), "all units complete") {
		t.Errorf("Expected error message about all units complete, got: %v", err)
	}
}

func TestResumeOrchestrator_PartialState(t *testing.T) {
	// Create discovery with partial state (some incomplete units)
	disc := &discovery.Discovery{
		Units: []*discovery.Unit{
			{
				ID:     "unit1",
				Status: discovery.UnitStatusInProgress,
				Tasks: []*discovery.Task{
					{
						Number: 1,
						Status: discovery.TaskStatusComplete,
					},
					{
						Number: 2,
						Status: discovery.TaskStatusPending,
					},
				},
			},
		},
	}

	err := validateResumeState(disc)
	if err != nil {
		t.Errorf("Expected no error for partial state, got: %v", err)
	}
}

func TestValidateResumeState_Valid(t *testing.T) {
	// Discovery with incomplete units
	disc := &discovery.Discovery{
		Units: []*discovery.Unit{
			{
				ID:     "unit1",
				Status: discovery.UnitStatusInProgress,
				Tasks: []*discovery.Task{
					{
						Number: 1,
						Status: discovery.TaskStatusComplete,
					},
					{
						Number: 2,
						Status: discovery.TaskStatusPending,
					},
				},
			},
		},
	}

	err := validateResumeState(disc)
	if err != nil {
		t.Errorf("Expected no error for valid state with incomplete units, got: %v", err)
	}
}

func TestValidateResumeState_Complete(t *testing.T) {
	// Discovery with all complete units
	disc := &discovery.Discovery{
		Units: []*discovery.Unit{
			{
				ID:     "unit1",
				Status: discovery.UnitStatusComplete,
				Tasks: []*discovery.Task{
					{
						Number: 1,
						Status: discovery.TaskStatusComplete,
					},
				},
			},
		},
	}

	err := validateResumeState(disc)
	if err == nil {
		t.Error("Expected error for fully complete state, got nil")
	}
	if !strings.Contains(err.Error(), "all units complete") {
		t.Errorf("Expected error message about all units complete, got: %v", err)
	}
}

func TestValidateResumeState_Corrupted(t *testing.T) {
	// Discovery with corrupted state (completed tasks after pending tasks)
	disc := &discovery.Discovery{
		Units: []*discovery.Unit{
			{
				ID:     "unit1",
				Status: discovery.UnitStatusInProgress,
				Tasks: []*discovery.Task{
					{
						Number: 1,
						Status: discovery.TaskStatusPending,
					},
					{
						Number: 2,
						Status: discovery.TaskStatusComplete,
					},
				},
			},
		},
	}

	err := validateResumeState(disc)
	if err == nil {
		t.Error("Expected error for corrupted state, got nil")
	}
	if !strings.Contains(err.Error(), "state corrupted") {
		t.Errorf("Expected error message about corrupted state, got: %v", err)
	}
	if !strings.Contains(err.Error(), "unit1") {
		t.Errorf("Expected error message to mention unit1, got: %v", err)
	}
}

func TestValidateResumeState_NilDiscovery(t *testing.T) {
	err := validateResumeState(nil)
	if err == nil {
		t.Error("Expected error for nil discovery, got nil")
	}
	if !strings.Contains(err.Error(), "no previous orchestration state found") {
		t.Errorf("Expected error message about no state, got: %v", err)
	}
}

func TestValidateResumeState_EmptyUnits(t *testing.T) {
	disc := &discovery.Discovery{
		Units: []*discovery.Unit{},
	}

	err := validateResumeState(disc)
	if err == nil {
		t.Error("Expected error for empty units, got nil")
	}
	if !strings.Contains(err.Error(), "no previous orchestration state found") {
		t.Errorf("Expected error message about no state, got: %v", err)
	}
}
