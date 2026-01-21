package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RevCBH/choo/internal/feature"
)

func TestFeatureResumeCmd_Args(t *testing.T) {
	app := New()
	cmd := NewFeatureResumeCmd(app)

	// Test with no arguments (should fail)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when no arguments provided")
	}

	// Test with exactly 1 argument (should succeed in parsing)
	cmd = NewFeatureResumeCmd(app)
	cmd.SetArgs([]string{"test-prd"})

	// This will fail in execution because workflow is not implemented,
	// but it should pass argument validation
	err = cmd.Execute()
	// We expect an error from RunFeatureResume (not implemented), not from arg validation
	if err == nil {
		t.Error("Expected error from unimplemented workflow")
	}
	if strings.Contains(err.Error(), "requires exactly 1 arg") {
		t.Error("Should not fail on argument count validation")
	}

	// Test with too many arguments (should fail)
	cmd = NewFeatureResumeCmd(app)
	cmd.SetArgs([]string{"test-prd", "extra-arg"})
	err = cmd.Execute()
	if err == nil {
		t.Error("Expected error when too many arguments provided")
	}
}

func TestFeatureResumeCmd_Flags(t *testing.T) {
	app := New()
	cmd := NewFeatureResumeCmd(app)

	// Check that all required flags exist
	skipReviewFlag := cmd.Flags().Lookup("skip-review")
	if skipReviewFlag == nil {
		t.Error("skip-review flag not registered")
	}
	if skipReviewFlag.DefValue != "false" {
		t.Errorf("skip-review default should be 'false', got '%s'", skipReviewFlag.DefValue)
	}

	fromValidationFlag := cmd.Flags().Lookup("from-validation")
	if fromValidationFlag == nil {
		t.Error("from-validation flag not registered")
	}
	if fromValidationFlag.DefValue != "false" {
		t.Errorf("from-validation default should be 'false', got '%s'", fromValidationFlag.DefValue)
	}

	fromTasksFlag := cmd.Flags().Lookup("from-tasks")
	if fromTasksFlag == nil {
		t.Error("from-tasks flag not registered")
	}
	if fromTasksFlag.DefValue != "false" {
		t.Errorf("from-tasks default should be 'false', got '%s'", fromTasksFlag.DefValue)
	}
}

func TestRunFeatureResume_NotBlocked(t *testing.T) {
	app := New()

	// Create temp directory for test
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, "docs", "prd")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a PRD file with non-blocked state
	prdContent := `---
prd_id: test-prd
title: Test PRD
status: approved
feature_status: generating_specs
---

Test PRD content.
`
	prdPath := filepath.Join(prdDir, "test-prd.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("Failed to create test PRD: %v", err)
	}

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	opts := FeatureResumeOptions{
		PRDID:      "test-prd",
		SkipReview: false,
	}

	err := app.RunFeatureResume(context.Background(), opts)
	if err == nil {
		t.Error("Expected error when feature is not in blocked state")
	}
	if !strings.Contains(err.Error(), "cannot resume") {
		t.Errorf("Expected 'cannot resume' error, got: %v", err)
	}
}

func TestRunFeatureResume_PRDNotFound(t *testing.T) {
	app := New()

	// Create temp directory for test
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, "docs", "prd")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	opts := FeatureResumeOptions{
		PRDID:      "nonexistent-prd",
		SkipReview: false,
	}

	err := app.RunFeatureResume(context.Background(), opts)
	if err == nil {
		t.Error("Expected error when PRD doesn't exist")
	}
	if !strings.Contains(err.Error(), "PRD not found") {
		t.Errorf("Expected 'PRD not found' error, got: %v", err)
	}
}

func TestValidateResumeState_Blocked(t *testing.T) {
	// Test that review_blocked state is allowed
	state := feature.FeatureState{
		PRDID:  "test-prd",
		Status: feature.StatusReviewBlocked,
	}

	err := validateFeatureResumeState(state)
	if err != nil {
		t.Errorf("Expected no error for review_blocked state, got: %v", err)
	}
}

func TestValidateResumeState_Other(t *testing.T) {
	// Test various non-blocked states
	testCases := []struct {
		name   string
		status feature.FeatureStatus
	}{
		{"generating_specs", feature.StatusGeneratingSpecs},
		{"reviewing_specs", feature.StatusReviewingSpecs},
		{"validating_specs", feature.StatusValidatingSpecs},
		{"generating_tasks", feature.StatusGeneratingTasks},
		{"specs_committed", feature.StatusSpecsCommitted},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := feature.FeatureState{
				PRDID:  "test-prd",
				Status: tc.status,
			}

			err := validateFeatureResumeState(state)
			if err == nil {
				t.Errorf("Expected error for %s state", tc.status)
			}
			if !strings.Contains(err.Error(), "cannot resume") {
				t.Errorf("Expected 'cannot resume' error, got: %v", err)
			}
		})
	}
}

func TestValidateResumeOptions_MutuallyExclusive(t *testing.T) {
	// Test that --from-validation and --from-tasks are mutually exclusive
	opts := FeatureResumeOptions{
		PRDID:          "test-prd",
		FromValidation: true,
		FromTasks:      true,
	}

	err := validateResumeOptions(opts)
	if err == nil {
		t.Error("Expected error when both --from-validation and --from-tasks are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("Expected 'mutually exclusive' error, got: %v", err)
	}
}

func TestRunFeatureResume_SkipReview(t *testing.T) {
	app := New()

	// Create temp directory for test
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, "docs", "prd")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a PRD file in blocked state
	prdContent := `---
prd_id: test-prd
title: Test PRD
status: approved
feature_status: review_blocked
---

Test PRD content.
`
	prdPath := filepath.Join(prdDir, "test-prd.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("Failed to create test PRD: %v", err)
	}

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	opts := FeatureResumeOptions{
		PRDID:      "test-prd",
		SkipReview: true,
	}

	err := app.RunFeatureResume(context.Background(), opts)
	// This will fail because workflow is not implemented, but should pass validation
	if err == nil {
		t.Error("Expected error from unimplemented workflow")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("Expected 'not yet implemented' error, got: %v", err)
	}
}

func TestRunFeatureResume_FromValidation(t *testing.T) {
	app := New()

	// Create temp directory for test
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, "docs", "prd")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a PRD file in blocked state
	prdContent := `---
prd_id: test-prd
title: Test PRD
status: approved
feature_status: review_blocked
---

Test PRD content.
`
	prdPath := filepath.Join(prdDir, "test-prd.md")
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("Failed to create test PRD: %v", err)
	}

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	opts := FeatureResumeOptions{
		PRDID:          "test-prd",
		FromValidation: true,
	}

	err := app.RunFeatureResume(context.Background(), opts)
	// This will fail because workflow is not implemented, but should pass validation
	if err == nil {
		t.Error("Expected error from unimplemented workflow")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("Expected 'not yet implemented' error, got: %v", err)
	}
}
