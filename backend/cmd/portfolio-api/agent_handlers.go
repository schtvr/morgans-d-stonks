package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	agentpkg "github.com/schtvr/morgans-d-stonks/internal/agent"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

// ── internal handlers ─────────────────────────────────────────────────────────

type agentDecisionCreateRequest struct {
	Request  agentpkg.DecisionRequest `json:"request"`
	Decision agentpkg.Decision        `json:"decision"`
}

func (a *app) handleInternalAgentDecisionCreate(w http.ResponseWriter, r *http.Request) {
	var body agentDecisionCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	symbol := ""
	if body.Request.Signal != nil {
		symbol = body.Request.Signal.Symbol
	}

	reqBytes, err := json.Marshal(body.Request)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	// Strip tool calls from response JSON.
	decisionNoTools := struct {
		Action             agentpkg.Action `json:"action"`
		Confidence         float64         `json:"confidence"`
		Rationale          string          `json:"rationale"`
		SizingHintNotional *float64        `json:"sizingHintNotional,omitempty"`
		Model              string          `json:"model"`
		LatencyMS          int64           `json:"latencyMs"`
		CostCents          int64           `json:"costCents"`
	}{
		Action:             body.Decision.Action,
		Confidence:         body.Decision.Confidence,
		Rationale:          body.Decision.Rationale,
		SizingHintNotional: body.Decision.SizingHintNotional,
		Model:              body.Decision.Model,
		LatencyMS:          body.Decision.LatencyMS,
		CostCents:          body.Decision.CostCents,
	}
	respBytes, err := json.Marshal(decisionNoTools)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	toolsBytes, err := json.Marshal(body.Decision.ToolCalls)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	// Check for existing row by idempotency key first.
	existing, err := a.repo.GetAgentDecisionByIdempotencyKey(r.Context(), body.Request.IdempotencyKey)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusOK, existing)
		return
	}

	d := portfolio.AgentDecision{
		TriggerKind:        string(body.Request.TriggerKind),
		TriggerAt:          body.Request.TriggerAt,
		IdempotencyKey:     body.Request.IdempotencyKey,
		Symbol:             symbol,
		Action:             string(body.Decision.Action),
		Confidence:         body.Decision.Confidence,
		Rationale:          body.Decision.Rationale,
		SizingHintNotional: body.Decision.SizingHintNotional,
		Model:              body.Decision.Model,
		PromptVersion:      body.Request.PromptVersion,
		LatencyMS:          body.Decision.LatencyMS,
		CostCents:          body.Decision.CostCents,
		RequestJSON:        json.RawMessage(reqBytes),
		ResponseJSON:       json.RawMessage(respBytes),
		ToolCallsJSON:      json.RawMessage(toolsBytes),
	}

	row, err := a.repo.InsertAgentDecision(r.Context(), d)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (a *app) handleInternalAgentDecisionsList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := clampInt(parsePositiveInt(q.Get("limit"), 50), 1, 200)
	filter := portfolio.AgentDecisionFilter{
		Symbol: strings.TrimSpace(q.Get("symbol")),
		Limit:  limit,
	}
	if raw := strings.TrimSpace(q.Get("since")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			filter.From = &t
		} else if d, err := time.ParseDuration(raw); err == nil {
			t := time.Now().UTC().Add(-d)
			filter.From = &t
		} else {
			http.Error(w, "invalid since: use RFC3339 or duration like 24h", http.StatusBadRequest)
			return
		}
	}

	items, err := a.repo.ListAgentDecisions(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	// Strip large blobs from list response.
	for i := range items {
		items[i].ToolCallsJSON = nil
	}
	writeJSON(w, http.StatusOK, portfolio.AgentDecisionsResponse{Decisions: items})
}

func (a *app) handleInternalAgentDecisionsCount(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	symbol := strings.TrimSpace(q.Get("symbol"))
	sinceRaw := strings.TrimSpace(q.Get("since"))
	if sinceRaw == "" {
		http.Error(w, "since is required (RFC3339)", http.StatusBadRequest)
		return
	}
	since, err := time.Parse(time.RFC3339, sinceRaw)
	if err != nil {
		http.Error(w, "invalid since: use RFC3339", http.StatusBadRequest)
		return
	}

	count, err := a.repo.CountDecisionsForSymbolSince(r.Context(), symbol, since)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (a *app) handleInternalAgentDecisionOutcomes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	horizon := strings.TrimSpace(q.Get("horizon"))
	if horizon == "" {
		horizon = "14d"
	}
	limit := clampInt(parsePositiveInt(q.Get("limit"), 20), 1, 50)

	filter := portfolio.AgentDecisionOutcomeFilter{
		Horizon: horizon,
	}
	// symbol filter: convert to decision IDs is not in scope; pass as symbol via
	// a separate listing of decisions, but the repo interface takes DecisionIDs.
	// For the internal list-outcomes endpoint we use symbol-scoped decisions then
	// fetch outcomes. If no symbol given we fetch outcomes globally via empty filter.
	symbol := strings.TrimSpace(q.Get("symbol"))
	if symbol != "" {
		decisions, err := a.repo.ListAgentDecisions(r.Context(), portfolio.AgentDecisionFilter{
			Symbol: symbol,
			Limit:  limit * 5,
		})
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		ids := make([]int64, 0, len(decisions))
		for _, d := range decisions {
			ids = append(ids, d.ID)
		}
		filter.DecisionIDs = ids
	}

	outcomes, err := a.repo.ListAgentDecisionOutcomes(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	// Cap to limit.
	if len(outcomes) > limit {
		outcomes = outcomes[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"outcomes": outcomes})
}

func (a *app) handleInternalAgentCostToday(w http.ResponseWriter, r *http.Request) {
	today := time.Now().UTC()
	cents, err := a.repo.SumCostCentsForDay(r.Context(), today)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"day":       today.Format("2006-01-02"),
		"costCents": cents,
	})
}

