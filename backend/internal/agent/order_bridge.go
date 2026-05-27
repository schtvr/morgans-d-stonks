package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// mcpTradeRequest is the wire shape expected by POST /mcp/v1/trades/create.
type mcpTradeRequest struct {
	SchemaVersion  string       `json:"schema_version"`
	IdempotencyKey string       `json:"idempotency_key"`
	Order          mcpTradeOrder `json:"order"`
}

type mcpTradeOrder struct {
	ProductID string  `json:"product_id"`
	Side      string  `json:"side"`
	Type      string  `json:"type"`
	QuoteSize float64 `json:"quote_size"`
}

// mcpTradeResponse is a partial decode of the /mcp/v1/trades/create response.
type mcpTradeResponse struct {
	Order    mcpTradeOrderStatus    `json:"order"`
	Decision mcpTradeDecisionStatus `json:"decision"`
}

type mcpTradeOrderStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type mcpTradeDecisionStatus struct {
	Allowed     bool     `json:"allowed"`
	ReasonCodes []string `json:"reasonCodes,omitempty"`
}

// maybeSubmitOrder posts a market order to portfolio-api when all execution gates pass.
// It is a best-effort side effect — errors are logged and never returned.
func (w *Worker) maybeSubmitOrder(ctx context.Context, req DecisionRequest, decision *Decision) {
	if !w.cfg.TradeEnabled {
		return
	}
	if decision.Action == ActionIgnore {
		return
	}
	// Only signal triggers carry a specific symbol to act on.
	if req.TriggerKind != TriggerSignal || req.Signal == nil || req.Signal.Symbol == "" {
		w.cfg.Log.Debug("agent_order_skip",
			"reason", "no_signal_symbol",
			"trigger_kind", req.TriggerKind,
			"idempotency_key", req.IdempotencyKey)
		return
	}

	minConf := w.cfg.MinTradeConfidence
	if minConf <= 0 {
		minConf = 0.70
	}
	if decision.Confidence < minConf {
		w.cfg.Log.Info("agent_order_skip",
			"reason", "low_confidence",
			"confidence", decision.Confidence,
			"min_confidence", minConf,
			"symbol", req.Signal.Symbol,
			"idempotency_key", req.IdempotencyKey)
		return
	}

	notional := w.cfg.DefaultTradeNotional
	if decision.SizingHintNotional != nil && *decision.SizingHintNotional > 0 {
		notional = *decision.SizingHintNotional
	}
	if notional <= 0 {
		w.cfg.Log.Warn("agent_order_skip",
			"reason", "no_notional",
			"symbol", req.Signal.Symbol,
			"idempotency_key", req.IdempotencyKey)
		return
	}

	tradeReq := mcpTradeRequest{
		SchemaVersion:  "v1",
		IdempotencyKey: "agent-" + req.IdempotencyKey,
		Order: mcpTradeOrder{
			ProductID: req.Signal.Symbol,
			Side:      string(decision.Action),
			Type:      "market",
			QuoteSize: notional,
		},
	}

	body, err := json.Marshal(tradeReq)
	if err != nil {
		w.cfg.Log.Error("agent_order: marshal trade request", "err", err, "idempotency_key", req.IdempotencyKey)
		return
	}

	url := strings.TrimRight(w.cfg.PortfolioAPIURL, "/") + "/mcp/v1/trades/create"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		w.cfg.Log.Error("agent_order: build request", "err", err, "idempotency_key", req.IdempotencyKey)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Key", w.cfg.InternalAPIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		w.cfg.Log.Error("agent_order: http error", "err", err, "idempotency_key", req.IdempotencyKey)
		return
	}
	defer resp.Body.Close()

	// 404 means TRADING_ENABLED=false on portfolio-api — misconfiguration.
	if resp.StatusCode == http.StatusNotFound {
		w.cfg.Log.Error("agent_order: 404 — TRADING_ENABLED is likely false on portfolio-api",
			"idempotency_key", req.IdempotencyKey)
		return
	}
	// 409 means idempotency key reuse — safe to treat as already-submitted.
	if resp.StatusCode == http.StatusConflict {
		w.cfg.Log.Info("agent_order: idempotency replay — order already submitted",
			"idempotency_key", req.IdempotencyKey)
		return
	}

	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		w.cfg.Log.Warn("agent_order: unexpected status",
			"status", resp.StatusCode,
			"body", strings.TrimSpace(string(rawBody)),
			"idempotency_key", req.IdempotencyKey)
		return
	}

	var tradeResp mcpTradeResponse
	if err := json.Unmarshal(rawBody, &tradeResp); err != nil {
		w.cfg.Log.Warn("agent_order: decode response", "err", err, "idempotency_key", req.IdempotencyKey)
		return
	}

	// HTTP 201 does NOT mean the order was sent to the broker — check decision.allowed.
	if !tradeResp.Decision.Allowed {
		w.cfg.Log.Warn("agent_order: rejected by policy",
			"order_id", tradeResp.Order.ID,
			"reason", tradeResp.Order.Reason,
			"reason_codes", fmt.Sprintf("%v", tradeResp.Decision.ReasonCodes),
			"symbol", req.Signal.Symbol,
			"idempotency_key", req.IdempotencyKey)
		return
	}

	w.cfg.Log.Info("agent_order: accepted",
		"order_id", tradeResp.Order.ID,
		"order_status", tradeResp.Order.Status,
		"symbol", req.Signal.Symbol,
		"action", decision.Action,
		"notional_usd", notional,
		"confidence", decision.Confidence,
		"idempotency_key", req.IdempotencyKey)
}
