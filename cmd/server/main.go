// Command server starts the Infisical MCP server.
//
// Configuration is provided via environment variables – see internal/config for
// the full list. The server communicates over stdio using the Model Context
// Protocol (MCP) JSON-RPC 2.0 framing so that it can be wired directly into any
// MCP-capable AI client (Claude Desktop, Cursor, VS Code Copilot, etc.).
//
// Quick start:
//
//	export INFISICAL_HOST=http://192.168.1.10:8080
//	export INFISICAL_CLIENT_ID=<id>
//	export INFISICAL_CLIENT_SECRET=<secret>
//	export INFISICAL_PROJECT_ID=<project-id>
//	export INFISICAL_ENVIRONMENT=dev
//	./infisical-mcp
package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/dipievil/infisical-mcp/internal/config"
	"github.com/dipievil/infisical-mcp/internal/infisical"
	"github.com/dipievil/infisical-mcp/internal/tools"
)

const (
	serverName    = "infisical-mcp"
	serverVersion = "0.1.0"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	client := infisical.New(
		cfg.InfisicalHost,
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.ProjectID,
		cfg.Environment,
	)

	s := server.NewMCPServer(serverName, serverVersion,
		server.WithToolCapabilities(false),
	)

	tools.Register(s, cfg, client)

	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}
