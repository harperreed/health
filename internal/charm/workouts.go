// ABOUTME: Workout and WorkoutMetric CRUD operations for Charm KV storage.
// ABOUTME: Handles cascade deletes manually since KV has no foreign keys.
package charm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
)

// CreateWorkout stores a new workout in the KV store.
func (c *Client) CreateWorkout(w *models.Workout) error {
	key := WorkoutPrefix + w.ID.String()
	data, err := marshalJSON(w)
	if err != nil {
		return fmt.Errorf("marshal workout: %w", err)
	}
	return c.set(key, data)
}

// GetWorkout retrieves a workout by ID or ID prefix (without metrics).
func (c *Client) GetWorkout(idOrPrefix string) (*models.Workout, error) {
	data, err := c.getByIDPrefix(WorkoutPrefix, idOrPrefix)
	if err != nil {
		return nil, fmt.Errorf("get workout: %w", err)
	}

	workout, err := unmarshalJSON[models.Workout](data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal workout: %w", err)
	}

	return workout, nil
}

// GetWorkoutWithMetrics retrieves a workout with all its associated metrics.
func (c *Client) GetWorkoutWithMetrics(idOrPrefix string) (*models.Workout, error) {
	w, err := c.GetWorkout(idOrPrefix)
	if err != nil {
		return nil, err
	}

	// Fetch all workout metrics and filter by workout ID
	allMetricData, err := c.listByPrefix(WorkoutMetricPrefix)
	if err != nil {
		return nil, fmt.Errorf("list workout metrics: %w", err)
	}

	for _, data := range allMetricData {
		wm, err := unmarshalJSON[models.WorkoutMetric](data)
		if err != nil {
			continue
		}
		if wm.WorkoutID == w.ID {
			w.Metrics = append(w.Metrics, *wm)
		}
	}

	return w, nil
}

// ListWorkouts retrieves workouts with optional filtering by type.
// Results are sorted by StartedAt descending (most recent first).
func (c *Client) ListWorkouts(workoutType *string, limit int) ([]*models.Workout, error) {
	allData, err := c.listByPrefix(WorkoutPrefix)
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}

	var workouts []*models.Workout
	for _, data := range allData {
		w, err := unmarshalJSON[models.Workout](data)
		if err != nil {
			continue
		}

		// Filter by type if specified
		if workoutType != nil && !strings.EqualFold(w.WorkoutType, *workoutType) {
			continue
		}

		workouts = append(workouts, w)
	}

	// Sort by StartedAt descending
	sort.Slice(workouts, func(i, j int) bool {
		return workouts[i].StartedAt.After(workouts[j].StartedAt)
	})

	// Apply limit
	if limit > 0 && len(workouts) > limit {
		workouts = workouts[:limit]
	}

	return workouts, nil
}

// DeleteWorkout removes a workout and all its metrics (cascade delete).
func (c *Client) DeleteWorkout(idOrPrefix string) error {
	// First get the workout to find its full ID
	w, err := c.GetWorkout(idOrPrefix)
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}

	// Delete all associated workout metrics first
	if err := c.deleteWorkoutMetricsByWorkoutID(w.ID); err != nil {
		return fmt.Errorf("delete workout metrics: %w", err)
	}

	// Then delete the workout itself
	key := WorkoutPrefix + w.ID.String()
	if err := c.delete(key); err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}

	return nil
}

// CreateWorkoutMetric stores a new workout metric in the KV store.
func (c *Client) CreateWorkoutMetric(wm *models.WorkoutMetric) error {
	key := WorkoutMetricPrefix + wm.ID.String()
	data, err := marshalJSON(wm)
	if err != nil {
		return fmt.Errorf("marshal workout metric: %w", err)
	}
	return c.set(key, data)
}

// AddWorkoutMetric is an alias for CreateWorkoutMetric for API compatibility.
func (c *Client) AddWorkoutMetric(wm *models.WorkoutMetric) error {
	return c.CreateWorkoutMetric(wm)
}

// GetWorkoutMetric retrieves a workout metric by ID or ID prefix.
func (c *Client) GetWorkoutMetric(idOrPrefix string) (*models.WorkoutMetric, error) {
	data, err := c.getByIDPrefix(WorkoutMetricPrefix, idOrPrefix)
	if err != nil {
		return nil, fmt.Errorf("get workout metric: %w", err)
	}

	wm, err := unmarshalJSON[models.WorkoutMetric](data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal workout metric: %w", err)
	}

	return wm, nil
}

// ListWorkoutMetrics retrieves all workout metrics for a specific workout.
func (c *Client) ListWorkoutMetrics(workoutID uuid.UUID) ([]*models.WorkoutMetric, error) {
	allData, err := c.listByPrefix(WorkoutMetricPrefix)
	if err != nil {
		return nil, fmt.Errorf("list workout metrics: %w", err)
	}

	var metrics []*models.WorkoutMetric
	for _, data := range allData {
		wm, err := unmarshalJSON[models.WorkoutMetric](data)
		if err != nil {
			continue
		}
		if wm.WorkoutID == workoutID {
			metrics = append(metrics, wm)
		}
	}

	// Sort by CreatedAt ascending (order added)
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].CreatedAt.Before(metrics[j].CreatedAt)
	})

	return metrics, nil
}

// DeleteWorkoutMetric removes a workout metric by ID or prefix.
func (c *Client) DeleteWorkoutMetric(idOrPrefix string) error {
	if err := c.deleteByIDPrefix(WorkoutMetricPrefix, idOrPrefix); err != nil {
		return fmt.Errorf("delete workout metric: %w", err)
	}
	return nil
}

// deleteWorkoutMetricsByWorkoutID removes all workout metrics for a specific workout.
func (c *Client) deleteWorkoutMetricsByWorkoutID(workoutID uuid.UUID) error {
	metrics, err := c.ListWorkoutMetrics(workoutID)
	if err != nil {
		return err
	}

	for _, wm := range metrics {
		key := WorkoutMetricPrefix + wm.ID.String()
		if err := c.delete(key); err != nil {
			return err
		}
	}

	return nil
}

// WorkoutFilter defines criteria for filtering workouts.
type WorkoutFilter struct {
	WorkoutType *string
	Limit       int
}

// ListWorkoutsFiltered retrieves workouts matching the filter criteria.
func (c *Client) ListWorkoutsFiltered(filter *WorkoutFilter) ([]*models.Workout, error) {
	if filter == nil {
		return c.ListWorkouts(nil, 0)
	}
	return c.ListWorkouts(filter.WorkoutType, filter.Limit)
}
