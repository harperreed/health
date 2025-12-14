// ABOUTME: MCP server setup for health metrics store.
// ABOUTME: Wraps MCP server with database connection.
package mcp

import (
	"context"
	"database/sql"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with database access.
type Server struct {
	mcpServer *mcp.Server
	db        *sql.DB
}

// NewServer creates a new MCP server with the given database connection.
func NewServer(db *sql.DB) (*Server, error) {
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "health",
			Version: "1.0.0",
		},
		nil,
	)

	s := &Server{
		mcpServer: mcpServer,
		db:        db,
	}

	s.registerTools()
	s.registerResources()

	return s, nil
}

// Serve starts the MCP server using stdio transport.
func (s *Server) Serve(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}
