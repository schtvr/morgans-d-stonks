package config

import (
	"fmt"
	"strings"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

// Broker config supports provider selection and permission-separated credentials.
type Broker struct {
	Provider string
	Env      string

	CoinbaseReadAPIKey     string
	CoinbaseReadAPISecret  string
	CoinbaseTradeAPIKey    string
	CoinbaseTradeAPISecret string
}

func LoadBroker() Broker {
	return Broker{
		Provider:               getenv("BROKER_PROVIDER", "coinbase"),
		Env:                    getenv("BROKER_ENV", "paper"),
		CoinbaseReadAPIKey:     getenv("COINBASE_READ_API_KEY", ""),
		CoinbaseReadAPISecret:  getenv("COINBASE_READ_API_SECRET", ""),
		CoinbaseTradeAPIKey:    getenv("COINBASE_TRADE_API_KEY", ""),
		CoinbaseTradeAPISecret: getenv("COINBASE_TRADE_API_SECRET", ""),
	}
}

func (c Broker) Validate() error {
	p := strings.ToLower(strings.TrimSpace(c.Provider))
	switch p {
	case "mock":
		return nil
	case "coinbase":
		if strings.TrimSpace(c.CoinbaseReadAPIKey) == "" || strings.TrimSpace(c.CoinbaseReadAPISecret) == "" {
			return fmt.Errorf("COINBASE_READ_API_KEY and COINBASE_READ_API_SECRET are required for provider=coinbase")
		}
	default:
		return fmt.Errorf("unknown BROKER_PROVIDER %q", c.Provider)
	}
	return nil
}

func (c Broker) ToLegacyBrokerConfig() broker.Config {
	return broker.Config{
		Provider:          strings.ToLower(c.Provider),
		Environment:       strings.ToLower(c.Env),
		CoinbaseAPIKey:    strings.TrimSpace(c.CoinbaseReadAPIKey),
		CoinbaseAPISecret: strings.TrimSpace(c.CoinbaseReadAPISecret),
	}
}
