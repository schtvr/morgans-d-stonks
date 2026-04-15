package ingest

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

// Client posts snapshots to the portfolio internal API.
type Client struct {
	BaseURL string
	APIKey  string
	hc      *http.Client
}

// NewClient constructs an HTTP client for the portfolio service.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		hc: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PostSnapshot sends a snapshot payload to POST /internal/snapshots.
func (c *Client) PostSnapshot(ctx context.Context, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/internal/snapshots", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", c.APIKey)
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ingest: portfolio api %s", resp.Status)
	}
	return nil
}

// PostSnapshotRetry posts once, then retries a single time on failure.
func (c *Client) PostSnapshotRetry(ctx context.Context, payload []byte) error {
	if err := c.PostSnapshot(ctx, payload); err == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}
	return c.PostSnapshot(ctx, payload)
}
