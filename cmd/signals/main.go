package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	osignal "os/signal"
	"syscall"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/config"
	"github.com/schtvr/morgans-d-stonks/internal/discord"
	"github.com/schtvr/morgans-d-stonks/internal/logging"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	sigpkg "github.com/schtvr/morgans-d-stonks/internal/signal"
)

func main() {
	log := logging.New("signals")

	cfg := config.LoadSignals()

	rules, err := sigpkg.LoadRulesFile(cfg.RulesPath)
	if err != nil {
		log.Error("load rules", "err", err)
		os.Exit(1)
	}
	ded, err := sigpkg.NewDedup(cfg.DedupPath)
	if err != nil {
		log.Error("dedup", "err", err)
		os.Exit(1)
	}
	dc := discord.NewClient(cfg.DiscordWebhookURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ch := make(chan os.Signal, 1)
		osignal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	t := time.NewTicker(cfg.Interval)
	defer t.Stop()

	hc := &http.Client{Timeout: 30 * time.Second}

	discordEnabled := cfg.DiscordWebhookURL != ""
	run := func() {
		if err := runOnce(ctx, log, hc, cfg.PortfolioAPIURL, cfg.InternalAPIKey, rules, ded, cfg.Cooldown, dc, discordEnabled, cfg.DiscordBotMention); err != nil {
			log.Warn("tick", "err", err)
		}
	}
	run()
	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-t.C:
			run()
		}
	}
}

func runOnce(
	ctx context.Context,
	log *slog.Logger,
	hc *http.Client,
	baseURL string,
	apiKey string,
	rules []sigpkg.Rule,
	ded *sigpkg.Dedup,
	cooldown time.Duration,
	dc *discord.Client,
	discordEnabled bool,
	discordBotMention string,
) error {
	snap, err := fetchSnapshot(ctx, hc, baseURL, apiKey)
	if err != nil {
		return err
	}
	evs, err := sigpkg.EvaluateAll(rules, snap)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, ev := range evs {
		if !ded.ShouldFire(ev.RuleID, ev.Symbol, cooldown, now) {
			continue
		}
		if !discordEnabled {
			log.Info("signal", "event", ev.Signal, "value", ev.Value)
			continue
		}
		msg := discord.SignalWebhookContent(discordBotMention, ev.Symbol, ev.RuleName)
		if err := dc.SendMessage(ctx, msg); err != nil {
			log.Warn("discord", "err", err)
		}
	}
	return nil
}

func fetchSnapshot(ctx context.Context, hc *http.Client, baseURL, apiKey string) (*portfolio.IngestSnapshotRequest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/internal/snapshot/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Key", apiKey)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return &portfolio.IngestSnapshotRequest{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("portfolio: %s: %s", resp.Status, string(b))
	}
	var snap portfolio.IngestSnapshotRequest
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

