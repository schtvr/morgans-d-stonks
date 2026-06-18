package portfolio

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// SnapshotRecord is one row from the snapshots table (raw JSON payload).
type SnapshotRecord struct {
	TakenAt time.Time
	Data    []byte
}

// PortfolioHistoryPoint is one observation derived from a stored snapshot.
type PortfolioHistoryPoint struct {
	AsOf       time.Time          `json:"asOf"`
	TotalValue float64            `json:"totalValue"`
	BySymbol   map[string]float64 `json:"bySymbol,omitempty"`
	Currency   string             `json:"currency,omitempty"`
}

// PortfolioHistoryResponse is returned by GET /api/portfolio/history.
type PortfolioHistoryResponse struct {
	Range  string                  `json:"range"`
	From   time.Time               `json:"from"`
	To     time.Time               `json:"to"`
	Points []PortfolioHistoryPoint `json:"points"`
}

// HistoryRangeDurations maps the public range query values to lookback durations.
var HistoryRangeDurations = map[string]time.Duration{
	"1h": time.Hour,
	"1d": 24 * time.Hour,
	"1w": 7 * 24 * time.Hour,
	"1m": 30 * 24 * time.Hour,
	"3m": 90 * 24 * time.Hour,
	"6m": 180 * 24 * time.Hour,
	"1y": 365 * 24 * time.Hour,
}

// ParseHistoryRange returns the duration for a range token, or ok=false.
func ParseHistoryRange(s string) (time.Duration, bool) {
	d, ok := HistoryRangeDurations[s]
	return d, ok
}

// HistoryPointsFromRecords unmarshals snapshot payloads into time series points.
func HistoryPointsFromRecords(rows []SnapshotRecord) ([]PortfolioHistoryPoint, error) {
	out := make([]PortfolioHistoryPoint, 0, len(rows))
	for _, row := range rows {
		var snap IngestSnapshotRequest
		if err := json.Unmarshal(row.Data, &snap); err != nil {
			return nil, fmt.Errorf("snapshot at %v: %w", row.TakenAt, err)
		}
		asOf := snap.TakenAt
		if asOf.IsZero() {
			asOf = row.TakenAt
		}
		pt := PortfolioHistoryPoint{
			AsOf:       asOf,
			TotalValue: snap.Summary.NetLiquidation,
			Currency:   snap.Summary.Currency,
			BySymbol:   make(map[string]float64, len(snap.Positions)),
		}
		for _, p := range snap.Positions {
			if p.Symbol == "" {
				continue
			}
			pt.BySymbol[p.Symbol] = p.MarketValue
		}
		if pt.Currency == "" {
			for _, p := range snap.Positions {
				if p.Currency != "" {
					pt.Currency = p.Currency
					break
				}
			}
		}
		out = append(out, pt)
	}
	return out, nil
}

// DownsampleHistoryPoints keeps at most max points, preserving endpoints and spreading samples.
func DownsampleHistoryPoints(pts []PortfolioHistoryPoint, max int) []PortfolioHistoryPoint {
	if max <= 0 || len(pts) <= max {
		return pts
	}
	n := len(pts)
	if max == 1 {
		return []PortfolioHistoryPoint{pts[n-1]}
	}
	idxSet := make(map[int]struct{}, max+2)
	for i := 0; i < max; i++ {
		idx := int(float64(i) * float64(n-1) / float64(max-1))
		idxSet[idx] = struct{}{}
	}
	idxSet[0] = struct{}{}
	idxSet[n-1] = struct{}{}
	keys := make([]int, 0, len(idxSet))
	for k := range idxSet {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	out := make([]PortfolioHistoryPoint, 0, len(keys))
	for _, idx := range keys {
		out = append(out, pts[idx])
	}
	return out
}
