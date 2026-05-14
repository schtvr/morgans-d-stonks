package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5"

	"github.com/schtvr/morgans-d-stonks/internal/auth"
	"github.com/schtvr/morgans-d-stonks/internal/config"
	"github.com/schtvr/morgans-d-stonks/internal/discord"
	"github.com/schtvr/morgans-d-stonks/internal/logging"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	pgstore "github.com/schtvr/morgans-d-stonks/internal/portfolio/postgres"
	"github.com/schtvr/morgans-d-stonks/internal/trading"
	tradepg "github.com/schtvr/morgans-d-stonks/internal/trading/postgres"
)

// REST API for portfolio snapshots and single-user session auth (SCH-18).
func main() {
	cfg := config.LoadPortfolioAPI()
	log := logging.New("portfolio-api")

	if cfg.DatabaseURL == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	if err := config.ValidatePortfolioAPI(cfg); err != nil {
		log.Error("invalid config", "err", err)
		os.Exit(1)
	}
	brokerCfg := config.LoadBroker()
	tradingCfg := config.LoadTrading()
	if err := tradingCfg.Validate(brokerCfg.Provider); err != nil {
		log.Error("invalid trading config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	repo, err := pgstore.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer repo.Close()

	if err := repo.RunMigrations(ctx); err != nil {
		log.Error("migrations", "err", err)
		os.Exit(1)
	}
	tradeRepo, err := tradepg.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("trading db connect", "err", err)
		os.Exit(1)
	}
	defer tradeRepo.Close()
	if err := tradeRepo.RunMigrations(ctx); err != nil {
		log.Error("trading migrations", "err", err)
		os.Exit(1)
	}
	if err := seedFollowedSymbolsFromLatestSnapshot(ctx, repo); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Warn("seed followed symbols", "err", err)
	}
	if err := repo.CompactLabOpenClawPayloads(ctx, time.Now().UTC().Add(-labPayloadRetention)); err != nil {
		log.Warn("compact lab payloads", "err", err)
	}

	app := &app{
		cfg:        cfg,
		tradingCfg: tradingCfg,
		repo:       repo,
		tradeRepo:  tradeRepo,
		tradeSvc: trading.NewService(tradeRepo, trading.Policy{
			MaxNotional:       tradingCfg.MaxNotional,
			Reserve:           tradingCfg.Reserve,
			KillSwitch:        tradingCfg.KillSwitch,
			AllowedProviders:  tradingCfg.AllowedProviders,
			AllowedSymbols:    tradingCfg.AllowedSymbols,
			DeniedSymbols:     tradingCfg.DeniedSymbols,
			SymbolCooldown:    tradingCfg.SymbolCooldown,
			GlobalMaxExposure: tradingCfg.GlobalMaxExposure,
		}),
		metrics: &trading.Metrics{},
		dc:      discord.NewClient(cfg.DiscordWebhookURL),
		log:     log,
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(logging.AccessLog(log))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/api/health", app.handleHealth)
	r.Get("/metrics", app.handleMetrics)

	r.Post("/api/auth/login", app.handleLogin)
	r.Group(func(r chi.Router) {
		r.Use(auth.SessionMiddleware(repo))
		r.Post("/api/auth/logout", app.handleLogout)
		r.Get("/api/portfolio/positions", app.handlePositions)
		r.Get("/api/portfolio/summary", app.handleSummary)
		r.Get("/api/trading/followed-symbols", app.handleFollowedSymbolsList)
		r.Post("/api/trading/followed-symbols", app.handleFollowedSymbolsAdd)
		r.Delete("/api/trading/followed-symbols/{symbol}", app.handleFollowedSymbolRemove)
		r.Get("/api/trading/alert-settings", app.handleAlertSettingsGet)
		r.Put("/api/trading/alert-settings", app.handleAlertSettingsUpdate)
		r.Get("/api/trading/recent-alerts", app.handleRecentAlertsList)
		r.Route("/api/lab", func(r chi.Router) {
			r.Get("/overview", app.handleLabOverview)
			r.Get("/signals", app.handleLabSignalsList)
			r.Get("/signals/{id}", app.handleLabSignalGet)
			r.Get("/runs", app.handleLabRunsList)
			r.Get("/runs/{requestId}", app.handleLabRunGet)
			r.Post("/runs/{requestId}/retry", app.handleLabRunRetry)
			r.Post("/openclaw/pause", app.handleLabOpenClawPause)
			r.Post("/openclaw/resume", app.handleLabOpenClawResume)
			r.Post("/openclaw/circuit/reset", app.handleLabOpenClawCircuitReset)
			r.Post("/notes", app.handleLabNoteCreate)
			r.Get("/telemetry", app.handleLabTelemetry)
			r.Get("/signal-settings/history", app.handleSignalSettingsHistory)
			r.Post("/signal-settings/revert", app.handleSignalSettingsRevert)
		})
		if app.tradingCfg.Enabled {
			r.Get("/api/trading/orders/open", app.handleOpenOrdersList)
		}
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.InternalKeyMiddleware(cfg.InternalAPIKey))
		r.Post("/internal/snapshots", app.handleInternalSnapshot)
		r.Get("/internal/snapshot/latest", app.handleInternalLatest)
		r.Get("/internal/followed-symbols", app.handleInternalFollowedSymbols)
		r.Get("/internal/signal-settings", app.handleInternalSignalSettings)
		r.Post("/internal/recent-alerts", app.handleInternalRecentAlertCreate)
		if app.tradingCfg.Enabled {
			r.Route("/internal/orders", func(r chi.Router) {
				r.Use(app.tradingGate)
				r.Post("/validate", app.handleOrderValidate)
				r.Post("/", app.handleOrderCreate)
				r.Get("/{id}", app.handleOrderGet)
				r.Post("/{id}/cancel", app.handleOrderCancel)
			})
			r.Route("/mcp/v1/trades", func(r chi.Router) {
				r.Use(app.tradingGate)
				r.Post("/validate", app.handleMCPOrderValidate)
				r.Post("/create", app.handleMCPOrderCreate)
				r.Get("/{id}", app.handleOrderGet)
				r.Post("/{id}/cancel", app.handleOrderCancel)
			})
		}
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	_ = srv.Shutdown(context.Background())
}

type app struct {
	cfg        config.PortfolioAPI
	tradingCfg config.Trading
	repo       portfolio.Repository
	tradeRepo  *tradepg.Repository
	tradeSvc   *trading.Service
	metrics    *trading.Metrics
	dc         *discord.Client
	log        *slog.Logger
}
