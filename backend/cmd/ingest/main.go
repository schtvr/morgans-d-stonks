package main

import (
	"context"
	"os"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/brokerwire"
	"github.com/schtvr/morgans-d-stonks/internal/config"
	"github.com/schtvr/morgans-d-stonks/internal/ingest"
	"github.com/schtvr/morgans-d-stonks/internal/logging"
)

func main() {
	log := logging.New("ingest")

	brokerCfg := config.LoadBroker()
	if err := brokerCfg.Validate(); err != nil {
		log.Error("broker-config", "err", err)
		os.Exit(1)
	}
	br, err := brokerwire.New(brokerCfg.ToLegacyBrokerConfig())
	if err != nil {
		log.Error("broker", "err", err)
		os.Exit(1)
	}
	defer func() { _ = br.Close() }()

	baseURL := getenv("PORTFOLIO_API_URL", "http://localhost:8080")
	apiKey := getenv("INTERNAL_API_KEY", "changeme")
	interval := getenvDuration("INGEST_INTERVAL", 10*time.Minute)

	client := ingest.NewClient(baseURL, apiKey)
	r := &ingest.Runner{
		Broker:   br,
		Client:   client,
		Interval: interval,
		Log:      log,
	}
	if err := r.Run(context.Background()); err != nil {
		log.Error("runner", "err", err)
		os.Exit(1)
	}
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
