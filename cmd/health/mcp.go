// ABOUTME: CLI command for starting MCP server.
// ABOUTME: Runs stdio-based MCP server for Claude integration.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/harperreed/health/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server",
	Long: `Start the Model Context Protocol (MCP) server for AI assistant integration.

MCP allows AI assistants like Claude to interact with your health data through
a standardized protocol. The server communicates via stdin/stdout.

CLAUDE DESKTOP CONFIGURATION:

  Add this to your Claude Desktop config (claude_desktop_config.json):

  {
    "mcpServers": {
      "health": {
        "command": "health",
        "args": ["mcp"]
      }
    }
  }

  On macOS, the config is at:
    ~/Library/Application Support/Claude/claude_desktop_config.json

AVAILABLE TOOLS:

  add_metric          Record a health metric
  list_metrics        List recent metrics
  delete_metric       Delete a metric by ID
  add_workout         Create a workout session
  add_workout_metric  Add a metric to a workout
  list_workouts       List recent workouts
  get_workout         Get workout with all metrics
  delete_workout      Delete a workout
  get_latest          Get most recent value for metric types

AVAILABLE RESOURCES:

  health://metrics/recent     Recent metrics summary
  health://metrics/today      Today's metrics
  health://workouts/recent    Recent workouts`,
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := mcp.NewServer(charmClient)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle shutdown signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			cancel()
		}()

		return server.Serve(ctx)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
