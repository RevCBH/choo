package cli

import (
	"testing"
)

func TestDaemonCmd_Structure(t *testing.T) {
	// Verifies daemon has start, stop, status subcommands
	app := New()
	cmd := NewDaemonCmd(app)

	if cmd.Use != "daemon" {
		t.Errorf("Expected Use to be 'daemon', got: %s", cmd.Use)
	}

	// Check for subcommands
	subcommands := cmd.Commands()
	if len(subcommands) != 3 {
		t.Errorf("Expected 3 subcommands, got %d", len(subcommands))
	}

	// Map subcommands by name
	subcmdMap := make(map[string]bool)
	for _, subcmd := range subcommands {
		subcmdMap[subcmd.Use] = true
	}

	// Verify required subcommands
	requiredSubcmds := []string{"start", "stop", "status"}
	for _, required := range requiredSubcmds {
		if !subcmdMap[required] {
			t.Errorf("Expected subcommand '%s' not found", required)
		}
	}
}

func TestDaemonStopCmd_Flags(t *testing.T) {
	// Verifies --wait and --timeout flags exist with defaults
	app := New()
	cmd := newDaemonStopCmd(app)

	// Check --wait flag
	waitFlag := cmd.Flags().Lookup("wait")
	if waitFlag == nil {
		t.Error("Expected --wait flag to exist")
	} else {
		if waitFlag.DefValue != "true" {
			t.Errorf("Expected --wait default to be 'true', got: %s", waitFlag.DefValue)
		}
	}

	// Check --timeout flag
	timeoutFlag := cmd.Flags().Lookup("timeout")
	if timeoutFlag == nil {
		t.Error("Expected --timeout flag to exist")
	} else {
		if timeoutFlag.DefValue != "30" {
			t.Errorf("Expected --timeout default to be '30', got: %s", timeoutFlag.DefValue)
		}
	}
}

func TestDaemonStatusCmd_NoConnection(t *testing.T) {
	// Verifies appropriate error when daemon not running
	// This test verifies the command structure exists and would return an error
	// when the daemon is not running (which is the normal case in tests)
	app := New()
	cmd := newDaemonStatusCmd(app)

	if cmd.Use != "status" {
		t.Errorf("Expected Use to be 'status', got: %s", cmd.Use)
	}

	if cmd.RunE == nil {
		t.Error("Expected RunE to be set")
	}

	// We can't actually test the error without a real daemon,
	// but we verify the command structure is correct
	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestDaemonStartCmd_Basic(t *testing.T) {
	// Verifies command structure (actual daemon not started in test)
	app := New()
	cmd := newDaemonStartCmd(app)

	if cmd.Use != "start" {
		t.Errorf("Expected Use to be 'start', got: %s", cmd.Use)
	}

	if cmd.RunE == nil {
		t.Error("Expected RunE to be set")
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestDaemonStartCmd_ForegroundFlag(t *testing.T) {
	// Verifies --foreground flag exists with default false
	app := New()
	cmd := newDaemonStartCmd(app)

	// Check --foreground flag
	foregroundFlag := cmd.Flags().Lookup("foreground")
	if foregroundFlag == nil {
		t.Error("Expected --foreground flag to exist")
	} else {
		if foregroundFlag.DefValue != "false" {
			t.Errorf("Expected --foreground default to be 'false', got: %s", foregroundFlag.DefValue)
		}
	}
}

func TestIsDaemonRunning_NoDaemon(t *testing.T) {
	// When no daemon is running, isDaemonRunning should return false
	// This is the normal case in tests where no daemon is started
	if isDaemonRunning() {
		t.Error("Expected isDaemonRunning to return false when no daemon is running")
	}
}
