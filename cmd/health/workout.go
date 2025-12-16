// ABOUTME: CLI commands for managing workouts.
// ABOUTME: Supports add, list, show, and metric subcommands.
package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/fatih/color"
	"github.com/harperreed/health/internal/db"
	"github.com/harperreed/health/internal/models"
	"github.com/harperreed/health/internal/sync"
	"github.com/spf13/cobra"
	"github.com/harperreed/sweet/vault"
)

var (
	workoutDuration int
	workoutNotes    string
	workoutType     string
	workoutLimit    int
)

var workoutCmd = &cobra.Command{
	Use:     "workout",
	Aliases: []string{"w"},
	Short:   "Manage workouts",
	Long: `Track workout sessions with custom metrics.

Unlike regular metrics which are single values, workouts are sessions that can
have multiple associated metrics (distance, pace, heart rate, sets, reps, etc.)

WORKFLOW:

  1. Create a workout:     health workout add run --duration 30
  2. Add metrics to it:    health workout metric abc123 distance 5.2 km
  3. View workout details: health workout show abc123

COMMANDS:

  add      Create a new workout session
  list     List recent workouts
  show     View workout with all its metrics
  metric   Add a metric to an existing workout

The workout type is freeform - use whatever makes sense for you:
  run, lift, swim, cycle, yoga, hiit, walk, climb, etc.`,
}

