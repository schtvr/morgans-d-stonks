package signal

import "time"

// SignalEvent is emitted when a rule fires (stable JSON for P1 consumers).
type SignalEvent struct {
	ID        string    `json:"id"`
	RuleID    string    `json:"ruleId"`
	RuleName  string    `json:"ruleName"`
	Symbol    string    `json:"symbol"`
	Signal    string    `json:"signal"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	FiredAt   time.Time `json:"firedAt"`
}

// RuleFile is the top-level YAML document.
type RuleFile struct {
	Version int    `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
}

// Rule is a deterministic rule definition.
type Rule struct {
	ID          string    `yaml:"id"`
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Condition   Condition `yaml:"condition"`
}

// Condition is evaluated per position.
type Condition struct {
	Type      string  `yaml:"type"`
	Field     string  `yaml:"field"`
	Operator  string  `yaml:"operator"`
	Threshold float64 `yaml:"threshold"`
}
