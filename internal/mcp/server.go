// ABOUTME: MCP server setup for health metrics store.
// ABOUTME: Wraps MCP server with storage Repository connection.
package mcp

import (
	"context"

	"github.com/harperreed/health/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with storage access.
type Server struct {
	mcpServer *mcp.Server
	repo      storage.Repository
}

// NewServer creates a new MCP server with the given storage.
func NewServer(repo storage.Repository) (*Server, error) {
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "health",
			Version: "1.0.0",
		},
		nil,
	)

	s := &Server{
		mcpServer: mcpServer,
		repo:      repo,
	}

	s.registerTools()
	s.registerResources()

	return s, nil
}

// Serve starts the MCP server using stdio transport.
func (s *Server) Serve(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}
