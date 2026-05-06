package signal

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAlertStateEvaluateTracksBaselineAndThreshold(t *testing.T) {
	state, err := NewAlertState(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()

	decision, err := state.Evaluate("BTC-USD", 100, 1.0, time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Alert {
		t.Fatal("first observation should not alert")
	}

	decision, err = state.Evaluate("BTC-USD", 100.5, 1.0, time.Minute, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Alert {
		t.Fatal("move below threshold should not alert")
	}
	if decision.PreviousPrice != 100 {
		t.Fatalf("previous price: %v", decision.PreviousPrice)
	}

	decision, err = state.Evaluate("BTC-USD", 102, 1.0, time.Minute, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Alert {
		t.Fatal("move above threshold should alert")
	}
	if decision.DeltaPct <= 1 {
		t.Fatalf("unexpected delta pct: %v", decision.DeltaPct)
	}
}

func TestAlertStateEvaluateCooldownSuppressesRepeat(t *testing.T) {
	state, err := NewAlertState(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()

	_, err = state.Evaluate("BTC-USD", 100, 1.0, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := state.Evaluate("BTC-USD", 102, 1.0, time.Hour, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Alert {
		t.Fatal("expected first alert")
	}
	decision, err = state.Evaluate("BTC-USD", 104, 1.0, time.Hour, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Alert {
		t.Fatal("cooldown should suppress repeat alert")
	}
}
