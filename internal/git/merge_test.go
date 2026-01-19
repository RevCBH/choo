package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// setupTestRepoWithRemote creates a test repo with a bare remote
func setupTestRepoWithRemote(t *testing.T) (repoPath string, remotePath string) {
	t.Helper()

	tmpDir := t.TempDir()

	// Create bare remote repository with main as default branch
	remotePath = filepath.Join(tmpDir, "remote.git")
	cmd := exec.Command("git", "init", "--bare", "--initial-branch=main", remotePath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init bare remote: %v", err)
	}

	// Create local repository
	repoPath = filepath.Join(tmpDir, "local")
	cmd = exec.Command("git", "clone", remotePath, repoPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to clone repo: %v", err)
	}

	// Ensure we're on main branch (in case git defaults to something else)
	cmd = exec.Command("git", "checkout", "-b", "main")
	cmd.Dir = repoPath
	// Ignore error - branch may already exist if git defaulted to main
	_ = cmd.Run()

	// Configure git user
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to config git user: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to config git email: %v", err)
	}

	// Create initial commit on main
	testFile := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repo"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add README: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	cmd = exec.Command("git", "push", "origin", "main")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to push: %v", err)
	}

	return repoPath, remotePath
}

func TestMergeManager_Serialization(t *testing.T) {
	manager := &MergeManager{}

	var wg sync.WaitGroup
	var order []int
	var mu sync.Mutex

	// Launch 5 goroutines that acquire the merge lock
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			mu.Lock()
			order = append(order, id)
			mu.Unlock()

			// Simulate merge work
			time.Sleep(10 * time.Millisecond)
		}(i)
	}

	wg.Wait()

	// All 5 goroutines should have completed
	if len(order) != 5 {
		t.Errorf("expected 5 completed merges, got %d", len(order))
	}
}

func TestMergeManager_AcquireLock(t *testing.T) {
	manager := &MergeManager{}

	// First goroutine acquires the lock
	manager.mutex.Lock()

	locked := make(chan bool, 1)

	// Second goroutine tries to acquire
	go func() {
		manager.mutex.Lock()
		locked <- true
		manager.mutex.Unlock()
	}()

	// Give second goroutine time to try
	time.Sleep(50 * time.Millisecond)

	select {
	case <-locked:
		t.Error("second goroutine should be blocked by mutex")
	default:
		// Expected: still blocked
	}

	// Release the lock
	manager.mutex.Unlock()

	// Now second goroutine should acquire it
	select {
	case <-locked:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("second goroutine should have acquired lock")
	}
}

func TestMergeManager_PendingDeletes(t *testing.T) {
	manager := &MergeManager{
		PendingDeletes: []string{},
	}

	manager.ScheduleBranchDelete("ralph/app-shell-sunset")
	manager.ScheduleBranchDelete("ralph/deck-list-harbor")

	if len(manager.PendingDeletes) != 2 {
		t.Errorf("expected 2 pending deletes, got %d", len(manager.PendingDeletes))
	}

	expected := map[string]bool{
		"ralph/app-shell-sunset": true,
		"ralph/deck-list-harbor": true,
	}

	for _, branch := range manager.PendingDeletes {
		if !expected[branch] {
			t.Errorf("unexpected branch in pending deletes: %s", branch)
		}
	}
}

func TestMergeManager_FlushDeletes(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	manager := &MergeManager{
		RepoRoot:       repoPath,
		PendingDeletes: []string{},
	}

	// Create a test branch
	cmd := exec.Command("git", "checkout", "-b", "test-branch")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create test branch: %v", err)
	}

	// Switch back to main
	cmd = exec.Command("git", "checkout", "main")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	// Schedule the branch for deletion
	manager.ScheduleBranchDelete("test-branch")

	// Flush deletes
	if err := manager.FlushDeletes(ctx); err != nil {
		t.Fatalf("FlushDeletes failed: %v", err)
	}

	// Verify pending deletes list is cleared
	if len(manager.PendingDeletes) != 0 {
		t.Errorf("expected pending deletes to be cleared, got %d items", len(manager.PendingDeletes))
	}

	// Verify branch was deleted
	cmd = exec.Command("git", "branch", "--list", "test-branch")
	cmd.Dir = repoPath
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) != "" {
		t.Error("expected test-branch to be deleted, but it still exists")
	}
}

