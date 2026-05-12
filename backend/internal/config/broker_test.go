package config

import "testing"

func TestBrokerValidate(t *testing.T) {
	if err := (Broker{Provider: "bad"}).Validate(); err == nil {
		t.Fatal("expected unknown provider error")
	}
	if err := (Broker{Provider: "coinbase"}).Validate(); err == nil {
		t.Fatal("expected missing coinbase creds error")
	}
	if err := (Broker{Provider: "coinbase", CoinbaseReadAPIKey: "k", CoinbaseReadAPISecret: "s"}).Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
