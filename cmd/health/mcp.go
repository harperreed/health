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
	Long:  `Start the Model Context Protocol server for integration with Claude and other MCP clients.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := mcp.NewServer(dbConn)
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
