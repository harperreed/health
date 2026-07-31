// ABOUTME: CLI command for adding health metrics.
// ABOUTME: Handles single metrics and blood pressure special case.
package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/harperreed/health/internal/models"
	"github.com/spf13/cobra"
)

var (
	addAt     string
	addNotes  string
	addSource string
	addDedupe bool
)

var addCmd = &cobra.Command{
	Use:     "add <type> <value> [value2]",
	Aliases: []string{"a"},
	Short:   "Add a health metric",
	Long: `Add a health metric to your personal health log.

METRIC TYPES:

  Biometrics:
    weight            Body weight in kg
    body_fat          Body fat percentage
    bp                Blood pressure (requires TWO values: systolic diastolic)
    heart_rate        Resting heart rate in bpm
    hrv               Heart rate variability in ms
    temperature       Body temperature in °C
    respiratory_rate  Breathing rate in brpm
    spo2              Blood oxygen saturation %

  Activity:
    steps             Daily step count
    sleep_hours       Hours of sleep
    active_calories   Calories burned through activity
    recovery          Recovery score (0-100%)
    strain            Strain score (0-21)

  Nutrition:
    water          Water intake in ml
    calories       Total calorie intake
    protein        Protein intake in grams
    carbs          Carbohydrate intake in grams
    fat            Fat intake in grams

  Mental Health (1-10 scale):
    mood           Overall mood rating
    energy         Energy level rating
    stress         Stress level rating
    anxiety        Anxiety level rating
    focus          Focus/concentration rating
    meditation     Meditation duration in minutes

SOURCES:

  Use --source to tag where the data came from.
  Known sources: whoop, withings, emfit, manual.
  Free-form values are accepted. Default is manual.

EXAMPLES:

  health add weight 82.5                    # Log weight
  health add bp 120 80                      # Blood pressure (sys/dia)
  health add hrv 48 --at "2024-12-14 07:00" # HRV with specific timestamp
  health add mood 7 --notes "Great day!"    # Mood with notes
  health add steps 10432                    # Daily steps
  health add sleep_hours 7.5                # Sleep duration
  health add hrv 48 --source whoop --dedupe   # Idempotent sync write
  health add recovery 85 --source whoop     # Recovery score from Whoop

TIMESTAMPS:

  Use --at to record a metric for a specific time:
    --at "2024-12-14 07:00"
    --at "2024-12-14T07:00"
    --at "2024-12-14"`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		metricType := args[0]

		// Handle blood pressure special case
		if metricType == "bp" {
			if len(args) < 3 {
				return fmt.Errorf("blood pressure requires two values: systolic and diastolic")
			}
			return addBloodPressure(args[1], args[2])
		}

		// Validate metric type
		if !models.IsValidMetricType(metricType) {
			return fmt.Errorf("unknown metric type: %s\nValid types: %s", metricType, models.ValidMetricTypesList())
		}

		value, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return fmt.Errorf("invalid value: %s", args[1])
		}
		if err := models.ValidateValue(models.MetricType(metricType), value); err != nil {
			return err
		}

		m := models.NewMetric(models.MetricType(metricType), value)

		// Handle --at flag
		if addAt != "" {
			t, err := parseTime(addAt)
			if err != nil {
				return fmt.Errorf("invalid timestamp: %s", addAt)
			}
			m.WithRecordedAt(t)
		}

		// Handle --notes flag
		if addNotes != "" {
			m.WithNotes(addNotes)
		}

		// Handle --source flag
		if addSource != "" {
			m.WithSource(addSource)
		}

		verb := "Added"
		if addDedupe {
			updated, err := repo.UpsertMetric(m)
			if err != nil {
				return fmt.Errorf("failed to upsert metric: %w", err)
			}
			if updated {
				verb = "Updated"
			}
		} else if err := repo.CreateMetric(m); err != nil {
			return fmt.Errorf("failed to create metric: %w", err)
		}

		color.Green("✓ %s %s", verb, metricType)
		fmt.Printf("  %s %.2f %s [%s]\n",
			color.New(color.Faint).Sprint(m.ID.String()[:8]),
			m.Value, m.Unit, m.Source)

		return nil
	},
}

func addBloodPressure(sysStr, diaStr string) error {
	sys, err := strconv.ParseFloat(sysStr, 64)
	if err != nil {
		return fmt.Errorf("invalid systolic value: %s", sysStr)
	}
	dia, err := strconv.ParseFloat(diaStr, 64)
	if err != nil {
		return fmt.Errorf("invalid diastolic value: %s", diaStr)
	}
	if err := models.ValidateValue(models.MetricBPSys, sys); err != nil {
		return err
	}
	if err := models.ValidateValue(models.MetricBPDia, dia); err != nil {
		return err
	}

	// Use same timestamp for both
	var recordedAt time.Time
	if addAt != "" {
		var err error
		recordedAt, err = parseTime(addAt)
		if err != nil {
			return fmt.Errorf("invalid timestamp: %s", addAt)
		}
	} else {
		recordedAt = time.Now().UTC()
	}

	mSys := models.NewMetric(models.MetricBPSys, sys).WithRecordedAt(recordedAt)
	mDia := models.NewMetric(models.MetricBPDia, dia).WithRecordedAt(recordedAt)

	if addNotes != "" {
		mSys.WithNotes(addNotes)
		mDia.WithNotes(addNotes)
	}

	if addSource != "" {
		mSys.WithSource(addSource)
		mDia.WithSource(addSource)
	}

	var sysUpdated, diaUpdated bool
	if addDedupe {
		sysUpdated, err = repo.UpsertMetric(mSys)
		if err != nil {
			return fmt.Errorf("failed to upsert bp_sys: %w", err)
		}
		diaUpdated, err = repo.UpsertMetric(mDia)
		if err != nil {
			return fmt.Errorf("failed to upsert bp_dia: %w", err)
		}
	} else {
		if err := repo.CreateMetric(mSys); err != nil {
			return fmt.Errorf("failed to create bp_sys: %w", err)
		}
		if err := repo.CreateMetric(mDia); err != nil {
			return fmt.Errorf("failed to create bp_dia: %w", err)
		}
	}

	verb := "Added"
	if sysUpdated || diaUpdated {
		verb = "Updated"
	}
	color.Green("✓ %s blood pressure", verb)
	fmt.Printf("  %s %.0f/%.0f mmHg\n",
		color.New(color.Faint).Sprint(mSys.ID.String()[:8]),
		sys, dia)

	return nil
}

func parseTime(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		"2006-01-02",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format")
}

func init() {
	addCmd.Flags().StringVar(&addAt, "at", "", "timestamp (YYYY-MM-DD HH:MM)")
	addCmd.Flags().StringVar(&addNotes, "notes", "", "notes for the metric")
	addCmd.Flags().StringVar(&addSource, "source", "", "data source: whoop, withings, emfit, manual, or free-form (default manual)")
	addCmd.Flags().BoolVar(&addDedupe, "dedupe", false, "replace an existing entry with the same source, type, and timestamp instead of adding a duplicate")
	rootCmd.AddCommand(addCmd)
}
