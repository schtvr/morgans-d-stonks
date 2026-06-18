package config

import (
	"testing"
)

func TestLoadAgent_Defaults(t *testing.T) {
	t.Parallel()
	a := LoadAgent()
	if a.Provider != "mock" {
		t.Errorf("expected default provider mock, got %q", a.Provider)
	}
	if a.Model != "claude-haiku-4-5" {
		t.Errorf("expected default model claude-haiku-4-5, got %q", a.Model)
	}
	if a.DailyCostCapCents != 500 {
		t.Errorf("expected default cap 500 cents (=$5), got %d", a.DailyCostCapCents)
	}
	if a.DailyTimerUTC != "12:00" {
		t.Errorf("expected default timer 12:00, got %q", a.DailyTimerUTC)
	}
	if a.Concurrency != 2 {
		t.Errorf("expected default concurrency 2, got %d", a.Concurrency)
	}
}

func TestAgent_Validate_Disabled(t *testing.T) {
	t.Parallel()
	a := Agent{Enabled: false}
	if err := a.Validate(); err != nil {
		t.Errorf("disabled agent should always validate: %v", err)
	}
}

func TestAgent_Validate_AnthropicRequiresKey(t *testing.T) {
	t.Parallel()
	a := Agent{Enabled: true, Provider: "anthropic", Concurrency: 1}
	if err := a.Validate(); err == nil {
		t.Error("expected error when anthropic key is missing")
	}
	a.AnthropicAPIKey = "sk-test"
	if err := a.Validate(); err != nil {
		t.Errorf("unexpected error with key set: %v", err)
	}
}

func TestAgent_Validate_MockNoKeyRequired(t *testing.T) {
	t.Parallel()
	a := Agent{Enabled: true, Provider: "mock", Concurrency: 1}
	if err := a.Validate(); err != nil {
		t.Errorf("mock provider should not require API key: %v", err)
	}
}

func TestAgent_Validate_UnknownProvider(t *testing.T) {
	t.Parallel()
	a := Agent{Enabled: true, Provider: "openai", Concurrency: 1}
	if err := a.Validate(); err == nil {
		t.Error("expected error for unknown provider")
	}
}
