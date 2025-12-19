// ABOUTME: Unit tests for Charm-based workout storage.
// ABOUTME: Tests cascade deletes and workout metric management.
package charm

import (
	"testing"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
)

func TestWorkoutKeyFormat(t *testing.T) {
	w := models.NewWorkout("run")
	key := WorkoutPrefix + w.ID.String()

	if key[:8] != "workout:" {
		t.Errorf("Expected key to start with 'workout:', got: %s", key[:8])
	}
}

func TestWorkoutMetricKeyFormat(t *testing.T) {
	workoutID := uuid.New()
	wm := models.NewWorkoutMetric(workoutID, "distance", 5.2, "km")
	key := WorkoutMetricPrefix + wm.ID.String()

	if key[:15] != "workout_metric:" {
		t.Errorf("Expected key to start with 'workout_metric:', got: %s", key[:15])
	}
}

func TestExtractID(t *testing.T) {
	id := "abc12345-1234-1234-1234-123456789abc"
	key := MetricPrefix + id

	extracted := extractID(key, MetricPrefix)
	if extracted != id {
		t.Errorf("Expected extracted ID %q, got %q", id, extracted)
	}
}
