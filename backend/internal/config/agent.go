package config

import (
	"fmt"
	"math"
	"strings"
)

// Agent holds environment for the in-process LLM decision agent (signals service).
type Agent struct {
	Enabled           bool
	Provider          string // "mock" | "anthropic"
	Model             string
	AnthropicAPIKey   string
	DailyCostCapCents int64  // derived from AGENT_DAILY_COST_CAP_USD * 100
	DailyTimerUTC     string // "HH:MM"
	Concurrency       int
	PromptPath        string
	PortfolioAPIURL   string // reuse PORTFOLIO_API_URL from signals
	InternalAPIKey    string // reuse INTERNAL_API_KEY from signals
}

// LoadAgent loads agent configuration from the environment.
func LoadAgent() Agent {
	capUSD := getenvFloat("AGENT_DAILY_COST_CAP_USD", 5.0)
	capCents := int64(math.Round(capUSD * 100))

	concurrency := int(getenvFloat("AGENT_CONCURRENCY", 2))
	if concurrency < 1 {
		concurrency = 1
	}

	return Agent{
		Enabled:           getenvBool("AGENT_ENABLED", false),
		Provider:          getenv("AGENT_PROVIDER", "mock"),
		Model:             getenv("AGENT_MODEL", "claude-sonnet-4-5"),
		AnthropicAPIKey:   strings.TrimSpace(getenv("ANTHROPIC_API_KEY", "")),
		DailyCostCapCents: capCents,
		DailyTimerUTC:     getenv("AGENT_DAILY_TIMER_UTC", "12:00"),
		Concurrency:       concurrency,
		PromptPath:        getenv("AGENT_PROMPT_PATH", "config/agent-prompt.md"),
		PortfolioAPIURL:   getenv("PORTFOLIO_API_URL", "http://portfolio-api:8080"),
		InternalAPIKey:    getenv("INTERNAL_API_KEY", ""),
	}
}

// Validate returns an error if the configuration is inconsistent or incomplete.
func (a Agent) Validate() error {
	if !a.Enabled {
		return nil
	}
	switch strings.ToLower(a.Provider) {
	case "mock", "anthropic":
	default:
		return fmt.Errorf("AGENT_PROVIDER must be \"mock\" or \"anthropic\", got %q", a.Provider)
	}
	if strings.EqualFold(a.Provider, "anthropic") && strings.TrimSpace(a.AnthropicAPIKey) == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is required when AGENT_PROVIDER=anthropic")
	}
	if a.Concurrency < 1 {
		return fmt.Errorf("AGENT_CONCURRENCY must be >= 1")
	}
	return nil
}
