package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStageAll(t *testing.T) {
	repoDir := setupTestRepo(t)
	ctx := context.Background()

	// Create multiple test files
	testFile1 := filepath.Join(repoDir, "test1.txt")
	testFile2 := filepath.Join(repoDir, "test2.txt")
	if err := os.WriteFile(testFile1, []byte("test content 1"), 0644); err != nil {
		t.Fatalf("failed to create test file 1: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte("test content 2"), 0644); err != nil {
		t.Fatalf("failed to create test file 2: %v", err)
	}

	// Stage all changes
	err := StageAll(ctx, repoDir)
	if err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}

	// Verify files are staged
	files, err := GetStagedFiles(ctx, repoDir)
	if err != nil {
		t.Fatalf("GetStagedFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 staged files, got %d: %v", len(files), files)
	}
}

func TestCommit_Basic(t *testing.T) {
	repoDir := setupTestRepo(t)
	ctx := context.Background()

	// Create and stage a file
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := StageAll(ctx, repoDir); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}

	// Commit the changes
	opts := CommitOptions{
		Message:  "Test commit",
		NoVerify: true,
	}
	err := Commit(ctx, repoDir, opts)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify commit was created
	output, err := gitExec(ctx, repoDir, "log", "--oneline", "-n", "1")
	if err != nil {
		t.Fatalf("failed to get log: %v", err)
	}

	if output == "" {
		t.Error("expected commit in log, got empty output")
	}
}

func TestCommit_NoVerify(t *testing.T) {
	repoDir := setupTestRepo(t)
	ctx := context.Background()

	// Create and stage a file
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := StageAll(ctx, repoDir); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}

	// Commit with NoVerify set to true
	opts := CommitOptions{
		Message:  "Test commit with no verify",
		NoVerify: true,
	}
	err := Commit(ctx, repoDir, opts)
	if err != nil {
		t.Fatalf("Commit with NoVerify failed: %v", err)
	}

	// Verify commit was created
	output, err := gitExec(ctx, repoDir, "log", "--oneline", "-n", "1")
	if err != nil {
		t.Fatalf("failed to get log: %v", err)
	}

	if output == "" {
		t.Error("expected commit in log, got empty output")
	}
}

func TestCommit_AllowEmpty(t *testing.T) {
	repoDir := setupTestRepo(t)
	ctx := context.Background()

	// Commit without any changes
	opts := CommitOptions{
		Message:    "Empty commit",
		NoVerify:   true,
		AllowEmpty: true,
	}
	err := Commit(ctx, repoDir, opts)
	if err != nil {
		t.Fatalf("Commit with AllowEmpty failed: %v", err)
	}

	// Verify commit was created
	output, err := gitExec(ctx, repoDir, "log", "--oneline", "-n", "1")
	if err != nil {
		t.Fatalf("failed to get log: %v", err)
	}

	if output == "" {
		t.Error("expected empty commit in log, got empty output")
	}
}

func TestCommit_FailsEmpty(t *testing.T) {
	repoDir := setupTestRepo(t)
	ctx := context.Background()

	// Try to commit without any changes and without AllowEmpty
	opts := CommitOptions{
		Message:    "This should fail",
		NoVerify:   true,
		AllowEmpty: false,
	}
	err := Commit(ctx, repoDir, opts)
	if err == nil {
		t.Fatal("expected error for empty commit without AllowEmpty, got nil")
	}
}

func TestHasUncommittedChanges_Clean(t *testing.T) {
	repoDir := setupTestRepo(t)
	ctx := context.Background()

	// Clean repository should have no uncommitted changes
	hasChanges, err := HasUncommittedChanges(ctx, repoDir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges failed: %v", err)
	}

	if hasChanges {
		t.Error("expected no uncommitted changes in clean repo, got true")
	}
}

func TestHasUncommittedChanges_Dirty(t *testing.T) {
	repoDir := setupTestRepo(t)
	ctx := context.Background()

	// Create a file without staging
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Should detect uncommitted changes
	hasChanges, err := HasUncommittedChanges(ctx, repoDir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges failed: %v", err)
	}

	if !hasChanges {
		t.Error("expected uncommitted changes for modified files, got false")
	}
}

func TestHasUncommittedChanges_Staged(t *testing.T) {
	repoDir := setupTestRepo(t)
	ctx := context.Background()

	// Create and stage a file
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := StageAll(ctx, repoDir); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}

	// Should detect staged changes as uncommitted
	hasChanges, err := HasUncommittedChanges(ctx, repoDir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges failed: %v", err)
	}

	if !hasChanges {
		t.Error("expected uncommitted changes for staged files, got false")
	}
}

func TestGetStagedFiles(t *testing.T) {
	repoDir := setupTestRepo(t)
	ctx := context.Background()

	// Create and stage multiple files
	testFile1 := filepath.Join(repoDir, "test1.txt")
	testFile2 := filepath.Join(repoDir, "test2.txt")
	if err := os.WriteFile(testFile1, []byte("test content 1"), 0644); err != nil {
		t.Fatalf("failed to create test file 1: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte("test content 2"), 0644); err != nil {
		t.Fatalf("failed to create test file 2: %v", err)
	}

	if err := StageAll(ctx, repoDir); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}

	// Get staged files
	files, err := GetStagedFiles(ctx, repoDir)
	if err != nil {
		t.Fatalf("GetStagedFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 staged files, got %d: %v", len(files), files)
	}

	// Verify file names
	expectedFiles := map[string]bool{
		"test1.txt": true,
		"test2.txt": true,
	}

	for _, file := range files {
		if !expectedFiles[file] {
			t.Errorf("unexpected file in staged files: %s", file)
		}
	}
}