var workoutAddCmd = &cobra.Command{
	Use:   "add <type>",
	Short: "Add a new workout",
	Long: `Add a new workout session.

Examples:
  health workout add run --duration 45
  health workout add lift --notes "Leg day"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workoutType := args[0]

		w := models.NewWorkout(workoutType)
		if workoutDuration > 0 {
			w.WithDuration(workoutDuration)
		}
		if workoutNotes != "" {
			w.WithNotes(workoutNotes)
		}

		if err := db.CreateWorkout(dbConn, w); err != nil {
			return fmt.Errorf("failed to create workout: %w", err)
		}

		// Queue for sync if configured
		if err := queueWorkoutSync(cmd.Context(), w, vault.OpUpsert); err != nil {
			color.Yellow("⚠ Sync queue failed: %v", err)
		}

		color.Green("✓ Added %s workout", workoutType)
		fmt.Printf("  ID: %s\n", w.ID.String()[:8])
		if w.DurationMinutes != nil {
			fmt.Printf("  Duration: %d min\n", *w.DurationMinutes)
		}

		return nil
	},
}

var workoutListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List workouts",
	RunE: func(cmd *cobra.Command, args []string) error {
		var wType *string
		if workoutType != "" {
			wType = &workoutType
		}

		workouts, err := db.ListWorkouts(dbConn, wType, workoutLimit)
		if err != nil {
			return fmt.Errorf("failed to list workouts: %w", err)
		}

		if len(workouts) == 0 {
			fmt.Println("No workouts found.")
			return nil
		}

		faint := color.New(color.Faint)
		for _, w := range workouts {
			duration := ""
			if w.DurationMinutes != nil {
				duration = fmt.Sprintf("%d min", *w.DurationMinutes)
			}
			fmt.Printf("%s %s %s %s\n",
				faint.Sprint(w.ID.String()[:8]),
				faint.Sprint(w.StartedAt.Format("2006-01-02 15:04")),
				padRight(w.WorkoutType, 12),
				duration)
		}

		return nil
	},
}

var workoutShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show workout details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, err := db.GetWorkoutWithMetrics(dbConn, args[0])
		if err != nil {
			return fmt.Errorf("failed to get workout: %w", err)
		}

		fmt.Printf("Workout: %s\n", w.ID.String()[:8])
		fmt.Printf("Type: %s\n", w.WorkoutType)
		fmt.Printf("Started: %s\n", w.StartedAt.Format("2006-01-02 15:04"))
		if w.DurationMinutes != nil {
			fmt.Printf("Duration: %d min\n", *w.DurationMinutes)
		}
		if w.Notes != nil {
			fmt.Printf("Notes: %s\n", *w.Notes)
		}

		if len(w.Metrics) > 0 {
			fmt.Println("\nMetrics:")
			for _, m := range w.Metrics {
				unit := ""
				if m.Unit != nil {
					unit = *m.Unit
				}
				fmt.Printf("  %s: %.2f %s\n", m.MetricName, m.Value, unit)
			}
		}

		return nil
	},
}

var workoutMetricCmd = &cobra.Command{
	Use:   "metric <workout-id> <name> <value> [unit]",
	Short: "Add a metric to a workout",
	Long: `Add a metric to an existing workout.

Examples:
  health workout metric abc123 distance 5.2 km
  health workout metric abc123 avg_hr 145 bpm
  health workout metric abc123 sets 4`,
	Args: cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		workoutID := args[0]
		metricName := args[1]
		value, err := strconv.ParseFloat(args[2], 64)
		if err != nil {
			return fmt.Errorf("invalid value: %s", args[2])
		}

		unit := ""
		if len(args) > 3 {
			unit = args[3]
		}

		// Verify workout exists
		w, err := db.GetWorkout(dbConn, workoutID)
		if err != nil {
			return fmt.Errorf("workout not found: %s", workoutID)
		}

		wm := models.NewWorkoutMetric(w.ID, metricName, value, unit)
		if err := db.AddWorkoutMetric(dbConn, wm); err != nil {
			return fmt.Errorf("failed to add workout metric: %w", err)
		}

		// Queue for sync if configured
		if err := queueWorkoutMetricSync(cmd.Context(), wm, vault.OpUpsert); err != nil {
			color.Yellow("⚠ Sync queue failed: %v", err)
		}

		color.Green("✓ Added %s to workout", metricName)
		fmt.Printf("  %.2f %s\n", value, unit)

		return nil
	},
}

func init() {
	workoutAddCmd.Flags().IntVarP(&workoutDuration, "duration", "d", 0, "duration in minutes")
	workoutAddCmd.Flags().StringVarP(&workoutNotes, "notes", "n", "", "workout notes")

	workoutListCmd.Flags().StringVarP(&workoutType, "type", "t", "", "filter by workout type")
	workoutListCmd.Flags().IntVarP(&workoutLimit, "limit", "n", 20, "max number of results")

	workoutCmd.AddCommand(workoutAddCmd)
	workoutCmd.AddCommand(workoutListCmd)
	workoutCmd.AddCommand(workoutShowCmd)
	workoutCmd.AddCommand(workoutMetricCmd)
	rootCmd.AddCommand(workoutCmd)
}

// queueWorkoutSync queues a workout change for sync if configured.
func queueWorkoutSync(ctx context.Context, w *models.Workout, op vault.Op) error {
	cfg, err := sync.LoadConfig()
	if err != nil {
		return nil // No config, skip silently
	}

	if !cfg.IsConfigured() {
		return nil // Not configured, skip silently
	}

	syncer, err := sync.NewSyncer(cfg, dbConn)
	if err != nil {
		return fmt.Errorf("create syncer: %w", err)
	}
	defer func() { _ = syncer.Close() }()

	return syncer.QueueWorkoutChange(ctx, w, op)
}

// queueWorkoutMetricSync queues a workout metric change for sync if configured.
func queueWorkoutMetricSync(ctx context.Context, wm *models.WorkoutMetric, op vault.Op) error {
	cfg, err := sync.LoadConfig()
	if err != nil {
		return nil // No config, skip silently
	}

	if !cfg.IsConfigured() {
		return nil // Not configured, skip silently
	}

	syncer, err := sync.NewSyncer(cfg, dbConn)
	if err != nil {
		return fmt.Errorf("create syncer: %w", err)
	}
	defer func() { _ = syncer.Close() }()

	return syncer.QueueWorkoutMetricChange(ctx, wm, op)
}
