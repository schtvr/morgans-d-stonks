package auth

import (
	"crypto/rand"
	"encoding/hex"
)

// NewSessionToken returns a random opaque session token.
func NewSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
