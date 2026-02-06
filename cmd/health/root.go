// ABOUTME: Root Cobra command for health CLI.
// ABOUTME: Handles storage lifecycle via PersistentPre/PostRunE using config-driven backend.
package main

import (
	"fmt"

	"github.com/harperreed/health/internal/config"
	"github.com/harperreed/health/internal/storage"
	"github.com/spf13/cobra"
)

var (
	repo storage.Repository
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

DATA EXPORT:

  $ health export json                  # Export to JSON
  $ health export yaml                  # Export to YAML
  $ health export markdown              # Export to Markdown
  $ health import backup.json           # Import from JSON

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

  Metrics are stored locally. Default backend is SQLite at ~/.local/share/health/health.db.
  Use 'health migrate --to markdown' to switch to markdown file storage.
  Configuration is at ~/.config/health/config.json.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip init for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		repo, err = cfg.OpenStorage()
		if err != nil {
			return fmt.Errorf("failed to open storage: %w", err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if repo != nil {
			return repo.Close()
		}
		return nil
	},
}

func init() {
	// No persistent flags needed - database location follows XDG spec
}
