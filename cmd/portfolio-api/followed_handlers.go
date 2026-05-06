package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
	sigpkg "github.com/schtvr/morgans-d-stonks/internal/signal"
)

func (a *app) handleFollowedSymbolsList(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListFollowedSymbols(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, portfolio.FollowedSymbolsResponse{Symbols: items})
}

func (a *app) handleFollowedSymbolsAdd(w http.ResponseWriter, r *http.Request) {
	symbol, err := decodeFollowedSymbolRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.repo.UpsertFollowedSymbol(r.Context(), symbol, "manual"); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if a.log != nil {
		a.log.Info("followed_symbol_add", "symbol", symbol)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) handleFollowedSymbolRemove(w http.ResponseWriter, r *http.Request) {
	symbol := normalizeFollowedSymbol(chi.URLParam(r, "symbol"))
	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}
	if err := a.repo.RemoveFollowedSymbol(r.Context(), symbol); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if a.log != nil {
		a.log.Info("followed_symbol_remove", "symbol", symbol)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) handleInternalFollowedSymbols(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListFollowedSymbols(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, portfolio.FollowedSymbolsResponse{Symbols: items})
}

func (a *app) handleAlertSettingsGet(w http.ResponseWriter, r *http.Request) {
	settings, err := a.repo.GetSignalSettings(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *app) handleAlertSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSignalSettingsRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.repo.UpdateSignalSettings(r.Context(), req); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	settings := portfolio.SignalSettings{
		MoveThresholdPct: req.MoveThresholdPct,
		Cooldown:         req.Cooldown,
		UpdatedAt:        time.Now().UTC(),
	}
	if a.log != nil {
		a.log.Info("signal_settings_update", "threshold_pct", req.MoveThresholdPct, "cooldown", req.Cooldown)
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *app) handleInternalSignalSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.repo.GetSignalSettings(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *app) handleRecentAlertsList(w http.ResponseWriter, r *http.Request) {
	limit := parsePositiveInt(r.URL.Query().Get("limit"), 20)
	items, err := a.repo.ListRecentAlerts(r.Context(), limit)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, portfolio.RecentAlertsResponse{Alerts: items})
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
	}, nil
}

func decodeFollowedSymbolRequest(r *http.Request) (string, error) {
	var req portfolio.FollowedSymbolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", err
	}
	symbol := normalizeFollowedSymbol(req.Symbol)
	if symbol == "" {
		return "", errors.New("symbol is required")
	}
	return symbol, nil
}

func normalizeFollowedSymbol(symbol string) string {
	return coinbase.CanonicalToProviderSymbol(strings.TrimSpace(symbol))
}

func decodeSignalSettingsRequest(r *http.Request) (portfolio.SignalSettingsRequest, error) {
	var req portfolio.SignalSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return portfolio.SignalSettingsRequest{}, err
	}
	req.Cooldown = strings.TrimSpace(req.Cooldown)
	if req.MoveThresholdPct <= 0 {
		return portfolio.SignalSettingsRequest{}, errors.New("moveThresholdPct must be greater than 0")
	}
	if req.Cooldown == "" {
		return portfolio.SignalSettingsRequest{}, errors.New("cooldown is required")
	}
	if _, err := time.ParseDuration(req.Cooldown); err != nil {
		return portfolio.SignalSettingsRequest{}, fmt.Errorf("cooldown must be a valid duration: %w", err)
	}
	return req, nil
}

func parsePositiveInt(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
