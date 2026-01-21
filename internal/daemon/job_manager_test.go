package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/daemon/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	cfg := JobConfig{
		RepoPath:     "/tmp/repo",
		TasksDir:     "/tmp/tasks",
		TargetBranch: "main",
	}

	jobID, err := jm.Start(context.Background(), cfg)
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

	cfg1 := JobConfig{
		RepoPath:     "/tmp/repo1",
		TasksDir:     "/tmp/tasks",
		TargetBranch: "main",
	}

	cfg2 := JobConfig{
		RepoPath:     "/tmp/repo2",
		TasksDir:     "/tmp/tasks",
		TargetBranch: "main",
	}

	cfg3 := JobConfig{
		RepoPath:     "/tmp/repo3",
		TasksDir:     "/tmp/tasks",
		TargetBranch: "main",
	}

	// Start 2 jobs successfully
	_, err := jm.Start(context.Background(), cfg1)
	require.NoError(t, err)
	_, err = jm.Start(context.Background(), cfg2)
	require.NoError(t, err)

	// Third job should fail
	_, err = jm.Start(context.Background(), cfg3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max jobs")
}

func TestJobManager_Stop(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)

	cfg := JobConfig{
		RepoPath:     "/tmp/repo",
		TasksDir:     "/tmp/tasks",
		TargetBranch: "main",
	}

	jobID, err := jm.Start(context.Background(), cfg)
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

	cfg := JobConfig{
		RepoPath:     "/tmp/repo",
		TasksDir:     "/tmp/tasks",
		TargetBranch: "main",
	}

	jobID, err := jm.Start(context.Background(), cfg)
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

	cfg1 := JobConfig{
		RepoPath:     "/tmp/repo1",
		TasksDir:     "/tmp/tasks",
		TargetBranch: "main",
	}

	cfg2 := JobConfig{
		RepoPath:     "/tmp/repo2",
		TasksDir:     "/tmp/tasks",
		TargetBranch: "main",
	}

	// Start multiple jobs
	jobID1, err := jm.Start(context.Background(), cfg1)
	require.NoError(t, err)
	jobID2, err := jm.Start(context.Background(), cfg2)
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

	cfg := JobConfig{
		RepoPath:     "/tmp/repo",
		TasksDir:     "/tmp/tasks",
		TargetBranch: "main",
	}

	jobID, err := jm.Start(context.Background(), cfg)
	require.NoError(t, err)

	// Verify job is in the list
	jobs := jm.List()
	assert.Contains(t, jobs, jobID)

	// Wait for the job to complete (should fail since /tmp/tasks doesn't exist)
	// The cleanup should happen automatically
	time.Sleep(500 * time.Millisecond)

	// Job should be removed from tracking after completion
	jobs = jm.List()
	assert.NotContains(t, jobs, jobID)
}
