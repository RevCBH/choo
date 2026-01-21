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

// TestUnitCreate verifies that CreateUnit inserts and GetUnit retrieves with matching fields
func TestUnitCreate(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create a parent run first
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

	// Create a unit
	unitID := "task-1"
	unit := &UnitRecord{
		ID:     MakeUnitRecordID(run.ID, unitID),
		RunID:  run.ID,
		UnitID: unitID,
		Status: string(UnitStatusPending),
	}

	err = db.CreateUnit(unit)
	if err != nil {
		t.Fatalf("CreateUnit failed: %v", err)
	}

	// Retrieve the unit
	retrieved, err := db.GetUnit(unit.ID)
	if err != nil {
		t.Fatalf("GetUnit failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetUnit returned nil")
	}

	if retrieved.ID != unit.ID {
		t.Errorf("Expected ID %s, got %s", unit.ID, retrieved.ID)
	}
	if retrieved.RunID != unit.RunID {
		t.Errorf("Expected RunID %s, got %s", unit.RunID, retrieved.RunID)
	}
	if retrieved.UnitID != unit.UnitID {
		t.Errorf("Expected UnitID %s, got %s", unit.UnitID, retrieved.UnitID)
	}
	if retrieved.Status != unit.Status {
		t.Errorf("Expected Status %s, got %s", unit.Status, retrieved.Status)
	}
}

// TestUnitCreateWithoutRun verifies that CreateUnit returns foreign key error for non-existent run
func TestUnitCreateWithoutRun(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Try to create a unit without a parent run
	unitID := "task-1"
	runID := "nonexistent-run-id"
	unit := &UnitRecord{
		ID:     MakeUnitRecordID(runID, unitID),
		RunID:  runID,
		UnitID: unitID,
		Status: string(UnitStatusPending),
	}

	err = db.CreateUnit(unit)
	if err == nil {
		t.Fatal("Expected error for unit without parent run, got nil")
	}
}

// TestUnitGetNotFound verifies that GetUnit returns nil, nil for non-existent ID
func TestUnitGetNotFound(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	retrieved, err := db.GetUnit("nonexistent")
	if err != nil {
		t.Fatalf("GetUnit failed: %v", err)
	}

	if retrieved != nil {
		t.Errorf("Expected nil unit, got %v", retrieved)
	}
}

// TestUnitUpdateStatus verifies that UpdateUnitStatus changes status and sets timestamps
func TestUnitUpdateStatus(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create a parent run
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

	// Create a unit
	unitID := "task-1"
	unit := &UnitRecord{
		ID:     MakeUnitRecordID(run.ID, unitID),
		RunID:  run.ID,
		UnitID: unitID,
		Status: string(UnitStatusPending),
	}

	err = db.CreateUnit(unit)
	if err != nil {
		t.Fatalf("CreateUnit failed: %v", err)
	}

	// Update to Running should set started_at
	err = db.UpdateUnitStatus(unit.ID, UnitStatusRunning, nil)
	if err != nil {
		t.Fatalf("UpdateUnitStatus failed: %v", err)
	}

	retrieved, err := db.GetUnit(unit.ID)
	if err != nil {
		t.Fatalf("GetUnit failed: %v", err)
	}

	if retrieved.Status != string(UnitStatusRunning) {
		t.Errorf("Expected status Running, got %s", retrieved.Status)
	}
	if retrieved.StartedAt == nil {
		t.Error("Expected started_at to be set")
	}

	// Update to Completed should set completed_at
	err = db.UpdateUnitStatus(unit.ID, UnitStatusCompleted, nil)
	if err != nil {
		t.Fatalf("UpdateUnitStatus failed: %v", err)
	}

	retrieved, err = db.GetUnit(unit.ID)
	if err != nil {
		t.Fatalf("GetUnit failed: %v", err)
	}

	if retrieved.Status != string(UnitStatusCompleted) {
		t.Errorf("Expected status Completed, got %s", retrieved.Status)
	}
	if retrieved.CompletedAt == nil {
		t.Error("Expected completed_at to be set")
	}
}

// TestUnitUpdateStatusWithError verifies that UpdateUnitStatus stores error message
func TestUnitUpdateStatusWithError(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create a parent run
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

	// Create a unit
	unitID := "task-1"
	unit := &UnitRecord{
		ID:     MakeUnitRecordID(run.ID, unitID),
		RunID:  run.ID,
		UnitID: unitID,
		Status: string(UnitStatusPending),
	}

	err = db.CreateUnit(unit)
	if err != nil {
		t.Fatalf("CreateUnit failed: %v", err)
	}

	// Update status with error
	errorMsg := "test error message"
	err = db.UpdateUnitStatus(unit.ID, UnitStatusFailed, &errorMsg)
	if err != nil {
		t.Fatalf("UpdateUnitStatus failed: %v", err)
	}

	retrieved, err := db.GetUnit(unit.ID)
	if err != nil {
		t.Fatalf("GetUnit failed: %v", err)
	}

	if retrieved.Status != string(UnitStatusFailed) {
		t.Errorf("Expected status Failed, got %s", retrieved.Status)
	}
	if retrieved.Error == nil {
		t.Fatal("Expected error to be set")
	}
	if *retrieved.Error != errorMsg {
		t.Errorf("Expected error %s, got %s", errorMsg, *retrieved.Error)
	}
}