func TestRebase_NoConflicts(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	// Create a new branch
	cmd := exec.Command("git", "checkout", "-b", "feature-branch")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	// Add a commit on feature branch
	testFile := filepath.Join(repoPath, "feature.txt")
	if err := os.WriteFile(testFile, []byte("feature content"), 0644); err != nil {
		t.Fatalf("failed to create feature file: %v", err)
	}

	cmd = exec.Command("git", "add", "feature.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add feature file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add feature")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit feature: %v", err)
	}

	// Rebase onto main (no conflicts expected)
	hasConflicts, err := Rebase(ctx, repoPath, "main")
	if err != nil {
		t.Fatalf("Rebase failed: %v", err)
	}

	if hasConflicts {
		t.Error("expected no conflicts, got hasConflicts=true")
	}
}

func TestRebase_WithConflicts(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	// Create and modify a file on main
	testFile := filepath.Join(repoPath, "conflict.txt")
	if err := os.WriteFile(testFile, []byte("main content"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	cmd := exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add conflict file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on main")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on main: %v", err)
	}

	// Create feature branch from before the main commit
	cmd = exec.Command("git", "checkout", "-b", "feature-branch", "HEAD~1")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	// Create conflicting content
	if err := os.WriteFile(testFile, []byte("feature content"), 0644); err != nil {
		t.Fatalf("failed to modify conflict file: %v", err)
	}

	cmd = exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add modified file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on feature")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on feature: %v", err)
	}

	// Rebase onto main (conflicts expected)
	hasConflicts, err := Rebase(ctx, repoPath, "main")
	if err != nil {
		t.Fatalf("Rebase failed with error: %v", err)
	}

	if !hasConflicts {
		t.Error("expected conflicts, got hasConflicts=false")
	}

	// Clean up: abort the rebase
	if err := abortRebase(ctx, repoPath); err != nil {
		t.Fatalf("failed to abort rebase: %v", err)
	}
}

func TestForcePushWithLease(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	// Create and push a new branch
	cmd := exec.Command("git", "checkout", "-b", "test-push-branch")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Create a commit
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add test file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Test commit")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Initial push to establish upstream
	cmd = exec.Command("git", "push", "-u", "origin", "test-push-branch")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to push branch: %v", err)
	}

	// Amend the commit (creates divergence)
	if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add modified file: %v", err)
	}

	cmd = exec.Command("git", "commit", "--amend", "-m", "Amended commit", "--no-edit")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to amend commit: %v", err)
	}

	// Force push with lease
	err := ForcePushWithLease(ctx, repoPath)
	if err != nil {
		t.Fatalf("ForcePushWithLease failed: %v", err)
	}
}

func TestFetch(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	// Fetch should succeed for main branch
	err := Fetch(ctx, repoPath, "main")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
}

func TestGetConflictedFiles(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	// Create a conflict scenario
	testFile := filepath.Join(repoPath, "conflict.txt")
	if err := os.WriteFile(testFile, []byte("main content"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	cmd := exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add conflict file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on main")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on main: %v", err)
	}

	// Create feature branch from before the main commit
	cmd = exec.Command("git", "checkout", "-b", "feature-conflict", "HEAD~1")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	// Create conflicting content
	if err := os.WriteFile(testFile, []byte("feature content"), 0644); err != nil {
		t.Fatalf("failed to modify conflict file: %v", err)
	}

	cmd = exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add modified file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on feature")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on feature: %v", err)
	}

	// Start rebase to create conflict
	cmd = exec.Command("git", "rebase", "main")
	cmd.Dir = repoPath
	_ = cmd.Run() // Expected to fail with conflict

	// Get conflicted files
	files, err := getConflictedFiles(ctx, repoPath)
	if err != nil {
		t.Fatalf("getConflictedFiles failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 conflicted file, got %d", len(files))
	}

	if len(files) > 0 && files[0] != "conflict.txt" {
		t.Errorf("expected conflict.txt, got %s", files[0])
	}

	// Clean up: abort the rebase
	if err := abortRebase(ctx, repoPath); err != nil {
		t.Fatalf("failed to abort rebase: %v", err)
	}
}

