package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Client posts messages to a Discord webhook with basic rate limiting.
type Client struct {
	URL string
	hc  *http.Client

	mu     sync.Mutex
	lastAt time.Time
}

// NewClient constructs a webhook client.
func NewClient(url string) *Client {
	return &Client{
		URL: url,
		hc: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type webhookPayload struct {
	Content string `json:"content"`
}

// SendMessage sends a plain-text webhook message (Discord allows ~2000 chars).
func (c *Client) SendMessage(ctx context.Context, text string) error {
	if c.URL == "" {
		return fmt.Errorf("discord: empty webhook url")
	}
	if err := c.pace(ctx); err != nil {
		return err
	}

	body, err := json.Marshal(webhookPayload{Content: text})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord: webhook %s", resp.Status)
	}
	return nil
}

func (c *Client) pace(ctx context.Context) error {
	const minGap = 400 * time.Millisecond
	c.mu.Lock()
	wait := minGap - time.Since(c.lastAt)
	if wait < 0 {
		wait = 0
	}
	c.mu.Unlock()

	if wait > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	c.mu.Lock()
	c.lastAt = time.Now()
	c.mu.Unlock()
	return nil
}
