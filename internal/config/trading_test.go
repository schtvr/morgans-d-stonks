package config

import "testing"

func TestTradingValidate(t *testing.T) {
	if err := (Trading{Enabled: false}).Validate("coinbase"); err != nil {
		t.Fatal(err)
	}
	if err := (Trading{Enabled: true, AllowedProviders: []string{"coinbase"}, AllowedSymbols: []string{"BTC-USD"}, MaxNotional: 100}).Validate("coinbase"); err != nil {
		t.Fatal(err)
	}
	if err := (Trading{Enabled: true, AllowedProviders: []string{"coinbase"}, AllowedSymbols: []string{"BTC-USD"}, MaxNotional: 100, KillSwitch: true}).Validate("coinbase"); err == nil {
		t.Fatal("expected kill switch validation error")
	}
}
