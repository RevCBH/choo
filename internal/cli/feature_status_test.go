package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RevCBH/choo/internal/feature"
)

func TestFeatureStatusCmd_NoArgs(t *testing.T) {
	app := New()
	cmd := NewFeatureStatusCmd(app)

	// Create temp directory and copy test fixtures
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".ralph", "prds")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Copy test fixtures
	copyTestFixtures(t, "testdata/prds", prdDir)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Capture stdout for output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run command with no args
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("Command should not error: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()

	// Should show all features with feature_status set
	if !strings.Contains(output, "in-progress") {
		t.Errorf("Output should contain in-progress feature. Got:\n%s", output)
	}
	if !strings.Contains(output, "blocked") {
		t.Errorf("Output should contain blocked feature. Got:\n%s", output)
	}
	if !strings.Contains(output, "ready") {
		t.Errorf("Output should contain ready feature. Got:\n%s", output)
	}

	// Should not show valid-prd (no feature_status)
	if strings.Contains(output, "valid-prd") {
		t.Error("Output should not contain PRDs without feature_status")
	}
}

func TestFeatureStatusCmd_WithPRDID(t *testing.T) {
	app := New()
	cmd := NewFeatureStatusCmd(app)

	// Create temp directory and copy test fixtures
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".ralph", "prds")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Copy test fixtures
	copyTestFixtures(t, "testdata/prds", prdDir)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Capture stdout for output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run command with specific PRD ID
	cmd.SetArgs([]string{"ready"})

	err := cmd.Execute()
	if err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("Command should not error: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()

	// Should show only ready feature
	if !strings.Contains(output, "ready") {
		t.Errorf("Output should contain ready feature. Got:\n%s", output)
	}

	// Should not show other features
	if strings.Contains(output, "in-progress") {
		t.Error("Output should not contain in-progress feature")
	}
	if strings.Contains(output, "blocked") {
		t.Error("Output should not contain blocked feature")
	}
}

func TestFeatureStatusCmd_JSON(t *testing.T) {
	app := New()
	cmd := NewFeatureStatusCmd(app)

	// Create temp directory and copy test fixtures
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".ralph", "prds")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Copy test fixtures
	copyTestFixtures(t, "testdata/prds", prdDir)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Capture stdout for JSON output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run command with --json flag
	cmd.SetArgs([]string{"--json"})

	err := cmd.Execute()
	if err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("Command should not error: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()

	// Parse JSON output
	var result FeatureStatusOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Output should be valid JSON: %v\nOutput: %s", err, output)
	}

	// Verify structure
	if len(result.Features) != 3 {
		t.Errorf("Expected 3 features, got %d", len(result.Features))
	}

	// Verify summary
	if result.Summary.Total != 3 {
		t.Errorf("Expected total of 3, got %d", result.Summary.Total)
	}
	if result.Summary.Ready != 1 {
		t.Errorf("Expected 1 ready, got %d", result.Summary.Ready)
	}
	if result.Summary.Blocked != 1 {
		t.Errorf("Expected 1 blocked, got %d", result.Summary.Blocked)
	}
	if result.Summary.InProgress != 1 {
		t.Errorf("Expected 1 in progress, got %d", result.Summary.InProgress)
	}

	// Verify feature details
	hasReady := false
	for _, f := range result.Features {
		if f.PRDID == "ready" {
			hasReady = true
			if f.Status != "specs_committed" {
				t.Errorf("Ready feature should have status 'specs_committed', got '%s'", f.Status)
			}
			if f.SpecCount != 4 {
				t.Errorf("Ready feature should have spec_count 4, got %d", f.SpecCount)
			}
			if f.TaskCount != 18 {
				t.Errorf("Ready feature should have task_count 18, got %d", f.TaskCount)
			}
			if f.NextAction == "" {
				t.Error("Ready feature should have next_action")
			}
		}
	}

	if !hasReady {
		t.Error("JSON output should include ready feature")
	}
}