// ── session-auth handlers ────────────────────────────────────────────────────

func (a *app) handleAgentDecisionsList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := portfolio.AgentDecisionFilter{
		Symbol: strings.TrimSpace(q.Get("symbol")),
		Action: strings.TrimSpace(q.Get("action")),
		Limit:  clampInt(parsePositiveInt(q.Get("limit"), 50), 1, 200),
	}
	if raw := strings.TrimSpace(q.Get("from")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "invalid from: use RFC3339", http.StatusBadRequest)
			return
		}
		filter.From = &t
	}
	if raw := strings.TrimSpace(q.Get("to")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "invalid to: use RFC3339", http.StatusBadRequest)
			return
		}
		filter.To = &t
	}

	items, err := a.repo.ListAgentDecisions(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	// Strip blobs from dashboard list.
	for i := range items {
		items[i].RequestJSON = nil
		items[i].ResponseJSON = nil
		items[i].ToolCallsJSON = nil
	}
	writeJSON(w, http.StatusOK, portfolio.AgentDecisionsResponse{Decisions: items})
}

func (a *app) handleAgentDecisionGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	row, err := a.repo.GetAgentDecision(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (a *app) handleAgentBenchmark(w http.ResponseWriter, r *http.Request) {
	window := strings.TrimSpace(r.URL.Query().Get("window"))
	if window == "" {
		window = "14d"
	}
	days, err := parseDayWindow(window)
	if err != nil {
		http.Error(w, "invalid window: use e.g. 7d, 14d, 30d", http.StatusBadRequest)
		return
	}

	points, err := a.repo.ListBenchmarkDaily(r.Context(), "14d", days)
	if err != nil {
		if a.log != nil {
			a.log.Error("agent benchmark daily", "err", err)
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	// Compute headline as mean of non-nil ExcessReturnPct across all points.
	var sum float64
	var n int
	for _, p := range points {
		if p.DecisionCount > 0 {
			sum += p.ExcessReturnPct
			n++
		}
	}
	var headline float64
	if n > 0 {
		headline = sum / float64(n)
	}

	writeJSON(w, http.StatusOK, portfolio.AgentBenchmarkResponse{
		Window:                window,
		HeadlineExcessPct:     headline,
		Points:                points,
		NoteShadowFeesNotPaid: true,
		NoteWindowIncomplete:  len(points) < days,
	})
}

func (a *app) handleAgentCost(w http.ResponseWriter, r *http.Request) {
	window := strings.TrimSpace(r.URL.Query().Get("window"))
	if window == "" {
		window = "7d"
	}
	days, err := parseDayWindow(window)
	if err != nil {
		http.Error(w, "invalid window: use e.g. 7d, 14d, 30d", http.StatusBadRequest)
		return
	}

	points, err := a.repo.ListAgentCostDaily(r.Context(), days)
	if err != nil {
		if a.log != nil {
			a.log.Error("agent cost daily", "err", err)
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	todayCents, err := a.repo.SumCostCentsForDay(r.Context(), time.Now().UTC())
	if err != nil {
		if a.log != nil {
			a.log.Error("agent cost today", "err", err)
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	// Read cap from env (AGENT_DAILY_COST_CAP_USD × 100).
	var capCents int64
	if raw := strings.TrimSpace(os.Getenv("AGENT_DAILY_COST_CAP_USD")); raw != "" {
		if f, err := strconv.ParseFloat(raw, 64); err == nil {
			capCents = int64(f * 100)
		}
	}

	todayStr := time.Now().UTC().Format("2006-01-02")
	var todayDecisions int
	for _, p := range points {
		if p.Day == todayStr {
			todayDecisions = p.Decisions
			break
		}
	}

	writeJSON(w, http.StatusOK, portfolio.AgentCostResponse{
		Window:   window,
		CapCents: capCents,
		Today: portfolio.AgentCostPoint{
			Day:       todayStr,
			CostCents: todayCents,
			Decisions: todayDecisions,
		},
		Points: points,
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseDayWindow(window string) (int, error) {
	window = strings.TrimSuffix(window, "d")
	n, err := strconv.Atoi(window)
	if err != nil || n <= 0 {
		return 0, errors.New("invalid window")
	}
	return n, nil
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
