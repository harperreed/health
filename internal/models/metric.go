// ABOUTME: Metric model and MetricType enum for health data.
// ABOUTME: Defines 25 metric types across biometrics, activity, nutrition, mental health.
package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// MetricType represents the type of health metric being recorded.
type MetricType string

const (
	// Biometrics.
	MetricWeight          MetricType = "weight"
	MetricBodyFat         MetricType = "body_fat"
	MetricBPSys           MetricType = "bp_sys"
	MetricBPDia           MetricType = "bp_dia"
	MetricHeartRate       MetricType = "heart_rate"
	MetricHRV             MetricType = "hrv"
	MetricTemperature     MetricType = "temperature"
	MetricRespiratoryRate MetricType = "respiratory_rate"
	MetricSpO2            MetricType = "spo2"

	// Activity.
	MetricSteps          MetricType = "steps"
	MetricSleepHours     MetricType = "sleep_hours"
	MetricActiveCalories MetricType = "active_calories"
	MetricRecovery       MetricType = "recovery"
	MetricStrain         MetricType = "strain"

	// Nutrition.
	MetricWater    MetricType = "water"
	MetricCalories MetricType = "calories"
	MetricProtein  MetricType = "protein"
	MetricCarbs    MetricType = "carbs"
	MetricFat      MetricType = "fat"

	// Mental Health.
	MetricMood       MetricType = "mood"
	MetricEnergy     MetricType = "energy"
	MetricStress     MetricType = "stress"
	MetricAnxiety    MetricType = "anxiety"
	MetricFocus      MetricType = "focus"
	MetricMeditation MetricType = "meditation"
)

// Known metric sources. Source is free-form; these are the built-in ones.
const (
	SourceManual   = "manual"
	SourceWhoop    = "whoop"
	SourceWithings = "withings"
	SourceEmfit    = "emfit"
)

// NormalizeSource canonicalizes a source string: trimmed, lowercased,
// empty defaults to manual.
func NormalizeSource(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return SourceManual
	}
	return s
}

// ValidMetricTypesList returns all metric type names joined for help/error text.
func ValidMetricTypesList() string {
	names := make([]string, len(AllMetricTypes))
	for i, mt := range AllMetricTypes {
		names[i] = string(mt)
	}
	return strings.Join(names, ", ")
}

// MetricUnits maps metric types to their display units.
var MetricUnits = map[MetricType]string{
	MetricWeight:          "kg",
	MetricBodyFat:         "%",
	MetricBPSys:           "mmHg",
	MetricBPDia:           "mmHg",
	MetricHeartRate:       "bpm",
	MetricHRV:             "ms",
	MetricTemperature:     "°C",
	MetricRespiratoryRate: "brpm",
	MetricSpO2:            "%",
	MetricSteps:           "steps",
	MetricSleepHours:      "hours",
	MetricActiveCalories:  "kcal",
	MetricRecovery:        "%",
	MetricStrain:          "score",
	MetricWater:           "ml",
	MetricCalories:        "kcal",
	MetricProtein:         "g",
	MetricCarbs:           "g",
	MetricFat:             "g",
	MetricMood:            "scale",
	MetricEnergy:          "scale",
	MetricStress:          "scale",
	MetricAnxiety:         "scale",
	MetricFocus:           "scale",
	MetricMeditation:      "min",
}

// AllMetricTypes returns all valid metric types.
var AllMetricTypes = []MetricType{
	MetricWeight, MetricBodyFat, MetricBPSys, MetricBPDia,
	MetricHeartRate, MetricHRV, MetricTemperature,
	MetricRespiratoryRate, MetricSpO2,
	MetricSteps, MetricSleepHours, MetricActiveCalories,
	MetricRecovery, MetricStrain,
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
	Source     string
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
		Source:     SourceManual,
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

// WithSource sets the data source (whoop, withings, emfit, manual, or custom).
func (m *Metric) WithSource(source string) *Metric {
	m.Source = NormalizeSource(source)
	return m
}
