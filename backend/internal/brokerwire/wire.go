// Package brokerwire constructs broker.Broker implementations without import cycles.
package brokerwire

import (
	"fmt"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/broker/mock"
)

// New returns a read broker based on cfg.Provider.
func New(cfg broker.Config) (broker.Broker, error) {
	provider := cfg.Provider
	if provider == "" {
		provider = "coinbase"
	}
	switch provider {
	case "mock":
		return mock.New(), nil
	case "coinbase":
		return coinbase.NewReadOnly(nil, "", cfg.CoinbaseAPIKey, cfg.CoinbaseAPISecret), nil
	default:
		return nil, fmt.Errorf("brokerwire: unknown BROKER_PROVIDER %q", provider)
	}
}

// NewExecution returns an execution broker only when provider supports it.
func NewExecution(cfg broker.Config) (broker.ExecutionBroker, error) {
	provider := cfg.Provider
	if provider == "" {
		provider = "coinbase"
	}
	switch provider {
	case "coinbase":
		if cfg.Environment == "paper" || cfg.Environment == "" {
			return coinbase.NewPaperExecution(), nil
		}
		return nil, fmt.Errorf("brokerwire: coinbase live execution is not enabled")
	case "mock":
		return nil, fmt.Errorf("brokerwire: mock broker does not support execution")
	default:
		return nil, fmt.Errorf("brokerwire: unknown BROKER_PROVIDER %q", provider)
	}
}
