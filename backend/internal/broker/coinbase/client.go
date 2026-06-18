package coinbase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

const defaultBaseURL = "https://api.coinbase.com"

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKeyID   string
	apiSecret  string

	mu    sync.RWMutex
	cache map[string]ProductMetadata
}

type ProductMetadata struct {
	ProductID       string
	BaseIncrement   float64
	QuoteIncrement  float64
	TradingDisabled bool
}

// NewReadOnly returns a Coinbase read client. apiKeyID and apiKeySecret are CDP credentials.
// Supported secrets: ECDSA private key as PEM (ES256), or base64-encoded 64-byte Ed25519 key (EdDSA).
// Coinbase documents ES256 as the supported choice for many App / Advanced Trade REST calls; Ed25519 may be rejected—prefer ECDSA if auth fails.
func NewReadOnly(httpClient *http.Client, baseURL, apiKeyID, apiKeySecret string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKeyID:   strings.TrimSpace(apiKeyID),
		apiSecret:  strings.TrimSpace(apiKeySecret),
		cache:      map[string]ProductMetadata{},
	}
}

func apiHostForJWT(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return "api.coinbase.com"
	}
	return u.Host
}

func apiPathForJWT(path string) string {
	if parsed, err := url.Parse(path); err == nil && parsed.Path != "" {
		return parsed.Path
	}
	if p, _, ok := strings.Cut(path, "?"); ok {
		return p
	}
	return path
}

func (c *Client) Capabilities() map[broker.Capability]bool {
	return map[broker.Capability]bool{broker.CapabilityReadPositions: true, broker.CapabilityReadSummary: true, broker.CapabilityQuote: true}
}
func (c *Client) Close() error                                   { return nil }
func (c *Client) IsMarketOpen(ctx context.Context) (bool, error) { return true, nil }

func (c *Client) Positions(ctx context.Context) ([]broker.Position, error) {
	rows, err := c.fetchAllAccountRows(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]broker.Position, 0, len(rows))
	now := time.Now().UTC()
	symbols := make([]string, 0, len(rows))
	for _, row := range rows {
		code := row.currencyCode
		q := row.balanceAmount
		if code == "" || q == 0 {
			continue
		}
		symbol := ProviderToCanonicalSymbol(strings.ToUpper(code) + "-USD")
		out = append(out, broker.Position{Symbol: symbol, Quantity: q, Currency: "USD", UpdatedAt: now})
		symbols = append(symbols, symbol)
	}
	if len(out) == 0 {
		return out, nil
	}

	quotes, err := c.Quotes(ctx, symbols)
	if err != nil {
		return nil, err
	}
	prices := make(map[string]float64, len(quotes))
	for _, q := range quotes {
		prices[strings.ToUpper(q.Symbol)] = q.Last
	}
	for i := range out {
		if out[i].Symbol == "USD-USD" {
			out[i].MarketValue = out[i].Quantity
			continue
		}
		if last, ok := prices[strings.ToUpper(out[i].Symbol)]; ok {
			out[i].MarketValue = out[i].Quantity * last
		}
	}
	return out, nil
}

type accountRow struct {
	currencyCode  string
	balanceAmount float64
}

type v3ListAccount struct {
	Currency         string          `json:"currency"`
	AvailableBalance json.RawMessage `json:"available_balance"`
	Hold             json.RawMessage `json:"hold"`
	Type             string          `json:"type"`
}

