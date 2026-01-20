package feature

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPRDStore_Load(t *testing.T) {
	store := NewPRDStore("testdata")

	meta, body, err := store.Load("valid")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if meta.Title != "Test Feature" {
		t.Errorf("meta.Title = %q, want %q", meta.Title, "Test Feature")
	}

	if meta.FeatureStatus != StatusNotStarted {
		t.Errorf("meta.FeatureStatus = %q, want %q", meta.FeatureStatus, StatusNotStarted)
	}

	if meta.Branch != "feature/test" {
		t.Errorf("meta.Branch = %q, want %q", meta.Branch, "feature/test")
	}

	if meta.ReviewIterations != 0 {
		t.Errorf("meta.ReviewIterations = %d, want 0", meta.ReviewIterations)
	}

	if meta.MaxReviewIter != 3 {
		t.Errorf("meta.MaxReviewIter = %d, want 3", meta.MaxReviewIter)
	}

	if meta.SpecCount != 5 {
		t.Errorf("meta.SpecCount = %d, want 5", meta.SpecCount)
	}

	if meta.TaskCount != 10 {
		t.Errorf("meta.TaskCount = %d, want 10", meta.TaskCount)
	}

	if body == "" {
		t.Error("body is empty, expected content")
	}

	if len(body) < 10 {
		t.Errorf("body length = %d, expected longer content", len(body))
	}
}

func TestPRDStore_Load_NotFound(t *testing.T) {
	store := NewPRDStore("testdata")

	_, _, err := store.Load("nonexistent")
	if err == nil {
		t.Fatal("Load() expected error for missing file, got nil")
	}
}

func TestPRDStore_Load_NoFrontmatter(t *testing.T) {
	store := NewPRDStore("testdata")

	_, _, err := store.Load("no-frontmatter")
	if err == nil {
		t.Fatal("Load() expected error for file without frontmatter, got nil")
	}
}

func TestPRDStore_UpdateStatus(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPRDStore(tmpDir)

	// Copy test file to temp directory
	srcPath := filepath.Join("testdata", "valid.md")
	dstPath := filepath.Join(tmpDir, "test-update.md")
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("Failed to read source file: %v", err)
	}
	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load original
	originalMeta, originalBody, err := store.Load("test-update")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Update status
	newStatus := StatusGeneratingSpecs
	if err := store.UpdateStatus("test-update", newStatus); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	// Verify update
	updatedMeta, updatedBody, err := store.Load("test-update")
	if err != nil {
		t.Fatalf("Load() after update error = %v", err)
	}

	if updatedMeta.FeatureStatus != newStatus {
		t.Errorf("status = %q, want %q", updatedMeta.FeatureStatus, newStatus)
	}

	// Verify other fields are preserved
	if updatedMeta.Title != originalMeta.Title {
		t.Errorf("title changed from %q to %q", originalMeta.Title, updatedMeta.Title)
	}

	if updatedMeta.Branch != originalMeta.Branch {
		t.Errorf("branch changed from %q to %q", originalMeta.Branch, updatedMeta.Branch)
	}

	if updatedMeta.ReviewIterations != originalMeta.ReviewIterations {
		t.Errorf("review_iterations changed from %d to %d", originalMeta.ReviewIterations, updatedMeta.ReviewIterations)
	}

	if updatedBody != originalBody {
		t.Error("body content was modified")
	}
}

func TestPRDStore_UpdateState(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewPRDStore(tmpDir)

	// Copy test file to temp directory
	srcPath := filepath.Join("testdata", "partial-state.md")
	dstPath := filepath.Join(tmpDir, "test-state.md")
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("Failed to read source file: %v", err)
	}
	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load original body
	_, originalBody, err := store.Load("test-state")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Create new state
	startTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	newState := FeatureState{
		PRDID:            "test-state",
		Status:           StatusValidatingSpecs,
		Branch:           "feature/updated",
		StartedAt:        startTime,
		ReviewIterations: 5,
		MaxReviewIter:    10,
		LastFeedback:     "Looks good",
		SpecCount:        8,
		TaskCount:        15,
	}

	// Update state
	if err := store.UpdateState("test-state", newState); err != nil {
		t.Fatalf("UpdateState() error = %v", err)
	}

	// Verify all fields were updated
	updatedMeta, updatedBody, err := store.Load("test-state")
	if err != nil {
		t.Fatalf("Load() after update error = %v", err)
	}

	if updatedMeta.FeatureStatus != newState.Status {
		t.Errorf("status = %q, want %q", updatedMeta.FeatureStatus, newState.Status)
	}

	if updatedMeta.Branch != newState.Branch {
		t.Errorf("branch = %q, want %q", updatedMeta.Branch, newState.Branch)
	}

	if updatedMeta.StartedAt == nil {
		t.Fatal("started_at is nil")
	}

	if !updatedMeta.StartedAt.Equal(startTime) {
		t.Errorf("started_at = %v, want %v", updatedMeta.StartedAt, startTime)
	}

	if updatedMeta.ReviewIterations != newState.ReviewIterations {
		t.Errorf("review_iterations = %d, want %d", updatedMeta.ReviewIterations, newState.ReviewIterations)
	}

	if updatedMeta.MaxReviewIter != newState.MaxReviewIter {
		t.Errorf("max_review_iter = %d, want %d", updatedMeta.MaxReviewIter, newState.MaxReviewIter)
	}

	if updatedMeta.LastFeedback != newState.LastFeedback {
		t.Errorf("last_feedback = %q, want %q", updatedMeta.LastFeedback, newState.LastFeedback)
	}

	if updatedMeta.SpecCount != newState.SpecCount {
		t.Errorf("spec_count = %d, want %d", updatedMeta.SpecCount, newState.SpecCount)
	}

	if updatedMeta.TaskCount != newState.TaskCount {
		t.Errorf("task_count = %d, want %d", updatedMeta.TaskCount, newState.TaskCount)
	}

	// Verify body is preserved
	if updatedBody != originalBody {
		t.Error("body content was modified")
	}
}

func TestPRDStore_Exists(t *testing.T) {
	store := NewPRDStore("testdata")

	if !store.Exists("valid") {
		t.Error("Exists() = false for valid.md, want true")
	}

	if store.Exists("nonexistent") {
		t.Error("Exists() = true for nonexistent.md, want false")
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantFront   string
		wantBody    string
		wantErr     bool
	}{
		{
			name: "valid frontmatter",
			content: `---
title: Test
status: active
---
Body content here`,
			wantFront: `title: Test
status: active`,
			wantBody: "Body content here",
			wantErr:  false,
		},
		{
			name:     "missing opening marker",
			content:  "title: Test\n---\nBody",
			wantErr:  true,
		},
		{
			name:     "missing closing marker",
			content:  "---\ntitle: Test\nBody",
			wantErr:  true,
		},
		{
			name: "empty body",
			content: `---
title: Test
---
`,
			wantFront: "title: Test",
			wantBody:  "",
			wantErr:   false,
		},
		{
			name: "multiline body",
			content: `---
title: Test
---

# Heading

Multiple lines
of content.
`,
			wantFront: "title: Test",
			wantBody: "\n# Heading\n\nMultiple lines\nof content.\n",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			front, body, err := parseFrontmatter(tt.content)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if front != tt.wantFront {
					t.Errorf("frontmatter = %q, want %q", front, tt.wantFront)
				}
				if body != tt.wantBody {
					t.Errorf("body = %q, want %q", body, tt.wantBody)
				}
			}
		})
	}
}
