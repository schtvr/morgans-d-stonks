package signal

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

// EvaluateAll runs all rules against the latest snapshot.
func EvaluateAll(rules []Rule, snap *portfolio.IngestSnapshotRequest) ([]SignalEvent, error) {
	var out []SignalEvent
	for _, rule := range rules {
		evs, err := Evaluate(rule, snap)
		if err != nil {
			return nil, err
		}
		out = append(out, evs...)
	}
	return out, nil
}

// Evaluate runs a single rule against the snapshot (pure).
func Evaluate(rule Rule, snap *portfolio.IngestSnapshotRequest) ([]SignalEvent, error) {
	if snap == nil {
		return nil, nil
	}
	totalMV := totalMarketValue(snap.Positions)
	var out []SignalEvent
	for _, p := range snap.Positions {
		val, ok, err := metricValue(rule.Condition, p, totalMV)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if !compare(rule.Condition.Operator, val, rule.Condition.Threshold) {
			continue
		}
		out = append(out, SignalEvent{
			ID:        uuid.NewString(),
			RuleID:    rule.ID,
			RuleName:  rule.Name,
			Symbol:    p.Symbol,
			Signal:    fmt.Sprintf("%s | %s", p.Symbol, rule.Name),
			Value:     val,
			Threshold: rule.Condition.Threshold,
			FiredAt:   time.Now().UTC(),
		})
	}
	return out, nil
}

func totalMarketValue(positions []broker.Position) float64 {
	var t float64
	for _, p := range positions {
		t += p.MarketValue
	}
	if t == 0 {
		return 1
	}
	return t
}

func metricValue(c Condition, p broker.Position, totalMV float64) (float64, bool, error) {
	switch c.Type {
	case "price_change_pct":
		cost := p.AvgCost * p.Quantity
		if cost == 0 {
			return 0, false, nil
		}
		pct := (p.UnrealizedPL / cost) * 100
		return pct, true, nil
	case "concentration":
		pct := (p.MarketValue / totalMV) * 100
		return pct, true, nil
	default:
		return 0, false, fmt.Errorf("signal: unknown condition type %q", c.Type)
	}
}

func compare(op string, value, threshold float64) bool {
	switch op {
	case "lte":
		return value <= threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "gt":
		return value > threshold
	default:
		return false
	}
}
