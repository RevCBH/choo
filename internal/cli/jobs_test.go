package cli

import (
	"testing"
)

func TestJobsCmd_NoFilter(t *testing.T) {
	// Verifies jobs command works without filter
	app := New()
	cmd := NewJobsCmd(app)

	if cmd.Use != "jobs" {
		t.Errorf("Expected Use to be 'jobs', got: %s", cmd.Use)
	}

	if cmd.RunE == nil {
		t.Error("Expected RunE to be set")
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestJobsCmd_StatusFlag(t *testing.T) {
	// Verifies --status flag exists
	app := New()
	cmd := NewJobsCmd(app)

	statusFlag := cmd.Flags().Lookup("status")
	if statusFlag == nil {
		t.Error("Expected --status flag to exist")
	} else {
		if statusFlag.DefValue != "" {
			t.Errorf("Expected --status default to be empty string, got: %s", statusFlag.DefValue)
		}
	}
}

func TestParseStatusFilter_Single(t *testing.T) {
	// Verifies single status parsed correctly
	result := parseStatusFilter("running")

	if len(result) != 1 {
		t.Errorf("Expected 1 status, got %d", len(result))
	}

	if result[0] != "running" {
		t.Errorf("Expected status 'running', got '%s'", result[0])
	}
}

func TestParseStatusFilter_Multiple(t *testing.T) {
	// Verifies "running,completed" parsed as two values
	result := parseStatusFilter("running,completed")

	if len(result) != 2 {
		t.Errorf("Expected 2 statuses, got %d", len(result))
	}

	if result[0] != "running" {
		t.Errorf("Expected first status 'running', got '%s'", result[0])
	}

	if result[1] != "completed" {
		t.Errorf("Expected second status 'completed', got '%s'", result[1])
	}
}

func TestParseStatusFilter_Whitespace(t *testing.T) {
	// Verifies " running , completed " trims whitespace
	result := parseStatusFilter(" running , completed ")

	if len(result) != 2 {
		t.Errorf("Expected 2 statuses, got %d", len(result))
	}

	if result[0] != "running" {
		t.Errorf("Expected first status 'running', got '%s'", result[0])
	}

	if result[1] != "completed" {
		t.Errorf("Expected second status 'completed', got '%s'", result[1])
	}
}

func TestParseStatusFilter_Empty(t *testing.T) {
	// Verifies empty string returns empty slice
	result := parseStatusFilter("")

	if len(result) != 0 {
		t.Errorf("Expected empty slice, got %d elements", len(result))
	}
}
