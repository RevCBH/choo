package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// AppendEvent records a new event with an auto-assigned sequence number.
// The sequence number is calculated within a transaction to avoid races.
// Payload is JSON-serialized if non-nil.
func (db *DB) AppendEvent(runID string, eventType string, unitID *string, payload interface{}) error {
	// Begin transaction
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get next sequence number for this run
	sequence, err := db.getNextSequenceInTx(tx, runID)
	if err != nil {
		return fmt.Errorf("failed to get next sequence: %w", err)
	}

	// JSON serialize payload if non-nil
	var payloadJSON *string
	if payload != nil {
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to serialize payload: %w", err)
		}
		jsonStr := string(jsonBytes)
		payloadJSON = &jsonStr
	}

	// INSERT INTO events
	query := `
		INSERT INTO events (run_id, sequence, event_type, unit_id, payload_json)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err = tx.Exec(query, runID, sequence, eventType, unitID, payloadJSON)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetNextSequence returns the next sequence number for a run.
// Used internally by AppendEvent but exposed for transaction support.
func (db *DB) GetNextSequence(runID string) (int, error) {
	query := `SELECT COALESCE(MAX(sequence), 0) + 1 FROM events WHERE run_id = ?`

	var nextSeq int
	err := db.conn.QueryRow(query, runID).Scan(&nextSeq)
	if err != nil {
		return 0, fmt.Errorf("failed to get next sequence: %w", err)
	}

	return nextSeq, nil
}

// getNextSequenceInTx returns the next sequence number within a transaction.
func (db *DB) getNextSequenceInTx(tx *sql.Tx, runID string) (int, error) {
	query := `SELECT COALESCE(MAX(sequence), 0) + 1 FROM events WHERE run_id = ?`

	var nextSeq int
	err := tx.QueryRow(query, runID).Scan(&nextSeq)
	if err != nil {
		return 0, fmt.Errorf("failed to get next sequence in transaction: %w", err)
	}

	return nextSeq, nil
}

// ListEvents returns all events for a run in sequence order.
func (db *DB) ListEvents(runID string) ([]*EventRecord, error) {
	query := `
		SELECT id, run_id, sequence, event_type, unit_id, payload_json, created_at
		FROM events
		WHERE run_id = ?
		ORDER BY sequence
	`

	rows, err := db.conn.Query(query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}
	defer rows.Close()

	var events []*EventRecord
	for rows.Next() {
		event := &EventRecord{}
		err := rows.Scan(
			&event.ID,
			&event.RunID,
			&event.Sequence,
			&event.EventType,
			&event.UnitID,
			&event.PayloadJSON,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	return events, nil
}

// ListEventsSince returns all events with sequence > the given value.
// Used for incremental event fetching (e.g., for live monitoring).
func (db *DB) ListEventsSince(runID string, sequence int) ([]*EventRecord, error) {
	query := `
		SELECT id, run_id, sequence, event_type, unit_id, payload_json, created_at
		FROM events
		WHERE run_id = ? AND sequence > ?
		ORDER BY sequence
	`

	rows, err := db.conn.Query(query, runID, sequence)
	if err != nil {
		return nil, fmt.Errorf("failed to list events since: %w", err)
	}
	defer rows.Close()

	var events []*EventRecord
	for rows.Next() {
		event := &EventRecord{}
		err := rows.Scan(
			&event.ID,
			&event.RunID,
			&event.Sequence,
			&event.EventType,
			&event.UnitID,
			&event.PayloadJSON,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	return events, nil
}
