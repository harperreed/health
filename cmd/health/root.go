// ABOUTME: Root Cobra command for health CLI.
// ABOUTME: Handles database lifecycle via PersistentPre/PostRunE.
package main

import (
	"database/sql"
	"fmt"

	"github.com/harperreed/health/internal/db"
	"github.com/spf13/cobra"
)

var (
	dbPath string
	dbConn *sql.DB
)

var rootCmd = &cobra.Command{
	Use:   "health",
	Short: "Personal health metrics tracker",
	Long: `Health is a CLI tool for tracking personal health metrics.

WHAT IT TRACKS:

  Biometrics     weight, body_fat, bp (blood pressure), heart_rate, hrv, temperature
  Activity       steps, sleep_hours, active_calories
  Nutrition      water, calories, protein, carbs, fat
  Mental Health  mood, energy, stress, anxiety, focus, meditation

QUICK START:

  $ health add weight 82.5              # Log your weight
  $ health add bp 120 80                # Log blood pressure (systolic/diastolic)
  $ health add mood 7 --notes "Great!"  # Log mood with notes
  $ health list                         # See recent metrics
  $ health list --type weight           # Filter by type

WORKOUTS:

  $ health workout add run --duration 30    # Log a workout
  $ health workout metric abc123 km 5.2     # Add distance to workout
  $ health workout show abc123              # View workout details

MCP INTEGRATION:

  Run 'health mcp' to start the Model Context Protocol server for use with
  Claude Desktop or other MCP-compatible AI assistants. Add to your Claude
  config:

  {
    "mcpServers": {
      "health": { "command": "health", "args": ["mcp"] }
    }
  }

DATA STORAGE:

  Metrics are stored in SQLite at ~/.local/share/health/health.db
  Use --db to specify an alternate location.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip DB init for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}

		var err error
		dbConn, err = db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if dbConn != nil {
			return dbConn.Close()
		}
		return nil
	},
}

func init() {
	defaultPath := db.GetDefaultDBPath()
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultPath, "database file path")
}
