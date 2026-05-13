package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/schtvr/morgans-d-stonks/internal/auth"
	"github.com/schtvr/morgans-d-stonks/internal/config"
)

func TestTradingOrderRoutesHiddenWhenDisabled(t *testing.T) {
	t.Parallel()
	repo := newFakePortfolioRepo()
	app := &app{
		repo:       repo,
		tradingCfg: config.Trading{Enabled: false},
	}
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.InternalKeyMiddleware("k"))
		if app.tradingCfg.Enabled {
			r.Route("/internal/orders", func(r chi.Router) {
				r.Use(app.tradingGate)
				r.Post("/validate", app.handleOrderValidate)
			})
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/internal/orders/validate", bytes.NewBufferString(`{}`))
	req.Header.Set("X-Internal-Key", "k")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 when trading disabled, got %d", rec.Code)
	}
}
