package trades

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/schtvr/morgans-d-stonks/internal/trading"
)

const DefaultSchemaVersion = "v1"

// Request is the MCP-facing request envelope for trade validate/create operations.
type Request struct {
	SchemaVersion  string `json:"schema_version"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	Order          Order  `json:"order"`
}

// Order captures the MCP-facing order body.
type Order struct {
	ProductID string  `json:"product_id"`
	Side      string  `json:"side"`
	Type      string  `json:"type"`
	QuoteSize float64 `json:"quote_size,omitempty"`
	BaseSize  float64 `json:"base_size,omitempty"`
}

// DecodeAndMap decodes an MCP request and maps it into trading.OrderRequest.
func DecodeAndMap(r *http.Request, schemaVersion string) (trading.OrderRequest, error) {
	if strings.TrimSpace(schemaVersion) == "" {
		schemaVersion = DefaultSchemaVersion
	}
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return trading.OrderRequest{}, err
	}
	if req.SchemaVersion != schemaVersion {
		return trading.OrderRequest{}, fmt.Errorf("schema_version must be %s", schemaVersion)
	}
	if !strings.EqualFold(req.Order.Type, "market") {
		return trading.OrderRequest{}, fmt.Errorf("only market orders are supported")
	}
	if strings.TrimSpace(req.Order.ProductID) == "" {
		return trading.OrderRequest{}, fmt.Errorf("product_id is required")
	}
	if (req.Order.QuoteSize > 0 && req.Order.BaseSize > 0) || (req.Order.QuoteSize <= 0 && req.Order.BaseSize <= 0) {
		return trading.OrderRequest{}, fmt.Errorf("exactly one of quote_size or base_size is required")
	}
	quantity := req.Order.BaseSize
	if quantity <= 0 {
		quantity = req.Order.QuoteSize
	}
	out := trading.OrderRequest{
		Symbol:         strings.ToUpper(strings.TrimSpace(req.Order.ProductID)),
		Side:           trading.OrderSide(strings.ToLower(strings.TrimSpace(req.Order.Side))),
		Quantity:       quantity,
		IdempotencyKey: req.IdempotencyKey,
		Provider:       "coinbase",
		ProviderEnv:    brokerEnv(),
	}
	out.RequestHash = trading.HashRequest(out)
	return out, nil
}

// brokerEnv returns the current BROKER_ENV value, defaulting to "paper".
// This is used for metadata on OrderRequest.ProviderEnv only — actual execution
// environment is controlled by the trading-worker's BROKER_ENV at runtime.
func brokerEnv() string {
	if v := strings.TrimSpace(os.Getenv("BROKER_ENV")); v != "" {
		return strings.ToLower(v)
	}
	return "paper"
}
