// ABOUTME: CLI command for adding health metrics.
// ABOUTME: Handles single metrics and blood pressure special case.
package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/harperreed/health/internal/db"
	"github.com/harperreed/health/internal/models"
	"github.com/spf13/cobra"
)

var (
	addAt    string
	addNotes string
)

var addCmd = &cobra.Command{
	Use:     "add <type> <value> [value2]",
	Aliases: []string{"a"},
	Short:   "Add a health metric",
	Long: `Add a health metric. For blood pressure, provide both systolic and diastolic values.

Examples:
  health add weight 82.5
  health add hrv 48 --at "2024-12-14 07:00"
  health add bp 120 80
  health add mood 7 --notes "Good day"`,
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
			return fmt.Errorf("unknown metric type: %s\nValid types: weight, body_fat, bp_sys, bp_dia, heart_rate, hrv, temperature, steps, sleep_hours, active_calories, water, calories, protein, carbs, fat, mood, energy, stress, anxiety, focus, meditation", metricType)
		}

		value, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return fmt.Errorf("invalid value: %s", args[1])
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

		if err := db.CreateMetric(dbConn, m); err != nil {
			return fmt.Errorf("failed to create metric: %w", err)
		}

		color.Green("✓ Added %s", metricType)
		fmt.Printf("  %s %.2f %s\n",
			color.New(color.Faint).Sprint(m.ID.String()[:8]),
			m.Value, m.Unit)

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

	// Use same timestamp for both
	var recordedAt time.Time
	if addAt != "" {
		var err error
		recordedAt, err = parseTime(addAt)
		if err != nil {
			return fmt.Errorf("invalid timestamp: %s", addAt)
		}
	} else {
		recordedAt = time.Now()
	}

	mSys := models.NewMetric(models.MetricBPSys, sys).WithRecordedAt(recordedAt)
	mDia := models.NewMetric(models.MetricBPDia, dia).WithRecordedAt(recordedAt)

	if addNotes != "" {
		mSys.WithNotes(addNotes)
		mDia.WithNotes(addNotes)
	}

	if err := db.CreateMetric(dbConn, mSys); err != nil {
		return fmt.Errorf("failed to create bp_sys: %w", err)
	}
	if err := db.CreateMetric(dbConn, mDia); err != nil {
		return fmt.Errorf("failed to create bp_dia: %w", err)
	}

	color.Green("✓ Added blood pressure")
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
	rootCmd.AddCommand(addCmd)
}
