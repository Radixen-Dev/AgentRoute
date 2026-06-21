// SPDX-License-Identifier: GPL-3.0-only

// Package openrouter is a minimal client for the OpenRouter model catalog
// and API key validation. AgentRoute v1 targets OpenRouter exclusively as
// its upstream; see docs/concepts.md for the v2 multi-provider roadmap.
package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

const (
	// CatalogURL is OpenRouter's model listing endpoint.
	CatalogURL = "https://openrouter.ai/api/v1/models"

	defaultTimeout = 15 * time.Second
)

// Model is a single entry from the OpenRouter model catalog, trimmed to
// the fields AgentRoute's model picker actually uses.
type Model struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ContextLength int     `json:"context_length"`
	Pricing       Pricing `json:"pricing"`
}

// Pricing holds OpenRouter's per-token pricing strings, which arrive as
// decimal strings (e.g. "0.000003") rather than numbers.
type Pricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
}

type catalogResponse struct {
	Data []Model `json:"data"`
}

// Client talks to the OpenRouter API.
type Client struct {
	APIKey     string
	HTTPClient *http.Client

	// BaseURL overrides CatalogURL; used by tests to point at an
	// httptest.Server. Leave empty in production.
	BaseURL string
}

// NewClient returns a Client configured with a sane default timeout.
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: defaultTimeout},
	}
}

func (c *Client) catalogURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return CatalogURL
}

// ErrNoAPIKey is returned by FetchModels and Validate when no API key has
// been configured.
var ErrNoAPIKey = fmt.Errorf("openrouter: no API key configured")

// FetchModels retrieves the full OpenRouter model catalog, sorted by ID.
func (c *Client) FetchModels(ctx context.Context) ([]Model, error) {
	if c.APIKey == "" {
		return nil, ErrNoAPIKey
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.catalogURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("openrouter: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openrouter: read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("openrouter: invalid API key (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter: unexpected status %d: %s", resp.StatusCode, truncate(body, 300))
	}

	var parsed catalogResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("openrouter: parse response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("openrouter: catalog returned zero models")
	}

	sort.Slice(parsed.Data, func(i, j int) bool { return parsed.Data[i].ID < parsed.Data[j].ID })
	return parsed.Data, nil
}

// Validate checks that the configured API key is accepted by OpenRouter
// by attempting a catalog fetch. It returns a nil error iff the key works.
func (c *Client) Validate(ctx context.Context) error {
	_, err := c.FetchModels(ctx)
	return err
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
