package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateRun inserts a new run into the database.
// Returns an error if a run already exists for the same branch/repo.
func (db *DB) CreateRun(run *Run) error {
	// Set started_at if status is Running
	var startedAt *time.Time
	if run.Status == RunStatusRunning {
		now := time.Now()
		startedAt = &now
	} else {
		startedAt = run.StartedAt
	}

	query := `
		INSERT INTO runs (
			id, feature_branch, repo_path, target_branch, tasks_dir,
			parallelism, status, daemon_version, started_at, completed_at,
			error, config_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.conn.Exec(
		query,
		run.ID,
		run.FeatureBranch,
		run.RepoPath,
		run.TargetBranch,
		run.TasksDir,
		run.Parallelism,
		run.Status,
		run.DaemonVersion,
		startedAt,
		run.CompletedAt,
		run.Error,
		run.ConfigJSON,
	)

	if err != nil {
		// Check for unique constraint violation
		if err.Error() == "constraint failed: UNIQUE constraint failed: runs.feature_branch, runs.repo_path" ||
			err.Error() == "UNIQUE constraint failed: runs.feature_branch, runs.repo_path" {
			return fmt.Errorf("run already exists for branch %s in repo %s", run.FeatureBranch, run.RepoPath)
		}
		return fmt.Errorf("failed to create run: %w", err)
	}

	// Update the run's started_at field if we set it
	if startedAt != nil && run.StartedAt == nil {
		run.StartedAt = startedAt
	}

	return nil
}

// GetRun retrieves a run by its ID.
// Returns nil, nil if the run does not exist.
func (db *DB) GetRun(id string) (*Run, error) {
	query := `
		SELECT id, feature_branch, repo_path, target_branch, tasks_dir,
		       parallelism, status, daemon_version, started_at, completed_at,
		       error, config_json
		FROM runs
		WHERE id = ?
	`

	run := &Run{}
	err := db.conn.QueryRow(query, id).Scan(
		&run.ID,
		&run.FeatureBranch,
		&run.RepoPath,
		&run.TargetBranch,
		&run.TasksDir,
		&run.Parallelism,
		&run.Status,
		&run.DaemonVersion,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Error,
		&run.ConfigJSON,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}

	return run, nil
}

// GetRunByBranch retrieves a run by feature branch and repo path.
// Returns nil, nil if no matching run exists.
func (db *DB) GetRunByBranch(featureBranch, repoPath string) (*Run, error) {
	query := `
		SELECT id, feature_branch, repo_path, target_branch, tasks_dir,
		       parallelism, status, daemon_version, started_at, completed_at,
		       error, config_json
		FROM runs
		WHERE feature_branch = ? AND repo_path = ?
	`

	run := &Run{}
	err := db.conn.QueryRow(query, featureBranch, repoPath).Scan(
		&run.ID,
		&run.FeatureBranch,
		&run.RepoPath,
		&run.TargetBranch,
		&run.TasksDir,
		&run.Parallelism,
		&run.Status,
		&run.DaemonVersion,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Error,
		&run.ConfigJSON,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get run by branch: %w", err)
	}

	return run, nil
}

// GetActiveRunByBranch retrieves a running job by feature branch and repo path.
// Returns nil, nil if no matching running job exists.
// This is used for CLI attach: if a job is already running, attach to it instead of starting new.
func (db *DB) GetActiveRunByBranch(featureBranch, repoPath string) (*Run, error) {
	query := `
		SELECT id, feature_branch, repo_path, target_branch, tasks_dir,
		       parallelism, status, daemon_version, started_at, completed_at,
		       error, config_json
		FROM runs
		WHERE feature_branch = ? AND repo_path = ? AND status = ?
	`

	run := &Run{}
	err := db.conn.QueryRow(query, featureBranch, repoPath, RunStatusRunning).Scan(
		&run.ID,
		&run.FeatureBranch,
		&run.RepoPath,
		&run.TargetBranch,
		&run.TasksDir,
		&run.Parallelism,
		&run.Status,
		&run.DaemonVersion,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Error,
		&run.ConfigJSON,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active run by branch: %w", err)
	}

	return run, nil
}

// UpdateRunStatus updates the status of a run.
// Sets started_at when transitioning to Running.
// Sets completed_at when transitioning to Completed/Failed/Cancelled.
func (db *DB) UpdateRunStatus(id string, status RunStatus, err *string) error {
	now := time.Now()

	// Determine which timestamps to update
	var query string
	var args []interface{}

	if status == RunStatusRunning {
		// Set started_at when transitioning to Running
		query = `UPDATE runs SET status = ?, error = ?, started_at = ? WHERE id = ?`
		args = []interface{}{status, err, now, id}
	} else if status == RunStatusCompleted || status == RunStatusFailed || status == RunStatusCancelled {
		// Set completed_at when transitioning to terminal states
		query = `UPDATE runs SET status = ?, error = ?, completed_at = ? WHERE id = ?`
		args = []interface{}{status, err, now, id}
	} else {
		// For other statuses, only update status and error
		query = `UPDATE runs SET status = ?, error = ? WHERE id = ?`
		args = []interface{}{status, err, id}
	}

	result, execErr := db.conn.Exec(query, args...)
	if execErr != nil {
		return fmt.Errorf("failed to update run status: %w", execErr)
	}

	// Check if the run was found
	rowsAffected, raErr := result.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("failed to check rows affected: %w", raErr)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("run not found: %s", id)
	}

	return nil
}

// ListRunsByStatus returns all runs with the given status.
func (db *DB) ListRunsByStatus(status RunStatus) ([]*Run, error) {
	query := `
		SELECT id, feature_branch, repo_path, target_branch, tasks_dir,
		       parallelism, status, daemon_version, started_at, completed_at,
		       error, config_json
		FROM runs
		WHERE status = ?
		ORDER BY id
	`

	rows, err := db.conn.Query(query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs by status: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		run := &Run{}
		err := rows.Scan(
			&run.ID,
			&run.FeatureBranch,
			&run.RepoPath,
			&run.TargetBranch,
			&run.TasksDir,
			&run.Parallelism,
			&run.Status,
			&run.DaemonVersion,
			&run.StartedAt,
			&run.CompletedAt,
			&run.Error,
			&run.ConfigJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}
		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating runs: %w", err)
	}

	return runs, nil
}

// ListIncompleteRuns returns all runs that are not completed/failed/cancelled.
// Used for resuming interrupted workflows after daemon restart.
func (db *DB) ListIncompleteRuns() ([]*Run, error) {
	query := `
		SELECT id, feature_branch, repo_path, target_branch, tasks_dir,
		       parallelism, status, daemon_version, started_at, completed_at,
		       error, config_json
		FROM runs
		WHERE status IN (?, ?)
		ORDER BY id
	`

	rows, err := db.conn.Query(query, RunStatusPending, RunStatusRunning)
	if err != nil {
		return nil, fmt.Errorf("failed to list incomplete runs: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		run := &Run{}
		err := rows.Scan(
			&run.ID,
			&run.FeatureBranch,
			&run.RepoPath,
			&run.TargetBranch,
			&run.TasksDir,
			&run.Parallelism,
			&run.Status,
			&run.DaemonVersion,
			&run.StartedAt,
			&run.CompletedAt,
			&run.Error,
			&run.ConfigJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}
		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating runs: %w", err)
	}

	return runs, nil
}

// DeleteRun removes a run and all associated units/events (cascade).
func (db *DB) DeleteRun(id string) error {
	query := `DELETE FROM runs WHERE id = ?`

	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete run: %w", err)
	}

	return nil
}

// DeleteNonActiveRunByBranch deletes any completed/failed/cancelled run for the given branch and repo.
// This is called before creating a new run to avoid UNIQUE constraint violations.
// Only deletes runs that are NOT in "running" or "pending" status.
// Returns the number of deleted runs.
func (db *DB) DeleteNonActiveRunByBranch(featureBranch, repoPath string) (int, error) {
	query := `
		DELETE FROM runs
		WHERE feature_branch = ? AND repo_path = ?
		AND status NOT IN (?, ?)
	`

	result, err := db.conn.Exec(query, featureBranch, repoPath, RunStatusRunning, RunStatusPending)
	if err != nil {
		return 0, fmt.Errorf("failed to delete non-active run: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(deleted), nil
}
