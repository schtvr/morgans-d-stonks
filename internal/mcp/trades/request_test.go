package trades

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestDecodeAndMap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "valid quote size", body: `{"schema_version":"v1","idempotency_key":"abc","order":{"product_id":"btc-usd","side":"buy","type":"market","quote_size":100}}`},
		{name: "valid base size", body: `{"schema_version":"v1","idempotency_key":"abc","order":{"product_id":"ETH-USD","side":"sell","type":"market","base_size":0.5}}`},
		{name: "missing schema", body: `{"order":{"product_id":"BTC-USD","side":"buy","type":"market","quote_size":100}}`, wantErr: true},
		{name: "bad order type", body: `{"schema_version":"v1","order":{"product_id":"BTC-USD","side":"buy","type":"limit","quote_size":100}}`, wantErr: true},
		{name: "both sizes", body: `{"schema_version":"v1","order":{"product_id":"BTC-USD","side":"buy","type":"market","quote_size":100,"base_size":1}}`, wantErr: true},
		{name: "no size", body: `{"schema_version":"v1","order":{"product_id":"BTC-USD","side":"buy","type":"market"}}`, wantErr: true},
		{name: "missing product", body: `{"schema_version":"v1","order":{"side":"buy","type":"market","quote_size":10}}`, wantErr: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest("POST", "/mcp/v1/trades/create", bytes.NewBufferString(tc.body))
			req, err := DecodeAndMap(r, DefaultSchemaVersion)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantErr && req.Symbol == "" {
				t.Fatal("expected symbol")
			}
		})
	}
}
