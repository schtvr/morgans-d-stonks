package config

import (
	"testing"
	"time"
)

func TestLoadSignals_defaults(t *testing.T) {
	t.Setenv("SIGNAL_RULES_PATH", "")
	t.Setenv("SIGNAL_COOLDOWN", "")
	t.Setenv("SIGNAL_INTERVAL", "")
	t.Setenv("SIGNAL_MOVE_THRESHOLD_PCT", "")
	t.Setenv("PORTFOLIO_API_URL", "")
	t.Setenv("INTERNAL_API_KEY", "")
	t.Setenv("SIGNAL_STATE_PATH", "")
	t.Setenv("DISCORD_WEBHOOK_URL", "")
	t.Setenv("DISCORD_SIGNAL_BOT_MENTION", "")

	cfg := LoadSignals()
	if cfg.RulesPath != "./config/signals.yaml" {
		t.Fatalf("RulesPath: %q", cfg.RulesPath)
	}
	if cfg.Cooldown != 15*time.Minute {
		t.Fatalf("Cooldown: %v", cfg.Cooldown)
	}
	if cfg.Interval != 5*time.Minute {
		t.Fatalf("Interval: %v", cfg.Interval)
	}
	if cfg.ThresholdPct != 1.0 {
		t.Fatalf("ThresholdPct: %v", cfg.ThresholdPct)
	}
	if cfg.PortfolioAPIURL != "http://localhost:8080" {
		t.Fatalf("PortfolioAPIURL: %q", cfg.PortfolioAPIURL)
	}
	if cfg.InternalAPIKey != "changeme" {
		t.Fatalf("InternalAPIKey: %q", cfg.InternalAPIKey)
	}
	if cfg.StatePath != "./data/signal-state.json" {
		t.Fatalf("StatePath: %q", cfg.StatePath)
	}
	if cfg.DiscordWebhookURL != "" || cfg.DiscordBotMention != "" {
		t.Fatalf("discord empty: webhook=%q mention=%q", cfg.DiscordWebhookURL, cfg.DiscordBotMention)
	}
}

func TestLoadSignals_discordAndMention(t *testing.T) {
	t.Setenv("DISCORD_WEBHOOK_URL", "  https://example.com/hook  ")
	t.Setenv("DISCORD_SIGNAL_BOT_MENTION", "  <@999>  ")
	t.Setenv("SIGNAL_MOVE_THRESHOLD_PCT", "0.75")

	cfg := LoadSignals()
	if cfg.DiscordWebhookURL != "https://example.com/hook" {
		t.Fatalf("webhook trim: %q", cfg.DiscordWebhookURL)
	}
	if cfg.DiscordBotMention != "<@999>" {
		t.Fatalf("mention trim: %q", cfg.DiscordBotMention)
	}
	if cfg.ThresholdPct != 0.75 {
		t.Fatalf("threshold trim: %v", cfg.ThresholdPct)
	}
}
