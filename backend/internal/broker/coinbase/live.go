package coinbase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

// LiveBroker executes real orders via the Coinbase Advanced Trade REST API.
type LiveBroker struct {
	httpClient *http.Client
	baseURL    string
	apiKeyID   string
	apiSecret  string
}

// NewLiveExecution returns a live execution broker using the given Coinbase trade credentials.
func NewLiveExecution(httpClient *http.Client, baseURL, apiKeyID, apiSecret string) broker.ExecutionBroker {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &LiveBroker{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKeyID:   strings.TrimSpace(apiKeyID),
		apiSecret:  strings.TrimSpace(apiSecret),
	}
}

type placeOrderRequest struct {
	ClientOrderID      string             `json:"client_order_id"`
	ProductID          string             `json:"product_id"`
	Side               string             `json:"side"`
	OrderConfiguration orderConfiguration `json:"order_configuration"`
}

type orderConfiguration struct {
	MarketMarketIOC *marketMarketIOC `json:"market_market_ioc,omitempty"`
}

type marketMarketIOC struct {
	QuoteSize string `json:"quote_size,omitempty"`
	BaseSize  string `json:"base_size,omitempty"`
}

type placeOrderResponse struct {
	Success         bool               `json:"success"`
	OrderID         string             `json:"order_id"`
	SuccessResponse *placeOrderSuccess `json:"success_response,omitempty"`
	ErrorResponse   *placeOrderError   `json:"error_response,omitempty"`
}

type placeOrderSuccess struct {
	OrderID       string `json:"order_id"`
	ProductID     string `json:"product_id"`
	Side          string `json:"side"`
	ClientOrderID string `json:"client_order_id"`
}

type placeOrderError struct {
	Error                string `json:"error"`
	Message              string `json:"message"`
	PreviewFailureReason string `json:"preview_failure_reason"`
}

