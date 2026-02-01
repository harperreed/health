// ABOUTME: Tests for Metric model and MetricType.
// ABOUTME: Validates type constants, units mapping, and constructor.
package models

import (
	"testing"
	"time"
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

func TestIsValidMetricType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid weight", "weight", true},
		{"valid body_fat", "body_fat", true},
		{"valid bp_sys", "bp_sys", true},
		{"valid bp_dia", "bp_dia", true},
		{"valid heart_rate", "heart_rate", true},
		{"valid hrv", "hrv", true},
		{"valid temperature", "temperature", true},
		{"valid steps", "steps", true},
		{"valid sleep_hours", "sleep_hours", true},
		{"valid active_calories", "active_calories", true},
		{"valid water", "water", true},
		{"valid calories", "calories", true},
		{"valid protein", "protein", true},
		{"valid carbs", "carbs", true},
		{"valid fat", "fat", true},
		{"valid mood", "mood", true},
		{"valid energy", "energy", true},
		{"valid stress", "stress", true},
		{"valid anxiety", "anxiety", true},
		{"valid focus", "focus", true},
		{"valid meditation", "meditation", true},
		{"invalid empty", "", false},
		{"invalid random", "random", false},
		{"invalid typo", "wieght", false},
		{"invalid uppercase", "WEIGHT", false},
		{"invalid mixed case", "Weight", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidMetricType(tt.input)
			if got != tt.expected {
				t.Errorf("IsValidMetricType(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMetricWithRecordedAt(t *testing.T) {
	m := NewMetric(MetricWeight, 82.5)
	originalTime := m.RecordedAt

	customTime := time.Date(2025, 1, 15, 8, 30, 0, 0, time.UTC)
	result := m.WithRecordedAt(customTime)

	if result != m {
		t.Error("WithRecordedAt should return the same metric for chaining")
	}
	if m.RecordedAt.Equal(originalTime) {
		t.Error("RecordedAt should have been updated")
	}
	if !m.RecordedAt.Equal(customTime) {
		t.Errorf("RecordedAt = %v, want %v", m.RecordedAt, customTime)
	}
}

func TestMetricWithNotes(t *testing.T) {
	m := NewMetric(MetricWeight, 82.5)

	if m.Notes != nil {
		t.Error("Notes should be nil initially")
	}

	result := m.WithNotes("morning weight")

	if result != m {
		t.Error("WithNotes should return the same metric for chaining")
	}
	if m.Notes == nil {
		t.Fatal("Notes should not be nil after setting")
	}
	if *m.Notes != "morning weight" {
		t.Errorf("Notes = %q, want %q", *m.Notes, "morning weight")
	}
}

func TestMetricWithEmptyNotes(t *testing.T) {
	m := NewMetric(MetricMood, 7)
	m.WithNotes("")

	if m.Notes == nil {
		t.Fatal("Notes should not be nil even for empty string")
	}
	if *m.Notes != "" {
		t.Errorf("Notes = %q, want empty string", *m.Notes)
	}
}

func TestNewMetricSetsCorrectUnit(t *testing.T) {
	tests := []struct {
		metricType MetricType
		wantUnit   string
	}{
		{MetricWeight, "kg"},
		{MetricBodyFat, "%"},
		{MetricBPSys, "mmHg"},
		{MetricBPDia, "mmHg"},
		{MetricHeartRate, "bpm"},
		{MetricHRV, "ms"},
		{MetricTemperature, "°C"},
		{MetricSteps, "steps"},
		{MetricSleepHours, "hours"},
		{MetricActiveCalories, "kcal"},
		{MetricWater, "ml"},
		{MetricCalories, "kcal"},
		{MetricProtein, "g"},
		{MetricCarbs, "g"},
		{MetricFat, "g"},
		{MetricMood, "scale"},
		{MetricEnergy, "scale"},
		{MetricStress, "scale"},
		{MetricAnxiety, "scale"},
		{MetricFocus, "scale"},
		{MetricMeditation, "min"},
	}

	for _, tt := range tests {
		t.Run(string(tt.metricType), func(t *testing.T) {
			m := NewMetric(tt.metricType, 1.0)
			if m.Unit != tt.wantUnit {
				t.Errorf("NewMetric(%s).Unit = %q, want %q", tt.metricType, m.Unit, tt.wantUnit)
			}
		})
	}
}

func TestNewMetricSetsUniqueIDs(t *testing.T) {
	m1 := NewMetric(MetricWeight, 82.5)
	m2 := NewMetric(MetricWeight, 82.5)

	if m1.ID == m2.ID {
		t.Error("NewMetric should generate unique IDs")
	}
}

func TestNewMetricSetsCreatedAt(t *testing.T) {
	before := time.Now()
	m := NewMetric(MetricWeight, 82.5)
	after := time.Now()

	if m.CreatedAt.Before(before) || m.CreatedAt.After(after) {
		t.Errorf("CreatedAt should be between %v and %v, got %v", before, after, m.CreatedAt)
	}
}

func TestAllMetricTypesSlice(t *testing.T) {
	expectedCount := 21 // Total number of metric types

	if len(AllMetricTypes) != expectedCount {
		t.Errorf("AllMetricTypes has %d types, want %d", len(AllMetricTypes), expectedCount)
	}

	// Verify all types are unique
	seen := make(map[MetricType]bool)
	for _, mt := range AllMetricTypes {
		if seen[mt] {
			t.Errorf("Duplicate metric type: %s", mt)
		}
		seen[mt] = true
	}
}

func TestMetricChaining(t *testing.T) {
	customTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	m := NewMetric(MetricWeight, 82.5).
		WithRecordedAt(customTime).
		WithNotes("chained call")

	if !m.RecordedAt.Equal(customTime) {
		t.Errorf("RecordedAt = %v, want %v", m.RecordedAt, customTime)
	}
	if m.Notes == nil || *m.Notes != "chained call" {
		t.Error("Notes should be 'chained call'")
	}
}
