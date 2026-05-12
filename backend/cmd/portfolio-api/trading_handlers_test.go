package main

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestDecodeMCPTradingRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "valid quote size", body: `{"schema_version":"v1","idempotency_key":"abc","order":{"product_id":"BTC-USD","side":"buy","type":"market","quote_size":100}}`},
		{name: "valid base size", body: `{"schema_version":"v1","idempotency_key":"abc","order":{"product_id":"ETH-USD","side":"sell","type":"market","base_size":0.5}}`},
		{name: "missing schema", body: `{"order":{"product_id":"BTC-USD","side":"buy","type":"market","quote_size":100}}`, wantErr: true},
		{name: "bad order type", body: `{"schema_version":"v1","order":{"product_id":"BTC-USD","side":"buy","type":"limit","quote_size":100}}`, wantErr: true},
		{name: "both sizes", body: `{"schema_version":"v1","order":{"product_id":"BTC-USD","side":"buy","type":"market","quote_size":100,"base_size":1}}`, wantErr: true},
		{name: "no size", body: `{"schema_version":"v1","order":{"product_id":"BTC-USD","side":"buy","type":"market"}}`, wantErr: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest("POST", "/mcp/v1/trades/create", bytes.NewBufferString(tc.body))
			_, err := decodeMCPTradingRequest(r, "v1")
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
