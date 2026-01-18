package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCmd_Output(t *testing.T) {
	app := New()
	app.SetVersion("1.2.3", "abc1234", "2024-01-15T10:30:00Z")

	cmd := NewVersionCmd(app)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()

	// Check that output contains version, commit, and date
	if !strings.Contains(output, "1.2.3") {
		t.Error("Output should contain version '1.2.3'")
	}

	if !strings.Contains(output, "abc1234") {
		t.Error("Output should contain commit 'abc1234'")
	}

	if !strings.Contains(output, "2024-01-15T10:30:00Z") {
		t.Error("Output should contain date '2024-01-15T10:30:00Z'")
	}
}

func TestVersionCmd_Format(t *testing.T) {
	app := New()
	app.SetVersion("1.2.3", "abc1234", "2024-01-15T10:30:00Z")

	cmd := NewVersionCmd(app)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines of output, got %d", len(lines))
	}

	// Check format of each line
	if !strings.HasPrefix(lines[0], "choo version ") {
		t.Errorf("First line should start with 'choo version ', got: %s", lines[0])
	}

	if !strings.HasPrefix(lines[1], "commit: ") {
		t.Errorf("Second line should start with 'commit: ', got: %s", lines[1])
	}

	if !strings.HasPrefix(lines[2], "built: ") {
		t.Errorf("Third line should start with 'built: ', got: %s", lines[2])
	}
}

func TestSetVersion(t *testing.T) {
	app := New()

	// Initially, version info should be empty (defaults to zero values)
	if app.versionInfo.Version != "" {
		t.Error("Initial version should be empty")
	}
	if app.versionInfo.Commit != "" {
		t.Error("Initial commit should be empty")
	}
	if app.versionInfo.Date != "" {
		t.Error("Initial date should be empty")
	}

	// Set version info
	app.SetVersion("1.2.3", "abc1234", "2024-01-15T10:30:00Z")

	// Verify version info was stored correctly
	if app.versionInfo.Version != "1.2.3" {
		t.Errorf("Expected version '1.2.3', got '%s'", app.versionInfo.Version)
	}

	if app.versionInfo.Commit != "abc1234" {
		t.Errorf("Expected commit 'abc1234', got '%s'", app.versionInfo.Commit)
	}

	if app.versionInfo.Date != "2024-01-15T10:30:00Z" {
		t.Errorf("Expected date '2024-01-15T10:30:00Z', got '%s'", app.versionInfo.Date)
	}
}

func TestVersionCmd_DefaultValues(t *testing.T) {
	app := New()
	// Don't call SetVersion - use defaults

	cmd := NewVersionCmd(app)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()

	// Check that output contains default values
	if !strings.Contains(output, "dev") {
		t.Error("Output should contain default version 'dev'")
	}

	if !strings.Contains(output, "unknown") {
		t.Error("Output should contain default commit 'unknown'")
	}

	// Count occurrences of "unknown" - should be 2 (commit and date)
	unknownCount := strings.Count(output, "unknown")
	if unknownCount != 2 {
		t.Errorf("Expected 2 occurrences of 'unknown', got %d", unknownCount)
	}
}