func TestGetConflictedFiles_NoConflicts(t *testing.T) {
	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// No conflicts in clean repo
	files, err := getConflictedFiles(ctx, repoPath)
	if err != nil {
		t.Fatalf("getConflictedFiles failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 conflicted files, got %d: %v", len(files), files)
	}
}

func TestDeleteBranch_Local(t *testing.T) {
	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Create an initial commit so we can create branches
	testFile := filepath.Join(repoPath, "initial.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	cmd := exec.Command("git", "add", "initial.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add initial file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Create a test branch
	cmd = exec.Command("git", "checkout", "-b", "test-delete-local")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Switch back to main (can't delete current branch)
	cmd = exec.Command("git", "checkout", "-")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout previous branch: %v", err)
	}

	// Delete the branch
	err := deleteBranch(ctx, repoPath, "test-delete-local", false)
	if err != nil {
		t.Fatalf("deleteBranch failed: %v", err)
	}

	// Verify branch is deleted
	cmd = exec.Command("git", "branch", "--list", "test-delete-local")
	cmd.Dir = repoPath
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) != "" {
		t.Error("expected branch to be deleted, but it still exists")
	}
}

func TestDeleteBranch_Remote(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	// Create and push a test branch
	cmd := exec.Command("git", "checkout", "-b", "test-delete-remote")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	cmd = exec.Command("git", "push", "-u", "origin", "test-delete-remote")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to push branch: %v", err)
	}

	// Delete the remote branch
	err := deleteBranch(ctx, repoPath, "test-delete-remote", true)
	if err != nil {
		t.Fatalf("deleteBranch remote failed: %v", err)
	}

	// Verify remote branch is deleted
	cmd = exec.Command("git", "ls-remote", "--heads", "origin", "test-delete-remote")
	cmd.Dir = repoPath
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) != "" {
		t.Error("expected remote branch to be deleted, but it still exists")
	}
}

func TestContinueRebase(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	// Set up a simple rebase scenario
	testFile := filepath.Join(repoPath, "file.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial file")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Create a branch and modify
	cmd = exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	if err := os.WriteFile(testFile, []byte("feature change"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	cmd = exec.Command("git", "add", "file.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add modified file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Feature change")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit feature: %v", err)
	}

	// Go back to main and make different change
	cmd = exec.Command("git", "checkout", "main")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	if err := os.WriteFile(testFile, []byte("main change"), 0644); err != nil {
		t.Fatalf("failed to modify file on main: %v", err)
	}

	cmd = exec.Command("git", "add", "file.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Main change")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on main: %v", err)
	}

	// Switch back to feature
	cmd = exec.Command("git", "checkout", "feature")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout feature: %v", err)
	}

	// Start rebase (will conflict)
	cmd = exec.Command("git", "rebase", "main")
	cmd.Dir = repoPath
	_ = cmd.Run() // Expected to fail

	// Resolve conflict by accepting ours
	if err := os.WriteFile(testFile, []byte("resolved"), 0644); err != nil {
		t.Fatalf("failed to resolve conflict: %v", err)
	}

	cmd = exec.Command("git", "add", "file.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add resolved file: %v", err)
	}

	// Continue rebase
	err := continueRebase(ctx, repoPath)
	if err != nil {
		t.Fatalf("continueRebase failed: %v", err)
	}
}

