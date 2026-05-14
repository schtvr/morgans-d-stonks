package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	sigpkg "github.com/schtvr/morgans-d-stonks/internal/signal"
)

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
	if err := seedFollowedSymbolsFromPositions(r.Context(), a.repo, req.Positions); err != nil && a.log != nil {
		a.log.Warn("seed followed symbols", "err", err)
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

func (a *app) handleInternalFollowedSymbols(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListFollowedSymbols(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, portfolio.FollowedSymbolsResponse{Symbols: items})
}

func (a *app) handleInternalSignalSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.repo.GetSignalSettings(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusOK, portfolio.SignalSettings{
				MoveThresholdPct: 1.0,
				Cooldown:         "15m",
				UpdatedAt:        time.Now().UTC(),
			})
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *app) handleInternalRecentAlertCreate(w http.ResponseWriter, r *http.Request) {
	alert, err := decodeCryptoAlert(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.repo.InsertRecentAlert(r.Context(), alert); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	a.recordLabSignal(r, alert)
	if a.log != nil {
		a.log.Info("recent_alert_insert", "symbol", alert.Symbol, "delta_pct", alert.DeltaPct, "threshold_pct", alert.ThresholdPct)
	}
	writeJSON(w, http.StatusCreated, alert)
}

func decodeCryptoAlert(r *http.Request) (portfolio.RecentAlert, error) {
	var payload sigpkg.CryptoAlert
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return portfolio.RecentAlert{}, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return portfolio.RecentAlert{}, err
	}
	return portfolio.RecentAlert{
		Type:            payload.Type,
		Symbol:          payload.Symbol,
		ProductID:       payload.ProductID,
		Source:          payload.Source,
		CurrentPrice:    payload.CurrentPrice,
		PreviousPrice:   payload.PreviousPrice,
		DeltaAmount:     payload.DeltaAmount,
		DeltaPct:        payload.DeltaPct,
		ThresholdPct:    payload.ThresholdPct,
		Quantity:        payload.Quantity,
		AvgCost:         payload.AvgCost,
		CostBasis:       payload.CostBasis,
		UnrealizedPL:    payload.UnrealizedPL,
		UnrealizedPLPct: payload.UnrealizedPLPct,
		FiredAt:         payload.FiredAt,
		PayloadJSON:     raw,
	}, nil
}

func seedFollowedSymbolsFromLatestSnapshot(ctx context.Context, repo portfolio.Repository) error {
	seeded, err := repo.FollowedSymbolsSeeded(ctx)
	if err != nil || seeded {
		return err
	}
	_, payload, err := repo.LatestSnapshot(ctx)
	if err != nil {
		return err
	}
	var snap portfolio.IngestSnapshotRequest
	if err := json.Unmarshal(payload, &snap); err != nil {
		return err
	}
	if err := seedFollowedSymbolsFromPositions(ctx, repo, snap.Positions); err != nil {
		return err
	}
	return repo.MarkFollowedSymbolsSeeded(ctx, time.Now().UTC())
}

func seedFollowedSymbolsFromPositions(ctx context.Context, repo portfolio.Repository, positions []broker.Position) error {
	seeded, err := repo.FollowedSymbolsSeeded(ctx)
	if err != nil || seeded {
		return err
	}
	for _, p := range positions {
		symbol := normalizeFollowedSymbol(p.Symbol)
		if symbol == "" {
			continue
		}
		if err := repo.UpsertFollowedSymbol(ctx, symbol, "seeded"); err != nil {
			return err
		}
	}
	return repo.MarkFollowedSymbolsSeeded(ctx, time.Now().UTC())
}
