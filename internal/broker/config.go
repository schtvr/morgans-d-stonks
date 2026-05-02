package broker

import (
	"os"
	"strconv"
)

// Config configures broker construction (IBKR / mock).
type Config struct {
	Provider    string // ibkr | coinbase
	Environment string // paper | live
	Mode        string // ibkr-only: mock | paper | live
	GatewayHost string
	GatewayPort int
	// PortalPort is the HTTPS Client Portal port (typically 5000). 0 means unset.
	PortalPort int
}

// LoadConfigFromEnv reads IBKR_* environment variables.
func LoadConfigFromEnv() Config {
	cfg := Config{
		Provider:    getenv("BROKER_PROVIDER", "ibkr"),
		Environment: getenv("BROKER_ENV", "paper"),
		Mode:        getenv("IBKR_MODE", "mock"),
		GatewayHost: getenv("IBKR_GATEWAY_HOST", "127.0.0.1"),
		GatewayPort: getenvInt("IBKR_GATEWAY_PORT", 4001),
		PortalPort:  getenvInt("IBKR_CLIENT_PORTAL_PORT", 5000),
	}
	return cfg
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
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
