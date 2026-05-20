package coinbase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchProductCandles_parsesResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method %s", r.Method)
		}
		if r.URL.Path != "/api/v3/brokerage/products/BTC-USD/candles" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candles":[
			{"start":"1700000000","low":"1","high":"3","open":"2","close":"2.5","volume":"10"},
			{"start":"1700003600","low":"2","high":"4","open":"2.5","close":"3.5","volume":"11"}
		]}`))
	}))
	defer ts.Close()

	c := NewReadOnly(ts.Client(), ts.URL, "", "")
	since := time.Unix(1699999000, 0).UTC()
	until := time.Unix(1700004000, 0).UTC()
	bars, err := c.FetchProductCandles(context.Background(), "BTC-USD", since, until)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 2 {
		t.Fatalf("len %d", len(bars))
	}
	if bars[0].Close != 2.5 || bars[1].Close != 3.5 {
		t.Fatalf("closes %+v", bars)
	}
}

func TestCandleGranularity(t *testing.T) {
	if g := CandleGranularityForSpan(30 * time.Minute); g != "ONE_MINUTE" {
		t.Fatalf("30m: %s", g)
	}
	if g := CandleGranularityForSpan(48 * time.Hour); g != "ONE_HOUR" {
		t.Fatalf("48h: %s", g)
	}
	if g := CandleGranularityForSpan(10 * 24 * time.Hour); g != "ONE_HOUR" {
		t.Fatalf("10d: %s", g)
	}
	if g := CandleGranularityForSpan(100 * 24 * time.Hour); g != "SIX_HOUR" {
		t.Fatalf("100d: %s", g)
	}
	if g := CandleGranularityForSpan(200 * 24 * time.Hour); g != "ONE_DAY" {
		t.Fatalf("200d: %s", g)
	}
	if g := CandleGranularityForSpan(400 * 24 * time.Hour); g != "ONE_DAY" {
		t.Fatalf("400d: %s", g)
	}
}
