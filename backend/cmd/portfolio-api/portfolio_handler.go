package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

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