func v3AvailableBalanceQuantity(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var obj struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(raw, &obj) == nil && strings.TrimSpace(obj.Value) != "" {
		f, _ := strconv.ParseFloat(obj.Value, 64)
		return f
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	return 0
}

func (c *Client) fetchAllAccountRows(ctx context.Context) ([]accountRow, error) {
	cursor := ""
	var out []accountRow
	for i := 0; i < 50; i++ {
		path := "/api/v3/brokerage/accounts?limit=250"
		if cursor != "" {
			path = fmt.Sprintf("/api/v3/brokerage/accounts?limit=250&cursor=%s", url.QueryEscape(cursor))
		}
		var page struct {
			Accounts []v3ListAccount `json:"accounts"`
			HasNext  bool            `json:"has_next"`
			Cursor   string          `json:"cursor"`
		}
		if err := c.doJSON(ctx, http.MethodGet, path, nil, &page); err != nil {
			return nil, err
		}
		for _, a := range page.Accounts {
			code := strings.TrimSpace(strings.ToUpper(a.Currency))
			amt := v3AvailableBalanceQuantity(a.AvailableBalance) + v3AvailableBalanceQuantity(a.Hold)
			out = append(out, accountRow{currencyCode: code, balanceAmount: amt})
		}
		if !page.HasNext {
			break
		}
		if page.Cursor == "" {
			break
		}
		cursor = page.Cursor
	}
	return out, nil
}

func (c *Client) AccountSummary(ctx context.Context) (*broker.AccountSummary, error) {
	positions, err := c.Positions(ctx)
	if err != nil {
		return nil, err
	}
	net := 0.0
	for _, p := range positions {
		net += p.MarketValue
	}
	return &broker.AccountSummary{AccountID: "coinbase", NetLiquidation: net, TotalCash: net, BuyingPower: net, Currency: "USD", UpdatedAt: time.Now().UTC()}, nil
}

func (c *Client) Quotes(ctx context.Context, symbols []string) ([]broker.Quote, error) {
	quotes := make([]broker.Quote, 0, len(symbols))
	for _, s := range symbols {
		productID := CanonicalToProviderSymbol(s)
		if err := c.ensureProductCached(ctx, productID); err != nil {
			return nil, err
		}
		var body struct {
			Data struct {
				Amount string `json:"amount"`
			} `json:"data"`
		}
		if err := c.doJSON(ctx, http.MethodGet, "/v2/prices/"+url.PathEscape(productID)+"/spot", nil, &body); err != nil {
			return nil, err
		}
		last, _ := strconv.ParseFloat(body.Data.Amount, 64)
		quotes = append(quotes, broker.Quote{Symbol: productID, Last: last, Bid: last, Ask: last, UpdatedAt: time.Now().UTC()})
	}
	return quotes, nil
}

func (c *Client) ensureProductCached(ctx context.Context, productID string) error {
	c.mu.RLock()
	_, ok := c.cache[productID]
	c.mu.RUnlock()
	if ok {
		return nil
	}
	var resp struct {
		Products []struct {
			ProductID       string `json:"product_id"`
			BaseIncrement   string `json:"base_increment"`
			QuoteIncrement  string `json:"quote_increment"`
			TradingDisabled bool   `json:"trading_disabled"`
		} `json:"products"`
	}
	// Public market endpoint (unauthenticated). /api/v3/brokerage/products requires JWT and returns 401 without auth.
	if err := c.doJSON(ctx, http.MethodGet, "/api/v3/brokerage/market/products", nil, &resp); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range resp.Products {
		bi := parseStep(p.BaseIncrement)
		qi := parseStep(p.QuoteIncrement)
		c.cache[strings.ToUpper(p.ProductID)] = ProductMetadata{ProductID: strings.ToUpper(p.ProductID), BaseIncrement: bi, QuoteIncrement: qi, TradingDisabled: p.TradingDisabled}
	}
	return nil
}

func parseStep(v string) float64 {
	f, _ := strconv.ParseFloat(v, 64)
	if f <= 0 {
		return 0
	}
	return math.Abs(f)
}

func (c *Client) bearerForRequest(method, path string) (string, error) {
	if c.apiKeyID == "" || c.apiSecret == "" {
		return "", nil
	}
	token, err := signCoinbaseAppRESTJWT(c.apiKeyID, c.apiSecret, method, apiHostForJWT(c.baseURL), apiPathForJWT(path))
	if err != nil {
		return "", fmt.Errorf("coinbase: jwt for %s %s: %w", method, path, err)
	}
	return "Bearer " + token, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body io.Reader, dst any) error {
	endpoint := c.baseURL + path
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
		if err != nil {
			return fmt.Errorf("coinbase: build request %s: %w", path, err)
		}
		authz, err := c.bearerForRequest(method, path)
		if err != nil {
			return err
		}
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("coinbase: %s status %d", path, resp.StatusCode)
				return
			}
			if resp.StatusCode >= 400 {
				b, _ := io.ReadAll(resp.Body)
				lastErr = fmt.Errorf("coinbase: %s status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(b)))
				return
			}
			if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
				lastErr = fmt.Errorf("coinbase: decode %s: %w", path, err)
				return
			}
			lastErr = nil
		}()
		if lastErr == nil {
			return nil
		}
		if resp != nil && !(resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
		}
	}
	return lastErr
}
