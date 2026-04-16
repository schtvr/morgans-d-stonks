package logging

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// AccessLog returns chi middleware that emits one JSON log line per request after completion.
// Fields: msg=http_request, method, path (route pattern when available), status, duration_ms, bytes, request_id.
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			path := requestPath(r)
			log.Info("http_request",
				"method", r.Method,
				"path", path,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"bytes", ww.BytesWritten(),
				"request_id", chimiddleware.GetReqID(r.Context()),
			)
		})
	}
}

func requestPath(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	p := r.URL.Path
	if i := strings.IndexByte(p, '?'); i >= 0 {
		return p[:i]
	}
	return p
}
