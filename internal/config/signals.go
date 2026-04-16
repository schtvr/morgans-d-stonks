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
	PortfolioAPIURL   string
	InternalAPIKey    string
	DedupPath         string
	DiscordWebhookURL string
	// DiscordBotMention is optional raw Discord mention text (e.g. "<@123456789>" or "<@&roleId>").
	// When set, signal webhook messages are prefixed with it so the bot receives a ping.
	DiscordBotMention string
}

// LoadSignals loads signals service configuration from the environment.
func LoadSignals() Signals {
	return Signals{
		RulesPath:         getenv("SIGNAL_RULES_PATH", "./config/signals.yaml"),
		Cooldown:          getenvDuration("SIGNAL_COOLDOWN", time.Hour),
		Interval:          getenvDuration("SIGNAL_INTERVAL", 5*time.Minute),
		PortfolioAPIURL:   getenv("PORTFOLIO_API_URL", "http://localhost:8080"),
		InternalAPIKey:    getenv("INTERNAL_API_KEY", "changeme"),
		DedupPath:         getenv("SIGNAL_DEDUP_PATH", "./data/signal-dedup.json"),
		DiscordWebhookURL: strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL")),
		DiscordBotMention: strings.TrimSpace(os.Getenv("DISCORD_SIGNAL_BOT_MENTION")),
	}
}
