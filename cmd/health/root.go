// ABOUTME: Root Cobra command for health CLI.
// ABOUTME: Handles Charm client lifecycle via PersistentPre/PostRunE.
package main

import (
	"fmt"

	"github.com/harperreed/health/internal/charm"
	"github.com/spf13/cobra"
)

var (
	charmClient *charm.Client
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

SYNC (AUTOMATIC):

  Sync health data across devices using Charm Cloud.
  Data is E2E encrypted with your SSH key.

  $ health sync link      # Link device to your Charm account
  $ health sync status    # Check sync status
  $ health sync wipe      # Reset local data from cloud

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

  Metrics are stored in Charm KV at ~/.local/share/charm/kv/health.
  Sync is automatic on each write operation.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip charm init for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}

		var err error
		charmClient, err = charm.InitClient()
		if err != nil {
			return fmt.Errorf("failed to initialize charm client: %w", err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if charmClient != nil {
			return charmClient.Close()
		}
		return nil
	},
}

func init() {
	// No persistent flags needed - Charm handles data location automatically
}
