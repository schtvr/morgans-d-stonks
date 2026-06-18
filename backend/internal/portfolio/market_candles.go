package portfolio

import (
	"slices"
	"time"
)

// MarketCandlePoint is one candle returned to the dashboard for charting (Coinbase OHLCV).
type MarketCandlePoint struct {
	AsOf   time.Time `json:"asOf"`
	Open   float64   `json:"open"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Close  float64   `json:"close"`
	Volume float64   `json:"volume"`
}

// MarketCandlesResponse is returned by GET /api/market/candles.
type MarketCandlesResponse struct {
	Symbol      string              `json:"symbol"`
	ProductID   string              `json:"productId"`
	Range       string              `json:"range"`
	Granularity string              `json:"granularity"`
	From        time.Time           `json:"from"`
	To          time.Time           `json:"to"`
	Points      []MarketCandlePoint `json:"points"`
}

const MarketCandlesMaxPoints = 480

// DownsampleMarketCandlePoints keeps at most max points, preserving endpoints.
func DownsampleMarketCandlePoints(pts []MarketCandlePoint, max int) []MarketCandlePoint {
	if max <= 0 || len(pts) <= max {
		return pts
	}
	n := len(pts)
	if max == 1 {
		return []MarketCandlePoint{pts[n-1]}
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
	out := make([]MarketCandlePoint, 0, len(keys))
	for _, idx := range keys {
		out = append(out, pts[idx])
	}
	return out
}
