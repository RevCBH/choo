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
