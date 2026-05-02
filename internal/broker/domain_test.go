package broker

import "testing"

func TestNormalizeInstrument(t *testing.T) {
	got := NormalizeInstrument(" aapl ", " ")
	if got.Symbol != "AAPL" || got.Venue != "SMART" {
		t.Fatalf("unexpected normalization: %+v", got)
	}
}

func TestMoneyAndQuantityDecimalValidation(t *testing.T) {
	if _, err := NewMoney("100.1234", "usd"); err != nil {
		t.Fatalf("expected valid money: %v", err)
	}
	if _, err := NewQuantity("0.0005"); err != nil {
		t.Fatalf("expected valid quantity: %v", err)
	}
	if _, err := NewMoney("nope", "USD"); err == nil {
		t.Fatal("expected invalid money error")
	}
	if _, err := NewQuantity("nanx"); err == nil {
		t.Fatal("expected invalid quantity error")
	}
}
