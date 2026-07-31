// ABOUTME: Metric model and MetricType enum for health data.
// ABOUTME: Defines 25 metric types across biometrics, activity, nutrition, mental health.
package models

import (
	"fmt"
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

// MetricRange bounds plausible values for a metric type. Bounds are
// inclusive and deliberately generous: they catch entry mistakes
// (a weight of 8250, an spo2 of 150), not clinical outliers.
type MetricRange struct {
	Min float64
	Max float64
}

// MetricRanges maps each metric type to its plausible value range.
var MetricRanges = map[MetricType]MetricRange{
	MetricWeight:    {0.5, 700},
	MetricBodyFat:   {1, 75},
	MetricBPSys:     {40, 300},
	MetricBPDia:     {20, 200},
	MetricHeartRate: {20, 300},
	MetricHRV:       {1, 500},
	// Temperature covers both body and ambient (Emfit logs bedroom temp);
	// the ceiling still catches Fahrenheit entered as Celsius.
	MetricTemperature:     {0, 50},
	MetricRespiratoryRate: {4, 60},
	MetricSpO2:            {50, 100},
	MetricSteps:           {0, 200000},
	MetricSleepHours:      {0, 24},
	MetricActiveCalories:  {0, 20000},
	MetricRecovery:        {0, 100},
	MetricStrain:          {0, 21},
	MetricWater:           {0, 20000},
	MetricCalories:        {0, 20000},
	MetricProtein:         {0, 2000},
	MetricCarbs:           {0, 2000},
	MetricFat:             {0, 2000},
	MetricMood:            {1, 10},
	MetricEnergy:          {1, 10},
	MetricStress:          {1, 10},
	MetricAnxiety:         {1, 10},
	MetricFocus:           {1, 10},
	MetricMeditation:      {0, 1440},
}

// ValidateValue rejects values outside the plausible range for the type.
// Provider sync and import bypass this on purpose: it guards human and
// agent entry, not machine round-trips of existing data.
func ValidateValue(metricType MetricType, value float64) error {
	r, ok := MetricRanges[metricType]
	if !ok {
		return fmt.Errorf("unknown metric type: %s", metricType)
	}
	if value < r.Min || value > r.Max {
		return fmt.Errorf("%s value %.2f out of range: must be between %g and %g %s",
			metricType, value, r.Min, r.Max, MetricUnits[metricType])
	}
	return nil
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
	now := time.Now().UTC()
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
