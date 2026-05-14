package coinbase

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoJSONSendsBearerWhenCredentialsSet(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	secret := base64.StdEncoding.EncodeToString(priv)

	var auth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	c := NewReadOnly(ts.Client(), ts.URL, "test-key-id", secret)
	var page struct {
		Data []struct{} `json:"data"`
	}
	if err := c.doJSON(context.Background(), http.MethodGet, "/v2/accounts?limit=100", nil, &page); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(auth, "Bearer ") || len(auth) < 30 {
		t.Fatalf("expected Bearer JWT, got %q", auth)
	}
	parts := strings.Split(auth[len("Bearer "):], ".")
	if len(parts) != 3 {
		t.Fatalf("expected JWT 3 segments")
	}
	// payload is middle segment, base64url
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if !strings.Contains(string(payload), `"uri"`) || strings.Contains(string(payload), `"uris"`) {
		t.Fatalf("expected singular uri claim in payload, got %s", string(payload))
	}
	if !strings.Contains(string(payload), "GET ") || !strings.Contains(string(payload), "/v2/accounts") {
		t.Fatalf("expected uri to contain method and path: %s", string(payload))
	}
	if !strings.Contains(string(payload), "cdp_service") {
		t.Fatalf("expected Ed25519 jwt to include cdp_service aud: %s", string(payload))
	}
}

func TestDoJSONSignsURIWithoutQueryString(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	secret := base64.StdEncoding.EncodeToString(priv)

	var auth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accounts":[],"has_next":false}`))
	}))
	defer ts.Close()

	c := NewReadOnly(ts.Client(), ts.URL, "test-key-id", secret)
	var page struct {
		Accounts []struct{} `json:"accounts"`
	}
	if err := c.doJSON(context.Background(), http.MethodGet, "/api/v3/brokerage/accounts?limit=250", nil, &page); err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(auth[len("Bearer "):], ".")
	if len(parts) != 3 {
		t.Fatalf("expected JWT 3 segments")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if strings.Contains(string(payload), "?limit=250") {
		t.Fatalf("expected signed uri to omit query string, got %s", string(payload))
	}
	if !strings.Contains(string(payload), `"uri":"GET `) || !strings.Contains(string(payload), `/api/v3/brokerage/accounts"`) {
		t.Fatalf("expected signed uri to include method, host, and path: %s", string(payload))
	}
}
