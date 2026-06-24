// Package infisical provides an HTTP client for the Infisical REST API.
// It implements Universal Auth (client credentials) to obtain a short-lived
// access token that is transparently refreshed on each request.
package infisical

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Secret represents a single Infisical secret.
type Secret struct {
	// Key is the secret name.
	Key string `json:"secretKey"`
	// Value is the plaintext secret value.
	Value string `json:"secretValue"`
	// Comment is an optional note attached to the secret.
	Comment string `json:"secretComment,omitempty"`
	// Version is the current revision number of the secret.
	Version int `json:"version,omitempty"`
}

// Client is a thin wrapper around the Infisical REST API v3.
type Client struct {
	host        string
	clientID    string
	clientSecret string
	projectID   string
	environment string
	httpClient  *http.Client

	// cached access token and its expiry
	accessToken string
	tokenExpiry time.Time
}

// New creates a new Client.
func New(host, clientID, clientSecret, projectID, environment string) *Client {
	return &Client{
		host:         host,
		clientID:     clientID,
		clientSecret: clientSecret,
		projectID:    projectID,
		environment:  environment,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ---- auth ---------------------------------------------------------------

type loginRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type loginResponse struct {
	AccessToken       string `json:"accessToken"`
	ExpiresIn         int    `json:"expiresIn"`
	AccessTokenMaxTTL int    `json:"accessTokenMaxTTL"`
	TokenType         string `json:"tokenType"`
}

// ensureToken obtains (or reuses) a Universal Auth access token.
func (c *Client) ensureToken(ctx context.Context) error {
	// Leave a 30-second buffer before the token expires.
	if c.accessToken != "" && time.Now().Add(30*time.Second).Before(c.tokenExpiry) {
		return nil
	}

	body, err := json.Marshal(loginRequest{
		ClientID:     c.clientID,
		ClientSecret: c.clientSecret,
	})
	if err != nil {
		return fmt.Errorf("marshal login request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.host+"/api/v1/auth/universal-auth/login", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute login request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, readBody(resp.Body))
	}

	var lr loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}

	c.accessToken = lr.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(lr.ExpiresIn) * time.Second)

	return nil
}

// ---- secrets API --------------------------------------------------------

type listSecretsResponse struct {
	Secrets []Secret `json:"secrets"`
}

// ListSecrets returns all secrets for the given project and configured environment.
// If projectID is empty, the client's default project ID is used.
func (c *Client) ListSecrets(ctx context.Context, projectID string) ([]Secret, error) {
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}

	if projectID == "" {
		projectID = c.projectID
	}

	url := fmt.Sprintf("%s/api/v3/secrets/raw?workspaceId=%s&environment=%s",
		c.host, projectID, c.environment)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create list-secrets request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute list-secrets request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list secrets failed with status %d: %s", resp.StatusCode, readBody(resp.Body))
	}

	var lr listSecretsResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, fmt.Errorf("decode list-secrets response: %w", err)
	}

	return lr.Secrets, nil
}

type getSecretResponse struct {
	Secret Secret `json:"secret"`
}

// GetSecret returns the value of a single secret identified by key.
// If projectID is empty, the client's default project ID is used.
func (c *Client) GetSecret(ctx context.Context, key, projectID string) (*Secret, error) {
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}

	if projectID == "" {
		projectID = c.projectID
	}

	url := fmt.Sprintf("%s/api/v3/secrets/raw/%s?workspaceId=%s&environment=%s",
		c.host, key, projectID, c.environment)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create get-secret request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute get-secret request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("secret %q not found", key)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get secret failed with status %d: %s", resp.StatusCode, readBody(resp.Body))
	}

	var gr getSecretResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, fmt.Errorf("decode get-secret response: %w", err)
	}

	return &gr.Secret, nil
}

type updateSecretRequest struct {
	WorkspaceID string `json:"workspaceId"`
	Environment string `json:"environment"`
	SecretValue string `json:"secretValue"`
}

// SetSecret updates (or creates) the value of a secret identified by key.
// If projectID is empty, the client's default project ID is used.
func (c *Client) SetSecret(ctx context.Context, key, value, projectID string) error {
	if err := c.ensureToken(ctx); err != nil {
		return err
	}

	if projectID == "" {
		projectID = c.projectID
	}

	body, err := json.Marshal(updateSecretRequest{
		WorkspaceID: projectID,
		Environment: c.environment,
		SecretValue: value,
	})
	if err != nil {
		return fmt.Errorf("marshal set-secret request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v3/secrets/raw/%s", c.host, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create set-secret request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute set-secret request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("set secret failed with status %d: %s", resp.StatusCode, readBody(resp.Body))
	}

	return nil
}

// ---- helpers ------------------------------------------------------------

// readBody reads up to 512 bytes from r and returns it as a string for
// inclusion in error messages.
func readBody(r io.Reader) string {
	buf := make([]byte, 512)
	n, _ := r.Read(buf)
	return string(buf[:n])
}
