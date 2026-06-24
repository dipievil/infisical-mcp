// Package tools registers the MCP tools exposed by the Infisical MCP server.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/dipievil/infisical-mcp/internal/config"
	"github.com/dipievil/infisical-mcp/internal/infisical"
)

// Register adds all Infisical tools to the given MCP server.
func Register(s *server.MCPServer, cfg *config.Config, client *infisical.Client) {
	registerListSecrets(s, client)
	registerGetSecret(s, client)
	registerSetSecret(s, cfg, client)
}

// ---- list_secrets -------------------------------------------------------

func registerListSecrets(s *server.MCPServer, client *infisical.Client) {
	tool := mcp.NewTool("list_secrets",
		mcp.WithDescription(
			"List all secret keys available in the configured Infisical project and environment. "+
				"Returns an array of objects containing each secret's key and an optional comment.",
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		secrets, err := client.ListSecrets(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list secrets: %v", err)), nil
		}

		type entry struct {
			Key     string `json:"key"`
			Comment string `json:"comment,omitempty"`
			Version int    `json:"version,omitempty"`
		}

		entries := make([]entry, 0, len(secrets))
		for _, s := range secrets {
			entries = append(entries, entry{
				Key:     s.Key,
				Comment: s.Comment,
				Version: s.Version,
			})
		}

		out, err := json.Marshal(entries)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to encode result: %v", err)), nil
		}

		return mcp.NewToolResultText(string(out)), nil
	})
}

// ---- get_secret ---------------------------------------------------------

func registerGetSecret(s *server.MCPServer, client *infisical.Client) {
	tool := mcp.NewTool("get_secret",
		mcp.WithDescription(
			"Retrieve the plaintext value of a single secret from Infisical.",
		),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("The name (key) of the secret to retrieve."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid parameter: %v", err)), nil
		}

		secret, err := client.GetSecret(ctx, key)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get secret: %v", err)), nil
		}

		type result struct {
			Key     string `json:"key"`
			Value   string `json:"value"`
			Comment string `json:"comment,omitempty"`
			Version int    `json:"version,omitempty"`
		}

		out, err := json.Marshal(result{
			Key:     secret.Key,
			Value:   secret.Value,
			Comment: secret.Comment,
			Version: secret.Version,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to encode result: %v", err)), nil
		}

		return mcp.NewToolResultText(string(out)), nil
	})
}

// ---- set_secret ---------------------------------------------------------

func registerSetSecret(s *server.MCPServer, cfg *config.Config, client *infisical.Client) {
	tool := mcp.NewTool("set_secret",
		mcp.WithDescription(
			"Update (or create) the value of a secret in Infisical. "+
				"This operation is only available when INFISICAL_SAFE_MODE is not set to \"true\".",
		),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("The name (key) of the secret to update."),
		),
		mcp.WithString("value",
			mcp.Required(),
			mcp.Description("The new plaintext value for the secret."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if cfg.SafeMode {
			return mcp.NewToolResultError(config.ErrSafeModeEnabled.Error()), nil
		}

		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid parameter 'key': %v", err)), nil
		}

		value, err := req.RequireString("value")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid parameter 'value': %v", err)), nil
		}

		if err := client.SetSecret(ctx, key, value); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to set secret: %v", err)), nil
		}

		out, err := json.Marshal(map[string]string{
			"status": "ok",
			"key":    key,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to encode result: %v", err)), nil
		}

		return mcp.NewToolResultText(string(out)), nil
	})
}
