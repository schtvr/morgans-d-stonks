package coinbase

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CandleBar is one OHLCV bucket from GET /api/v3/brokerage/products/{product_id}/candles.
type CandleBar struct {
	Start  time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// CandleGranularityForSpan returns the Coinbase API granularity string used for a lookback span.
func CandleGranularityForSpan(span time.Duration) string {
	g, _ := candleGranularity(span)
	return g
}

// candleGranularity maps lookback span to Coinbase granularity and bucket width.
func candleGranularity(span time.Duration) (gran string, bucket time.Duration) {
	switch {
	case span <= time.Hour+time.Minute:
		return "ONE_MINUTE", time.Minute
	case span <= 36*time.Hour:
		return "FIVE_MINUTE", 5 * time.Minute
	case span <= 21*24*time.Hour:
		return "ONE_HOUR", time.Hour
	case span <= 120*24*time.Hour:
		return "SIX_HOUR", 6 * time.Hour
	default:
		return "ONE_DAY", 24 * time.Hour
	}
}

const maxCandlesPerRequest = 350

// FetchProductCandles returns candles for [since, until] (UTC), ascending by time.
// Coinbase caps each response; this function issues chunked requests when needed.
func (c *Client) FetchProductCandles(ctx context.Context, productID string, since, until time.Time) ([]CandleBar, error) {
	pid := strings.ToUpper(strings.TrimSpace(productID))
	if pid == "" {
		return nil, fmt.Errorf("coinbase candles: empty product_id")
	}
	if until.Before(since) {
		return nil, fmt.Errorf("coinbase candles: until before since")
	}
	gran, bucket := candleGranularity(until.Sub(since))
	maxSpan := time.Duration(maxCandlesPerRequest) * bucket

	byStart := make(map[int64]CandleBar)
	for chunkStart := since; chunkStart.Before(until); {
		chunkEnd := chunkStart.Add(maxSpan)
		if chunkEnd.After(until) {
			chunkEnd = until
		}
		bars, err := c.fetchProductCandlesOnce(ctx, pid, gran, chunkStart, chunkEnd)
		if err != nil {
			return nil, err
		}
		for _, b := range bars {
			byStart[b.Start.Unix()] = b
		}
		if !chunkEnd.Before(until) {
			break
		}
		chunkStart = chunkEnd.Add(time.Nanosecond)
	}
	out := make([]CandleBar, 0, len(byStart))
	for _, b := range byStart {
		if !b.Start.Before(since) && !b.Start.After(until) {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}

func (c *Client) fetchProductCandlesOnce(ctx context.Context, productID, granularity string, start, end time.Time) ([]CandleBar, error) {
	if end.Before(start) {
		return nil, nil
	}
	q := url.Values{}
	q.Set("start", strconv.FormatInt(start.Unix(), 10))
	q.Set("end", strconv.FormatInt(end.Unix(), 10))
	q.Set("granularity", granularity)
	path := "/api/v3/brokerage/products/" + url.PathEscape(productID) + "/candles?" + q.Encode()

	var resp struct {
		Candles []struct {
			Start  string `json:"start"`
			Low    string `json:"low"`
			High   string `json:"high"`
			Open   string `json:"open"`
			Close  string `json:"close"`
			Volume string `json:"volume"`
		} `json:"candles"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]CandleBar, 0, len(resp.Candles))
	for _, row := range resp.Candles {
		ts, err := parseCandleStart(row.Start)
		if err != nil {
			continue
		}
		out = append(out, CandleBar{
			Start:  ts,
			Open:   parseFloat(row.Open),
			High:   parseFloat(row.High),
			Low:    parseFloat(row.Low),
			Close:  parseFloat(row.Close),
			Volume: parseFloat(row.Volume),
		})
	}
	return out, nil
}

func parseCandleStart(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty candle start")
	}
	if sec, err := strconv.ParseInt(s, 10, 64); err == nil {
		if sec > 1_000_000_000_000 {
			return time.UnixMilli(sec).UTC(), nil
		}
		return time.Unix(sec, 0).UTC(), nil
	}
	return time.Parse(time.RFC3339, s)
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}
