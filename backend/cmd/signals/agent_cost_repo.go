package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpCostRepo implements agent.CostRepo by calling the portfolio-api internal endpoint.
type httpCostRepo struct {
	hc      *http.Client
	baseURL string
	apiKey  string
}

type costTodayResponse struct {
	Day       string `json:"day"`
	CostCents int64  `json:"costCents"`
}

// SumCostCentsForDay calls GET /internal/agent-cost/today.
// The day parameter is ignored — the endpoint always returns today's total.
func (r *httpCostRepo) SumCostCentsForDay(ctx context.Context, _ time.Time) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/internal/agent-cost/today", nil)
	if err != nil {
		return 0, fmt.Errorf("agent cost: build request: %w", err)
	}
	req.Header.Set("X-Internal-Key", r.apiKey)

	resp, err := r.hc.Do(req)
	if err != nil {
		return 0, fmt.Errorf("agent cost: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("agent cost: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("agent cost: unexpected status=%d", resp.StatusCode)
	}

	var res costTodayResponse
	if err := json.Unmarshal(b, &res); err != nil {
		return 0, fmt.Errorf("agent cost: parse response: %w", err)
	}
	return res.CostCents, nil
}
