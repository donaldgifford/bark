// Package apiclient provides a typed HTTP client for the bark API.
package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/donaldgifford/bark/pkg/types"
)

// Client is a typed HTTP client for the bark API.
// It injects a Bearer token returned by the provided getToken function
// into every request.
type Client struct {
	baseURL    string
	httpClient *http.Client
	getToken   func(ctx context.Context) (string, error)
}

// New creates a Client. getToken is called before each request to obtain a
// valid access token (the auth.Manager.GetToken method satisfies this).
func New(baseURL string, httpClient *http.Client, getToken func(context.Context) (string, error)) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		getToken:   getToken,
	}
}

// List returns all published packages.
func (c *Client) List(ctx context.Context) (*types.ListPackagesResponse, error) {
	var out types.ListPackagesResponse
	if err := c.get(ctx, "/v1/packages", nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// Search searches packages by query string.
func (c *Client) Search(ctx context.Context, query string) (*types.SearchResponse, error) {
	params := url.Values{"q": {query}}

	var out types.SearchResponse
	if err := c.get(ctx, "/v1/packages/search", params, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// ResolveLatest returns the latest approved version manifest for name.
func (c *Client) ResolveLatest(ctx context.Context, name string) (*types.ResolveResponse, error) {
	var out types.ResolveResponse
	if err := c.get(ctx, "/v1/packages/"+name, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// ResolveVersion returns the manifest for a specific version.
func (c *Client) ResolveVersion(ctx context.Context, name, version string) (*types.ResolveResponse, error) {
	var out types.ResolveResponse
	if err := c.get(ctx, "/v1/packages/"+name+"/"+version, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GetSigningKey returns the current active signing key.
func (c *Client) GetSigningKey(ctx context.Context) (*types.SigningKeyResponse, error) {
	var out types.SigningKeyResponse
	if err := c.get(ctx, "/v1/signing-keys/current", nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// =============================================================================
// HTTP helpers
// =============================================================================

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return fmt.Errorf("get auth token: %w", err)
	}

	rawURL := c.baseURL + path
	if len(params) > 0 {
		rawURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp types.ErrorResponse
		if decErr := json.NewDecoder(resp.Body).Decode(&errResp); decErr == nil && errResp.Error != "" {
			return fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}

		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, path)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response from %s: %w", path, err)
	}

	return nil
}
