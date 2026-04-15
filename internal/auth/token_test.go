package auth

import "testing"

func TestNewSessionToken(t *testing.T) {
	tok, err := NewSessionToken()
	if err != nil || len(tok) < 16 {
		t.Fatalf("token %q %v", tok, err)
	}
}
