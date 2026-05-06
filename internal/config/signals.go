package config

import (
	"os"
	"strings"
	"time"
)

// Signals holds environment for cmd/signals.
type Signals struct {
	RulesPath         string
	Cooldown          time.Duration
	Interval          time.Duration
	ThresholdPct      float64
	PortfolioAPIURL   string
	InternalAPIKey    string
	StatePath         string
	DiscordWebhookURL string
	// DiscordBotMention is optional raw Discord mention text (e.g. "<@123456789>" or "<@&roleId>").
	// The crypto alert payload itself remains machine-readable JSON.
	DiscordBotMention string
}

// LoadSignals loads signals service configuration from the environment.
func LoadSignals() Signals {
	return Signals{
		RulesPath:         getenv("SIGNAL_RULES_PATH", "./config/signals.yaml"),
		Cooldown:          getenvDuration("SIGNAL_COOLDOWN", 15*time.Minute),
		Interval:          getenvDuration("SIGNAL_INTERVAL", 5*time.Minute),
		ThresholdPct:      getenvFloat("SIGNAL_MOVE_THRESHOLD_PCT", 1.0),
		PortfolioAPIURL:   getenv("PORTFOLIO_API_URL", "http://localhost:8080"),
		InternalAPIKey:    getenv("INTERNAL_API_KEY", "changeme"),
		StatePath:         getenv("SIGNAL_STATE_PATH", "./data/signal-state.json"),
		DiscordWebhookURL: strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL")),
		DiscordBotMention: strings.TrimSpace(os.Getenv("DISCORD_SIGNAL_BOT_MENTION")),
	}
}
