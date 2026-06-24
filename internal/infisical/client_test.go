package infisical_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dipievil/infisical-mcp/internal/infisical"
)

// newTestServer creates a mock Infisical HTTP server.
// loginHandler handles the Universal Auth login endpoint.
// apiHandler handles all other requests.
func newTestServer(t *testing.T, loginHandler, apiHandler http.HandlerFunc) (*httptest.Server, *infisical.Client) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/universal-auth/login", loginHandler)
	mux.HandleFunc("/", apiHandler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := infisical.New(srv.URL, "client-id", "client-secret", "proj-id", "dev")
	return srv, client
}

func loginOKHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"accessToken":       "test-token",
			"expiresIn":         3600,
			"accessTokenMaxTTL": 86400,
			"tokenType":         "Bearer",
		})
	}
}

func TestListSecrets(t *testing.T) {
	apiCalled := false
	_, client := newTestServer(t, loginOKHandler(t), func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/secrets/raw" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("unexpected authorization header: %q", auth)
		}
		apiCalled = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"secrets": []map[string]any{
				{"secretKey": "DB_HOST", "secretValue": "localhost"},
				{"secretKey": "DB_PASS", "secretValue": "secret"},
			},
		})
	})

	secrets, err := client.ListSecrets(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !apiCalled {
		t.Error("API endpoint was not called")
	}
	if len(secrets) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(secrets))
	}
	if secrets[0].Key != "DB_HOST" {
		t.Errorf("unexpected first key: %q", secrets[0].Key)
	}
}

func TestGetSecret(t *testing.T) {
	_, client := newTestServer(t, loginOKHandler(t), func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/secrets/raw/MY_SECRET" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"secret": map[string]any{
				"secretKey":   "MY_SECRET",
				"secretValue": "my-value",
			},
		})
	})

	secret, err := client.GetSecret(context.Background(), "MY_SECRET", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret.Key != "MY_SECRET" {
		t.Errorf("unexpected key: %q", secret.Key)
	}
	if secret.Value != "my-value" {
		t.Errorf("unexpected value: %q", secret.Value)
	}
}

func TestGetSecret_NotFound(t *testing.T) {
	_, client := newTestServer(t, loginOKHandler(t), func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	_, err := client.GetSecret(context.Background(), "MISSING", "")
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestSetSecret(t *testing.T) {
	called := false
	_, client := newTestServer(t, loginOKHandler(t), func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/secrets/raw/DB_HOST" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		called = true

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if body["secretValue"] != "newvalue" {
			t.Errorf("unexpected secretValue: %q", body["secretValue"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"secret": map[string]any{"secretKey": "DB_HOST", "secretValue": "newvalue"},
		})
	})

	if err := client.SetSecret(context.Background(), "DB_HOST", "newvalue", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("PATCH endpoint was not called")
	}
}

func TestSetSecret_ServerError(t *testing.T) {
	_, client := newTestServer(t, loginOKHandler(t), func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})

	if err := client.SetSecret(context.Background(), "KEY", "val", ""); err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestLoginFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/universal-auth/login", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Error("API should not be called after login failure")
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := infisical.New(srv.URL, "bad-id", "bad-secret", "proj", "dev")

	_, err := client.ListSecrets(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for login failure")
	}
}
