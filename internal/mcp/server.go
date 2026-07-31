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

// NewServer creates a new MCP server with the given storage. The version
// is reported in the MCP server info; empty falls back to "dev".
func NewServer(repo storage.Repository, version string) (*Server, error) {
	if version == "" {
		version = "dev"
	}
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "health",
			Version: version,
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
