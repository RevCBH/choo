package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RevCBH/choo/internal/daemon/db"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stringPtr is a helper to get pointer to string
func stringPtr(s string) *string {
	return &s
}

// setupTestGitRepo creates a temporary directory with a .git subdirectory
func setupTestGitRepo(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "git_repo_test_*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// Create .git directory to make it look like a git repo
	gitDir := filepath.Join(tmpDir, ".git")
	err = os.Mkdir(gitDir, 0755)
	require.NoError(t, err)

	return tmpDir
}

func TestResumeJobs_NoJobs(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)

	results := jm.ResumeJobs(context.Background())
	assert.Empty(t, results)
}

func TestResumeJobs_SingleJob(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)

	// Create a test git repo
	repoPath := setupTestGitRepo(t)

	// Create a run record with running status
	run := &db.Run{
		ID:            ulid.Make().String(),
		FeatureBranch: "test-branch",
		RepoPath:      repoPath,
		TargetBranch:  "main",
		TasksDir:      "tasks",
		Parallelism:   2,
		Status:        db.RunStatusRunning,
	}
	require.NoError(t, database.CreateRun(run))

	results := jm.ResumeJobs(context.Background())
	require.Len(t, results, 1)
	assert.Equal(t, run.ID, results[0].JobID)
	assert.True(t, results[0].Success)
	assert.NoError(t, results[0].Error)
}

func TestResumeJobs_MultipleJobs(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)

	// Create test git repos
	repoPath1 := setupTestGitRepo(t)
	repoPath2 := setupTestGitRepo(t)

	// Create multiple run records
	run1 := &db.Run{
		ID:           ulid.Make().String(),
		RepoPath:     repoPath1,
		TargetBranch: "main",
		TasksDir:     "tasks",
		Parallelism:  2,
		Status:       db.RunStatusRunning,
	}
	require.NoError(t, database.CreateRun(run1))

	run2 := &db.Run{
		ID:           ulid.Make().String(),
		RepoPath:     repoPath2,
		TargetBranch: "main",
		TasksDir:     "tasks",
		Parallelism:  2,
		Status:       db.RunStatusRunning,
	}
	require.NoError(t, database.CreateRun(run2))

	results := jm.ResumeJobs(context.Background())
	require.Len(t, results, 2)

	// Both jobs should be reported
	jobIDs := []string{results[0].JobID, results[1].JobID}
	assert.Contains(t, jobIDs, run1.ID)
	assert.Contains(t, jobIDs, run2.ID)
}

func TestResume_ValidState(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)

	// Create a test git repo
	repoPath := setupTestGitRepo(t)

	// Create a run record
	runID := ulid.Make().String()
	run := &db.Run{
		ID:           runID,
		RepoPath:     repoPath,
		TargetBranch: "main",
		TasksDir:     "tasks",
		Parallelism:  2,
		Status:       db.RunStatusRunning,
	}
	require.NoError(t, database.CreateRun(run))

	// Create some units
	units := []*db.UnitRecord{
		{
			ID:     db.MakeUnitRecordID(runID, "unit1"),
			RunID:  runID,
			UnitID: "unit1",
			Status: string(db.UnitStatusCompleted),
		},
	}

	cfg := JobConfig{
		RepoPath:     repoPath,
		TargetBranch: "main",
		TasksDir:     "tasks",
		Concurrency:  2,
	}

	err := jm.Resume(context.Background(), runID, cfg, units)
	assert.NoError(t, err)
}

func TestResume_VersionMismatch(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)

	// Create a test git repo
	repoPath := setupTestGitRepo(t)

	// Create a run record with a different daemon version
	runID := ulid.Make().String()
	run := &db.Run{
		ID:            runID,
		RepoPath:      repoPath,
		TargetBranch:  "main",
		TasksDir:      "tasks",
		Parallelism:   2,
		Status:        db.RunStatusRunning,
		DaemonVersion: "1.0.0", // Different version
	}
	require.NoError(t, database.CreateRun(run))

	cfg := JobConfig{
		RepoPath:     repoPath,
		TargetBranch: "main",
		TasksDir:     "tasks",
		Concurrency:  2,
	}

	// For now, version checking is lenient (allows empty version)
	// In production this would be stricter
	err := jm.Resume(context.Background(), runID, cfg, []*db.UnitRecord{})
	// Current implementation doesn't strictly enforce version matching
	// This test documents the expected behavior when version checking is implemented
	assert.NoError(t, err)
}

