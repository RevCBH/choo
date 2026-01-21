package db

import (
	"testing"
)

// TestOpen verifies that opening an in-memory database works without error
func TestOpen(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()
}

// TestOpenWALMode verifies that WAL mode is enabled after open
func TestOpenWALMode(t *testing.T) {
	// Use a temporary file for WAL mode test (in-memory databases don't support WAL)
	tmpDB := t.TempDir() + "/test.db"
	db, err := Open(tmpDB)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	var journalMode string
	err = db.conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("Failed to query journal_mode: %v", err)
	}

	if journalMode != "wal" {
		t.Errorf("Expected WAL mode, got %s", journalMode)
	}
}

// TestOpenForeignKeys verifies that foreign keys are enabled after open
func TestOpenForeignKeys(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	var foreignKeys int
	err = db.conn.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
	if err != nil {
		t.Fatalf("Failed to query foreign_keys: %v", err)
	}

	if foreignKeys != 1 {
		t.Errorf("Expected foreign keys enabled (1), got %d", foreignKeys)
	}
}

// TestOpenMigration verifies that all tables exist after open
func TestOpenMigration(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	tables := []string{"runs", "units", "events"}
	for _, table := range tables {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
		err = db.conn.QueryRow(query, table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s does not exist: %v", table, err)
		}
		if name != table {
			t.Errorf("Expected table %s, got %s", table, name)
		}
	}
}

// TestClose verifies that Close returns no error
func TestClose(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestRunCreate verifies that CreateRun inserts and GetRun retrieves with matching fields
func TestRunCreate(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	run := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test",
		RepoPath:      "/path/to/repo",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusPending,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}

	err = db.CreateRun(run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	retrieved, err := db.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetRun returned nil")
	}

	if retrieved.ID != run.ID {
		t.Errorf("Expected ID %s, got %s", run.ID, retrieved.ID)
	}
	if retrieved.FeatureBranch != run.FeatureBranch {
		t.Errorf("Expected FeatureBranch %s, got %s", run.FeatureBranch, retrieved.FeatureBranch)
	}
	if retrieved.RepoPath != run.RepoPath {
		t.Errorf("Expected RepoPath %s, got %s", run.RepoPath, retrieved.RepoPath)
	}
	if retrieved.Status != run.Status {
		t.Errorf("Expected Status %s, got %s", run.Status, retrieved.Status)
	}
}

// TestRunCreateDuplicate verifies that CreateRun returns error for same branch/repo
func TestRunCreateDuplicate(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	run1 := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test",
		RepoPath:      "/path/to/repo",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusPending,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}

	err = db.CreateRun(run1)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	run2 := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test",
		RepoPath:      "/path/to/repo",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusPending,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}

	err = db.CreateRun(run2)
	if err == nil {
		t.Fatal("Expected error for duplicate run, got nil")
	}
}

// TestRunGetByBranch verifies that GetRunByBranch finds run by branch and repo path
func TestRunGetByBranch(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	run := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test",
		RepoPath:      "/path/to/repo",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusPending,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}

	err = db.CreateRun(run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	retrieved, err := db.GetRunByBranch("feature/test", "/path/to/repo")
	if err != nil {
		t.Fatalf("GetRunByBranch failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetRunByBranch returned nil")
	}

	if retrieved.ID != run.ID {
		t.Errorf("Expected ID %s, got %s", run.ID, retrieved.ID)
	}
}

// TestRunGetNotFound verifies that GetRun returns nil, nil for non-existent ID
func TestRunGetNotFound(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	retrieved, err := db.GetRun("nonexistent")
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved != nil {
		t.Errorf("Expected nil run, got %v", retrieved)
	}
}

// TestRunUpdateStatus verifies that UpdateRunStatus changes status and sets timestamps
func TestRunUpdateStatus(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	run := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test",
		RepoPath:      "/path/to/repo",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusPending,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}

	err = db.CreateRun(run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Update to Running should set started_at
	err = db.UpdateRunStatus(run.ID, RunStatusRunning, nil)
	if err != nil {
		t.Fatalf("UpdateRunStatus failed: %v", err)
	}

	retrieved, err := db.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved.Status != RunStatusRunning {
		t.Errorf("Expected status Running, got %s", retrieved.Status)
	}
	if retrieved.StartedAt == nil {
		t.Error("Expected started_at to be set")
	}

	// Update to Completed should set completed_at
	err = db.UpdateRunStatus(run.ID, RunStatusCompleted, nil)
	if err != nil {
		t.Fatalf("UpdateRunStatus failed: %v", err)
	}

	retrieved, err = db.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved.Status != RunStatusCompleted {
		t.Errorf("Expected status Completed, got %s", retrieved.Status)
	}
	if retrieved.CompletedAt == nil {
		t.Error("Expected completed_at to be set")
	}
}

