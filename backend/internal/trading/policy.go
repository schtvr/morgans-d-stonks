package trading

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Policy configures deterministic pre-trade checks.
type Policy struct {
	MaxNotional       float64
	Reserve           float64
	KillSwitch        bool
	AllowedSymbols    []string
	DeniedSymbols     []string
	AllowedProviders  []string
	SymbolCooldown    time.Duration
	GlobalMaxExposure float64
	MinHoldings       map[string]float64
}

// PolicyContext supplies runtime values for reserve checks.
type PolicyContext struct {
	Provider      string
	AvailableCash    float64
	PositionQuantity float64
	OpenOrders       []Order
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
	if p.SymbolCooldown > 0 {
		for _, ord := range ctx.OpenOrders {
			if !strings.EqualFold(ord.Symbol, req.Symbol) {
				continue
			}
			if time.Since(ord.CreatedAt) < p.SymbolCooldown {
				reasonCodes = append(reasonCodes, "symbol_cooldown")
				break
			}
		}
	}
	if p.GlobalMaxExposure > 0 {
		var exposure float64
		for _, ord := range ctx.OpenOrders {
			if ord.Side == OrderSideBuy {
				exposure += ord.Notional
			}
		}
		if req.Side == OrderSideBuy {
			exposure += notional
		}
		if exposure > p.GlobalMaxExposure {
			reasonCodes = append(reasonCodes, "global_max_exposure")
		}
	}
	if req.Side == OrderSideSell {
		var pendingSellQty float64
		for _, ord := range ctx.OpenOrders {
			if !strings.EqualFold(ord.Symbol, req.Symbol) {
				continue
			}
			if ord.Side == OrderSideSell {
				pendingSellQty += ord.Quantity
			}
		}
		heldQty := ctx.PositionQuantity
		if heldQty > 0 && heldQty-pendingSellQty < req.Quantity {
			reasonCodes = append(reasonCodes, "no_shorting")
		} else if heldQty == 0 && pendingSellQty < req.Quantity {
			reasonCodes = append(reasonCodes, "no_shorting")
		}
		if minQty := p.minHolding(req.Symbol); minQty > 0 && ctx.PositionQuantity > 0 {
			remaining := ctx.PositionQuantity - pendingSellQty - req.Quantity
			if remaining+1e-12 < minQty {
				reasonCodes = append(reasonCodes, "min_holding")
			}
		}
	}
	sort.Strings(reasonCodes)
	return RiskDecision{
		Allowed:     len(reasonCodes) == 0,
		ReasonCodes: reasonCodes,
		Notional:    notional,
		CheckedAt:   time.Now().UTC(),
	}
}

func (p Policy) minHolding(symbol string) float64 {
	for sym, min := range p.MinHoldings {
		if strings.EqualFold(sym, symbol) {
			return min
		}
	}
	return 0
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
