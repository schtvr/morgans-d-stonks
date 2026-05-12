package ibkr

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

// Client implements broker.Broker using the Client Portal HTTPS API.
type Client struct {
	hc   *http.Client
	base string
}

// New builds a Client Portal-backed broker client.
func New(cfg broker.Config) (broker.Broker, error) {
	port := cfg.PortalPort
	if port == 0 {
		port = 5000
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // homelab: self-signed gateway certs
		},
	}
	hc := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}
	base := fmt.Sprintf("https://%s:%d/v1/api", cfg.GatewayHost, port)
	return &Client{
		hc:   hc,
		base: base,
	}, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("ibkr GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ibkr GET %s: status %s", path, resp.Status)
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("ibkr decode %s: %w", path, err)
	}
	return nil
}

func (c *Client) Positions(ctx context.Context) ([]broker.Position, error) {
	var accounts []struct {
		ID string `json:"id"`
	}
	if err := c.getJSON(ctx, "/portfolio/accounts", &accounts); err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("ibkr: no accounts")
	}
	acct := accounts[0].ID
	var rows []cpPosition
	if err := c.getJSON(ctx, "/portfolio/"+acct+"/positions/0", &rows); err != nil {
		return nil, err
	}
	return mapPositions(rows), nil
}

func (c *Client) AccountSummary(ctx context.Context) (*broker.AccountSummary, error) {
	var accounts []struct {
		ID string `json:"id"`
	}
	if err := c.getJSON(ctx, "/portfolio/accounts", &accounts); err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("ibkr: no accounts")
	}
	acct := accounts[0].ID
	// Summary payload shape varies by gateway version; parse flexibly.
	var raw []map[string]any
	if err := c.getJSON(ctx, "/portfolio/"+acct+"/summary", &raw); err != nil {
		return nil, err
	}
	s := summarizeFromRows(raw, acct)
	return s, nil
}

func summarizeFromRows(rows []map[string]any, accountID string) *broker.AccountSummary {
	out := &broker.AccountSummary{
		AccountID: accountID,
		Currency:  "USD",
		UpdatedAt: time.Now().UTC(),
	}
	for _, row := range rows {
		if v, ok := row["netliquidation"].(float64); ok {
			out.NetLiquidation = v
		}
		if v, ok := row["totalcashvalue"].(float64); ok {
			out.TotalCash = v
		}
		if v, ok := row["buyingpower"].(float64); ok {
			out.BuyingPower = v
		}
		if v, ok := row["currency"].(string); ok && v != "" {
			out.Currency = v
		}
	}
	return out
}

func (c *Client) Quotes(ctx context.Context, symbols []string) ([]broker.Quote, error) {
	now := time.Now().UTC()
	out := make([]broker.Quote, 0, len(symbols))
	for _, s := range symbols {
		out = append(out, broker.Quote{
			Symbol:    s,
			Last:      0,
			UpdatedAt: now,
		})
	}
	return out, nil
}

func (c *Client) IsMarketOpen(ctx context.Context) (bool, error) {
	var clock struct {
		Open bool `json:"isOpen"`
	}
	if err := c.getJSON(ctx, "/iserver/marketdata/clock", &clock); err != nil {
		return false, err
	}
	return clock.Open, nil
}

func (c *Client) Close() error { return nil }
