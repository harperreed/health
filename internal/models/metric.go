// ABOUTME: Metric model and MetricType enum for health data.
// ABOUTME: Defines 22 metric types across biometrics, activity, nutrition, mental health.
package models

import (
	"time"

	"github.com/google/uuid"
)

// MetricType represents the type of health metric being recorded.
type MetricType string

const (
	// Biometrics
	MetricWeight      MetricType = "weight"
	MetricBodyFat     MetricType = "body_fat"
	MetricBPSys       MetricType = "bp_sys"
	MetricBPDia       MetricType = "bp_dia"
	MetricHeartRate   MetricType = "heart_rate"
	MetricHRV         MetricType = "hrv"
	MetricTemperature MetricType = "temperature"

	// Activity
	MetricSteps          MetricType = "steps"
	MetricSleepHours     MetricType = "sleep_hours"
	MetricActiveCalories MetricType = "active_calories"

	// Nutrition
	MetricWater    MetricType = "water"
	MetricCalories MetricType = "calories"
	MetricProtein  MetricType = "protein"
	MetricCarbs    MetricType = "carbs"
	MetricFat      MetricType = "fat"

	// Mental Health
	MetricMood       MetricType = "mood"
	MetricEnergy     MetricType = "energy"
	MetricStress     MetricType = "stress"
	MetricAnxiety    MetricType = "anxiety"
	MetricFocus      MetricType = "focus"
	MetricMeditation MetricType = "meditation"
)

// MetricUnits maps metric types to their display units.
var MetricUnits = map[MetricType]string{
	MetricWeight:         "kg",
	MetricBodyFat:        "%",
	MetricBPSys:          "mmHg",
	MetricBPDia:          "mmHg",
	MetricHeartRate:      "bpm",
	MetricHRV:            "ms",
	MetricTemperature:    "°C",
	MetricSteps:          "steps",
	MetricSleepHours:     "hours",
	MetricActiveCalories: "kcal",
	MetricWater:          "ml",
	MetricCalories:       "kcal",
	MetricProtein:        "g",
	MetricCarbs:          "g",
	MetricFat:            "g",
	MetricMood:           "scale",
	MetricEnergy:         "scale",
	MetricStress:         "scale",
	MetricAnxiety:        "scale",
	MetricFocus:          "scale",
	MetricMeditation:     "min",
}

// AllMetricTypes returns all valid metric types.
var AllMetricTypes = []MetricType{
	MetricWeight, MetricBodyFat, MetricBPSys, MetricBPDia,
	MetricHeartRate, MetricHRV, MetricTemperature,
	MetricSteps, MetricSleepHours, MetricActiveCalories,
	MetricWater, MetricCalories, MetricProtein, MetricCarbs, MetricFat,
	MetricMood, MetricEnergy, MetricStress, MetricAnxiety, MetricFocus, MetricMeditation,
}

// IsValidMetricType checks if a string is a valid metric type.
func IsValidMetricType(s string) bool {
	for _, mt := range AllMetricTypes {
		if string(mt) == s {
			return true
		}
	}
	return false
}

// Metric represents a single health metric entry.
type Metric struct {
	ID         uuid.UUID
	MetricType MetricType
	Value      float64
	Unit       string
	RecordedAt time.Time
	Notes      *string
	CreatedAt  time.Time
}

// NewMetric creates a new Metric with generated UUID and current timestamp.
func NewMetric(metricType MetricType, value float64) *Metric {
	now := time.Now()
	return &Metric{
		ID:         uuid.New(),
		MetricType: metricType,
		Value:      value,
		Unit:       MetricUnits[metricType],
		RecordedAt: now,
		CreatedAt:  now,
	}
}

// WithRecordedAt sets a custom recorded_at timestamp.
func (m *Metric) WithRecordedAt(t time.Time) *Metric {
	m.RecordedAt = t
	return m
}

// WithNotes sets notes on the metric.
func (m *Metric) WithNotes(notes string) *Metric {
	m.Notes = &notes
	return m
}
