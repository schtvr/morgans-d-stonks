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

// CryptoAlert is the compact machine-readable payload sent to Discord/OpenClaw.
type CryptoAlert struct {
	Type            string    `json:"type"`
	Symbol          string    `json:"symbol"`
	ProductID       string    `json:"productId,omitempty"`
	Source          string    `json:"source,omitempty"`
	CurrentPrice    float64   `json:"currentPrice"`
	PreviousPrice   *float64  `json:"previousPrice,omitempty"`
	DeltaAmount     *float64  `json:"deltaAmount,omitempty"`
	DeltaPct        float64   `json:"deltaPct"`
	ThresholdPct    float64   `json:"thresholdPct"`
	Quantity        *float64  `json:"quantity,omitempty"`
	AvgCost         *float64  `json:"avgCost,omitempty"`
	CostBasis       *float64  `json:"costBasis,omitempty"`
	UnrealizedPL    *float64  `json:"unrealizedPl,omitempty"`
	UnrealizedPLPct *float64  `json:"unrealizedPlPct,omitempty"`
	FiredAt         time.Time `json:"firedAt"`
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
