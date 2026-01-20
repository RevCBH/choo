package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFeatureStartCmd_Args(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "one arg",
			args:    []string{"test-prd"},
			wantErr: false,
		},
		{
			name:    "too many args",
			args:    []string{"test-prd", "extra"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := New()
			cmd := NewFeatureStartCmd(app)

			// Create temp dir for PRD
			tmpDir := t.TempDir()
			createValidTestPRD(t, tmpDir, "test-prd.md")

			// Add prd-dir flag and dry-run to avoid actual execution
			fullArgs := append(tt.args, "--prd-dir", tmpDir, "--dry-run")
			cmd.SetArgs(fullArgs)

			err := cmd.Execute()
			if tt.wantErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestFeatureStartCmd_Flags(t *testing.T) {
	app := New()
	cmd := NewFeatureStartCmd(app)

	// Create temp dir for PRD
	tmpDir := t.TempDir()
	createValidTestPRD(t, tmpDir, "test-prd.md")

	// Test all flags are registered with correct defaults
	tests := []struct {
		flagName     string
		defaultValue string
	}{
		{"prd-dir", "docs/prds"},
		{"specs-dir", "specs/tasks"},
		{"max-review-iter", "3"},
		{"skip-spec-review", "false"},
		{"dry-run", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.flagName, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("Flag %s not found", tt.flagName)
			}
			if flag.DefValue != tt.defaultValue {
				t.Errorf("Flag %s default value = %s, want %s",
					tt.flagName, flag.DefValue, tt.defaultValue)
			}
		})
	}
}

func TestFeatureStartCmd_DryRun(t *testing.T) {
	app := New()

	// Create temp dir and PRD
	tmpDir := t.TempDir()
	createValidTestPRD(t, tmpDir, "test-feature.md")

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := FeatureStartOptions{
		PRDID:          "test-feature",
		PRDDir:         tmpDir,
		SpecsDir:       "specs/tasks",
		SkipSpecReview: false,
		MaxReviewIter:  3,
		DryRun:         true,
	}

	err := app.RunFeatureStart(context.Background(), opts)

	// Restore stdout
	w.Close()
	os.Stdout = old

	// Read output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Assertions
	if err != nil {
		t.Fatalf("Dry run failed: %v", err)
	}

	// Verify dry run output contains expected steps
	expectedSteps := []string{
		"Dry run - showing planned actions:",
		"1. Read PRD from",
		"2. Create branch feature/test-feature",
		"3. Generate specs using spec-generator agent",
		"4. Review specs (max 3 iterations)",
		"5. Validate specs using spec-validator agent",
		"6. Generate tasks using task-generator agent",
		"7. Commit specs and tasks to feature/test-feature",
		"Run without --dry-run to execute.",
	}

	for _, step := range expectedSteps {
		if !strings.Contains(output, step) {
			t.Errorf("Dry run output missing step: %q\nGot: %s", step, output)
		}
	}
}

func TestFeatureStartCmd_DryRun_SkipReview(t *testing.T) {
	app := New()

	// Create temp dir and PRD
	tmpDir := t.TempDir()
	createValidTestPRD(t, tmpDir, "test-feature.md")

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := FeatureStartOptions{
		PRDID:          "test-feature",
		PRDDir:         tmpDir,
		SpecsDir:       "specs/tasks",
		SkipSpecReview: true,
		MaxReviewIter:  3,
		DryRun:         true,
	}

	err := app.RunFeatureStart(context.Background(), opts)

	// Restore stdout
	w.Close()
	os.Stdout = old

	// Read output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Assertions
	if err != nil {
		t.Fatalf("Dry run failed: %v", err)
	}

	// Verify skip-review message appears
	if !strings.Contains(output, "Skip spec review (--skip-spec-review)") {
		t.Errorf("Expected skip-review message in output, got: %s", output)
	}

	// Verify review iterations message does NOT appear
	if strings.Contains(output, "Review specs (max") {
		t.Errorf("Should not contain review iterations when skipped, got: %s", output)
	}
}

func TestRunFeatureStart_PRDNotFound(t *testing.T) {
	app := New()

	// Empty temp dir (no PRD)
	tmpDir := t.TempDir()

	opts := FeatureStartOptions{
		PRDID:         "nonexistent",
		PRDDir:        tmpDir,
		SpecsDir:      "specs/tasks",
		MaxReviewIter: 3,
		DryRun:        false,
	}

	err := app.RunFeatureStart(context.Background(), opts)

	// Should return error about missing PRD
	if err == nil {
		t.Fatal("Expected error for nonexistent PRD, got nil")
	}

	if !strings.Contains(err.Error(), "PRD not found") {
		t.Errorf("Expected 'PRD not found' error, got: %v", err)
	}
}

func TestRunFeatureStart_ValidatesInput(t *testing.T) {
	tests := []struct {
		name    string
		prdID   string
		wantErr bool
	}{
		{
			name:    "valid prd id",
			prdID:   "valid-feature",
			wantErr: false,
		},
		{
			name:    "valid prd with dashes",
			prdID:   "my-test-feature",
			wantErr: false,
		},
		{
			name:    "valid prd with numbers",
			prdID:   "feature-123",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := New()

			// Create temp dir and PRD
			tmpDir := t.TempDir()
			createValidTestPRD(t, tmpDir, tt.prdID+".md")

			opts := FeatureStartOptions{
				PRDID:         tt.prdID,
				PRDDir:        tmpDir,
				SpecsDir:      "specs/tasks",
				MaxReviewIter: 3,
				DryRun:        true, // Use dry-run to avoid workflow execution
			}

			err := app.RunFeatureStart(context.Background(), opts)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// Helper function to create a valid test PRD
func createValidTestPRD(t *testing.T, dir, filename string) {
	t.Helper()

	content := `---
title: Valid Test PRD
---

# Valid Test PRD

This is a valid PRD file used for testing.

## Overview

Test feature overview.
`

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test PRD: %v", err)
	}
}
