package broker

import (
	"os"
	"strings"
)

// Config configures broker construction (Coinbase read or in-process mock).
type Config struct {
	Provider          string // coinbase | mock
	Environment       string // paper | live
	CoinbaseAPIKey    string
	CoinbaseAPISecret string
}

// LoadConfigFromEnv reads BROKER_* and Coinbase CDP variables (legacy helper).
func LoadConfigFromEnv() Config {
	return Config{
		Provider:          strings.ToLower(getenv("BROKER_PROVIDER", "coinbase")),
		Environment:       strings.ToLower(getenv("BROKER_ENV", "paper")),
		CoinbaseAPIKey:    getenv("COINBASE_READ_API_KEY", ""),
		CoinbaseAPISecret: getenv("COINBASE_READ_API_SECRET", ""),
	}
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
