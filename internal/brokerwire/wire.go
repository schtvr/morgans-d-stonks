// Package brokerwire constructs broker.Broker implementations without import cycles.
package brokerwire

import (
	"fmt"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/broker/ibkr"
	"github.com/schtvr/morgans-d-stonks/internal/broker/mock"
)

// New returns a read broker based on cfg.Mode.
func New(cfg broker.Config) (broker.Broker, error) {
	switch cfg.Provider {
	case "":
		cfg.Provider = "ibkr"
	}
	switch cfg.Provider {
	case "ibkr":
		switch cfg.Mode {
		case "mock":
			return mock.New(), nil
		case "paper", "live":
			return ibkr.New(cfg)
		default:
			return nil, fmt.Errorf("brokerwire: unknown IBKR_MODE %q", cfg.Mode)
		}
	case "coinbase":
		return coinbase.NewReadOnly(nil, ""), nil
	default:
		return nil, fmt.Errorf("brokerwire: unknown BROKER_PROVIDER %q", cfg.Provider)
	}
}

// NewExecution returns an execution broker only when provider supports it.
func NewExecution(cfg broker.Config) (broker.ExecutionBroker, error) {
	switch cfg.Provider {
	case "coinbase":
		if cfg.Environment == "paper" || cfg.Environment == "" {
			return coinbase.NewPaperExecution(), nil
		}
		return nil, fmt.Errorf("brokerwire: coinbase live execution is not enabled")
	case "ibkr":
		return nil, fmt.Errorf("brokerwire: ibkr execution is not implemented")
	default:
		return nil, fmt.Errorf("brokerwire: unknown BROKER_PROVIDER %q", cfg.Provider)
	}
}