func TestAbortRebase(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	// Create a conflict scenario
	testFile := filepath.Join(repoPath, "conflict.txt")
	if err := os.WriteFile(testFile, []byte("main content"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	cmd := exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add conflict file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on main")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on main: %v", err)
	}

	// Create feature branch from before the main commit
	cmd = exec.Command("git", "checkout", "-b", "feature-abort", "HEAD~1")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	// Create conflicting content
	if err := os.WriteFile(testFile, []byte("feature content"), 0644); err != nil {
		t.Fatalf("failed to modify conflict file: %v", err)
	}

	cmd = exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add modified file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on feature")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on feature: %v", err)
	}

	// Start rebase (will conflict)
	cmd = exec.Command("git", "rebase", "main")
	cmd.Dir = repoPath
	_ = cmd.Run() // Expected to fail

	// Abort the rebase
	err := abortRebase(ctx, repoPath)
	if err != nil {
		t.Fatalf("abortRebase failed: %v", err)
	}

	// Verify we're back to normal state
	cmd = exec.Command("git", "status")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	if strings.Contains(string(output), "rebase in progress") {
		t.Error("expected rebase to be aborted, but it's still in progress")
	}
}

func TestNewMergeManager(t *testing.T) {
	repoPath := setupTestRepo(t)

	manager := NewMergeManager(repoPath, nil)

	if manager.RepoRoot != repoPath {
		t.Errorf("expected RepoRoot=%s, got %s", repoPath, manager.RepoRoot)
	}

	if manager.MaxConflictAttempts != 3 {
		t.Errorf("expected MaxConflictAttempts=3, got %d", manager.MaxConflictAttempts)
	}

	if manager.PendingDeletes == nil {
		t.Error("expected PendingDeletes to be initialized, got nil")
	}

	if len(manager.PendingDeletes) != 0 {
		t.Errorf("expected empty PendingDeletes, got %d items", len(manager.PendingDeletes))
	}
}

func TestMergeManager_Merge(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	manager := NewMergeManager(repoPath, nil)

	branch := &Branch{
		Name:         "test-branch",
		UnitID:       "test-unit",
		TargetBranch: "main",
	}

	// The Merge function should acquire lock and fetch
	result, err := manager.Merge(ctx, branch)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if !result.Success {
		t.Error("expected successful merge result")
	}
}

func TestResolveConflicts_Success(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	// Create a conflict scenario
	testFile := filepath.Join(repoPath, "conflict.txt")
	if err := os.WriteFile(testFile, []byte("main content"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	cmd := exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add conflict file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on main")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on main: %v", err)
	}

	// Create feature branch from before the main commit
	cmd = exec.Command("git", "checkout", "-b", "feature-resolve", "HEAD~1")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	// Create conflicting content
	if err := os.WriteFile(testFile, []byte("feature content"), 0644); err != nil {
		t.Fatalf("failed to modify conflict file: %v", err)
	}

	cmd = exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add modified file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on feature")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on feature: %v", err)
	}

	// Start rebase to create conflict
	cmd = exec.Command("git", "rebase", "main")
	cmd.Dir = repoPath
	_ = cmd.Run() // Expected to fail with conflict

	// Mock Claude client that resolves conflicts
	mock := &mockClaudeClient{
		resolveFunc: func(ctx context.Context, opts InvokeOptions) (string, error) {
			// Resolve conflict by keeping "ours" side
			conflicts, _ := getConflictedFiles(ctx, repoPath)
			for _, f := range conflicts {
				filePath := filepath.Join(repoPath, f)
				// Write resolved content (remove markers)
				if err := os.WriteFile(filePath, []byte("resolved content"), 0644); err != nil {
					return "", err
				}
				// Stage the resolved file
				cmd := exec.Command("git", "add", f)
				cmd.Dir = repoPath
				if err := cmd.Run(); err != nil {
					return "", err
				}
			}
			return "", nil
		},
	}

	manager := NewMergeManager(repoPath, mock)

	// Resolve conflicts
	err := manager.ResolveConflicts(ctx, repoPath)
	if err != nil {
		t.Fatalf("ResolveConflicts failed: %v", err)
	}

	// Verify conflicts are resolved
	conflicts, _ := getConflictedFiles(ctx, repoPath)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts after resolution, got %d", len(conflicts))
	}

	// Verify Claude was called once
	if mock.callCount != 1 {
		t.Errorf("expected Claude to be called 1 time, got %d", mock.callCount)
	}
}

