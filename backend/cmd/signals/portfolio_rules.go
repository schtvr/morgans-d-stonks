package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	sigpkg "github.com/schtvr/morgans-d-stonks/internal/signal"
)

type portfolioRulesStats struct {
	Fired            int
	AgentEnqueued    int
	SkippedDedup     int
	SkippedAgentCap  int
}

type firedRuleEvent struct {
	ev   sigpkg.SignalEvent
	rule sigpkg.Rule
}

func processPortfolioRules(
	ctx context.Context,
	log *slog.Logger,
	hc *http.Client,
	rules []sigpkg.Rule,
	ruleByID map[string]sigpkg.Rule,
	snap *portfolio.IngestSnapshotRequest,
	ruleDedup *sigpkg.Dedup,
	defaultRuleCooldown time.Duration,
	maxAgentPerSymbol24h int,
	baseURL, apiKey, promptVersion string,
	now time.Time,
) (portfolioRulesStats, error) {
	var stats portfolioRulesStats
	if len(rules) == 0 || snap == nil {
		return stats, nil
	}
	if len(snap.Positions) == 0 && snap.Summary.NetLiquidation <= 0 {
		return stats, nil
	}

	events, err := sigpkg.EvaluateAll(rules, snap)
	if err != nil {
		return stats, err
	}

	positions := make(map[string]broker.Position, len(snap.Positions))
	for _, p := range snap.Positions {
		positions[normalizeSymbol(p.Symbol)] = p
	}

	bySymbol := make(map[string][]firedRuleEvent)
	for _, ev := range events {
		rule, ok := ruleByID[ev.RuleID]
		if !ok {
			continue
		}
		cd := sigpkg.RuleCooldown(rule, defaultRuleCooldown)
		if ruleDedup != nil && !ruleDedup.ShouldFire(ev.RuleID, ev.Symbol, cd, now) {
			stats.SkippedDedup++
			continue
		}
		stats.Fired++
		sym := normalizeSymbol(ev.Symbol)
		bySymbol[sym] = append(bySymbol[sym], firedRuleEvent{ev: ev, rule: rule})

		if log != nil {
			log.Info("portfolio_rule_fired",
				"rule_id", ev.RuleID,
				"rule_name", ev.RuleName,
				"symbol", ev.Symbol,
				"value", ev.Value,
				"threshold", ev.Threshold,
				"agent", rule.Agent,
			)
		}
	}

	for sym, batch := range bySymbol {
		alert := buildCoalescedPortfolioRuleAlert(batch, positions[sym], now)
		if err := persistRecentAlert(ctx, hc, baseURL, apiKey, alert); err != nil && log != nil {
			log.Warn("recent alert persist", "symbol", alert.Symbol, "err", err)
		}

		wantAgent := false
		for _, fr := range batch {
			if fr.rule.Agent {
				wantAgent = true
				break
			}
		}
		if !wantAgent {
			continue
		}
		ok, reason := tryEnqueueAgent(ctx, log, hc, snap, alert, promptVersion, maxAgentPerSymbol24h, baseURL, apiKey, now)
		if ok {
			stats.AgentEnqueued++
		} else if reason == "symbol_decision_cap" {
			stats.SkippedAgentCap++
		}
	}
	return stats, nil
}

func buildCoalescedPortfolioRuleAlert(batch []firedRuleEvent, pos broker.Position, firedAt time.Time) sigpkg.CryptoAlert {
	if len(batch) == 0 {
		return sigpkg.CryptoAlert{}
	}
	first := batch[0].ev
	symbol := coinbase.CanonicalToProviderSymbol(first.Symbol)
	if symbol == "" {
		symbol = strings.ToUpper(strings.TrimSpace(first.Symbol))
	}
	if symbol == sigpkg.PortfolioRuleSymbol {
		symbol = sigpkg.PortfolioRuleSymbol
	}

	flags := make([]string, 0, len(batch))
	seen := make(map[string]struct{}, len(batch))
	var primaryValue, primaryThreshold float64
	for _, fr := range batch {
		if _, ok := seen[fr.ev.RuleID]; ok {
			continue
		}
		seen[fr.ev.RuleID] = struct{}{}
		flags = append(flags, fr.ev.RuleID)
	}
	sort.Strings(flags)
	primaryValue = batch[0].ev.Value
	primaryThreshold = batch[0].ev.Threshold

	var currentPrice float64
	if pos.Quantity != 0 {
		currentPrice = pos.MarketValue / pos.Quantity
	}

	alert := sigpkg.CryptoAlert{
		SchemaVersion: sigpkg.CryptoSignalSchemaVersion,
		ID:            stableCoalescedPortfolioAlertID(symbol, firedAt),
		Type:          "portfolio_rule",
		ReasonFlags:   flags,
		Symbol:        symbol,
		ProductID:     symbol,
		Source:        "snapshot_rule",
		CurrentPrice:  currentPrice,
		DeltaPct:      primaryValue,
		ThresholdPct:  primaryThreshold,
		FiredAt:       firedAt,
	}
	if pos.Quantity != 0 {
		qty := pos.Quantity
		alert.Quantity = &qty
	}
	if pos.AvgCost != 0 {
		avgCost := pos.AvgCost
		alert.AvgCost = &avgCost
		costBasis := pos.AvgCost * pos.Quantity
		alert.CostBasis = &costBasis
		if costBasis != 0 {
			pl := pos.UnrealizedPL
			alert.UnrealizedPL = &pl
			plPct := (pl / costBasis) * 100
			alert.UnrealizedPLPct = &plPct
		}
	}
	return alert
}

func stableCoalescedPortfolioAlertID(symbol string, firedAt time.Time) string {
	sym := strings.ToLower(strings.ReplaceAll(symbol, "-", "_"))
	return fmt.Sprintf("portfolio-%s-%s", sym, firedAt.UTC().Format("20060102T150405Z"))
}

func normalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

func indexRules(rules []sigpkg.Rule) map[string]sigpkg.Rule {
	m := make(map[string]sigpkg.Rule, len(rules))
	for _, r := range rules {
		m[r.ID] = r
	}
	return m
}
