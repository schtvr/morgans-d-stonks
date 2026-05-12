package coinbase

import "strings"

// CanonicalToProviderSymbol maps internal canonical symbols to Coinbase product IDs.
// Accepted canonical forms include "BTC-USD", "BTC/USD", and "BTCUSD".
func CanonicalToProviderSymbol(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "/", "-")
	if strings.Contains(s, "-") {
		parts := strings.Split(s, "-")
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0] + "-" + parts[1]
		}
	}
	if len(s) > 3 {
		for _, quote := range []string{"USD", "USDC", "EUR", "GBP"} {
			if strings.HasSuffix(s, quote) && len(s) > len(quote) {
				base := strings.TrimSuffix(s, quote)
				if base != "" {
					return base + "-" + quote
				}
			}
		}
	}
	if strings.Contains(s, "-") {
		return s
	}
	return s + "-USD"
}

// ProviderToCanonicalSymbol maps Coinbase product IDs to canonical symbol format.
func ProviderToCanonicalSymbol(productID string) string {
	s := strings.ToUpper(strings.TrimSpace(productID))
	s = strings.ReplaceAll(s, "/", "-")
	if strings.Contains(s, "-") {
		return s
	}
	return s + "-USD"
}
