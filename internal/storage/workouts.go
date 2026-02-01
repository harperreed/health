// ABOUTME: Workout and WorkoutMetric CRUD operations for SQLite storage.
// ABOUTME: Implements Repository interface methods for workouts with cascade delete.
package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
)

// CreateWorkout stores a new workout in the database.
func (d *DB) CreateWorkout(w *models.Workout) error {
	query := `
		INSERT INTO workouts (id, workout_type, started_at, duration_minutes, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.Exec(query,
		w.ID.String(),
		w.WorkoutType,
		w.StartedAt.Format(time.RFC3339),
		w.DurationMinutes,
		w.Notes,
		w.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create workout: %w", err)
	}
	return nil
}

// GetWorkout retrieves a workout by ID or ID prefix (without metrics).
func (d *DB) GetWorkout(idOrPrefix string) (*models.Workout, error) {
	id, err := d.resolveWorkoutID(idOrPrefix)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, workout_type, started_at, duration_minutes, notes, created_at
		FROM workouts
		WHERE id = ?
	`
	return d.scanWorkout(d.db.QueryRow(query, id))
}

// GetWorkoutWithMetrics retrieves a workout with all its associated metrics.
func (d *DB) GetWorkoutWithMetrics(idOrPrefix string) (*models.Workout, error) {
	w, err := d.GetWorkout(idOrPrefix)
	if err != nil {
		return nil, err
	}

	metrics, err := d.ListWorkoutMetrics(w.ID)
	if err != nil {
		return nil, fmt.Errorf("list workout metrics: %w", err)
	}

	for _, m := range metrics {
		w.Metrics = append(w.Metrics, *m)
	}

	return w, nil
}

// ListWorkouts retrieves workouts with optional filtering by type.
// Results are sorted by StartedAt descending (most recent first).
func (d *DB) ListWorkouts(workoutType *string, limit int) ([]*models.Workout, error) {
	var query string
	var args []interface{}

	if workoutType != nil {
		query = `
			SELECT id, workout_type, started_at, duration_minutes, notes, created_at
			FROM workouts
			WHERE LOWER(workout_type) = LOWER(?)
			ORDER BY started_at DESC
		`
		args = append(args, *workoutType)
	} else {
		query = `
			SELECT id, workout_type, started_at, duration_minutes, notes, created_at
			FROM workouts
			ORDER BY started_at DESC
		`
	}

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}
	defer rows.Close()

	return d.scanWorkouts(rows)
}

// DeleteWorkout removes a workout and all its metrics (cascade delete).
func (d *DB) DeleteWorkout(idOrPrefix string) error {
	id, err := d.resolveWorkoutID(idOrPrefix)
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}

	// CASCADE is enabled, so deleting the workout deletes its metrics
	result, err := d.db.Exec("DELETE FROM workouts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("not found: %s", idOrPrefix)
	}

	return nil
}

// AddWorkoutMetric stores a new workout metric in the database.
func (d *DB) AddWorkoutMetric(wm *models.WorkoutMetric) error {
	query := `
		INSERT INTO workout_metrics (id, workout_id, metric_name, value, unit, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.Exec(query,
		wm.ID.String(),
		wm.WorkoutID.String(),
		wm.MetricName,
		wm.Value,
		wm.Unit,
		wm.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("add workout metric: %w", err)
	}
	return nil
}

// GetWorkoutMetric retrieves a workout metric by ID or ID prefix.
func (d *DB) GetWorkoutMetric(idOrPrefix string) (*models.WorkoutMetric, error) {
	id, err := d.resolveWorkoutMetricID(idOrPrefix)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, workout_id, metric_name, value, unit, created_at
		FROM workout_metrics
		WHERE id = ?
	`
	return d.scanWorkoutMetric(d.db.QueryRow(query, id))
}

// ListWorkoutMetrics retrieves all workout metrics for a specific workout.
func (d *DB) ListWorkoutMetrics(workoutID uuid.UUID) ([]*models.WorkoutMetric, error) {
	query := `
		SELECT id, workout_id, metric_name, value, unit, created_at
		FROM workout_metrics
		WHERE workout_id = ?
		ORDER BY created_at ASC
	`
	rows, err := d.db.Query(query, workoutID.String())
	if err != nil {
		return nil, fmt.Errorf("list workout metrics: %w", err)
	}
	defer rows.Close()

	return d.scanWorkoutMetrics(rows)
}

// DeleteWorkoutMetric removes a workout metric by ID or prefix.
func (d *DB) DeleteWorkoutMetric(idOrPrefix string) error {
	id, err := d.resolveWorkoutMetricID(idOrPrefix)
	if err != nil {
		return fmt.Errorf("delete workout metric: %w", err)
	}

	result, err := d.db.Exec("DELETE FROM workout_metrics WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete workout metric: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete workout metric: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("not found: %s", idOrPrefix)
	}

	return nil
}

