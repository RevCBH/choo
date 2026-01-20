package feature

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPRDs_Valid(t *testing.T) {
	// Create temporary directory with test PRDs
	tmpDir := t.TempDir()

	// Create two valid PRD files
	prd1 := `---
title: Authentication Feature
depends_on: [database]
status: ready
priority: high
---

# Authentication System

This is the authentication PRD.
`

	prd2 := `# User Profile

This is the user profile PRD without frontmatter.
`

	if err := os.WriteFile(filepath.Join(tmpDir, "auth.md"), []byte(prd1), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "profile.md"), []byte(prd2), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Load PRDs
	prds, err := LoadPRDs(tmpDir)
	if err != nil {
		t.Fatalf("LoadPRDs failed: %v", err)
	}

	// Verify we loaded exactly 2 PRDs
	if len(prds) != 2 {
		t.Fatalf("Expected 2 PRDs, got %d", len(prds))
	}

	// Verify auth.md
	var authPRD *PRDForPrioritization
	for _, prd := range prds {
		if prd.ID == "auth" {
			authPRD = prd
			break
		}
	}

	if authPRD == nil {
		t.Fatal("auth.md not found in loaded PRDs")
	}

	if authPRD.Title != "Authentication Feature" {
		t.Errorf("Expected title 'Authentication Feature', got '%s'", authPRD.Title)
	}

	if len(authPRD.DependsOn) != 1 || authPRD.DependsOn[0] != "database" {
		t.Errorf("Expected depends_on [database], got %v", authPRD.DependsOn)
	}

	// Verify profile.md
	var profilePRD *PRDForPrioritization
	for _, prd := range prds {
		if prd.ID == "profile" {
			profilePRD = prd
			break
		}
	}

	if profilePRD == nil {
		t.Fatal("profile.md not found in loaded PRDs")
	}

	if profilePRD.Title != "User Profile" {
		t.Errorf("Expected title 'User Profile', got '%s'", profilePRD.Title)
	}

	if len(profilePRD.DependsOn) != 0 {
		t.Errorf("Expected empty depends_on, got %v", profilePRD.DependsOn)
	}
}

func TestLoadPRDs_EmptyDir(t *testing.T) {
	// Create empty directory
	tmpDir := t.TempDir()

	// Attempt to load PRDs
	_, err := LoadPRDs(tmpDir)
	if err == nil {
		t.Fatal("Expected error for empty directory, got nil")
	}

	// Verify error message is helpful
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message should not be empty")
	}
}

func TestLoadPRDs_NoMarkdown(t *testing.T) {
	// Create directory with non-markdown files
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("some notes"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Attempt to load PRDs
	_, err := LoadPRDs(tmpDir)
	if err == nil {
		t.Fatal("Expected error when no .md files found, got nil")
	}

	// Verify error mentions markdown files
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message should not be empty")
	}
}

func TestParsePRDFrontmatter_Complete(t *testing.T) {
	content := []byte(`---
title: Test PRD
depends_on: [dep1, dep2]
status: ready
priority: high
---

# Test Content
`)

	fm, err := ParsePRDFrontmatter(content)
	if err != nil {
		t.Fatalf("ParsePRDFrontmatter failed: %v", err)
	}

	if fm == nil {
		t.Fatal("Expected non-nil frontmatter")
	}

	if fm.Title != "Test PRD" {
		t.Errorf("Expected title 'Test PRD', got '%s'", fm.Title)
	}

	if len(fm.DependsOn) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(fm.DependsOn))
	}

	if fm.Status != "ready" {
		t.Errorf("Expected status 'ready', got '%s'", fm.Status)
	}

	if fm.Priority != "high" {
		t.Errorf("Expected priority 'high', got '%s'", fm.Priority)
	}
}

func TestParsePRDFrontmatter_None(t *testing.T) {
	content := []byte(`# Test Content

No frontmatter here.
`)

	fm, err := ParsePRDFrontmatter(content)
	if err != nil {
		t.Fatalf("Expected nil error for no frontmatter, got: %v", err)
	}

	if fm != nil {
		t.Errorf("Expected nil frontmatter, got: %+v", fm)
	}
}

func TestParsePRDFrontmatter_Empty(t *testing.T) {
	content := []byte(`---
---

# Test Content
`)

	fm, err := ParsePRDFrontmatter(content)
	if err != nil {
		t.Fatalf("Expected nil error for empty frontmatter, got: %v", err)
	}

	if fm != nil {
		t.Errorf("Expected nil frontmatter for empty block, got: %+v", fm)
	}
}

func TestParsePRDFrontmatter_Malformed(t *testing.T) {
	content := []byte(`---
title: Test
invalid yaml here: [
---

# Test Content
`)

	_, err := ParsePRDFrontmatter(content)
	if err == nil {
		t.Fatal("Expected error for malformed YAML, got nil")
	}
}

func TestExtractPRDTitle_Found(t *testing.T) {
	content := []byte(`# My Feature Title

Some content here.
`)

	title := ExtractPRDTitle(content)
	if title != "My Feature Title" {
		t.Errorf("Expected 'My Feature Title', got '%s'", title)
	}
}

func TestExtractPRDTitle_AfterFrontmatter(t *testing.T) {
	content := []byte(`---
title: Frontmatter Title
---

# Markdown Title

Some content here.
`)

	title := ExtractPRDTitle(content)
	if title != "Markdown Title" {
		t.Errorf("Expected 'Markdown Title', got '%s'", title)
	}
}

func TestExtractPRDTitle_NoH1(t *testing.T) {
	content := []byte(`## Only H2 Here

Some content.
`)

	title := ExtractPRDTitle(content)
	if title != "" {
		t.Errorf("Expected empty string for no H1, got '%s'", title)
	}
}

func TestExtractPRDTitle_WithWhitespace(t *testing.T) {
	content := []byte(`#   Spaced Title

Some content.
`)

	title := ExtractPRDTitle(content)
	if title != "Spaced Title" {
		t.Errorf("Expected 'Spaced Title', got '%s'", title)
	}
}

func TestLoadPRDs_SkipsREADME(t *testing.T) {
	tmpDir := t.TempDir()

	// Create README.md (should be skipped)
	readme := `# README

This is a readme file.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(readme), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a valid PRD
	prd := `# Valid PRD

This is a valid PRD.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "feature.md"), []byte(prd), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Load PRDs
	prds, err := LoadPRDs(tmpDir)
	if err != nil {
		t.Fatalf("LoadPRDs failed: %v", err)
	}

	// Should only have 1 PRD (README.md skipped)
	if len(prds) != 1 {
		t.Fatalf("Expected 1 PRD (README skipped), got %d", len(prds))
	}

	if prds[0].ID != "feature" {
		t.Errorf("Expected PRD ID 'feature', got '%s'", prds[0].ID)
	}
}

func TestLoadPRDs_FallbackTitle(t *testing.T) {
	tmpDir := t.TempDir()

	// Create PRD without H1 or frontmatter title
	prd := `This is a PRD with no title heading.

Just some content.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "untitled.md"), []byte(prd), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Load PRDs
	prds, err := LoadPRDs(tmpDir)
	if err != nil {
		t.Fatalf("LoadPRDs failed: %v", err)
	}

	if len(prds) != 1 {
		t.Fatalf("Expected 1 PRD, got %d", len(prds))
	}

	// Title should fallback to filename
	if prds[0].Title != "untitled" {
		t.Errorf("Expected title 'untitled' (from filename), got '%s'", prds[0].Title)
	}
}