// TestRunUpdateStatusWithError verifies that UpdateRunStatus stores error message
func TestRunUpdateStatusWithError(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	run := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test",
		RepoPath:      "/path/to/repo",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusPending,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}

	err = db.CreateRun(run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	errorMsg := "test error message"
	err = db.UpdateRunStatus(run.ID, RunStatusFailed, &errorMsg)
	if err != nil {
		t.Fatalf("UpdateRunStatus failed: %v", err)
	}

	retrieved, err := db.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved.Status != RunStatusFailed {
		t.Errorf("Expected status Failed, got %s", retrieved.Status)
	}
	if retrieved.Error == nil {
		t.Fatal("Expected error to be set")
	}
	if *retrieved.Error != errorMsg {
		t.Errorf("Expected error %s, got %s", errorMsg, *retrieved.Error)
	}
}

// TestRunListByStatus verifies that ListRunsByStatus returns only matching runs
func TestRunListByStatus(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create runs with different statuses
	run1 := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test1",
		RepoPath:      "/path/to/repo1",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusPending,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}
	run2 := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test2",
		RepoPath:      "/path/to/repo2",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusRunning,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}
	run3 := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test3",
		RepoPath:      "/path/to/repo3",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusPending,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}

	err = db.CreateRun(run1)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}
	err = db.CreateRun(run2)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}
	err = db.CreateRun(run3)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// List pending runs
	pending, err := db.ListRunsByStatus(RunStatusPending)
	if err != nil {
		t.Fatalf("ListRunsByStatus failed: %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("Expected 2 pending runs, got %d", len(pending))
	}

	// List running runs
	running, err := db.ListRunsByStatus(RunStatusRunning)
	if err != nil {
		t.Fatalf("ListRunsByStatus failed: %v", err)
	}

	if len(running) != 1 {
		t.Errorf("Expected 1 running run, got %d", len(running))
	}
}

// TestRunListIncomplete verifies that ListIncompleteRuns returns pending and running runs
func TestRunListIncomplete(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create runs with different statuses
	run1 := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test1",
		RepoPath:      "/path/to/repo1",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusPending,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}
	run2 := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test2",
		RepoPath:      "/path/to/repo2",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusRunning,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}
	run3 := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test3",
		RepoPath:      "/path/to/repo3",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusCompleted,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}

	err = db.CreateRun(run1)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}
	err = db.CreateRun(run2)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}
	err = db.CreateRun(run3)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// List incomplete runs
	incomplete, err := db.ListIncompleteRuns()
	if err != nil {
		t.Fatalf("ListIncompleteRuns failed: %v", err)
	}

	if len(incomplete) != 2 {
		t.Errorf("Expected 2 incomplete runs, got %d", len(incomplete))
	}

	// Verify that they are the pending and running ones
	statuses := make(map[RunStatus]bool)
	for _, r := range incomplete {
		statuses[r.Status] = true
	}

	if !statuses[RunStatusPending] || !statuses[RunStatusRunning] {
		t.Error("Expected incomplete runs to be pending and running")
	}
}

// TestRunDelete verifies that DeleteRun removes run from database
func TestRunDelete(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	run := &Run{
		ID:            NewRunID(),
		FeatureBranch: "feature/test",
		RepoPath:      "/path/to/repo",
		TargetBranch:  "main",
		TasksDir:      "/path/to/tasks",
		Parallelism:   4,
		Status:        RunStatusPending,
		DaemonVersion: "1.0.0",
		ConfigJSON:    "{}",
	}

	err = db.CreateRun(run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	err = db.DeleteRun(run.ID)
	if err != nil {
		t.Fatalf("DeleteRun failed: %v", err)
	}

	// Verify run no longer exists
	retrieved, err := db.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved != nil {
		t.Errorf("Expected nil run after delete, got %v", retrieved)
	}
}
