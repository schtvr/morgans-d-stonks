package config

import "time"

// Signals holds environment for backend/cmd/signals.
type Signals struct {
	RulesPath              string
	RulesDedupPath         string
	RuleCooldown           time.Duration
	AgentMaxPerSymbol24h   int
	Cooldown               time.Duration
	Interval               time.Duration
	ThresholdPct           float64
	PortfolioAPIURL        string
	InternalAPIKey         string
	StatePath              string
	WSEnabled              bool
	CoinbaseWSURL          string
}

// LoadSignals loads signals service configuration from the environment.
func LoadSignals() Signals {
	maxAgent := int(getenvFloat("SIGNAL_AGENT_MAX_PER_SYMBOL_24H", 2))
	if maxAgent < 0 {
		maxAgent = 0
	}

	return Signals{
		RulesPath:            getenv("SIGNAL_RULES_PATH", "./config/signals.yaml"),
		RulesDedupPath:       getenv("SIGNAL_RULE_DEDUP_PATH", "./data/signal-rules-dedup.json"),
		RuleCooldown:         getenvDuration("SIGNAL_RULE_COOLDOWN", 24*time.Hour),
		AgentMaxPerSymbol24h: maxAgent,
		Cooldown:             getenvDuration("SIGNAL_COOLDOWN", time.Hour),
		Interval:             getenvDuration("SIGNAL_INTERVAL", 5*time.Minute),
		ThresholdPct:         getenvFloat("SIGNAL_MOVE_THRESHOLD_PCT", 2.5),
		PortfolioAPIURL: getenv("PORTFOLIO_API_URL", "http://localhost:8080"),
		InternalAPIKey:  getenv("INTERNAL_API_KEY", "changeme"),
		StatePath:       getenv("SIGNAL_STATE_PATH", "./data/signal-state.json"),
		WSEnabled:       getenvBool("SIGNALS_WS_ENABLED", false),
		CoinbaseWSURL:   getenv("COINBASE_WS_URL", "wss://advanced-trade-ws.coinbase.com"),
	}
}
