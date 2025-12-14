// ABOUTME: Tests for Metric model and MetricType.
// ABOUTME: Validates type constants, units mapping, and constructor.
package models

import (
	"testing"
)

func TestMetricTypeUnit(t *testing.T) {
	tests := []struct {
		metricType MetricType
		wantUnit   string
	}{
		{MetricWeight, "kg"},
		{MetricHRV, "ms"},
		{MetricMood, "scale"},
		{MetricCalories, "kcal"},
	}

	for _, tt := range tests {
		t.Run(string(tt.metricType), func(t *testing.T) {
			got := MetricUnits[tt.metricType]
			if got != tt.wantUnit {
				t.Errorf("MetricUnits[%s] = %s, want %s", tt.metricType, got, tt.wantUnit)
			}
		})
	}
}

func TestNewMetric(t *testing.T) {
	m := NewMetric(MetricWeight, 82.5)

	if m.ID.String() == "" {
		t.Error("expected UUID to be set")
	}
	if m.MetricType != MetricWeight {
		t.Errorf("MetricType = %s, want weight", m.MetricType)
	}
	if m.Value != 82.5 {
		t.Errorf("Value = %f, want 82.5", m.Value)
	}
	if m.Unit != "kg" {
		t.Errorf("Unit = %s, want kg", m.Unit)
	}
	if m.RecordedAt.IsZero() {
		t.Error("expected RecordedAt to be set")
	}
}

func TestAllMetricTypesHaveUnits(t *testing.T) {
	types := []MetricType{
		MetricWeight, MetricBodyFat, MetricBPSys, MetricBPDia,
		MetricHeartRate, MetricHRV, MetricTemperature,
		MetricSteps, MetricSleepHours, MetricActiveCalories,
		MetricWater, MetricCalories, MetricProtein, MetricCarbs, MetricFat,
		MetricMood, MetricEnergy, MetricStress, MetricAnxiety, MetricFocus, MetricMeditation,
	}

	for _, mt := range types {
		if _, ok := MetricUnits[mt]; !ok {
			t.Errorf("MetricType %s has no unit defined", mt)
		}
	}
}