// resolveWorkoutID finds the full ID from a prefix.
func (d *DB) resolveWorkoutID(idOrPrefix string) (string, error) {
	if len(idOrPrefix) == 36 && strings.Count(idOrPrefix, "-") == 4 {
		return idOrPrefix, nil
	}

	query := `SELECT id FROM workouts WHERE id LIKE ? || '%'`
	rows, err := d.db.Query(query, idOrPrefix)
	if err != nil {
		return "", fmt.Errorf("resolve workout ID: %w", err)
	}
	defer rows.Close()

	var matches []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("scan workout ID: %w", err)
		}
		matches = append(matches, id)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("not found: %s", idOrPrefix)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous prefix %s: matches multiple records", idOrPrefix)
	}

	return matches[0], nil
}

// resolveWorkoutMetricID finds the full ID from a prefix.
func (d *DB) resolveWorkoutMetricID(idOrPrefix string) (string, error) {
	if len(idOrPrefix) == 36 && strings.Count(idOrPrefix, "-") == 4 {
		return idOrPrefix, nil
	}

	query := `SELECT id FROM workout_metrics WHERE id LIKE ? || '%'`
	rows, err := d.db.Query(query, idOrPrefix)
	if err != nil {
		return "", fmt.Errorf("resolve workout metric ID: %w", err)
	}
	defer rows.Close()

	var matches []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("scan workout metric ID: %w", err)
		}
		matches = append(matches, id)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("not found: %s", idOrPrefix)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous prefix %s: matches multiple records", idOrPrefix)
	}

	return matches[0], nil
}

// scanWorkout scans a single row into a Workout struct.
func (d *DB) scanWorkout(row *sql.Row) (*models.Workout, error) {
	var w models.Workout
	var idStr, startedAt, createdAt string
	var durationMinutes sql.NullInt64
	var notes sql.NullString

	err := row.Scan(&idStr, &w.WorkoutType, &startedAt, &durationMinutes, &notes, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("not found")
		}
		return nil, fmt.Errorf("scan workout: %w", err)
	}

	w.ID, _ = uuid.Parse(idStr)
	w.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	w.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if durationMinutes.Valid {
		d := int(durationMinutes.Int64)
		w.DurationMinutes = &d
	}
	if notes.Valid {
		w.Notes = &notes.String
	}

	return &w, nil
}

// scanWorkouts scans multiple rows into a slice of Workouts.
func (d *DB) scanWorkouts(rows *sql.Rows) ([]*models.Workout, error) {
	var workouts []*models.Workout

	for rows.Next() {
		var w models.Workout
		var idStr, startedAt, createdAt string
		var durationMinutes sql.NullInt64
		var notes sql.NullString

		err := rows.Scan(&idStr, &w.WorkoutType, &startedAt, &durationMinutes, &notes, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan workout: %w", err)
		}

		w.ID, _ = uuid.Parse(idStr)
		w.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		w.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if durationMinutes.Valid {
			d := int(durationMinutes.Int64)
			w.DurationMinutes = &d
		}
		if notes.Valid {
			w.Notes = &notes.String
		}

		workouts = append(workouts, &w)
	}

	return workouts, rows.Err()
}

// scanWorkoutMetric scans a single row into a WorkoutMetric struct.
func (d *DB) scanWorkoutMetric(row *sql.Row) (*models.WorkoutMetric, error) {
	var wm models.WorkoutMetric
	var idStr, workoutIDStr, createdAt string
	var unit sql.NullString

	err := row.Scan(&idStr, &workoutIDStr, &wm.MetricName, &wm.Value, &unit, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("not found")
		}
		return nil, fmt.Errorf("scan workout metric: %w", err)
	}

	wm.ID, _ = uuid.Parse(idStr)
	wm.WorkoutID, _ = uuid.Parse(workoutIDStr)
	wm.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if unit.Valid {
		wm.Unit = &unit.String
	}

	return &wm, nil
}

// scanWorkoutMetrics scans multiple rows into a slice of WorkoutMetrics.
func (d *DB) scanWorkoutMetrics(rows *sql.Rows) ([]*models.WorkoutMetric, error) {
	var metrics []*models.WorkoutMetric

	for rows.Next() {
		var wm models.WorkoutMetric
		var idStr, workoutIDStr, createdAt string
		var unit sql.NullString

		err := rows.Scan(&idStr, &workoutIDStr, &wm.MetricName, &wm.Value, &unit, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan workout metric: %w", err)
		}

		wm.ID, _ = uuid.Parse(idStr)
		wm.WorkoutID, _ = uuid.Parse(workoutIDStr)
		wm.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if unit.Valid {
			wm.Unit = &unit.String
		}

		metrics = append(metrics, &wm)
	}

	return metrics, rows.Err()
}
