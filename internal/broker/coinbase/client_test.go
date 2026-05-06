package coinbase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestQuotesRetryAndCache(t *testing.T) {
	var productsHits int32
	var spotHits int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/brokerage/products":
			atomic.AddInt32(&productsHits, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"products":[{"product_id":"BTC-USD","base_increment":"0.00000001","quote_increment":"0.01","trading_disabled":false}]}`))
		case "/v2/prices/BTC-USD/spot":
			n := atomic.AddInt32(&spotHits, 1)
			if n == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"amount":"65000.12"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := NewReadOnly(ts.Client(), ts.URL)
	ctx := context.Background()
	quotes, err := c.Quotes(ctx, []string{"BTC"})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 1 || quotes[0].Symbol != "BTC-USD" {
		t.Fatalf("unexpected quotes: %+v", quotes)
	}
	_, err = c.Quotes(ctx, []string{"BTC-USD"})
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&productsHits) != 1 {
		t.Fatalf("expected cached products call once, got %d", productsHits)
	}
	if atomic.LoadInt32(&spotHits) < 2 {
		t.Fatalf("expected spot retries, got %d", spotHits)
	}
}

func TestPositionsAndAccountSummaryComputeMarketValue(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/accounts":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accounts":[{"currency":"BTC","available_balance":"2"},{"currency":"USD","available_balance":"10"}]}`))
		case "/api/v3/brokerage/products":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"products":[{"product_id":"BTC-USD","base_increment":"0.00000001","quote_increment":"0.01","trading_disabled":false},{"product_id":"USD-USD","base_increment":"0.01","quote_increment":"0.01","trading_disabled":false}]}`))
		case "/v2/prices/BTC-USD/spot":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"amount":"100"}}`))
		case "/v2/prices/USD-USD/spot":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"amount":"1"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := NewReadOnly(ts.Client(), ts.URL)
	ctx := context.Background()

	positions, err := c.Positions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(positions))
	}
	if positions[0].Symbol != "BTC-USD" || positions[0].MarketValue != 200 {
		t.Fatalf("unexpected BTC position: %+v", positions[0])
	}
	if positions[1].Symbol != "USD-USD" || positions[1].MarketValue != 10 {
		t.Fatalf("unexpected USD position: %+v", positions[1])
	}

	summary, err := c.AccountSummary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if summary.NetLiquidation != 210 || summary.TotalCash != 210 || summary.BuyingPower != 210 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}
