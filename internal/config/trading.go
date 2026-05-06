package config

import (
	"fmt"
	"os"
	"strings"
)

// Trading holds env for the trading API / worker rollout controls.
type Trading struct {
	Enabled          bool
	KillSwitch       bool
	MaxNotional      float64
	Reserve          float64
	AllowedProviders []string
	AllowedSymbols   []string
	DeniedSymbols    []string
}

// LoadTrading loads trading rollout config from the environment.
func LoadTrading() Trading {
	return Trading{
		Enabled:          getenvBool("TRADING_ENABLED", false),
		KillSwitch:       getenvBool("TRADING_KILL_SWITCH", false),
		MaxNotional:      getenvFloat("TRADING_MAX_NOTIONAL", 0),
		Reserve:          getenvFloat("TRADING_RESERVE", 0),
		AllowedProviders: getenvCSVList("TRADING_ALLOWED_PROVIDERS", ""),
		AllowedSymbols:   getenvCSVList("TRADING_ALLOWED_SYMBOLS", ""),
		DeniedSymbols:    getenvCSVList("TRADING_DENIED_SYMBOLS", ""),
	}
}

// Validate returns an error for unsafe trading combinations.
func (c Trading) Validate(provider string) error {
	if !c.Enabled {
		return nil
	}
	if c.KillSwitch {
		return fmt.Errorf("TRADING_ENABLED cannot be true while TRADING_KILL_SWITCH is enabled")
	}
	if len(c.AllowedProviders) == 0 {
		return fmt.Errorf("TRADING_ALLOWED_PROVIDERS is required when TRADING_ENABLED=true")
	}
	if len(c.AllowedSymbols) == 0 {
		return fmt.Errorf("TRADING_ALLOWED_SYMBOLS is required when TRADING_ENABLED=true")
	}
	if !containsFoldLocal(c.AllowedProviders, provider) {
		return fmt.Errorf("provider %q is not permitted by TRADING_ALLOWED_PROVIDERS", provider)
	}
	if c.MaxNotional <= 0 {
		return fmt.Errorf("TRADING_MAX_NOTIONAL must be > 0 when trading is enabled")
	}
	return nil
}

func containsFoldLocal(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), want) {
			return true
		}
	}
	return false
}

func getenvBool(k string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func getenvFloat(k string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	var out float64
	if _, err := fmt.Sscanf(v, "%f", &out); err != nil {
		return def
	}
	return out
}
