// ABOUTME: Workout and WorkoutMetric models for exercise tracking.
// ABOUTME: Workouts contain sub-metrics like distance, pace, sets, reps.
package models

import (
	"time"

	"github.com/google/uuid"
)

// Workout represents an exercise session.
type Workout struct {
	ID              uuid.UUID
	WorkoutType     string
	StartedAt       time.Time
	DurationMinutes *int
	Notes           *string
	CreatedAt       time.Time
	Metrics         []WorkoutMetric // Populated when fetching full workout
}

// NewWorkout creates a new Workout with generated UUID and current timestamp.
func NewWorkout(workoutType string) *Workout {
	now := time.Now()
	return &Workout{
		ID:          uuid.New(),
		WorkoutType: workoutType,
		StartedAt:   now,
		CreatedAt:   now,
	}
}

// WithDuration sets the duration in minutes.
func (w *Workout) WithDuration(minutes int) *Workout {
	w.DurationMinutes = &minutes
	return w
}

// WithNotes sets notes on the workout.
func (w *Workout) WithNotes(notes string) *Workout {
	w.Notes = &notes
	return w
}

// WithStartedAt sets a custom start timestamp.
func (w *Workout) WithStartedAt(t time.Time) *Workout {
	w.StartedAt = t
	return w
}

// WorkoutMetric represents a measurement within a workout.
type WorkoutMetric struct {
	ID         uuid.UUID
	WorkoutID  uuid.UUID
	MetricName string
	Value      float64
	Unit       *string
	CreatedAt  time.Time
}

// NewWorkoutMetric creates a new WorkoutMetric.
func NewWorkoutMetric(workoutID uuid.UUID, name string, value float64, unit string) *WorkoutMetric {
	var unitPtr *string
	if unit != "" {
		unitPtr = &unit
	}
	return &WorkoutMetric{
		ID:         uuid.New(),
		WorkoutID:  workoutID,
		MetricName: name,
		Value:      value,
		Unit:       unitPtr,
		CreatedAt:  time.Now(),
	}
}
