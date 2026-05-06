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
