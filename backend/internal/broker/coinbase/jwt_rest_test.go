package coinbase

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestSignCoinbaseAppRESTJWT_ES256(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))

	s, err := signCoinbaseAppRESTJWT("11111111-1111-1111-1111-111111111111", pemStr, "GET", "api.coinbase.com", "/api/v3/brokerage/accounts")
	if err != nil {
		t.Fatal(err)
	}
	if c := strings.Count(s, "."); c != 2 {
		t.Fatalf("expected JWT, got %q", s)
	}
}

func TestSignCoinbaseAppRESTJWT_ES256_oneLinePEMWithBackslashN(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pemMulti := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
	oneLine := strings.ReplaceAll(pemMulti, "\n", `\n`)

	s, err := signCoinbaseAppRESTJWT("11111111-1111-1111-1111-111111111111", oneLine, "GET", "api.coinbase.com", "/api/v3/brokerage/accounts")
	if err != nil {
		t.Fatal(err)
	}
	if c := strings.Count(s, "."); c != 2 {
		t.Fatalf("expected JWT, got %q", s)
	}
}
