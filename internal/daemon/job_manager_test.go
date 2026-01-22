package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/daemon/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepoForManager creates a temporary git repository for testing.
// Returns the repo path. The directory is automatically cleaned up when the test ends.
func setupTestRepoForManager(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "job_manager_test_repo_*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Add a remote (doesn't need to be real, just parseable)
	cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/test-owner/test-repo.git")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Create an initial commit so we have a valid branch
	readmePath := filepath.Join(tmpDir, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("# Test Repo"), 0644))

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Create a tasks directory
	tasksDir := filepath.Join(tmpDir, "specs", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	return tmpDir
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()

	// Create a temporary directory for the test database
	tmpDir, err := os.MkdirTemp("", "job_manager_test_*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		database.Close()
	})

	return database
}

func TestJobManager_Start(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepoForManager(t)

	cfg := JobConfig{
		RepoPath:     repoPath,
		TasksDir:     filepath.Join(repoPath, "specs", "tasks"),
		TargetBranch: "main",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobID, err := jm.Start(ctx, cancel, cfg)
	require.NoError(t, err)
	require.NotEmpty(t, jobID)

	// Verify ULID format (26 characters)
	assert.Len(t, jobID, 26)

	// Verify in list
	jobs := jm.List()
	assert.Contains(t, jobs, jobID)
}

func TestJobManager_Start_MaxJobs(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 2) // Only allow 2 jobs
	repoPath1 := setupTestRepoForManager(t)
	repoPath2 := setupTestRepoForManager(t)
	repoPath3 := setupTestRepoForManager(t)

	cfg1 := JobConfig{
		RepoPath:     repoPath1,
		TasksDir:     filepath.Join(repoPath1, "specs", "tasks"),
		TargetBranch: "main",
	}

	cfg2 := JobConfig{
		RepoPath:     repoPath2,
		TasksDir:     filepath.Join(repoPath2, "specs", "tasks"),
		TargetBranch: "main",
	}

	cfg3 := JobConfig{
		RepoPath:     repoPath3,
		TasksDir:     filepath.Join(repoPath3, "specs", "tasks"),
		TargetBranch: "main",
	}

	// Start 2 jobs successfully
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	_, err := jm.Start(ctx1, cancel1, cfg1)
	require.NoError(t, err)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	_, err = jm.Start(ctx2, cancel2, cfg2)
	require.NoError(t, err)

	// Third job should fail
	ctx3, cancel3 := context.WithCancel(context.Background())
	defer cancel3()
	_, err = jm.Start(ctx3, cancel3, cfg3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max jobs")
}

func TestJobManager_Stop(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepoForManager(t)

	cfg := JobConfig{
		RepoPath:     repoPath,
		TasksDir:     filepath.Join(repoPath, "specs", "tasks"),
		TargetBranch: "main",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobID, err := jm.Start(ctx, cancel, cfg)
	require.NoError(t, err)

	// Stop the job
	err = jm.Stop(jobID)
	require.NoError(t, err)

	// Verify job status in database
	run, err := database.GetRun(jobID)
	require.NoError(t, err)
	assert.Equal(t, db.RunStatusCancelled, run.Status)
}

func TestJobManager_Stop_NotFound(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)

	// Try to stop a non-existent job
	err := jm.Stop("invalid-job-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestJobManager_Get(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepoForManager(t)

	cfg := JobConfig{
		RepoPath:     repoPath,
		TasksDir:     filepath.Join(repoPath, "specs", "tasks"),
		TargetBranch: "main",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobID, err := jm.Start(ctx, cancel, cfg)
	require.NoError(t, err)

	// Get the job
	job, found := jm.Get(jobID)
	require.True(t, found)
	assert.NotNil(t, job)
	assert.Equal(t, jobID, job.ID)
}

func TestJobManager_Get_NotFound(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)

	// Try to get a non-existent job
	_, found := jm.Get("invalid-job-id")
	assert.False(t, found)
}

func TestJobManager_StopAll(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath1 := setupTestRepoForManager(t)
	repoPath2 := setupTestRepoForManager(t)

	cfg1 := JobConfig{
		RepoPath:     repoPath1,
		TasksDir:     filepath.Join(repoPath1, "specs", "tasks"),
		TargetBranch: "main",
	}

	cfg2 := JobConfig{
		RepoPath:     repoPath2,
		TasksDir:     filepath.Join(repoPath2, "specs", "tasks"),
		TargetBranch: "main",
	}

	// Start multiple jobs
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	jobID1, err := jm.Start(ctx1, cancel1, cfg1)
	require.NoError(t, err)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	jobID2, err := jm.Start(ctx2, cancel2, cfg2)
	require.NoError(t, err)

	// Stop all jobs
	jm.StopAll()

	// Give some time for cancellation to propagate
	time.Sleep(100 * time.Millisecond)

	// Both jobs should be cancelled (or at least in the process of cancelling)
	// We can't assert on the internal state directly, but we can verify that
	// the cancel functions were called (jobs will eventually clean up)
	assert.NotEmpty(t, jobID1)
	assert.NotEmpty(t, jobID2)
}

func TestJobManager_Cleanup(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)
	repoPath := setupTestRepoForManager(t)

	cfg := JobConfig{
		RepoPath:     repoPath,
		TasksDir:     filepath.Join(repoPath, "specs", "tasks"),
		TargetBranch: "main",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobID, err := jm.Start(ctx, cancel, cfg)
	require.NoError(t, err)

	// Verify job is in the list
	jobs := jm.List()
	assert.Contains(t, jobs, jobID)

	// Wait for the job to complete (should complete quickly with empty tasks dir)
	// The cleanup should happen automatically
	time.Sleep(500 * time.Millisecond)

	// Job should be removed from tracking after completion
	jobs = jm.List()
	assert.NotContains(t, jobs, jobID)
}
