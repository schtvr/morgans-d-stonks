package coinbase

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// signCoinbaseAppRESTJWT builds a JWT for Coinbase App / Advanced Trade REST on api.coinbase.com.
// It follows the Coinbase docs: claim "uri" (singular) = "METHOD host/path?query", headers kid + nonce.
// - ECDSA PEM (ES256): matches Python App API samples (no aud claim).
// - Ed25519 base64 64-byte secret (EdDSA): matches Ruby samples (includes aud cdp_service).
//
// Note: Coinbase documents ES256 as the supported algorithm for many App APIs; Ed25519 may still fail server-side.
func signCoinbaseAppRESTJWT(keyID, keySecret, method, host, path string) (string, error) {
	if keyID == "" || keySecret == "" {
		return "", errors.New("coinbase jwt: missing key id or secret")
	}
	keySecret = normalizeAPIKeySecret(keySecret)
	uri := fmt.Sprintf("%s %s%s", method, host, path)

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("coinbase jwt: nonce: %w", err)
	}
	now := time.Now()
	exp := now.Add(120 * time.Second)
	nonceHex := hex.EncodeToString(nonce)

	if ec := parseECPrivateKeyPEM(keySecret); ec != nil {
		return signJWTES256(keyID, uri, now, exp, nonceHex, ec)
	}

	// Ed25519: Coinbase often ships 64-byte expanded secret as standard base64; sometimes 32-byte seed.
	for _, dec := range []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
	} {
		decoded, err := dec(strings.TrimSpace(keySecret))
		if err != nil {
			continue
		}
		switch len(decoded) {
		case ed25519.SeedSize + ed25519.PublicKeySize:
			pk := ed25519.PrivateKey(decoded)
			return signJWTEd25519(keyID, uri, now, exp, nonceHex, pk)
		case ed25519.SeedSize:
			pk := ed25519.NewKeyFromSeed(decoded)
			return signJWTEd25519(keyID, uri, now, exp, nonceHex, pk)
		}
	}

	return "", fmt.Errorf("coinbase jwt: secret must be ECDSA PEM (ES256) or base64 Ed25519 (%d-byte expanded or %d-byte seed); if using PEM in .env, use real newlines or \\n escapes between lines", ed25519.SeedSize+ed25519.PublicKeySize, ed25519.SeedSize)
}

// normalizeAPIKeySecret fixes common .env / Docker compose quirks (literal \n instead of newlines).
func normalizeAPIKeySecret(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.TrimSpace(s)
	return s
}

func parseECPrivateKeyPEM(pemStr string) *ecdsa.PrivateKey {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil
	}
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil
	}
	return ec
}

func signJWTES256(keyID, uri string, nbf, exp time.Time, nonceHex string, pk *ecdsa.PrivateKey) (string, error) {
	claims := jwt.MapClaims{
		"sub": keyID,
		"iss": "cdp",
		"nbf": nbf.Unix(),
		"iat": nbf.Unix(),
		"exp": exp.Unix(),
		"uri": uri,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = keyID
	tok.Header["nonce"] = nonceHex
	s, err := tok.SignedString(pk)
	if err != nil {
		return "", fmt.Errorf("coinbase jwt ES256: %w", err)
	}
	return s, nil
}

func signJWTEd25519(keyID, uri string, nbf, exp time.Time, nonceHex string, pk ed25519.PrivateKey) (string, error) {
	claims := jwt.MapClaims{
		"sub": keyID,
		"iss": "cdp",
		"nbf": nbf.Unix(),
		"iat": nbf.Unix(),
		"exp": exp.Unix(),
		"uri": uri,
		"aud": []string{"cdp_service"},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = keyID
	tok.Header["nonce"] = nonceHex
	s, err := tok.SignedString(pk)
	if err != nil {
		return "", fmt.Errorf("coinbase jwt EdDSA: %w", err)
	}
	return s, nil
}