func TestResolveConflicts_Retry(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	// Create a conflict scenario
	testFile := filepath.Join(repoPath, "conflict.txt")
	if err := os.WriteFile(testFile, []byte("main content"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	cmd := exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add conflict file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on main")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on main: %v", err)
	}

	// Create feature branch from before the main commit
	cmd = exec.Command("git", "checkout", "-b", "feature-retry", "HEAD~1")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	// Create conflicting content
	if err := os.WriteFile(testFile, []byte("feature content"), 0644); err != nil {
		t.Fatalf("failed to modify conflict file: %v", err)
	}

	cmd = exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add modified file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on feature")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on feature: %v", err)
	}

	// Start rebase to create conflict
	cmd = exec.Command("git", "rebase", "main")
	cmd.Dir = repoPath
	_ = cmd.Run() // Expected to fail with conflict

	// Mock Claude client that fails first, then succeeds
	attemptCount := 0
	mock := &mockClaudeClient{
		resolveFunc: func(ctx context.Context, opts InvokeOptions) (string, error) {
			attemptCount++
			if attemptCount == 1 {
				// First attempt: fail
				return "", fmt.Errorf("first attempt fails")
			}
			// Second attempt: resolve
			conflicts, _ := getConflictedFiles(ctx, repoPath)
			for _, f := range conflicts {
				filePath := filepath.Join(repoPath, f)
				if err := os.WriteFile(filePath, []byte("resolved content"), 0644); err != nil {
					return "", err
				}
				cmd := exec.Command("git", "add", f)
				cmd.Dir = repoPath
				if err := cmd.Run(); err != nil {
					return "", err
				}
			}
			return "", nil
		},
	}

	manager := NewMergeManager(repoPath, mock)

	// Resolve conflicts (should retry and succeed on second attempt)
	err := manager.ResolveConflicts(ctx, repoPath)
	if err != nil {
		t.Fatalf("ResolveConflicts failed: %v", err)
	}

	// Verify Claude was called twice
	if mock.callCount != 2 {
		t.Errorf("expected Claude to be called 2 times, got %d", mock.callCount)
	}
}

func TestResolveConflicts_MaxAttempts(t *testing.T) {
	repoPath, _ := setupTestRepoWithRemote(t)
	ctx := context.Background()

	// Create a conflict scenario
	testFile := filepath.Join(repoPath, "conflict.txt")
	if err := os.WriteFile(testFile, []byte("main content"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	cmd := exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add conflict file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on main")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on main: %v", err)
	}

	// Create feature branch from before the main commit
	cmd = exec.Command("git", "checkout", "-b", "feature-maxattempts", "HEAD~1")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	// Create conflicting content
	if err := os.WriteFile(testFile, []byte("feature content"), 0644); err != nil {
		t.Fatalf("failed to modify conflict file: %v", err)
	}

	cmd = exec.Command("git", "add", "conflict.txt")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add modified file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add conflict file on feature")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on feature: %v", err)
	}

	// Start rebase to create conflict
	cmd = exec.Command("git", "rebase", "main")
	cmd.Dir = repoPath
	_ = cmd.Run() // Expected to fail with conflict

	// Mock Claude client that always fails
	mock := &mockClaudeClient{
		resolveFunc: func(ctx context.Context, opts InvokeOptions) (string, error) {
			return "", fmt.Errorf("resolution always fails")
		},
	}

	manager := NewMergeManager(repoPath, mock)

	// Resolve conflicts (should fail after MaxConflictAttempts)
	err := manager.ResolveConflicts(ctx, repoPath)
	if err == nil {
		t.Fatal("expected ResolveConflicts to fail after max attempts")
	}

	// Verify error message mentions max attempts
	if !strings.Contains(err.Error(), "3 attempts") {
		t.Errorf("expected error to mention 3 attempts, got: %v", err)
	}

	// Verify Claude was called 3 times (MaxConflictAttempts)
	if mock.callCount != 3 {
		t.Errorf("expected Claude to be called 3 times, got %d", mock.callCount)
	}

	// Clean up: abort the rebase
	if err := abortRebase(ctx, repoPath); err != nil {
		t.Fatalf("failed to abort rebase: %v", err)
	}
}

