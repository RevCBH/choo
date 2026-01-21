package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFeatureStartCmd_Args(t *testing.T) {
	app := New()
	cmd := NewFeatureStartCmd(app)

	// Test with no arguments (should fail)
	cmd.SetArgs([]string{})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when no arguments provided")
	}

	// Test with exactly 1 argument (should succeed in parsing)
	cmd = NewFeatureStartCmd(app)
	cmd.SetArgs([]string{"test-prd"})
	buf = new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Note: This will fail in execution because workflow is not implemented,
	// but it should pass argument validation
	err = cmd.Execute()
	// We expect an error from RunFeatureStart (not implemented), not from arg validation
	if err == nil {
		t.Error("Expected error from unimplemented workflow")
	}
	if strings.Contains(err.Error(), "requires exactly 1 arg") {
		t.Error("Should not fail on argument count validation")
	}

	// Test with too many arguments (should fail)
	cmd = NewFeatureStartCmd(app)
	cmd.SetArgs([]string{"test-prd", "extra-arg"})
	buf = new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err = cmd.Execute()
	if err == nil {
		t.Error("Expected error when too many arguments provided")
	}
}

func TestFeatureStartCmd_Flags(t *testing.T) {
	app := New()
	cmd := NewFeatureStartCmd(app)

	// Check that all required flags exist
	prdDirFlag := cmd.Flags().Lookup("prd-dir")
	if prdDirFlag == nil {
		t.Error("prd-dir flag not registered")
	}
	if prdDirFlag.DefValue != "docs/prd" {
		t.Errorf("prd-dir default should be 'docs/prd', got '%s'", prdDirFlag.DefValue)
	}

	specsDirFlag := cmd.Flags().Lookup("specs-dir")
	if specsDirFlag == nil {
		t.Error("specs-dir flag not registered")
	}
	if specsDirFlag.DefValue != "specs/tasks" {
		t.Errorf("specs-dir default should be 'specs/tasks', got '%s'", specsDirFlag.DefValue)
	}

	skipReviewFlag := cmd.Flags().Lookup("skip-spec-review")
	if skipReviewFlag == nil {
		t.Error("skip-spec-review flag not registered")
	}
	if skipReviewFlag.DefValue != "false" {
		t.Errorf("skip-spec-review default should be 'false', got '%s'", skipReviewFlag.DefValue)
	}

	maxIterFlag := cmd.Flags().Lookup("max-review-iter")
	if maxIterFlag == nil {
		t.Error("max-review-iter flag not registered")
	}
	if maxIterFlag.DefValue != "3" {
		t.Errorf("max-review-iter default should be '3', got '%s'", maxIterFlag.DefValue)
	}

	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("dry-run flag not registered")
	}
	if dryRunFlag.DefValue != "false" {
		t.Errorf("dry-run default should be 'false', got '%s'", dryRunFlag.DefValue)
	}
}

func TestFeatureStartCmd_DryRun(t *testing.T) {
	app := New()
	cmd := NewFeatureStartCmd(app)

	// Set args with dry-run flag
	cmd.SetArgs([]string{"test-prd", "--dry-run"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Dry run should not error: %v", err)
	}

	output := buf.String()

	// Verify dry run output contains expected text
	if !strings.Contains(output, "Dry run - showing planned actions") {
		t.Error("Dry run output should contain 'Dry run - showing planned actions'")
	}

	if !strings.Contains(output, "1. Read PRD from docs/prd/test-prd.md") {
		t.Error("Dry run should show PRD read step")
	}

	if !strings.Contains(output, "2. Create branch feature/test-prd from main") {
		t.Error("Dry run should show branch creation step")
	}

	if !strings.Contains(output, "3. Generate specs using spec-generator agent") {
		t.Error("Dry run should show spec generation step")
	}

	if !strings.Contains(output, "4. Review specs (max 3 iterations)") {
		t.Error("Dry run should show review step with iteration count")
	}

	if !strings.Contains(output, "5. Validate specs using spec-validator agent") {
		t.Error("Dry run should show validation step")
	}

	if !strings.Contains(output, "6. Generate tasks using task-generator agent") {
		t.Error("Dry run should show task generation step")
	}

	if !strings.Contains(output, "7. Commit specs and tasks to feature/test-prd") {
		t.Error("Dry run should show commit step")
	}

	if !strings.Contains(output, "Run without --dry-run to execute") {
		t.Error("Dry run should prompt to run without flag")
	}
}

func TestFeatureStartCmd_DryRunSkipReview(t *testing.T) {
	app := New()
	cmd := NewFeatureStartCmd(app)

	// Set args with dry-run and skip-spec-review flags
	cmd.SetArgs([]string{"test-prd", "--dry-run", "--skip-spec-review"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Dry run should not error: %v", err)
	}

	output := buf.String()

	// When skipping review, output should mention it
	if !strings.Contains(output, "Skip spec review (--skip-spec-review)") {
		t.Error("Dry run should indicate review is skipped")
	}

	// Should NOT contain the normal review message
	if strings.Contains(output, "Review specs (max") {
		t.Error("Dry run should not show review step when skipping")
	}
}

func TestRunFeatureStart_PRDNotFound(t *testing.T) {
	app := New()

	// Create temp directory for test
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, "prds")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	opts := FeatureStartOptions{
		PRDID:         "nonexistent-prd",
		PRDDir:        prdDir,
		SpecsDir:      filepath.Join(tmpDir, "specs"),
		MaxReviewIter: 3,
		DryRun:        false,
	}

	// Create a mock command for testing
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := app.RunFeatureStart(cmd, opts)
	if err == nil {
		t.Error("Expected error when PRD doesn't exist")
	}

	// Note: The current implementation returns "not yet implemented" error
	// This test will need to be updated when the workflow is implemented
	// to check for PRD not found errors specifically
}

func TestRunFeatureStart_ValidatesInput(t *testing.T) {
	app := New()

	// Test empty PRD ID
	opts := FeatureStartOptions{
		PRDID:         "",
		PRDDir:        "docs/prd",
		SpecsDir:      "specs/tasks",
		MaxReviewIter: 3,
		DryRun:        false,
	}

	// Create a mock command for testing
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := app.RunFeatureStart(cmd, opts)
	if err == nil {
		t.Error("Expected error when PRD ID is empty")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected 'cannot be empty' error, got: %v", err)
	}
}
