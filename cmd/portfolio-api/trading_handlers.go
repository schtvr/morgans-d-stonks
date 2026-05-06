package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/schtvr/morgans-d-stonks/internal/trading"
)

func (a *app) tradingGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.tradingCfg.Enabled {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *app) handleOrderValidate(w http.ResponseWriter, r *http.Request) {
	req, err := decodeTradingRequest(r)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	dec, err := a.tradeSvc.Validate(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if a.log != nil {
		a.log.Info("order_validate", "request_id", chimiddleware.GetReqID(r.Context()), "symbol", req.Symbol, "provider", req.Provider, "allowed", dec.Allowed, "reason_codes", strings.Join(dec.ReasonCodes, ","))
	}
	writeJSON(w, http.StatusOK, trading.OrderResponse{Decision: dec})
}

func (a *app) handleOrderCreate(w http.ResponseWriter, r *http.Request) {
	req, err := decodeTradingRequest(r)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = r.Header.Get("X-Idempotency-Key")
	}
	if req.RequestHash == "" {
		req.RequestHash = r.Header.Get("X-Request-Hash")
	}
	if req.RequestHash == "" {
		req.RequestHash = trading.HashRequest(req)
	}
	start := time.Now()
	resp, err := a.tradeSvc.Create(r.Context(), req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "idempotency key reuse") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if a.metrics != nil {
		a.metrics.IncOrderCreate()
		a.metrics.ObservePlacementLatency(time.Since(start))
		if !resp.Decision.Allowed {
			a.metrics.IncOrderReject()
		}
	}
	if a.log != nil {
		a.log.Info("order_create", "request_id", chimiddleware.GetReqID(r.Context()), "order_id", resp.Order.ID, "symbol", resp.Order.Symbol, "status", resp.Order.Status, "idempotency_key", resp.Order.IdempotencyKey)
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (a *app) handleOrderGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := a.tradeSvc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, trading.ErrOrderNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if a.log != nil {
		a.log.Info("order_get", "request_id", chimiddleware.GetReqID(r.Context()), "order_id", id, "status", order.Status)
	}
	writeJSON(w, http.StatusOK, order)
}

func (a *app) handleOrderCancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := a.tradeSvc.Cancel(r.Context(), id)
	if err != nil {
		if errors.Is(err, trading.ErrOrderNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if a.log != nil {
		a.log.Info("order_cancel", "request_id", chimiddleware.GetReqID(r.Context()), "order_id", id, "status", order.Status)
	}
	writeJSON(w, http.StatusOK, order)
}

func (a *app) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if a.metrics == nil {
		http.Error(w, "metrics unavailable", http.StatusServiceUnavailable)
		return
	}
	a.metrics.ServeHTTP(w, r)
}

func decodeTradingRequest(r *http.Request) (trading.OrderRequest, error) {
	var req trading.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return trading.OrderRequest{}, err
	}
	if req.RequestHash == "" {
		req.RequestHash = r.Header.Get("X-Request-Hash")
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = r.Header.Get("X-Idempotency-Key")
	}
	if req.Provider == "" {
		req.Provider = "coinbase"
	}
	if req.ProviderEnv == "" {
		req.ProviderEnv = "paper"
	}
	return req, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