func TestResolveConflicts_NoConflicts(t *testing.T) {
	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Mock Claude client (should not be called)
	mock := &mockClaudeClient{
		resolveFunc: func(ctx context.Context, opts InvokeOptions) (string, error) {
			t.Error("Claude should not be called when there are no conflicts")
			return "", nil
		},
	}

	manager := NewMergeManager(repoPath, mock)

	// Resolve conflicts when there are no conflicts
	err := manager.ResolveConflicts(ctx, repoPath)
	if err != nil {
		t.Fatalf("ResolveConflicts failed: %v", err)
	}

	// Verify Claude was never called
	if mock.callCount != 0 {
		t.Errorf("expected Claude to be called 0 times, got %d", mock.callCount)
	}
}

func TestBuildConflictPrompt(t *testing.T) {
	conflicts := []string{"file1.txt", "file2.go", "file3.md"}
	worktreePath := "/tmp/test-worktree"

	prompt := buildConflictPrompt(conflicts, worktreePath)

	// Verify prompt includes worktree path
	if !strings.Contains(prompt, worktreePath) {
		t.Error("expected prompt to contain worktree path")
	}

	// Verify prompt includes all conflicted files
	for _, file := range conflicts {
		if !strings.Contains(prompt, file) {
			t.Errorf("expected prompt to contain file %s", file)
		}
	}

	// Verify prompt includes instructions
	if !strings.Contains(prompt, "conflict markers") {
		t.Error("expected prompt to mention conflict markers")
	}

	if !strings.Contains(prompt, "git add") {
		t.Error("expected prompt to mention git add")
	}
}

func TestBuildConflictPrompt_Content(t *testing.T) {
	conflicts := []string{"test.txt"}
	worktreePath := "/path/to/worktree"

	prompt := buildConflictPrompt(conflicts, worktreePath)

	// Verify prompt includes conflict marker symbols
	expectedMarkers := []string{"<<<<<<<", "=======", ">>>>>>>"}
	for _, marker := range expectedMarkers {
		if !strings.Contains(prompt, marker) {
			t.Errorf("expected prompt to contain conflict marker %s", marker)
		}
	}

	// Verify prompt has all the required steps
	requiredSteps := []string{
		"Reading the file",
		"Choosing the correct resolution",
		"Removing all conflict markers",
		"Saving the resolved file",
		"Staging the file with git add",
	}

	for _, step := range requiredSteps {
		if !strings.Contains(prompt, step) {
			t.Errorf("expected prompt to contain step: %s", step)
		}
	}
}

func TestReadConflictFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "conflict.txt")

	conflictContent := `line 1
<<<<<<< HEAD
main content
=======
feature content
>>>>>>> feature-branch
line 2`

	if err := os.WriteFile(testFile, []byte(conflictContent), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	content, err := readConflictFile(testFile)
	if err != nil {
		t.Fatalf("readConflictFile failed: %v", err)
	}

	if content != conflictContent {
		t.Errorf("expected content to match, got:\n%s", content)
	}

	// Verify conflict markers are present
	if !strings.Contains(content, "<<<<<<<") {
		t.Error("expected content to contain <<<<<<< marker")
	}

	if !strings.Contains(content, "=======") {
		t.Error("expected content to contain ======= marker")
	}

	if !strings.Contains(content, ">>>>>>>") {
		t.Error("expected content to contain >>>>>>> marker")
	}
}

func TestReadConflictFile_NonExistent(t *testing.T) {
	_, err := readConflictFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected readConflictFile to fail for nonexistent file")
	}
}
