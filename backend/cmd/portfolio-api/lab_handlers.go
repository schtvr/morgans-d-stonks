package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

const labPayloadRetention = 60 * 24 * time.Hour

func (a *app) handleLabOverview(w http.ResponseWriter, r *http.Request) {
	control, err := a.repo.GetLabControlState(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	signals, err := a.repo.ListLabSignalEvents(r.Context(), portfolio.LabSignalFilter{Limit: 8})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	runs, err := a.repo.ListLabOpenClawRuns(r.Context(), portfolio.LabRunFilter{Limit: 8})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, portfolio.LabOverviewResponse{
		Control:       *control,
		RecentSignals: signals,
		RecentRuns:    runs,
	})
}

func (a *app) handleLabSignalsList(w http.ResponseWriter, r *http.Request) {
	filter, err := parseLabSignalFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	items, err := a.repo.ListLabSignalEvents(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, portfolio.LabSignalsResponse{Signals: items})
}

func (a *app) handleLabSignalGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid signal id", http.StatusBadRequest)
		return
	}
	item, err := a.repo.GetLabSignalEvent(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *app) handleLabRunsList(w http.ResponseWriter, r *http.Request) {
	signalID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("signalId")), 10, 64)
	runs, err := a.repo.ListLabOpenClawRuns(r.Context(), portfolio.LabRunFilter{
		Limit:    parsePositiveInt(r.URL.Query().Get("limit"), 50),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		SignalID: signalID,
	})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, portfolio.LabRunsResponse{Runs: runs})
}

func (a *app) handleLabRunGet(w http.ResponseWriter, r *http.Request) {
	run, err := a.repo.GetLabOpenClawRun(r.Context(), chi.URLParam(r, "requestId"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// TODO(SCH-AG1): remove this handler in a follow-up. OpenClaw retry is no longer
// supported — no new runs are enqueued. Returning 410 Gone so the frontend
// surfaces a clear error rather than silently succeeding.
func (a *app) handleLabRunRetry(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "OpenClaw integration removed; retry is no longer supported", http.StatusGone)
}

func (a *app) handleLabOpenClawPause(w http.ResponseWriter, r *http.Request) {
	a.updateLabControl(w, r, func(c portfolio.LabControlState) portfolio.LabControlState {
		c.OpenClawPaused = true
		return c
	}, "OpenClaw forwarding paused")
}

func (a *app) handleLabOpenClawResume(w http.ResponseWriter, r *http.Request) {
	a.updateLabControl(w, r, func(c portfolio.LabControlState) portfolio.LabControlState {
		c.OpenClawPaused = false
		return c
	}, "OpenClaw forwarding resumed")
}

func (a *app) handleLabOpenClawCircuitReset(w http.ResponseWriter, r *http.Request) {
	a.updateLabControl(w, r, func(c portfolio.LabControlState) portfolio.LabControlState {
		c.CircuitOpen = false
		c.CircuitReason = ""
		return c
	}, "OpenClaw circuit reset")
}

func (a *app) updateLabControl(w http.ResponseWriter, r *http.Request, mutate func(portfolio.LabControlState) portfolio.LabControlState, msg string) {
	control, err := a.repo.GetLabControlState(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	next, err := a.repo.UpdateLabControlState(r.Context(), mutate(*control))
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if a.log != nil {
		a.log.Info("lab_control_update", "message", msg, "paused", next.OpenClawPaused, "circuit_open", next.CircuitOpen)
	}
	writeJSON(w, http.StatusOK, portfolio.LabOperationResponse{Control: *next, Message: msg})
}

func (a *app) handleLabNoteCreate(w http.ResponseWriter, r *http.Request) {
	var req portfolio.LabNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.Body == "" {
		http.Error(w, "body is required", http.StatusBadRequest)
		return
	}
	if req.SignalID == nil && req.RequestID == "" {
		http.Error(w, "signalId or requestId is required", http.StatusBadRequest)
		return
	}
	note, err := a.repo.InsertLabNote(r.Context(), req)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

func (a *app) handleLabTelemetry(w http.ResponseWriter, r *http.Request) {
	window := strings.TrimSpace(r.URL.Query().Get("window"))
	if window == "" {
		window = "24h"
	}
	if _, err := time.ParseDuration(window); err != nil {
		http.Error(w, "window must be a valid duration", http.StatusBadRequest)
		return
	}
	points, err := a.repo.ListLabTelemetry(r.Context(), strings.TrimSpace(r.URL.Query().Get("symbol")), window)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, portfolio.LabTelemetryResponse{Points: points})
}

func (a *app) handleSignalSettingsHistory(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListSignalSettingsVersions(r.Context(), parsePositiveInt(r.URL.Query().Get("limit"), 10))
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, portfolio.SignalSettingsHistoryResponse{Versions: items})
}

func (a *app) handleSignalSettingsRevert(w http.ResponseWriter, r *http.Request) {
	var req portfolio.SignalSettingsRevertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.VersionID <= 0 {
		http.Error(w, "versionId is required", http.StatusBadRequest)
		return
	}
	settings, err := a.repo.RevertSignalSettings(r.Context(), req.VersionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *app) recordLabSignal(ctxReq *http.Request, alert portfolio.RecentAlert) {
	event, err := a.repo.InsertLabSignalEvent(ctxReq.Context(), alert)
	if err != nil {
		if a.log != nil {
			a.log.Warn("lab_signal_insert", "err", err, "symbol", alert.Symbol)
		}
		return
	}
	control, err := a.repo.GetLabControlState(ctxReq.Context())
	if err != nil {
		if a.log != nil {
			a.log.Warn("lab_control_get", "err", err)
		}
		return
	}
	if a.log != nil {
		a.log.Debug("lab_signal_recorded", "signal_id", event.ID, "paused", control.OpenClawPaused, "circuit_open", control.CircuitOpen)
	}
	// OpenClaw enqueue removed 2026-05-18 (SCH-AG1). The Agent (internal/agent)
	// handles decisions in-process in the signals service. lab_openclaw_runs
	// table preserved for historical data; no new rows written.
}

func parseLabSignalFilter(r *http.Request) (portfolio.LabSignalFilter, error) {
	filter := portfolio.LabSignalFilter{
		Limit:  parsePositiveInt(r.URL.Query().Get("limit"), 50),
		Symbol: strings.TrimSpace(r.URL.Query().Get("symbol")),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, fmt.Errorf("from must be RFC3339: %w", err)
		}
		filter.From = &t
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, fmt.Errorf("to must be RFC3339: %w", err)
		}
		filter.To = &t
	}
	return filter, nil
}

func labRequestID(signalID int64, attempt int) string {
	return fmt.Sprintf("lab-signal-%d-attempt-%d", signalID, attempt)
}

func hashRaw(raw json.RawMessage) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func timePtr(t time.Time) *time.Time {
	return &t
}
