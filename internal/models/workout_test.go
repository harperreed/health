// ABOUTME: Tests for Workout and WorkoutMetric models.
// ABOUTME: Validates constructors and builder methods.
package models

import (
	"testing"
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
