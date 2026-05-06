package trading

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Policy configures deterministic pre-trade checks.
type Policy struct {
	MaxNotional      float64
	Reserve          float64
	KillSwitch       bool
	AllowedSymbols   []string
	DeniedSymbols    []string
	AllowedProviders []string
}

// PolicyContext supplies runtime values for reserve checks.
type PolicyContext struct {
	Provider      string
	AvailableCash float64
}

// Evaluate applies all configured rules and returns a deterministic decision.
func (p Policy) Evaluate(ctx PolicyContext, req OrderRequest) RiskDecision {
	notional := req.Quantity * req.LimitPrice
	if notional == 0 && req.Quantity > 0 {
		notional = req.Quantity
	}
	var reasonCodes []string
	if p.KillSwitch {
		reasonCodes = append(reasonCodes, "kill_switch")
	}
	if len(p.AllowedProviders) > 0 && !containsFold(p.AllowedProviders, ctx.Provider) {
		reasonCodes = append(reasonCodes, "provider_not_allowed")
	}
	if containsFold(p.DeniedSymbols, req.Symbol) {
		reasonCodes = append(reasonCodes, "symbol_denied")
	}
	if len(p.AllowedSymbols) > 0 && !containsFold(p.AllowedSymbols, req.Symbol) {
		reasonCodes = append(reasonCodes, "symbol_not_allowed")
	}
	if p.MaxNotional > 0 && notional > p.MaxNotional {
		reasonCodes = append(reasonCodes, "max_notional")
	}
	if p.Reserve > 0 && ctx.AvailableCash > 0 && ctx.AvailableCash-notional < p.Reserve {
		reasonCodes = append(reasonCodes, "reserve")
	}
	sort.Strings(reasonCodes)
	return RiskDecision{
		Allowed:     len(reasonCodes) == 0,
		ReasonCodes: reasonCodes,
		Notional:    notional,
		CheckedAt:   time.Now().UTC(),
	}
}

func containsFold(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), want) {
			return true
		}
	}
	return false
}

// Validate performs basic request validation.
func Validate(req OrderRequest) error {
	if strings.TrimSpace(req.Symbol) == "" {
		return fmt.Errorf("symbol is required")
	}
	if req.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	switch strings.ToLower(string(req.Side)) {
	case string(OrderSideBuy), string(OrderSideSell):
	default:
		return fmt.Errorf("invalid side %q", req.Side)
	}
	return nil
}
