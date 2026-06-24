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
	registerListSecrets(s, cfg, client)
	registerGetSecret(s, cfg, client)
	registerSetSecret(s, cfg, client)
}

// ---- list_secrets -------------------------------------------------------

func registerListSecrets(s *server.MCPServer, cfg *config.Config, client *infisical.Client) {
	tool := mcp.NewTool("list_secrets",
		mcp.WithDescription(
			"List all secret keys available in the configured Infisical project and environment. "+
				"Returns an array of objects containing each secret's key and an optional comment. "+
				"Optionally supply a project_id to target a specific project; when INFISICAL_SAFE_MODE "+
				"is enabled the default project is always used regardless of project_id.",
		),
		mcp.WithString("project_id",
			mcp.Description("Optional project / workspace ID. Falls back to DEFAULT_INFISICAL_PROJECT_ID when not provided or when safe mode is enabled."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID := resolveProjectID(req, cfg)

		secrets, err := client.ListSecrets(ctx, projectID)
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

func registerGetSecret(s *server.MCPServer, cfg *config.Config, client *infisical.Client) {
	tool := mcp.NewTool("get_secret",
		mcp.WithDescription(
			"Retrieve the plaintext value of a single secret from Infisical. "+
				"Optionally supply a project_id to target a specific project; when INFISICAL_SAFE_MODE "+
				"is enabled the default project is always used regardless of project_id.",
		),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("The name (key) of the secret to retrieve."),
		),
		mcp.WithString("project_id",
			mcp.Description("Optional project / workspace ID. Falls back to DEFAULT_INFISICAL_PROJECT_ID when not provided or when safe mode is enabled."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid parameter: %v", err)), nil
		}

		projectID := resolveProjectID(req, cfg)

		secret, err := client.GetSecret(ctx, key, projectID)
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
				"Optionally supply a project_id to target a specific project; when INFISICAL_SAFE_MODE "+
				"is enabled the default project is always used regardless of project_id.",
		),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("The name (key) of the secret to update."),
		),
		mcp.WithString("value",
			mcp.Required(),
			mcp.Description("The new plaintext value for the secret."),
		),
		mcp.WithString("project_id",
			mcp.Description("Optional project / workspace ID. Falls back to DEFAULT_INFISICAL_PROJECT_ID when not provided or when safe mode is enabled."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid parameter 'key': %v", err)), nil
		}

		value, err := req.RequireString("value")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid parameter 'value': %v", err)), nil
		}

		projectID := resolveProjectID(req, cfg)

		if err := client.SetSecret(ctx, key, value, projectID); err != nil {
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

// ---- helpers ------------------------------------------------------------

// resolveProjectID returns the project_id to use for a tool call.
// When SafeMode is true the configured default project is always used.
// Otherwise the value of the "project_id" argument from the request is used,
// falling back to an empty string (which causes the client to use its default).
func resolveProjectID(req mcp.CallToolRequest, cfg *config.Config) string {
	if cfg.SafeMode {
		return cfg.ProjectID
	}
	if id, ok := req.GetArguments()["project_id"].(string); ok {
		return id
	}
	return ""
}
