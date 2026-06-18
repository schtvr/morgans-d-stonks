package agent

import (
	"encoding/json"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/signal"
)

// Action is the decision the agent takes on a trigger.
type Action string

const (
	ActionBuy    Action = "buy"
	ActionSell   Action = "sell"
	ActionIgnore Action = "ignore"
)

// TriggerKind distinguishes how the agent was activated.
type TriggerKind string

const (
	TriggerSignal TriggerKind = "signal"
	TriggerDaily  TriggerKind = "daily"
)

// DecisionRequest is the full input to the agent for one decision cycle.
type DecisionRequest struct {
	TriggerKind    TriggerKind         `json:"triggerKind"`
	TriggerAt      time.Time           `json:"triggerAt"`
	IdempotencyKey string              `json:"idempotencyKey"`
	Signal         *signal.CryptoAlert `json:"signal,omitempty"`
	EagerContext   EagerContext        `json:"eagerContext"`
	PromptVersion  string              `json:"promptVersion"`
}

// EagerContext is the pre-fetched context attached to every DecisionRequest.
type EagerContext struct {
	PortfolioSummary PortfolioSummaryLine `json:"portfolioSummary"`
	// DecisionsForSymbol24h is nil for daily triggers (no symbol); count of prior
	// decisions on this symbol in the last 24h for signal triggers.
	DecisionsForSymbol24h *int `json:"decisionsForSymbol24h,omitempty"`
	// MinCashUSD is the minimum USD cash that must remain after buys (TRADING_RESERVE).
	MinCashUSD *float64 `json:"minCashUsd,omitempty"`
	// MinHoldings are per-symbol base-asset quantity floors that must remain after sells.
	MinHoldings []HoldingFloor `json:"minHoldings,omitempty"`
}

// HoldingFloor is a minimum quantity floor for one symbol.
type HoldingFloor struct {
	Symbol string  `json:"symbol"`
	MinQty float64 `json:"minQty"`
}

// PortfolioSummaryLine is the compact portfolio snapshot embedded in EagerContext.
type PortfolioSummaryLine struct {
	NetLiquidation float64        `json:"netLiquidation"`
	TotalCash      float64        `json:"totalCash"`
	TopPositions   []PositionLine `json:"topPositions"`
}

// PositionLine is one holding in the compact portfolio summary.
type PositionLine struct {
	Symbol      string  `json:"symbol"`
	MarketValue float64 `json:"marketValue"`
	Quantity    float64 `json:"quantity"`
}

// Decision is the structured output from the agent for one trigger.
type Decision struct {
	Action             Action     `json:"action"`
	Confidence         float64    `json:"confidence"`
	Rationale          string     `json:"rationale"`
	SizingHintNotional *float64   `json:"sizingHintNotional,omitempty"`
	ToolCalls          []ToolCall `json:"toolCalls"`
	Model              string     `json:"model"`
	LatencyMS          int64      `json:"latencyMs"`
	CostCents          int64      `json:"costCents"`
}

// ToolCall records one MCP tool invocation made during the agent loop.
type ToolCall struct {
	Name       string          `json:"name"`
	Input      json.RawMessage `json:"input"`
	Output     json.RawMessage `json:"output"`
	DurationMS int64           `json:"durationMs"`
}
