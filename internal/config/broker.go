package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

// Broker config supports provider selection and permission-separated credentials.
type Broker struct {
	Provider string
	Env      string

	IBKRMode        string
	IBKRGatewayHost string
	IBKRGatewayPort int
	IBKRPortalPort  int

	CoinbaseReadAPIKey     string
	CoinbaseReadAPISecret  string
	CoinbaseTradeAPIKey    string
	CoinbaseTradeAPISecret string
}

func LoadBroker() Broker {
	return Broker{
		Provider:               getenv("BROKER_PROVIDER", "ibkr"),
		Env:                    getenv("BROKER_ENV", "paper"),
		IBKRMode:               getenv("IBKR_MODE", "mock"),
		IBKRGatewayHost:        getenv("IBKR_GATEWAY_HOST", "127.0.0.1"),
		IBKRGatewayPort:        getenvInt("IBKR_GATEWAY_PORT", 4001),
		IBKRPortalPort:         getenvInt("IBKR_CLIENT_PORTAL_PORT", 5000),
		CoinbaseReadAPIKey:     getenv("COINBASE_READ_API_KEY", ""),
		CoinbaseReadAPISecret:  getenv("COINBASE_READ_API_SECRET", ""),
		CoinbaseTradeAPIKey:    getenv("COINBASE_TRADE_API_KEY", ""),
		CoinbaseTradeAPISecret: getenv("COINBASE_TRADE_API_SECRET", ""),
	}
}

func (c Broker) Validate() error {
	p := strings.ToLower(strings.TrimSpace(c.Provider))
	switch p {
	case "ibkr":
		if c.IBKRGatewayHost == "" {
			return fmt.Errorf("IBKR_GATEWAY_HOST is required for provider=ibkr")
		}
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
		Provider:    strings.ToLower(c.Provider),
		Environment: strings.ToLower(c.Env),
		Mode:        c.IBKRMode,
		GatewayHost: c.IBKRGatewayHost,
		GatewayPort: c.IBKRGatewayPort,
		PortalPort:  c.IBKRPortalPort,
	}
}

func getenvInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
