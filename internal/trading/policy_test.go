package trading

import (
	"testing"
	"time"
)

func TestPolicyEvaluate(t *testing.T) {
	p := Policy{
		MaxNotional:      100,
		Reserve:          10,
		AllowedSymbols:   []string{"BTC-USD"},
		DeniedSymbols:    []string{"DOGE-USD"},
		AllowedProviders: []string{"coinbase"},
	}
	dec := p.Evaluate(PolicyContext{Provider: "coinbase", AvailableCash: 120}, OrderRequest{
		Symbol:     "BTC-USD",
		Side:       OrderSideBuy,
		Quantity:   1,
		LimitPrice: 50,
	})
	if !dec.Allowed {
		t.Fatalf("expected allowed decision: %+v", dec)
	}
	dec = p.Evaluate(PolicyContext{Provider: "coinbase", AvailableCash: 20}, OrderRequest{
		Symbol:     "ETH-USD",
		Side:       OrderSideBuy,
		Quantity:   2,
		LimitPrice: 60,
	})
	if dec.Allowed || len(dec.ReasonCodes) == 0 {
		t.Fatalf("expected rejection: %+v", dec)
	}
	dec = p.Evaluate(PolicyContext{Provider: "coinbase", AvailableCash: 120}, OrderRequest{
		Symbol:     "DOGE-USD",
		Side:       OrderSideBuy,
		Quantity:   1,
		LimitPrice: 1,
	})
	if dec.Allowed || len(dec.ReasonCodes) == 0 {
		t.Fatalf("expected denylist rejection: %+v", dec)
	}
}

func TestValidateRequest(t *testing.T) {
	if err := Validate(OrderRequest{}); err == nil {
		t.Fatal("expected validation error")
	}
	if err := Validate(OrderRequest{Symbol: "BTC-USD", Side: OrderSideBuy, Quantity: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyEvaluate_CooldownExposureAndNoShorting(t *testing.T) {
	now := time.Now().UTC()
	p := Policy{SymbolCooldown: 10 * time.Minute, GlobalMaxExposure: 1000}
	open := []Order{{Symbol: "BTC-USD", Side: OrderSideBuy, Quantity: 1, Notional: 900, CreatedAt: now.Add(-5 * time.Minute)}}

	dec := p.Evaluate(PolicyContext{Provider: "coinbase", OpenOrders: open}, OrderRequest{Symbol: "BTC-USD", Side: OrderSideBuy, Quantity: 0.5, LimitPrice: 300})
	if dec.Allowed {
		t.Fatalf("expected rejection due to cooldown/exposure: %+v", dec)
	}

	dec = p.Evaluate(PolicyContext{Provider: "coinbase", OpenOrders: nil}, OrderRequest{Symbol: "BTC-USD", Side: OrderSideSell, Quantity: 1, LimitPrice: 100})
	if dec.Allowed {
		t.Fatalf("expected no_shorting rejection: %+v", dec)
	}
}
