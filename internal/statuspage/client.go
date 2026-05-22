// Package statuspage fetches the overall status from a Statuspage v2 API.
package statuspage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Status is the subset of the Statuspage response we care about.
type Status struct {
	Indicator   string // "none" | "minor" | "major" | "critical"
	Description string
}

// Client fetches Status from a Statuspage v2 endpoint.
type Client struct {
	url       string
	userAgent string
	http      *http.Client
}

// NewClient builds a Client pointed at url (e.g. https://status.claude.com/api/v2/status.json).
func NewClient(url, userAgent string, timeout time.Duration) *Client {
	return &Client{
		url:       url,
		userAgent: userAgent,
		http:      &http.Client{Timeout: timeout},
	}
}

type apiResponse struct {
	Status struct {
		Indicator   string `json:"indicator"`
		Description string `json:"description"`
	} `json:"status"`
}

// Fetch retrieves the current overall Status.
func (c *Client) Fetch(ctx context.Context) (Status, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return Status{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Status{}, fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Status{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var body apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Status{}, fmt.Errorf("decode: %w", err)
	}
	return Status{Indicator: body.Status.Indicator, Description: body.Status.Description}, nil
}
