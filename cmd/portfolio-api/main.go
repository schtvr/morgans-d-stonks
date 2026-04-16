package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5"

	"github.com/schtvr/morgans-d-stonks/internal/auth"
	"github.com/schtvr/morgans-d-stonks/internal/config"
	"github.com/schtvr/morgans-d-stonks/internal/logging"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	pgstore "github.com/schtvr/morgans-d-stonks/internal/portfolio/postgres"
)

// REST API for portfolio snapshots and single-user session auth (SCH-18).
func main() {
	cfg := config.LoadPortfolioAPI()
	log := logging.New("portfolio-api")

	if cfg.DatabaseURL == "" {
		log.Error("DATABASE_URL is required")
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

	app := &app{cfg: cfg, repo: repo, log: log}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(logging.AccessLog(log))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/api/health", app.handleHealth)

	r.Post("/api/auth/login", app.handleLogin)
	r.Group(func(r chi.Router) {
		r.Use(auth.SessionMiddleware(repo))
		r.Post("/api/auth/logout", app.handleLogout)
		r.Get("/api/portfolio/positions", app.handlePositions)
		r.Get("/api/portfolio/summary", app.handleSummary)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.InternalKeyMiddleware(cfg.InternalAPIKey))
		r.Post("/internal/snapshots", app.handleInternalSnapshot)
		r.Get("/internal/snapshot/latest", app.handleInternalLatest)
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
	cfg  config.PortfolioAPI
	repo *pgstore.Repository
	log  *slog.Logger
}

func (a *app) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (a *app) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req portfolio.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Username != a.cfg.AuthUsername || req.Password != a.cfg.AuthPassword {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	token, err := auth.NewSessionToken()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	exp := time.Now().Add(a.cfg.SessionTTL)
	if err := a.repo.CreateSession(r.Context(), token, req.Username, exp); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(portfolio.LoginResponse{Token: token})
}

func (a *app) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := extractBearerOrCookie(r)
	if token != "" {
		_ = a.repo.DeleteSession(r.Context(), token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func extractBearerOrCookie(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	c, err := r.Cookie("session")
	if err == nil {
		return c.Value
	}
	return ""
}

func (a *app) handlePositions(w http.ResponseWriter, r *http.Request) {
	takenAt, payload, err := a.repo.LatestSnapshot(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"positions":[]}`))
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var snap portfolio.IngestSnapshotRequest
	if err := json.Unmarshal(payload, &snap); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	resp := portfolio.MapIngestToViews(&snap)
	if resp.AsOf == nil {
		t := takenAt
		resp.AsOf = &t
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (a *app) handleSummary(w http.ResponseWriter, r *http.Request) {
	_, payload, err := a.repo.LatestSnapshot(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var snap portfolio.IngestSnapshotRequest
	if err := json.Unmarshal(payload, &snap); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	resp := portfolio.MapSummary(&snap.Summary, snap.TakenAt)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (a *app) handleInternalSnapshot(w http.ResponseWriter, r *http.Request) {
	var req portfolio.IngestSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	taken := req.TakenAt.UTC().Truncate(time.Minute)
	b, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := a.repo.UpsertSnapshot(r.Context(), taken, b); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) handleInternalLatest(w http.ResponseWriter, r *http.Request) {
	_, payload, err := a.repo.LatestSnapshot(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(payload)
}
