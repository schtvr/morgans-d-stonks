package coinbase

import "testing"

func TestCanonicalToProviderSymbol(t *testing.T) {
	tests := map[string]string{
		"BTC-USD": "BTC-USD",
		"btc-usd": "BTC-USD",
		"BTC/USD": "BTC-USD",
		"BTCUSD":  "BTC-USD",
		"ethusdc": "ETH-USDC",
		"AAPL":    "AAPL-USD",
	}
	for in, want := range tests {
		if got := CanonicalToProviderSymbol(in); got != want {
			t.Fatalf("CanonicalToProviderSymbol(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProviderToCanonicalSymbol(t *testing.T) {
	tests := map[string]string{
		"BTC-USD": "BTC-USD",
		"btc/usd": "BTC-USD",
	}
	for in, want := range tests {
		if got := ProviderToCanonicalSymbol(in); got != want {
			t.Fatalf("ProviderToCanonicalSymbol(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSymbolRoundTripConsistency(t *testing.T) {
	inputs := []string{"BTC-USD", "BTC/USD", "btcusd", "ETHUSDC", "SOL-USD"}
	for _, in := range inputs {
		provider := CanonicalToProviderSymbol(in)
		canon := ProviderToCanonicalSymbol(provider)
		if canon != provider {
			t.Fatalf("round-trip mismatch for %q: provider=%q canon=%q", in, provider, canon)
		}
	}
}
