package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateUnit inserts a new unit into the database.
// The unit ID should be created using MakeUnitRecordID(runID, unitID).
// Returns an error if the parent run does not exist.
func (db *DB) CreateUnit(unit *UnitRecord) error {
	query := `
		INSERT INTO units (
			id, run_id, unit_id, status, branch, worktree_path,
			started_at, completed_at, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.conn.Exec(
		query,
		unit.ID,
		unit.RunID,
		unit.UnitID,
		unit.Status,
		unit.Branch,
		unit.WorktreePath,
		unit.StartedAt,
		unit.CompletedAt,
		unit.Error,
	)

	if err != nil {
		return fmt.Errorf("failed to create unit: %w", err)
	}

	return nil
}

// GetUnit retrieves a unit by its composite ID.
// Returns nil, nil if the unit does not exist.
func (db *DB) GetUnit(id string) (*UnitRecord, error) {
	query := `
		SELECT id, run_id, unit_id, status, branch, worktree_path,
		       started_at, completed_at, error
		FROM units
		WHERE id = ?
	`

	unit := &UnitRecord{}
	err := db.conn.QueryRow(query, id).Scan(
		&unit.ID,
		&unit.RunID,
		&unit.UnitID,
		&unit.Status,
		&unit.Branch,
		&unit.WorktreePath,
		&unit.StartedAt,
		&unit.CompletedAt,
		&unit.Error,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get unit: %w", err)
	}

	return unit, nil
}

// UpdateUnitStatus updates the status of a unit.
// Sets started_at when transitioning to InProgress.
// Sets completed_at when transitioning to Complete/Failed.
func (db *DB) UpdateUnitStatus(id string, status UnitStatus, err *string) error {
	now := time.Now()

	// Determine which timestamps to update
	var query string
	var args []interface{}

	if status == UnitStatusRunning {
		// Set started_at when transitioning to Running
		query = `UPDATE units SET status = ?, error = ?, started_at = ? WHERE id = ?`
		args = []interface{}{status, err, now, id}
	} else if status == UnitStatusCompleted || status == UnitStatusFailed {
		// Set completed_at when transitioning to terminal states
		query = `UPDATE units SET status = ?, error = ?, completed_at = ? WHERE id = ?`
		args = []interface{}{status, err, now, id}
	} else {
		// For other statuses, only update status and error
		query = `UPDATE units SET status = ?, error = ? WHERE id = ?`
		args = []interface{}{status, err, id}
	}

	result, execErr := db.conn.Exec(query, args...)
	if execErr != nil {
		return fmt.Errorf("failed to update unit status: %w", execErr)
	}

	// Check if the unit was found
	rowsAffected, raErr := result.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("failed to check rows affected: %w", raErr)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("unit not found: %s", id)
	}

	return nil
}

// UpdateUnitBranch sets the git branch and worktree path for a unit.
// Called when a worktree is created for the unit's work.
func (db *DB) UpdateUnitBranch(id string, branch, worktreePath string) error {
	query := `UPDATE units SET branch = ?, worktree_path = ? WHERE id = ?`

	result, err := db.conn.Exec(query, branch, worktreePath, id)
	if err != nil {
		return fmt.Errorf("failed to update unit branch: %w", err)
	}

	// Check if the unit was found
	rowsAffected, raErr := result.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("failed to check rows affected: %w", raErr)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("unit not found: %s", id)
	}

	return nil
}

// ListUnitsByRun returns all units belonging to a run.
func (db *DB) ListUnitsByRun(runID string) ([]*UnitRecord, error) {
	query := `
		SELECT id, run_id, unit_id, status, branch, worktree_path,
		       started_at, completed_at, error
		FROM units
		WHERE run_id = ?
		ORDER BY unit_id
	`

	rows, err := db.conn.Query(query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list units by run: %w", err)
	}
	defer rows.Close()

	var units []*UnitRecord
	for rows.Next() {
		unit := &UnitRecord{}
		err := rows.Scan(
			&unit.ID,
			&unit.RunID,
			&unit.UnitID,
			&unit.Status,
			&unit.Branch,
			&unit.WorktreePath,
			&unit.StartedAt,
			&unit.CompletedAt,
			&unit.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan unit: %w", err)
		}
		units = append(units, unit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating units: %w", err)
	}

	return units, nil
}

// ListUnitsByStatus returns all units with the given status within a run.
func (db *DB) ListUnitsByStatus(runID string, status UnitStatus) ([]*UnitRecord, error) {
	query := `
		SELECT id, run_id, unit_id, status, branch, worktree_path,
		       started_at, completed_at, error
		FROM units
		WHERE run_id = ? AND status = ?
		ORDER BY unit_id
	`

	rows, err := db.conn.Query(query, runID, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list units by status: %w", err)
	}
	defer rows.Close()

	var units []*UnitRecord
	for rows.Next() {
		unit := &UnitRecord{}
		err := rows.Scan(
			&unit.ID,
			&unit.RunID,
			&unit.UnitID,
			&unit.Status,
			&unit.Branch,
			&unit.WorktreePath,
			&unit.StartedAt,
			&unit.CompletedAt,
			&unit.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan unit: %w", err)
		}
		units = append(units, unit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating units: %w", err)
	}

	return units, nil
}
