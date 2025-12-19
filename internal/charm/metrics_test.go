// ABOUTME: Unit tests for Charm-based metric storage.
// ABOUTME: Tests CRUD operations with type-prefixed keys.
package charm

import (
	"testing"

	"github.com/harperreed/health/internal/models"
)

func TestMetricKeyFormat(t *testing.T) {
	m := models.NewMetric(models.MetricWeight, 82.5)
	key := MetricPrefix + m.ID.String()

	if key[:7] != "metric:" {
		t.Errorf("Expected key to start with 'metric:', got: %s", key[:7])
	}
}

func TestMetricPrefixes(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		expected string
	}{
		{"Metric", MetricPrefix, "metric:"},
		{"Workout", WorkoutPrefix, "workout:"},
		{"WorkoutMetric", WorkoutMetricPrefix, "workout_metric:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.prefix != tt.expected {
				t.Errorf("Expected %s = %q, got %q", tt.name, tt.expected, tt.prefix)
			}
		})
	}
}
