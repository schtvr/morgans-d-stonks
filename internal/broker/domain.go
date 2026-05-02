package broker

import (
	"fmt"
	"math/big"
	"strings"
)

// Instrument is a provider-agnostic tradable identifier.
type Instrument struct {
	Symbol string
	Venue  string
}

// Money is a fixed-precision decimal amount in a currency.
type Money struct {
	Amount   string
	Currency string
}

// Quantity is a fixed-precision decimal quantity.
type Quantity struct {
	Value string
}

// Fill is a provider-agnostic execution fill.
type Fill struct {
	OrderID   string
	Quantity  Quantity
	Price     Money
	Liquidity string
}

// NormalizeInstrument applies canonical symbol/venue formatting.
func NormalizeInstrument(symbol, venue string) Instrument {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	v := strings.ToUpper(strings.TrimSpace(venue))
	if v == "" {
		v = "SMART"
	}
	return Instrument{Symbol: s, Venue: v}
}

// NewMoney creates decimal-safe money.
func NewMoney(amount, currency string) (Money, error) {
	if _, ok := new(big.Rat).SetString(strings.TrimSpace(amount)); !ok {
		return Money{}, fmt.Errorf("invalid money amount %q", amount)
	}
	c := strings.ToUpper(strings.TrimSpace(currency))
	if c == "" {
		return Money{}, fmt.Errorf("currency is required")
	}
	return Money{Amount: strings.TrimSpace(amount), Currency: c}, nil
}

// NewQuantity creates decimal-safe quantity.
func NewQuantity(value string) (Quantity, error) {
	if _, ok := new(big.Rat).SetString(strings.TrimSpace(value)); !ok {
		return Quantity{}, fmt.Errorf("invalid quantity %q", value)
	}
	return Quantity{Value: strings.TrimSpace(value)}, nil
}
