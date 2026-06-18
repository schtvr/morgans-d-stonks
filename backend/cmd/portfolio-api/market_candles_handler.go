package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

func (a *app) handleMarketCandles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.cb == nil {
		http.Error(w, "market candles require COINBASE_READ_API_KEY and COINBASE_READ_API_SECRET", http.StatusServiceUnavailable)
		return
	}
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	if symbol == "" {
		http.Error(w, "symbol query parameter is required", http.StatusBadRequest)
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

	productID := coinbase.CanonicalToProviderSymbol(symbol)
	if strings.EqualFold(productID, "USD-USD") {
		http.Error(w, "no market candles for USD cash", http.StatusBadRequest)
		return
	}

	bars, err := a.cb.FetchProductCandles(r.Context(), productID, since, now)
	if err != nil {
		a.log.Error("coinbase candles", "symbol", symbol, "product", productID, "err", err)
		http.Error(w, "failed to load candles", http.StatusBadGateway)
		return
	}
	points := make([]portfolio.MarketCandlePoint, 0, len(bars))
	for _, b := range bars {
		points = append(points, portfolio.MarketCandlePoint{
			AsOf:   b.Start,
			Open:   b.Open,
			High:   b.High,
			Low:    b.Low,
			Close:  b.Close,
			Volume: b.Volume,
		})
	}
	points = portfolio.DownsampleMarketCandlePoints(points, portfolio.MarketCandlesMaxPoints)

	resp := portfolio.MarketCandlesResponse{
		Symbol:      symbol,
		ProductID:   productID,
		Range:       rangeKey,
		Granularity: coinbase.CandleGranularityForSpan(dur),
		From:        since,
		To:          now,
		Points:      points,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
