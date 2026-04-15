package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

type ctxKey string

const userCtx ctxKey = "authUser"

// UserFromContext returns the authenticated username, if any.
func UserFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(userCtx)
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// SessionMiddleware validates opaque session tokens (Authorization: Bearer or Cookie session=).
func SessionMiddleware(repo portfolio.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			user, err := repo.SessionUser(r.Context(), token)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userCtx, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	c, err := r.Cookie("session")
	if err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}

// InternalKeyMiddleware validates X-Internal-Key for service-to-service routes.
func InternalKeyMiddleware(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Internal-Key") != key {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
