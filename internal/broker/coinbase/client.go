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

	mu    sync.RWMutex
	cache map[string]ProductMetadata
}

type ProductMetadata struct {
	ProductID       string
	BaseIncrement   float64
	QuoteIncrement  float64
	TradingDisabled bool
}

func NewReadOnly(httpClient *http.Client, baseURL string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &Client{httpClient: httpClient, baseURL: strings.TrimRight(baseURL, "/"), cache: map[string]ProductMetadata{}}
}

func (c *Client) Capabilities() map[broker.Capability]bool {
	return map[broker.Capability]bool{broker.CapabilityReadPositions: true, broker.CapabilityReadSummary: true, broker.CapabilityQuote: true}
}
func (c *Client) Close() error                                   { return nil }
func (c *Client) IsMarketOpen(ctx context.Context) (bool, error) { return true, nil }

func (c *Client) Positions(ctx context.Context) ([]broker.Position, error) {
	var resp struct {
		Accounts []struct {
			Currency         string `json:"currency"`
			AvailableBalance string `json:"available_balance"`
		} `json:"accounts"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/v2/accounts", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]broker.Position, 0, len(resp.Accounts))
	now := time.Now().UTC()
	symbols := make([]string, 0, len(resp.Accounts))
	for _, a := range resp.Accounts {
		q, _ := strconv.ParseFloat(a.AvailableBalance, 64)
		if q == 0 {
			continue
		}
		symbol := strings.ToUpper(a.Currency) + "-USD"
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
		productID := normalizeProductID(s)
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

func normalizeProductID(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if strings.Contains(s, "-") {
		return s
	}
	return s + "-USD"
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
	if err := c.doJSON(ctx, http.MethodGet, "/api/v3/brokerage/products", nil, &resp); err != nil {
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

func (c *Client) doJSON(ctx context.Context, method, path string, body io.Reader, dst any) error {
	endpoint := c.baseURL + path
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
		if err != nil {
			return fmt.Errorf("coinbase: build request %s: %w", path, err)
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