// TestUnitUpdateBranch verifies that UpdateUnitBranch sets branch and worktree path
func TestUnitUpdateBranch(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create a parent run
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

	// Create a unit
	unitID := "task-1"
	unit := &UnitRecord{
		ID:     MakeUnitRecordID(run.ID, unitID),
		RunID:  run.ID,
		UnitID: unitID,
		Status: string(UnitStatusPending),
	}

	err = db.CreateUnit(unit)
	if err != nil {
		t.Fatalf("CreateUnit failed: %v", err)
	}

	// Update branch and worktree path
	branch := "ralph/task-1"
	worktreePath := "/path/to/worktree"
	err = db.UpdateUnitBranch(unit.ID, branch, worktreePath)
	if err != nil {
		t.Fatalf("UpdateUnitBranch failed: %v", err)
	}

	retrieved, err := db.GetUnit(unit.ID)
	if err != nil {
		t.Fatalf("GetUnit failed: %v", err)
	}

	if retrieved.Branch == nil {
		t.Fatal("Expected branch to be set")
	}
	if *retrieved.Branch != branch {
		t.Errorf("Expected branch %s, got %s", branch, *retrieved.Branch)
	}
	if retrieved.WorktreePath == nil {
		t.Fatal("Expected worktree_path to be set")
	}
	if *retrieved.WorktreePath != worktreePath {
		t.Errorf("Expected worktree_path %s, got %s", worktreePath, *retrieved.WorktreePath)
	}
}

// TestUnitListByRun verifies that ListUnitsByRun returns all units for a run
func TestUnitListByRun(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create a parent run
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

	// Create multiple units
	unit1 := &UnitRecord{
		ID:     MakeUnitRecordID(run.ID, "task-1"),
		RunID:  run.ID,
		UnitID: "task-1",
		Status: string(UnitStatusPending),
	}
	unit2 := &UnitRecord{
		ID:     MakeUnitRecordID(run.ID, "task-2"),
		RunID:  run.ID,
		UnitID: "task-2",
		Status: string(UnitStatusRunning),
	}
	unit3 := &UnitRecord{
		ID:     MakeUnitRecordID(run.ID, "task-3"),
		RunID:  run.ID,
		UnitID: "task-3",
		Status: string(UnitStatusCompleted),
	}

	err = db.CreateUnit(unit1)
	if err != nil {
		t.Fatalf("CreateUnit failed: %v", err)
	}
	err = db.CreateUnit(unit2)
	if err != nil {
		t.Fatalf("CreateUnit failed: %v", err)
	}
	err = db.CreateUnit(unit3)
	if err != nil {
		t.Fatalf("CreateUnit failed: %v", err)
	}

	// List all units for the run
	units, err := db.ListUnitsByRun(run.ID)
	if err != nil {
		t.Fatalf("ListUnitsByRun failed: %v", err)
	}

	if len(units) != 3 {
		t.Errorf("Expected 3 units, got %d", len(units))
	}

	// Verify ordering by unit_id
	if len(units) >= 3 {
		if units[0].UnitID != "task-1" {
			t.Errorf("Expected first unit to be task-1, got %s", units[0].UnitID)
		}
		if units[1].UnitID != "task-2" {
			t.Errorf("Expected second unit to be task-2, got %s", units[1].UnitID)
		}
		if units[2].UnitID != "task-3" {
			t.Errorf("Expected third unit to be task-3, got %s", units[2].UnitID)
		}
	}
}

// TestUnitListByStatus verifies that ListUnitsByStatus returns only units with matching status
func TestUnitListByStatus(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create a parent run
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

	// Create multiple units with different statuses
	unit1 := &UnitRecord{
		ID:     MakeUnitRecordID(run.ID, "task-1"),
		RunID:  run.ID,
		UnitID: "task-1",
		Status: string(UnitStatusPending),
	}
	unit2 := &UnitRecord{
		ID:     MakeUnitRecordID(run.ID, "task-2"),
		RunID:  run.ID,
		UnitID: "task-2",
		Status: string(UnitStatusPending),
	}
	unit3 := &UnitRecord{
		ID:     MakeUnitRecordID(run.ID, "task-3"),
		RunID:  run.ID,
		UnitID: "task-3",
		Status: string(UnitStatusRunning),
	}

	err = db.CreateUnit(unit1)
	if err != nil {
		t.Fatalf("CreateUnit failed: %v", err)
	}
	err = db.CreateUnit(unit2)
	if err != nil {
		t.Fatalf("CreateUnit failed: %v", err)
	}
	err = db.CreateUnit(unit3)
	if err != nil {
		t.Fatalf("CreateUnit failed: %v", err)
	}

	// List pending units
	pending, err := db.ListUnitsByStatus(run.ID, UnitStatusPending)
	if err != nil {
		t.Fatalf("ListUnitsByStatus failed: %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("Expected 2 pending units, got %d", len(pending))
	}

	// List running units
	running, err := db.ListUnitsByStatus(run.ID, UnitStatusRunning)
	if err != nil {
		t.Fatalf("ListUnitsByStatus failed: %v", err)
	}

	if len(running) != 1 {
		t.Errorf("Expected 1 running unit, got %d", len(running))
	}
}
