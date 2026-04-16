package config

import "testing"

func TestValidatePortfolioAPI_requiresInternalKey(t *testing.T) {
	t.Parallel()
	err := ValidatePortfolioAPI(PortfolioAPI{InternalAPIKey: ""})
	if err == nil {
		t.Fatal("expected error")
	}
	err = ValidatePortfolioAPI(PortfolioAPI{InternalAPIKey: "   "})
	if err == nil {
		t.Fatal("expected error for whitespace-only key")
	}
	if ValidatePortfolioAPI(PortfolioAPI{InternalAPIKey: "x"}) != nil {
		t.Fatal("unexpected error")
	}
}
