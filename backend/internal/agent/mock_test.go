package agent

import (
	"context"
	"testing"

	"github.com/schtvr/morgans-d-stonks/internal/signal"
)

func makeSignalReq(deltaPct, thresholdPct float64) DecisionRequest {
	alert := &signal.CryptoAlert{
		DeltaPct:     deltaPct,
		ThresholdPct: thresholdPct,
		Symbol:       "BTC-USD",
	}
	return DecisionRequest{
		TriggerKind:    TriggerSignal,
		IdempotencyKey: "test-key",
		Signal:         alert,
	}
}

func TestMockProvider_Buy(t *testing.T) {
	t.Parallel()
	p := NewMockProvider()
	// DeltaPct = ThresholdPct * 3 → buy
	req := makeSignalReq(3.0, 1.0)
	d, err := p.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != ActionBuy {
		t.Errorf("expected buy, got %s", d.Action)
	}
	if d.Confidence != 0.7 {
		t.Errorf("expected confidence 0.7, got %v", d.Confidence)
	}
	if d.Rationale != "mock: strong move" {
		t.Errorf("unexpected rationale: %s", d.Rationale)
	}
}

func TestMockProvider_Sell(t *testing.T) {
	t.Parallel()
	p := NewMockProvider()
	// DeltaPct = -ThresholdPct * 3 → sell
	req := makeSignalReq(-3.0, 1.0)
	d, err := p.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != ActionSell {
		t.Errorf("expected sell, got %s", d.Action)
	}
	if d.Confidence != 0.7 {
		t.Errorf("expected confidence 0.7, got %v", d.Confidence)
	}
}

func TestMockProvider_Ignore_WeakSignal(t *testing.T) {
	t.Parallel()
	p := NewMockProvider()
	// DeltaPct = ThresholdPct * 0.5 → insufficient
	req := makeSignalReq(0.5, 1.0)
	d, err := p.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != ActionIgnore {
		t.Errorf("expected ignore, got %s", d.Action)
	}
	if d.Confidence != 0.3 {
		t.Errorf("expected confidence 0.3, got %v", d.Confidence)
	}
}

func TestMockProvider_Ignore_NilSignal(t *testing.T) {
	t.Parallel()
	p := NewMockProvider()
	req := DecisionRequest{
		TriggerKind:    TriggerDaily,
		IdempotencyKey: "daily-2026-05-18",
	}
	d, err := p.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != ActionIgnore {
		t.Errorf("expected ignore for nil signal, got %s", d.Action)
	}
}

func TestMockProvider_AtBoundary(t *testing.T) {
	t.Parallel()
	p := NewMockProvider()
	// Exactly 2× threshold → buy
	req := makeSignalReq(2.0, 1.0)
	d, err := p.Decide(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionBuy {
		t.Errorf("at 2× threshold expected buy, got %s", d.Action)
	}
	// Exactly -2× threshold → sell
	req2 := makeSignalReq(-2.0, 1.0)
	d2, err := p.Decide(context.Background(), req2)
	if err != nil {
		t.Fatal(err)
	}
	if d2.Action != ActionSell {
		t.Errorf("at -2× threshold expected sell, got %s", d2.Action)
	}
}
