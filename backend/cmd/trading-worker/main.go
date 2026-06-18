package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/brokerwire"
	"github.com/schtvr/morgans-d-stonks/internal/config"
	"github.com/schtvr/morgans-d-stonks/internal/discord"
	"github.com/schtvr/morgans-d-stonks/internal/logging"
	"github.com/schtvr/morgans-d-stonks/internal/trading"
	tradepg "github.com/schtvr/morgans-d-stonks/internal/trading/postgres"
)

func main() {
	log := logging.New("trading-worker")
	brokerCfg := config.LoadBroker()
	tradingCfg := config.LoadTrading()
	if err := brokerCfg.Validate(); err != nil {
		log.Error("invalid broker config", "err", err)
		os.Exit(1)
	}
	if err := tradingCfg.Validate(brokerCfg.Provider, brokerCfg.Env); err != nil {
		if tradingCfg.Enabled {
			log.Error("invalid trading config", "err", err)
			os.Exit(1)
		}
		log.Info("trading disabled", "provider", brokerCfg.Provider)
	}

	ctx, cancel := signalContext()
	defer cancel()

	repo, err := tradepg.New(ctx, getenv("DATABASE_URL", "postgres://portfolio:changeme@db:5432/portfolio?sslmode=disable"))
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer repo.Close()
	if err := repo.RunMigrations(ctx); err != nil {
		log.Error("migrations", "err", err)
		os.Exit(1)
	}

	var exec trading.Executor
	if tradingCfg.Enabled {
		e, err := brokerwire.NewExecution(brokerCfg.ToLegacyBrokerConfig())
		if err != nil {
			log.Error("execution broker unavailable", "err", err, "execution_mode", brokerCfg.Env)
			os.Exit(1)
		}
		exec = e
		log.Info("execution broker ready", "provider", brokerCfg.Provider, "env", brokerCfg.Env)
	}

	// Wire Discord notifications for fills and rejected/cancelled outcomes.
	var notifyFn func(ctx context.Context, msg string)
	if webhookURL := getenv("DISCORD_WEBHOOK_URL", ""); webhookURL != "" {
		dc := discord.NewClient(webhookURL)
		notifyFn = func(ctx context.Context, msg string) {
			if err := dc.SendMessage(ctx, msg); err != nil {
				log.Warn("discord notify", "err", err)
			}
		}
		log.Info("discord notifications enabled")
	}

	worker := &trading.Worker{
		Repo:     repo,
		Executor: exec,
		Interval: getenvDuration("TRADING_INTERVAL", 30*time.Second),
		Metrics:  &trading.Metrics{},
		Log:      log,
		Notify:   notifyFn,
	}
	if err := worker.Run(ctx); err != nil && err != context.Canceled {
		log.Error("worker", "err", err)
		os.Exit(1)
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func getenvDuration(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
