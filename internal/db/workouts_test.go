// ABOUTME: Tests for workout CRUD operations.
// ABOUTME: Validates workout and workout_metrics functions.
package db

import (
	"testing"

	"github.com/harperreed/health/internal/models"
)

func TestCreateAndGetWorkout(t *testing.T) {
	db := setupTestDB(t)

	w := models.NewWorkout("run").WithDuration(45)
	if err := CreateWorkout(db, w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	got, err := GetWorkout(db, w.ID.String())
	if err != nil {
		t.Fatalf("GetWorkout failed: %v", err)
	}

	if got.ID != w.ID {
		t.Errorf("ID mismatch")
	}
	if got.WorkoutType != "run" {
		t.Errorf("WorkoutType = %s, want run", got.WorkoutType)
	}
	if got.DurationMinutes == nil || *got.DurationMinutes != 45 {
		t.Error("DurationMinutes mismatch")
	}
}

func TestAddWorkoutMetric(t *testing.T) {
	db := setupTestDB(t)

	w := models.NewWorkout("run")
	CreateWorkout(db, w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.2, "km")
	if err := AddWorkoutMetric(db, wm); err != nil {
		t.Fatalf("AddWorkoutMetric failed: %v", err)
	}

	// Get workout with metrics
	got, err := GetWorkoutWithMetrics(db, w.ID.String())
	if err != nil {
		t.Fatalf("GetWorkoutWithMetrics failed: %v", err)
	}

	if len(got.Metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(got.Metrics))
	}
	if got.Metrics[0].MetricName != "distance" {
		t.Errorf("MetricName = %s, want distance", got.Metrics[0].MetricName)
	}
}

func TestListWorkouts(t *testing.T) {
	db := setupTestDB(t)

	w1 := models.NewWorkout("run")
	w2 := models.NewWorkout("lift")
	CreateWorkout(db, w1)
	CreateWorkout(db, w2)

	workouts, err := ListWorkouts(db, nil, 10)
	if err != nil {
		t.Fatalf("ListWorkouts failed: %v", err)
	}
	if len(workouts) != 2 {
		t.Errorf("expected 2 workouts, got %d", len(workouts))
	}

	// Filter by type
	runType := "run"
	workouts, err = ListWorkouts(db, &runType, 10)
	if err != nil {
		t.Fatalf("ListWorkouts by type failed: %v", err)
	}
	if len(workouts) != 1 {
		t.Errorf("expected 1 run workout, got %d", len(workouts))
	}
}

func TestDeleteWorkoutCascades(t *testing.T) {
	db := setupTestDB(t)

	w := models.NewWorkout("run")
	CreateWorkout(db, w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.2, "km")
	AddWorkoutMetric(db, wm)

	if err := DeleteWorkout(db, w.ID.String()); err != nil {
		t.Fatalf("DeleteWorkout failed: %v", err)
	}

	// Verify cascade delete
	var count int
	db.QueryRow("SELECT COUNT(*) FROM workout_metrics WHERE workout_id = ?", w.ID.String()).Scan(&count)
	if count != 0 {
		t.Error("expected workout_metrics to be cascade deleted")
	}
}