// PlaceOrder submits a market order to Coinbase Advanced Trade.
// Quantity is always treated as USD notional: BUY uses quote_size directly;
// SELL fetches the spot price and converts to base_size (Coinbase market sells require base_size).
func (b *LiveBroker) PlaceOrder(ctx context.Context, intent broker.OrderIntent) (*broker.Order, error) {
	productID := CanonicalToProviderSymbol(intent.Symbol)
	side := strings.ToUpper(strings.TrimSpace(intent.Side))

	cfg, err := b.buildOrderConfiguration(ctx, side, productID, intent.Quantity)
	if err != nil {
		return nil, fmt.Errorf("coinbase live: build order config for %s %s: %w", side, productID, err)
	}

	body, err := json.Marshal(placeOrderRequest{
		ClientOrderID:      uuid.NewString(),
		ProductID:          productID,
		Side:               side,
		OrderConfiguration: cfg,
	})
	if err != nil {
		return nil, fmt.Errorf("coinbase live: marshal order: %w", err)
	}

	const path = "/api/v3/brokerage/orders"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("coinbase live: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if authz, err := b.bearerFor(http.MethodPost, path); err != nil {
		return nil, err
	} else if authz != "" {
		req.Header.Set("Authorization", authz)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("coinbase live: place order HTTP: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("coinbase live: place order status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result placeOrderResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("coinbase live: decode place order response: %w", err)
	}
	if !result.Success {
		reason := "unknown"
		if result.ErrorResponse != nil {
			reason = result.ErrorResponse.Error
			if result.ErrorResponse.Message != "" {
				reason += ": " + result.ErrorResponse.Message
			}
		}
		return nil, fmt.Errorf("coinbase live: order rejected: %s", reason)
	}

	orderID := result.OrderID
	if orderID == "" && result.SuccessResponse != nil {
		orderID = result.SuccessResponse.OrderID
	}

	return &broker.Order{
		ID:        orderID,
		Symbol:    productID,
		Status:    "accepted",
		CreatedAt: time.Now().UTC(),
	}, nil
}

// buildOrderConfiguration produces the Coinbase order_configuration block.
// BUY: spend `quantity` USD (quote_size).
// SELL: convert `quantity` USD to base units via the current spot price (base_size).
func (b *LiveBroker) buildOrderConfiguration(ctx context.Context, side, productID string, quantity float64) (orderConfiguration, error) {
	switch side {
	case "BUY":
		return orderConfiguration{MarketMarketIOC: &marketMarketIOC{QuoteSize: formatSize(quantity)}}, nil
	case "SELL":
		price, err := b.spotPrice(ctx, productID)
		if err != nil {
			return orderConfiguration{}, fmt.Errorf("spot price for %s: %w", productID, err)
		}
		if price <= 0 {
			return orderConfiguration{}, fmt.Errorf("invalid spot price %v for %s", price, productID)
		}
		return orderConfiguration{MarketMarketIOC: &marketMarketIOC{BaseSize: formatSize(quantity / price)}}, nil
	default:
		return orderConfiguration{}, fmt.Errorf("unsupported order side %q", side)
	}
}

// spotPrice fetches the current spot price for a Coinbase product ID.
func (b *LiveBroker) spotPrice(ctx context.Context, productID string) (float64, error) {
	path := "/v2/prices/" + url.PathEscape(productID) + "/spot"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+path, nil)
	if err != nil {
		return 0, err
	}
	if authz, err := b.bearerFor(http.MethodGet, path); err != nil {
		return 0, err
	} else if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("spot price status %d", resp.StatusCode)
	}
	var body struct {
		Data struct {
			Amount string `json:"amount"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	return strconv.ParseFloat(body.Data.Amount, 64)
}

type batchCancelRequest struct {
	OrderIDs []string `json:"order_ids"`
}

type batchCancelResponse struct {
	Results []cancelResult `json:"results"`
}

type cancelResult struct {
	Success       bool   `json:"success"`
	OrderID       string `json:"order_id"`
	FailureReason string `json:"failure_reason,omitempty"`
}

// CancelOrder cancels a live Coinbase order via the batch cancel endpoint.
func (b *LiveBroker) CancelOrder(ctx context.Context, orderID string) error {
	body, err := json.Marshal(batchCancelRequest{OrderIDs: []string{orderID}})
	if err != nil {
		return fmt.Errorf("coinbase live: marshal cancel: %w", err)
	}

	const path = "/api/v3/brokerage/orders/batch_cancel"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("coinbase live: build cancel request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if authz, err := b.bearerFor(http.MethodPost, path); err != nil {
		return err
	} else if authz != "" {
		req.Header.Set("Authorization", authz)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("coinbase live: cancel HTTP: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("coinbase live: cancel status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result batchCancelResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("coinbase live: decode cancel response: %w", err)
	}
	for _, r := range result.Results {
		if r.OrderID == orderID && !r.Success {
			return fmt.Errorf("coinbase live: cancel failed: %s", r.FailureReason)
		}
	}
	return nil
}

func (b *LiveBroker) bearerFor(method, path string) (string, error) {
	if b.apiKeyID == "" || b.apiSecret == "" {
		return "", fmt.Errorf("coinbase live: COINBASE_TRADE_API_KEY or COINBASE_TRADE_API_SECRET is not set")
	}
	token, err := signCoinbaseAppRESTJWT(b.apiKeyID, b.apiSecret, method, apiHostForJWT(b.baseURL), apiPathForJWT(path))
	if err != nil {
		return "", fmt.Errorf("coinbase live: jwt sign: %w", err)
	}
	return "Bearer " + token, nil
}

// formatSize formats a float64 as a plain decimal string with up to 8 places, trimming trailing zeros.
func formatSize(v float64) string {
	s := strconv.FormatFloat(v, 'f', 8, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

type getOrderResponse struct {
	Order struct {
		OrderID            string `json:"order_id"`
		ProductID          string `json:"product_id"`
		Status             string `json:"status"`
		CreatedTime        string `json:"created_time"`
		AverageFilledPrice string `json:"average_filled_price"`
		FilledSize         string `json:"filled_size"`
	} `json:"order"`
}

// GetOrder fetches the current status of a live order from Coinbase.
// Implements broker.OrderPoller so the trading worker can poll for fills.
func (b *LiveBroker) GetOrder(ctx context.Context, providerOrderID string) (*broker.Order, error) {
	path := "/api/v3/brokerage/orders/historical/" + url.PathEscape(providerOrderID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("coinbase live: build get order request: %w", err)
	}
	if authz, err := b.bearerFor(http.MethodGet, path); err != nil {
		return nil, err
	} else if authz != "" {
		req.Header.Set("Authorization", authz)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("coinbase live: get order HTTP: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("coinbase live: get order status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result getOrderResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("coinbase live: decode get order: %w", err)
	}

	createdAt, _ := time.Parse(time.RFC3339, result.Order.CreatedTime)
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	filledPrice, _ := strconv.ParseFloat(result.Order.AverageFilledPrice, 64)
	filledSize, _ := strconv.ParseFloat(result.Order.FilledSize, 64)

	return &broker.Order{
		ID:          result.Order.OrderID,
		Symbol:      result.Order.ProductID,
		Status:      mapCoinbaseStatus(result.Order.Status),
		CreatedAt:   createdAt,
		FilledPrice: filledPrice,
		FilledSize:  filledSize,
	}, nil
}

// mapCoinbaseStatus normalises Coinbase uppercase status strings to our lowercase conventions.
func mapCoinbaseStatus(s string) string {
	switch strings.ToUpper(s) {
	case "FILLED":
		return "filled"
	case "CANCELLED":
		return "canceled"
	case "EXPIRED", "FAILED":
		return "rejected"
	default: // OPEN, PENDING, QUEUED, unknown
		return "accepted"
	}
}

var _ broker.ExecutionBroker = (*LiveBroker)(nil)
var _ broker.OrderPoller = (*LiveBroker)(nil)
