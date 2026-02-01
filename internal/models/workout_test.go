// ABOUTME: Tests for Workout and WorkoutMetric models.
// ABOUTME: Validates constructors and builder methods.
package models

import (
	"testing"
	"time"
)

func TestNewWorkout(t *testing.T) {
	w := NewWorkout("run")

	if w.ID.String() == "" {
		t.Error("expected UUID to be set")
	}
	if w.WorkoutType != "run" {
		t.Errorf("WorkoutType = %s, want run", w.WorkoutType)
	}
	if w.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
}

func TestWorkoutWithDuration(t *testing.T) {
	w := NewWorkout("lift").WithDuration(45)

	if w.DurationMinutes == nil || *w.DurationMinutes != 45 {
		t.Error("expected DurationMinutes to be 45")
	}
}

func TestNewWorkoutMetric(t *testing.T) {
	w := NewWorkout("run")
	wm := NewWorkoutMetric(w.ID, "distance", 5.2, "km")

	if wm.WorkoutID != w.ID {
		t.Error("expected WorkoutID to match")
	}
	if wm.MetricName != "distance" {
		t.Errorf("MetricName = %s, want distance", wm.MetricName)
	}
	if wm.Value != 5.2 {
		t.Errorf("Value = %f, want 5.2", wm.Value)
	}
	if wm.Unit == nil || *wm.Unit != "km" {
		t.Error("expected Unit to be km")
	}
}

func TestWorkoutWithNotes(t *testing.T) {
	w := NewWorkout("run")

	if w.Notes != nil {
		t.Error("Notes should be nil initially")
	}

	result := w.WithNotes("Morning run")

	if result != w {
		t.Error("WithNotes should return the same workout for chaining")
	}
	if w.Notes == nil {
		t.Fatal("Notes should not be nil after setting")
	}
	if *w.Notes != "Morning run" {
		t.Errorf("Notes = %q, want %q", *w.Notes, "Morning run")
	}
}

func TestWorkoutWithStartedAt(t *testing.T) {
	w := NewWorkout("lift")
	originalTime := w.StartedAt

	customTime := time.Date(2025, 6, 15, 7, 0, 0, 0, time.UTC)
	result := w.WithStartedAt(customTime)

	if result != w {
		t.Error("WithStartedAt should return the same workout for chaining")
	}
	if w.StartedAt.Equal(originalTime) {
		t.Error("StartedAt should have been updated")
	}
	if !w.StartedAt.Equal(customTime) {
		t.Errorf("StartedAt = %v, want %v", w.StartedAt, customTime)
	}
}

func TestWorkoutChaining(t *testing.T) {
	customTime := time.Date(2025, 6, 15, 8, 0, 0, 0, time.UTC)

	w := NewWorkout("swim").
		WithDuration(60).
		WithNotes("Pool workout").
		WithStartedAt(customTime)

	if w.WorkoutType != "swim" {
		t.Errorf("WorkoutType = %s, want swim", w.WorkoutType)
	}
	if w.DurationMinutes == nil || *w.DurationMinutes != 60 {
		t.Error("Duration should be 60")
	}
	if w.Notes == nil || *w.Notes != "Pool workout" {
		t.Error("Notes should be 'Pool workout'")
	}
	if !w.StartedAt.Equal(customTime) {
		t.Errorf("StartedAt = %v, want %v", w.StartedAt, customTime)
	}
}

func TestNewWorkoutSetsUniqueIDs(t *testing.T) {
	w1 := NewWorkout("run")
	w2 := NewWorkout("run")

	if w1.ID == w2.ID {
		t.Error("NewWorkout should generate unique IDs")
	}
}

func TestNewWorkoutSetsCreatedAt(t *testing.T) {
	before := time.Now()
	w := NewWorkout("run")
	after := time.Now()

	if w.CreatedAt.Before(before) || w.CreatedAt.After(after) {
		t.Errorf("CreatedAt should be between %v and %v, got %v", before, after, w.CreatedAt)
	}
}

func TestNewWorkoutMetricEmptyUnit(t *testing.T) {
	w := NewWorkout("lift")
	wm := NewWorkoutMetric(w.ID, "sets", 4, "")

	if wm.Unit != nil {
		t.Error("Unit should be nil when empty string provided")
	}
}

func TestNewWorkoutMetricSetsUniqueIDs(t *testing.T) {
	w := NewWorkout("run")
	wm1 := NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	wm2 := NewWorkoutMetric(w.ID, "distance", 5.0, "km")

	if wm1.ID == wm2.ID {
		t.Error("NewWorkoutMetric should generate unique IDs")
	}
}

func TestNewWorkoutMetricSetsCreatedAt(t *testing.T) {
	before := time.Now()
	w := NewWorkout("run")
	wm := NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	after := time.Now()

	if wm.CreatedAt.Before(before) || wm.CreatedAt.After(after) {
		t.Errorf("CreatedAt should be between %v and %v, got %v", before, after, wm.CreatedAt)
	}
}

func TestWorkoutMetricsSliceInitialization(t *testing.T) {
	w := NewWorkout("run")

	// Metrics slice should be nil/empty initially
	if len(w.Metrics) != 0 {
		t.Errorf("Metrics should be empty initially, got %d", len(w.Metrics))
	}
}
