package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInternalKeyMiddleware_rejectsWrongKey(t *testing.T) {
	t.Parallel()
	h := InternalKeyMiddleware("correct")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Internal-Key", "wrong")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestInternalKeyMiddleware_acceptsMatchingKey(t *testing.T) {
	t.Parallel()
	h := InternalKeyMiddleware("secret-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Internal-Key", "secret-key")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusTeapot {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestConstantTimeStringEqual_lengthMismatch(t *testing.T) {
	t.Parallel()
	if constantTimeStringEqual("a", "ab") {
		t.Fatal("expected false")
	}
	if constantTimeStringEqual("ab", "a") {
		t.Fatal("expected false")
	}
}
