// ABOUTME: Workout CRUD operations for the health database.
// ABOUTME: Supports workouts with sub-metrics, cascade delete.
package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
)

// CreateWorkout inserts a new workout into the database.
func CreateWorkout(db *sql.DB, w *models.Workout) error {
	_, err := db.Exec(`
		INSERT INTO workouts (id, workout_type, started_at, duration_minutes, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		w.ID.String(), w.WorkoutType, w.StartedAt.Format(time.RFC3339),
		w.DurationMinutes, w.Notes, w.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to create workout: %w", err)
	}
	return nil
}

// GetWorkout retrieves a workout by ID or prefix (without metrics).
func GetWorkout(db *sql.DB, idOrPrefix string) (*models.Workout, error) {
	var row *sql.Row
	if len(idOrPrefix) < 36 {
		row = db.QueryRow(`
			SELECT id, workout_type, started_at, duration_minutes, notes, created_at
			FROM workouts WHERE id LIKE ? LIMIT 1`, idOrPrefix+"%")
	} else {
		row = db.QueryRow(`
			SELECT id, workout_type, started_at, duration_minutes, notes, created_at
			FROM workouts WHERE id = ?`, idOrPrefix)
	}
	return scanWorkout(row)
}

// GetWorkoutWithMetrics retrieves a workout with all its metrics.
func GetWorkoutWithMetrics(db *sql.DB, idOrPrefix string) (*models.Workout, error) {
	w, err := GetWorkout(db, idOrPrefix)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT id, workout_id, metric_name, value, unit, created_at
		FROM workout_metrics WHERE workout_id = ?`, w.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get workout metrics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		wm, err := scanWorkoutMetric(rows)
		if err != nil {
			return nil, err
		}
		w.Metrics = append(w.Metrics, *wm)
	}

	return w, rows.Err()
}

// AddWorkoutMetric adds a metric to an existing workout.
func AddWorkoutMetric(db *sql.DB, wm *models.WorkoutMetric) error {
	_, err := db.Exec(`
		INSERT INTO workout_metrics (id, workout_id, metric_name, value, unit, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		wm.ID.String(), wm.WorkoutID.String(), wm.MetricName,
		wm.Value, wm.Unit, wm.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to add workout metric: %w", err)
	}
	return nil
}

// ListWorkouts retrieves recent workouts, optionally filtered by type.
func ListWorkouts(db *sql.DB, workoutType *string, limit int) ([]*models.Workout, error) {
	var rows *sql.Rows
	var err error

	if workoutType != nil {
		rows, err = db.Query(`
			SELECT id, workout_type, started_at, duration_minutes, notes, created_at
			FROM workouts WHERE workout_type = ?
			ORDER BY started_at DESC LIMIT ?`, *workoutType, limit)
	} else {
		rows, err = db.Query(`
			SELECT id, workout_type, started_at, duration_minutes, notes, created_at
			FROM workouts ORDER BY started_at DESC LIMIT ?`, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list workouts: %w", err)
	}
	defer rows.Close()

	var workouts []*models.Workout
	for rows.Next() {
		w, err := scanWorkoutRows(rows)
		if err != nil {
			return nil, err
		}
		workouts = append(workouts, w)
	}

	return workouts, rows.Err()
}

// DeleteWorkout removes a workout and its metrics (cascade).
func DeleteWorkout(db *sql.DB, idOrPrefix string) error {
	var result sql.Result
	var err error

	if len(idOrPrefix) < 36 {
		result, err = db.Exec("DELETE FROM workouts WHERE id LIKE ?", idOrPrefix+"%")
	} else {
		result, err = db.Exec("DELETE FROM workouts WHERE id = ?", idOrPrefix)
	}

	if err != nil {
		return fmt.Errorf("failed to delete workout: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("workout not found: %s", idOrPrefix)
	}

	return nil
}

func scanWorkout(row *sql.Row) (*models.Workout, error) {
	var w models.Workout
	var idStr, startedAt, createdAt string

	err := row.Scan(&idStr, &w.WorkoutType, &startedAt, &w.DurationMinutes, &w.Notes, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan workout: %w", err)
	}

	w.ID, err = uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid workout ID in database: %w", err)
	}
	w.StartedAt, err = time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid started_at timestamp: %w", err)
	}
	w.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at timestamp: %w", err)
	}

	return &w, nil
}

func scanWorkoutRows(rows *sql.Rows) (*models.Workout, error) {
	var w models.Workout
	var idStr, startedAt, createdAt string

	err := rows.Scan(&idStr, &w.WorkoutType, &startedAt, &w.DurationMinutes, &w.Notes, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan workout: %w", err)
	}

	w.ID, err = uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid workout ID in database: %w", err)
	}
	w.StartedAt, err = time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid started_at timestamp: %w", err)
	}
	w.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at timestamp: %w", err)
	}

	return &w, nil
}

func scanWorkoutMetric(rows *sql.Rows) (*models.WorkoutMetric, error) {
	var wm models.WorkoutMetric
	var idStr, workoutIDStr, createdAt string

	err := rows.Scan(&idStr, &workoutIDStr, &wm.MetricName, &wm.Value, &wm.Unit, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan workout metric: %w", err)
	}

	wm.ID, err = uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid workout_metric ID in database: %w", err)
	}
	wm.WorkoutID, err = uuid.Parse(workoutIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid workout_id in database: %w", err)
	}
	wm.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at timestamp: %w", err)
	}

	return &wm, nil
}
