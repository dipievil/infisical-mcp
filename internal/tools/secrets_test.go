package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"

	"github.com/dipievil/infisical-mcp/internal/config"
	"github.com/dipievil/infisical-mcp/internal/infisical"
	"github.com/dipievil/infisical-mcp/internal/tools"
)

// setupServer creates a test MCP server wired to a mock Infisical HTTP server.
func setupServer(t *testing.T, safeMode bool, infisicalHandler http.HandlerFunc) *server.MCPServer {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/universal-auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"accessToken":       "tok",
			"expiresIn":         3600,
			"accessTokenMaxTTL": 86400,
			"tokenType":         "Bearer",
		})
	})
	mux.HandleFunc("/", infisicalHandler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		InfisicalHost: srv.URL,
		ClientID:      "id",
		ClientSecret:  "secret",
		ProjectID:     "proj",
		Environment:   "dev",
		SafeMode:      safeMode,
	}

	client := infisical.New(srv.URL, "id", "secret", "proj", "dev")

	s := server.NewMCPServer("test", "0.0.0", server.WithToolCapabilities(false))
	tools.Register(s, cfg, client)

	return s
}

// toolCallMsg builds a JSON-RPC 2.0 tools/call request message.
func toolCallMsg(name string, args map[string]any) []byte {
	argsJSON, _ := json.Marshal(args)
	msg := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s}}`,
		name, argsJSON)
	return []byte(msg)
}

// toolResult holds the parsed result of a tools/call response.
type toolResult struct {
	IsError bool
	Text    string
}

// parseToolResponse parses the JSON-RPC response from HandleMessage into a toolResult.
func parseToolResponse(t *testing.T, resp any) toolResult {
	t.Helper()

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var raw struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nraw: %s", err, string(b))
	}

	tr := toolResult{IsError: raw.Result.IsError}
	for _, c := range raw.Result.Content {
		tr.Text += c.Text
	}
	return tr
}

// ---- list_secrets tests -------------------------------------------------

func TestListSecrets_Tool(t *testing.T) {
	s := setupServer(t, false, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/secrets/raw" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"secrets": []map[string]any{
				{"secretKey": "FOO", "secretValue": "bar"},
			},
		})
	})

	resp := s.HandleMessage(context.Background(), toolCallMsg("list_secrets", nil))
	result := parseToolResponse(t, resp)

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Text)
	}
	if !strings.Contains(result.Text, "FOO") {
		t.Errorf("expected result to contain FOO, got: %s", result.Text)
	}
}

// ---- get_secret tests ---------------------------------------------------

func TestGetSecret_Tool(t *testing.T) {
	s := setupServer(t, false, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/secrets/raw/MY_KEY" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"secret": map[string]any{
				"secretKey":   "MY_KEY",
				"secretValue": "MY_VALUE",
			},
		})
	})

	resp := s.HandleMessage(context.Background(), toolCallMsg("get_secret", map[string]any{"key": "MY_KEY"}))
	result := parseToolResponse(t, resp)

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Text)
	}
	if !strings.Contains(result.Text, "MY_VALUE") {
		t.Errorf("expected result to contain MY_VALUE, got: %s", result.Text)
	}
}

func TestGetSecret_MissingKey(t *testing.T) {
	s := setupServer(t, false, func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	})

	resp := s.HandleMessage(context.Background(), toolCallMsg("get_secret", map[string]any{}))
	result := parseToolResponse(t, resp)

	if !result.IsError {
		t.Error("expected tool to return an error for missing key param")
	}
}

// ---- set_secret tests ---------------------------------------------------

func TestSetSecret_Tool_SafeModeOff(t *testing.T) {
	called := false
	s := setupServer(t, false, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			called = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"secret": map[string]any{"secretKey": "K", "secretValue": "V"},
			})
			return
		}
		http.NotFound(w, r)
	})

	resp := s.HandleMessage(context.Background(), toolCallMsg("set_secret", map[string]any{
		"key":   "K",
		"value": "V",
	}))
	result := parseToolResponse(t, resp)

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Text)
	}
	if !called {
		t.Error("expected PATCH to be called on Infisical")
	}
}

func TestSetSecret_Tool_SafeModeOn(t *testing.T) {
	called := false
	s := setupServer(t, true, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			called = true
			// Verify the request uses the default project ID ("proj")
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("failed to decode request body: %v", err)
			}
			if body["workspaceId"] != "proj" {
				t.Errorf("expected default project 'proj', got %q", body["workspaceId"])
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"secret": map[string]any{"secretKey": "K", "secretValue": "V"},
			})
			return
		}
		http.NotFound(w, r)
	})

	// Pass a custom project_id; SafeMode should ignore it and use the default.
	resp := s.HandleMessage(context.Background(), toolCallMsg("set_secret", map[string]any{
		"key":        "K",
		"value":      "V",
		"project_id": "other-project",
	}))
	result := parseToolResponse(t, resp)

	if result.IsError {
		t.Fatalf("set_secret should succeed in safe mode (restricted to default project): %s", result.Text)
	}
	if !called {
		t.Error("expected PATCH to be called on Infisical even in safe mode")
	}
}
