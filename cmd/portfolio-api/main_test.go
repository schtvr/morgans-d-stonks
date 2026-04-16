package main

import (
	"net/http/httptest"
	"testing"
)

func TestLooksLikeBrowserLogin(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	r.Header.Set("Accept", "application/json")
	if looksLikeBrowserLogin(r) {
		t.Fatal("json accept should not be browser login")
	}
	r2 := httptest.NewRequest("POST", "/api/auth/login", nil)
	r2.Header.Set("Accept", "*/*")
	if !looksLikeBrowserLogin(r2) {
		t.Fatal("*/* should be browser login")
	}
	r3 := httptest.NewRequest("POST", "/api/auth/login", nil)
	r3.Header.Set("Accept", "text/html,application/xhtml+xml")
	if !looksLikeBrowserLogin(r3) {
		t.Fatal("html accept should be browser login")
	}
	r4 := httptest.NewRequest("POST", "/api/auth/login", nil)
	r4.Header.Set("Accept", "application/json")
	r4.Header.Set("Sec-Purpose", "prefetch")
	if looksLikeBrowserLogin(r4) {
		t.Fatal("prefetch should not omit token heuristic as browser")
	}
}

func TestIsForwardedHTTPS(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest("GET", "/", nil)
	if isForwardedHTTPS(r) {
		t.Fatal("no headers")
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if !isForwardedHTTPS(r) {
		t.Fatal("x-forwarded-proto")
	}
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Forwarded", `proto=https;host=example.com`)
	if !isForwardedHTTPS(r2) {
		t.Fatal("forwarded header")
	}
}
