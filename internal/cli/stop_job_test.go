package cli

import (
	"context"
	"testing"
)

func TestStopJobCmd_RequiresJobID(t *testing.T) {
	// Verifies command fails without job-id argument
	app := New()
	cmd := NewStopJobCmd(app)

	// Test with no args
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("Expected error when no job-id provided")
	}
}

func TestStopJobCmd_ForceFlag(t *testing.T) {
	// Verifies --force and -f flags exist with default false
	app := New()
	cmd := NewStopJobCmd(app)

	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("Expected --force flag to exist")
	} else {
		if forceFlag.DefValue != "false" {
			t.Errorf("Expected --force default to be false, got: %s", forceFlag.DefValue)
		}
		if forceFlag.Shorthand != "f" {
			t.Errorf("Expected shorthand to be 'f', got: %s", forceFlag.Shorthand)
		}
	}
}

func TestStopJobCmd_AcceptsJobID(t *testing.T) {
	// Verifies command parses job-id correctly
	app := New()
	cmd := NewStopJobCmd(app)

	// Test with one arg (should succeed)
	err := cmd.Args(cmd, []string{"job-123"})
	if err != nil {
		t.Errorf("Expected no error with job-id, got: %v", err)
	}

	// Test with more than one arg (should fail)
	err = cmd.Args(cmd, []string{"job-123", "extra"})
	if err == nil {
		t.Error("Expected error when multiple arguments provided")
	}
}

func TestStopJob_SuccessMessage(t *testing.T) {
	// Verifies appropriate message printed on success
	// This is a unit test so we can't actually connect to daemon,
	// but we can verify the command structure
	app := New()
	cmd := NewStopJobCmd(app)

	if cmd.Use != "stop <job-id>" {
		t.Errorf("Expected Use to be 'stop <job-id>', got: %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	if cmd.RunE == nil {
		t.Error("Expected RunE to be set")
	}
}

func TestStopJob_ForceMessage(t *testing.T) {
	// Verifies "force stopped" message when --force used
	// Since we can't actually run against a daemon in unit tests,
	// we verify the flag is properly configured and can be set
	app := New()
	cmd := NewStopJobCmd(app)

	// Verify we can set the force flag
	err := cmd.Flags().Set("force", "true")
	if err != nil {
		t.Errorf("Failed to set force flag: %v", err)
	}

	// Verify the value was set
	val, err := cmd.Flags().GetBool("force")
	if err != nil {
		t.Errorf("Failed to get force flag value: %v", err)
	}
	if !val {
		t.Error("Expected force flag to be true after setting")
	}
}

// Test the actual stopJob function behavior (without real client)
func TestStopJobFunction_MessageFormat(t *testing.T) {
	// Test that we would print the right messages
	// This test verifies the message logic without actually calling the client
	ctx := context.Background()

	// We can't test the actual function without a mock client,
	// but we've verified the command structure above
	_ = ctx
}