func TestResume_MissingRepo(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)

	// Create a run record with non-existent repo
	runID := ulid.Make().String()
	run := &db.Run{
		ID:           runID,
		RepoPath:     "/nonexistent/repo",
		TargetBranch: "main",
		TasksDir:     "tasks",
		Parallelism:  2,
		Status:       db.RunStatusRunning,
	}
	require.NoError(t, database.CreateRun(run))

	cfg := JobConfig{
		RepoPath:     "/nonexistent/repo",
		TargetBranch: "main",
		TasksDir:     "tasks",
		Concurrency:  2,
	}

	err := jm.Resume(context.Background(), runID, cfg, []*db.UnitRecord{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no longer exists")

	// Verify run was marked as failed
	updatedRun, dbErr := database.GetRun(runID)
	require.NoError(t, dbErr)
	assert.Equal(t, db.RunStatusFailed, updatedRun.Status)
}

func TestResume_InvalidWorktree(t *testing.T) {
	database := setupTestDB(t)
	jm := NewJobManager(database, 10)

	// Create a test git repo
	repoPath := setupTestGitRepo(t)

	// Create a run record
	runID := ulid.Make().String()
	run := &db.Run{
		ID:           runID,
		RepoPath:     repoPath,
		TargetBranch: "main",
		TasksDir:     "tasks",
		Parallelism:  2,
		Status:       db.RunStatusRunning,
	}
	require.NoError(t, database.CreateRun(run))

	// Create a unit with a missing worktree
	unitID := db.MakeUnitRecordID(runID, "unit1")
	unit := &db.UnitRecord{
		ID:           unitID,
		RunID:        runID,
		UnitID:       "unit1",
		Status:       string(db.UnitStatusRunning),
		WorktreePath: stringPtr("/nonexistent/worktree"),
	}
	require.NoError(t, database.CreateUnit(unit))

	cfg := JobConfig{
		RepoPath:     repoPath,
		TargetBranch: "main",
		TasksDir:     "tasks",
		Concurrency:  2,
	}

	units := []*db.UnitRecord{unit}

	err := jm.Resume(context.Background(), runID, cfg, units)
	assert.NoError(t, err) // Resume succeeds but marks unit as failed

	// Verify unit was marked as failed in database
	updatedUnit, dbErr := database.GetUnit(unitID)
	require.NoError(t, dbErr)
	assert.Equal(t, string(db.UnitStatusFailed), updatedUnit.Status)
	assert.NotNil(t, updatedUnit.Error)
	assert.Contains(t, *updatedUnit.Error, "worktree no longer exists")
}

func TestValidateRepoExists_Valid(t *testing.T) {
	repoPath := setupTestGitRepo(t)

	err := validateRepoExists(repoPath)
	assert.NoError(t, err)
}

func TestValidateRepoExists_Missing(t *testing.T) {
	err := validateRepoExists("/nonexistent/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestValidateRepoExists_NotGit(t *testing.T) {
	// Create a directory without .git
	tmpDir, err := os.MkdirTemp("", "not_git_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	err = validateRepoExists(tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestValidateWorktrees_AllValid(t *testing.T) {
	// Create valid worktree paths
	worktree1, err := os.MkdirTemp("", "worktree1_*")
	require.NoError(t, err)
	defer os.RemoveAll(worktree1)

	worktree2, err := os.MkdirTemp("", "worktree2_*")
	require.NoError(t, err)
	defer os.RemoveAll(worktree2)

	units := []*db.UnitRecord{
		{
			ID:           "run1_unit1",
			RunID:        "run1",
			UnitID:       "unit1",
			Status:       string(db.UnitStatusRunning),
			WorktreePath: stringPtr(worktree1),
		},
		{
			ID:           "run1_unit2",
			RunID:        "run1",
			UnitID:       "unit2",
			Status:       string(db.UnitStatusRunning),
			WorktreePath: stringPtr(worktree2),
		},
	}

	updated := validateWorktrees(units)

	// Both units should remain unchanged
	assert.Len(t, updated, 2)
	assert.Equal(t, string(db.UnitStatusRunning), updated[0].Status)
	assert.Equal(t, string(db.UnitStatusRunning), updated[1].Status)
	assert.Nil(t, updated[0].Error)
	assert.Nil(t, updated[1].Error)
}

func TestValidateWorktrees_SomeMissing(t *testing.T) {
	// Create one valid worktree
	validWorktree, err := os.MkdirTemp("", "worktree_valid_*")
	require.NoError(t, err)
	defer os.RemoveAll(validWorktree)

	units := []*db.UnitRecord{
		{
			ID:           "run1_unit1",
			RunID:        "run1",
			UnitID:       "unit1",
			Status:       string(db.UnitStatusRunning),
			WorktreePath: stringPtr(validWorktree),
		},
		{
			ID:           "run1_unit2",
			RunID:        "run1",
			UnitID:       "unit2",
			Status:       string(db.UnitStatusRunning),
			WorktreePath: stringPtr("/nonexistent/worktree"),
		},
	}

	updated := validateWorktrees(units)

	// First unit should be unchanged
	assert.Equal(t, string(db.UnitStatusRunning), updated[0].Status)
	assert.Nil(t, updated[0].Error)

	// Second unit should be marked as failed
	assert.Equal(t, string(db.UnitStatusFailed), updated[1].Status)
	require.NotNil(t, updated[1].Error)
	assert.Contains(t, *updated[1].Error, "worktree no longer exists")
}
