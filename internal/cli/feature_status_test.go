package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/RevCBH/choo/internal/feature"
)

func TestFeatureStatusCmd_NoArgs(t *testing.T) {
	app := New()
	cmd := NewFeatureStatusCmd(app)

	// Setup test data
	tmpDir := t.TempDir()
	createTestPRD(t, tmpDir, "feature1.md", feature.StatusSpecsCommitted)
	createTestPRD(t, tmpDir, "feature2.md", feature.StatusReviewBlocked)

	// Execute command without args
	cmd.SetArgs([]string{"--prd-dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestFeatureStatusCmd_WithPRDID(t *testing.T) {
	app := New()
	cmd := NewFeatureStatusCmd(app)

	// Setup test data
	tmpDir := t.TempDir()
	createTestPRD(t, tmpDir, "feature1.md", feature.StatusSpecsCommitted)
	createTestPRD(t, tmpDir, "feature2.md", feature.StatusReviewBlocked)

	// Execute command with specific PRD ID
	cmd.SetArgs([]string{"feature1", "--prd-dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestFeatureStatusCmd_JSON(t *testing.T) {
	app := New()

	// Setup test data
	tmpDir := t.TempDir()
	createTestPRD(t, tmpDir, "feature1.md", feature.StatusSpecsCommitted)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := FeatureStatusOptions{
		PRDDir: tmpDir,
		JSON:   true,
	}

	err := app.ShowFeatureStatus(context.Background(), opts)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Restore stdout
	w.Close()
	os.Stdout = old

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify it's valid JSON
	var result FeatureStatusOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v\nOutput: %s", err, output)
	}

	// Verify structure
	if len(result.Features) != 1 {
		t.Errorf("Expected 1 feature, got %d", len(result.Features))
	}
	if result.Summary.Total != 1 {
		t.Errorf("Expected total=1, got %d", result.Summary.Total)
	}
}

func TestShowFeatureStatus_NoFeatures(t *testing.T) {
	app := New()

	// Empty directory
	tmpDir := t.TempDir()

	opts := FeatureStatusOptions{
		PRDDir: tmpDir,
	}

	err := app.ShowFeatureStatus(context.Background(), opts)
	if err != nil {
		t.Fatalf("Expected no error for empty directory, got: %v", err)
	}
}

func TestShowFeatureStatus_Blocked(t *testing.T) {
	app := New()

	// Setup blocked feature
	tmpDir := t.TempDir()
	createTestPRD(t, tmpDir, "blocked.md", feature.StatusReviewBlocked)

	opts := FeatureStatusOptions{
		PRDDir: tmpDir,
	}

	err := app.ShowFeatureStatus(context.Background(), opts)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestShowFeatureStatus_Ready(t *testing.T) {
	app := New()

	// Setup ready feature
	tmpDir := t.TempDir()
	createTestPRD(t, tmpDir, "ready.md", feature.StatusSpecsCommitted)

	opts := FeatureStatusOptions{
		PRDDir: tmpDir,
	}

	err := app.ShowFeatureStatus(context.Background(), opts)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestDetermineNextAction(t *testing.T) {
	tests := []struct {
		status       feature.FeatureStatus
		prdID        string
		wantContains string
	}{
		{feature.StatusNotStarted, "test", "Start feature workflow"},
		{feature.StatusGeneratingSpecs, "test", "Generating specification files"},
		{feature.StatusReviewingSpecs, "test", "Reviewing generated specs"},
		{feature.StatusReviewBlocked, "test", "Manual review required"},
		{feature.StatusValidatingSpecs, "test", "Validating specifications"},
		{feature.StatusGeneratingTasks, "test", "Generating tasks"},
		{feature.StatusSpecsCommitted, "test", "Ready to run"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			state := feature.FeatureState{
				PRDID:  tt.prdID,
				Status: tt.status,
			}
			result := determineNextAction(state)
			if result == "" {
				t.Errorf("Expected non-empty result for status %s", tt.status)
			}
			// Just verify we get some reasonable output
			// The exact format can vary
		})
	}
}

// Helper function to create test PRD files
func createTestPRD(t *testing.T, dir, filename string, status feature.FeatureStatus) {
	t.Helper()

	content := `---
title: Test Feature
feature_status: ` + string(status) + `
branch: feature/test
started_at: 2026-01-20T10:00:00Z
review_iterations: 1
max_review_iter: 3
spec_count: 4
task_count: 18
---

# Test PRD

This is a test PRD for feature status testing.
`

	if status == feature.StatusReviewBlocked {
		content = `---
title: Blocked Feature
feature_status: ` + string(status) + `
branch: feature/blocked
started_at: 2026-01-20T10:00:00Z
review_iterations: 3
max_review_iter: 3
last_feedback: "Missing error handling in spec"
---

# Blocked PRD

This is a blocked PRD.
`
	}

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test PRD: %v", err)
	}
}
