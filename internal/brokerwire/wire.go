// Package brokerwire constructs broker.Broker implementations without import cycles.
package brokerwire

import (
	"fmt"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/broker/ibkr"
	"github.com/schtvr/morgans-d-stonks/internal/broker/mock"
)

// New returns a Broker based on cfg.Mode.
func New(cfg broker.Config) (broker.Broker, error) {
	switch cfg.Mode {
	case "mock":
		return mock.New(), nil
	case "paper", "live":
		return ibkr.New(cfg)
	default:
		return nil, fmt.Errorf("brokerwire: unknown IBKR_MODE %q", cfg.Mode)
	}
}
