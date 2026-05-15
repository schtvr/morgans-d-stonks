package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

const portfolioHistoryMaxRows = 8000
const portfolioHistoryMaxPoints = 480

func (a *app) handlePortfolioHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rangeKey := r.URL.Query().Get("range")
	dur, ok := portfolio.ParseHistoryRange(rangeKey)
	if !ok {
		rangeKey = "1w"
		dur = portfolio.HistoryRangeDurations[rangeKey]
	}
	now := time.Now().UTC()
	since := now.Add(-dur)

	rows, err := a.repo.ListSnapshotsSince(r.Context(), since, portfolioHistoryMaxRows)
	if err != nil {
		a.log.Error("list snapshots for history", "err", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	points, err := portfolio.HistoryPointsFromRecords(rows)
	if err != nil {
		a.log.Error("build history points", "err", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	points = portfolio.DownsampleHistoryPoints(points, portfolioHistoryMaxPoints)

	resp := portfolio.PortfolioHistoryResponse{
		Range:  rangeKey,
		From:   since,
		To:     now,
		Points: points,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
