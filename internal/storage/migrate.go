// ABOUTME: Data migration between health storage backends.
// ABOUTME: Copies metrics, workouts, and workout metrics from source to destination.

package storage

import (
	"fmt"
	"os"
)

// MigrateSummary holds counts of migrated entities.
type MigrateSummary struct {
	Metrics        int
	Workouts       int
	WorkoutMetrics int
}

// MigrateData copies all data from src to dst storage.
// It iterates through metrics and workouts in order,
// creating each entity in the destination. The destination should be empty
// before calling this function.
func MigrateData(src, dst Repository) (*MigrateSummary, error) {
	summary := &MigrateSummary{}

	// Migrate all metrics
	metrics, err := src.ListMetrics(nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list source metrics: %w", err)
	}

	for _, m := range metrics {
		if err := dst.CreateMetric(m); err != nil {
			return nil, fmt.Errorf("create metric %s: %w", m.ID, err)
		}
		summary.Metrics++
	}

	// Migrate all workouts with their metrics
	workouts, err := src.ListWorkouts(nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list source workouts: %w", err)
	}

	for _, w := range workouts {
		// Get the full workout with metrics
		fullWorkout, err := src.GetWorkoutWithMetrics(w.ID.String())
		if err != nil {
			return nil, fmt.Errorf("get workout %s with metrics: %w", w.ID, err)
		}

		// Save metrics and clear them from the workout before creating.
		// CreateWorkout should only create the workout itself; we add
		// metrics separately via AddWorkoutMetric to avoid duplicates.
		workoutMetrics := fullWorkout.Metrics
		fullWorkout.Metrics = nil

		if err := dst.CreateWorkout(fullWorkout); err != nil {
			return nil, fmt.Errorf("create workout %s: %w", w.ID, err)
		}
		summary.Workouts++

		// Migrate workout metrics
		for _, wm := range workoutMetrics {
			wm.WorkoutID = fullWorkout.ID
			if err := dst.AddWorkoutMetric(&wm); err != nil {
				return nil, fmt.Errorf("add workout metric %s: %w", wm.ID, err)
			}
			summary.WorkoutMetrics++
		}
	}

	return summary, nil
}

// IsDirNonEmpty checks whether a directory exists and contains any files or subdirectories.
// Returns false if the directory does not exist or is empty.
func IsDirNonEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read directory %q: %w", path, err)
	}
	return len(entries) > 0, nil
}