func TestShowFeatureStatus_NoFeatures(t *testing.T) {
	app := New()

	// Create temp directory with empty PRD directory
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".ralph", "prds")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := FeatureStatusOptions{
		PRDID: "",
		JSON:  false,
	}

	err := app.ShowFeatureStatus(opts)
	if err != nil {
		t.Fatalf("Should not error on empty directory: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()

	// Should show appropriate message
	if !strings.Contains(output, "No feature workflows in progress") {
		t.Error("Should show 'No feature workflows in progress' message")
	}
}

func TestShowFeatureStatus_Blocked(t *testing.T) {
	app := New()

	// Create temp directory and copy test fixtures
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".ralph", "prds")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Copy only blocked fixture
	copyFile(t, "testdata/prds/blocked.md", filepath.Join(prdDir, "blocked.md"))

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := FeatureStatusOptions{
		PRDID: "",
		JSON:  false,
	}

	err := app.ShowFeatureStatus(opts)
	if err != nil {
		t.Fatalf("Should not error: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()

	// Should show blocked status details
	if !strings.Contains(output, "[blocked]") {
		t.Error("Output should contain blocked feature")
	}
	if !strings.Contains(output, "review_blocked") {
		t.Error("Output should contain review_blocked status")
	}
	if !strings.Contains(output, "Last feedback:") {
		t.Error("Output should show last feedback")
	}
	if !strings.Contains(output, "Manual intervention required") {
		t.Error("Output should show manual intervention message")
	}
	if !strings.Contains(output, "choo feature resume blocked --skip-review") {
		t.Error("Output should show resume command")
	}
}

func TestShowFeatureStatus_Ready(t *testing.T) {
	app := New()

	// Create temp directory and copy test fixtures
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, ".ralph", "prds")
	if err := os.MkdirAll(prdDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Copy only ready fixture
	copyFile(t, "testdata/prds/ready.md", filepath.Join(prdDir, "ready.md"))

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := FeatureStatusOptions{
		PRDID: "",
		JSON:  false,
	}

	err := app.ShowFeatureStatus(opts)
	if err != nil {
		t.Fatalf("Should not error: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()

	// Should show ready status details
	if !strings.Contains(output, "[ready]") {
		t.Error("Output should contain ready feature")
	}
	if !strings.Contains(output, "specs_committed") {
		t.Error("Output should contain specs_committed status")
	}
	if !strings.Contains(output, "Specs: 4 units, 18 tasks") {
		t.Error("Output should show spec and task counts")
	}
	if !strings.Contains(output, "Ready for: choo run --feature ready") {
		t.Error("Output should show run command")
	}
}

func TestDetermineNextAction(t *testing.T) {
	tests := []struct {
		name     string
		state    feature.FeatureState
		expected string
	}{
		{
			name: "specs_committed shows run command",
			state: feature.FeatureState{
				PRDID:  "test",
				Status: feature.StatusSpecsCommitted,
			},
			expected: "choo run --feature test",
		},
		{
			name: "review_blocked shows resume command",
			state: feature.FeatureState{
				PRDID:  "test",
				Status: feature.StatusReviewBlocked,
			},
			expected: "choo feature resume test --skip-review",
		},
		{
			name: "generating_specs shows wait message",
			state: feature.FeatureState{
				PRDID:  "test",
				Status: feature.StatusGeneratingSpecs,
			},
			expected: "Wait for spec generation to complete",
		},
		{
			name: "reviewing_specs shows wait message",
			state: feature.FeatureState{
				PRDID:  "test",
				Status: feature.StatusReviewingSpecs,
			},
			expected: "Wait for spec review to complete",
		},
		{
			name: "validating_specs shows wait message",
			state: feature.FeatureState{
				PRDID:  "test",
				Status: feature.StatusValidatingSpecs,
			},
			expected: "Wait for spec validation to complete",
		},
		{
			name: "generating_tasks shows wait message",
			state: feature.FeatureState{
				PRDID:  "test",
				Status: feature.StatusGeneratingTasks,
			},
			expected: "Wait for task generation to complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineNextAction(tt.state)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// Helper function to copy test fixtures
func copyTestFixtures(t *testing.T, srcDir, dstDir string) {
	t.Helper()

	files := []string{"in-progress.md", "blocked.md", "ready.md", "valid-prd.md"}
	for _, file := range files {
		src := filepath.Join(srcDir, file)
		dst := filepath.Join(dstDir, file)
		copyFile(t, src, dst)
	}
}

// Helper function to copy a single file
func copyFile(t *testing.T, src, dst string) {
	t.Helper()

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", src, err)
	}

	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("Failed to write destination file %s: %v", dst, err)
	}
}
