package config

import "testing"

func TestTradingValidate(t *testing.T) {
	if err := (Trading{Enabled: false}).Validate("coinbase", "paper"); err != nil {
		t.Fatal(err)
	}
	if err := (Trading{Enabled: true, AllowedProviders: []string{"coinbase"}, AllowedSymbols: []string{"BTC-USD"}, MaxNotional: 100}).Validate("coinbase", "paper"); err != nil {
		t.Fatal(err)
	}
	if err := (Trading{Enabled: true, AllowedProviders: []string{"coinbase"}, AllowedSymbols: []string{"BTC-USD"}, MaxNotional: 100, KillSwitch: true}).Validate("coinbase", "paper"); err == nil {
		t.Fatal("expected kill switch validation error")
	}
	if err := (Trading{Enabled: true, AllowedProviders: []string{"coinbase"}, AllowedSymbols: []string{"BTC-USD"}, MaxNotional: 100, LiveAck: true}).Validate("coinbase", "live"); err != nil {
		t.Fatalf("expected live trading ok with ack: %v", err)
	}
	if err := (Trading{Enabled: true, AllowedProviders: []string{"coinbase"}, AllowedSymbols: []string{"BTC-USD"}, MaxNotional: 100}).Validate("coinbase", "live"); err == nil {
		t.Fatal("expected live ack required")
	}
	if err := (Trading{Enabled: true, AllowedProviders: []string{"coinbase"}, AllowedSymbols: []string{"BTC-USD"}, MaxNotional: 600, LiveAck: true, LiveMaxNotionalCap: 500}).Validate("coinbase", "live"); err == nil {
		t.Fatal("expected live notional cap error")
	}
}

func TestParseMinHoldings(t *testing.T) {
	got := parseMinHoldings("BTC-USD:0.0075,SOL-USD:4")
	if got["BTC-USD"] != 0.0075 {
		t.Fatalf("BTC-USD min: got %v", got["BTC-USD"])
	}
	if got["SOL-USD"] != 4 {
		t.Fatalf("SOL-USD min: got %v", got["SOL-USD"])
	}
	if len(parseMinHoldings("bad,ETH-USD:not-a-number")) != 0 {
		t.Fatal("expected empty map for invalid entries")
	}
}
