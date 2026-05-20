package config

import (
	"testing"
	"time"
)

func TestLoadSignals_defaults(t *testing.T) {
	t.Setenv("SIGNAL_RULES_PATH", "")
	t.Setenv("SIGNAL_RULE_DEDUP_PATH", "")
	t.Setenv("SIGNAL_RULE_COOLDOWN", "")
	t.Setenv("SIGNAL_COOLDOWN", "")
	t.Setenv("SIGNAL_INTERVAL", "")
	t.Setenv("SIGNAL_MOVE_THRESHOLD_PCT", "")
	t.Setenv("PORTFOLIO_API_URL", "")
	t.Setenv("INTERNAL_API_KEY", "")
	t.Setenv("SIGNAL_STATE_PATH", "")
	t.Setenv("SIGNALS_WS_ENABLED", "")
	t.Setenv("COINBASE_WS_URL", "")

	cfg := LoadSignals()
	if cfg.RulesPath != "./config/signals.yaml" {
		t.Fatalf("RulesPath: %q", cfg.RulesPath)
	}
	if cfg.RulesDedupPath != "./data/signal-rules-dedup.json" {
		t.Fatalf("RulesDedupPath: %q", cfg.RulesDedupPath)
	}
	if cfg.RuleCooldown != 24*time.Hour {
		t.Fatalf("RuleCooldown: %v", cfg.RuleCooldown)
	}
	if cfg.AgentMaxPerSymbol24h != 2 {
		t.Fatalf("AgentMaxPerSymbol24h: %d", cfg.AgentMaxPerSymbol24h)
	}
	if cfg.Cooldown != time.Hour {
		t.Fatalf("Cooldown: %v", cfg.Cooldown)
	}
	if cfg.Interval != 5*time.Minute {
		t.Fatalf("Interval: %v", cfg.Interval)
	}
	if cfg.ThresholdPct != 2.5 {
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
	if cfg.WSEnabled {
		t.Fatal("ws should be disabled by default")
	}
	if cfg.CoinbaseWSURL != "wss://advanced-trade-ws.coinbase.com" {
		t.Fatalf("ws url default: %q", cfg.CoinbaseWSURL)
	}
}

func TestLoadSignals_overrides(t *testing.T) {
	t.Setenv("SIGNAL_MOVE_THRESHOLD_PCT", "0.75")
	t.Setenv("SIGNALS_WS_ENABLED", "true")
	t.Setenv("COINBASE_WS_URL", "wss://example.test/ws")
	t.Setenv("SIGNAL_RULE_COOLDOWN", "12h")

	cfg := LoadSignals()
	if cfg.ThresholdPct != 0.75 {
		t.Fatalf("threshold: %v", cfg.ThresholdPct)
	}
	if !cfg.WSEnabled {
		t.Fatal("expected ws enabled")
	}
	if cfg.CoinbaseWSURL != "wss://example.test/ws" {
		t.Fatalf("ws url: %q", cfg.CoinbaseWSURL)
	}
	if cfg.RuleCooldown != 12*time.Hour {
		t.Fatalf("rule cooldown: %v", cfg.RuleCooldown)
	}
}
