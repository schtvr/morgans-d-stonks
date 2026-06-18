package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// WorkerConfig wires all dependencies for the worker pool.
type WorkerConfig struct {
	Provider        Provider
	Concurrency     int
	CostTracker     *CostTracker
	PortfolioAPIURL string
	InternalAPIKey  string
	Log             *slog.Logger

	// Trade execution — only active when TradeEnabled=true.
	// All other Trade* fields are ignored when TradeEnabled=false.
	TradeEnabled         bool
	MinTradeConfidence   float64 // minimum confidence to submit; defaults to 0.70
	DefaultTradeNotional float64 // USD fallback when agent omits sizingHintNotional; 0 = skip
}

// Worker is a bounded goroutine pool that consumes DecisionRequests, calls the
// Provider, and POSTs the result to portfolio-api.
type Worker struct {
	cfg     WorkerConfig
	ch      chan DecisionRequest
	wg      sync.WaitGroup
	mutexes sync.Map // key string -> *sync.Mutex
}

// NewWorker creates a Worker. Call Start to begin processing.
func NewWorker(cfg WorkerConfig) *Worker {
	cap := cfg.Concurrency * 4
	if cap < 4 {
		cap = 4
	}
	return &Worker{
		cfg: cfg,
		ch:  make(chan DecisionRequest, cap),
	}
}

// Start launches the worker goroutines. ctx controls the lifetime.
func (w *Worker) Start(ctx context.Context) {
	for i := 0; i < w.cfg.Concurrency; i++ {
		w.wg.Add(1)
		go w.run(ctx)
	}
}

// Stop drains the channel and waits for all workers to exit.
func (w *Worker) Stop() {
	close(w.ch)
	w.wg.Wait()
}

// Enqueue submits a request to the worker pool. If the channel is full the
// request is dropped with a warning (no retry in v0).
func (w *Worker) Enqueue(req DecisionRequest) {
	select {
	case w.ch <- req:
	default:
		w.cfg.Log.Warn("agent_worker: channel full, dropping request",
			"idempotency_key", req.IdempotencyKey)
	}
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()
	for {
		select {
		case req, ok := <-w.ch:
			if !ok {
				return
			}
			w.process(ctx, req)
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) process(ctx context.Context, req DecisionRequest) {
	// Per-symbol serialization: acquire symbol mutex before deciding.
	mu := w.mutexFor(req)
	mu.Lock()
	defer mu.Unlock()

	// 1. Check daily cost cap.
	capErr := w.cfg.CostTracker.CheckAndReserve(ctx, 20)
	if capErr != nil {
		synthetic := &Decision{
			Action:    ActionIgnore,
			Rationale: "daily cost cap reached",
			Model:     "(skipped)",
			ToolCalls: []ToolCall{},
		}
		w.cfg.Log.Warn("agent_worker: cost cap reached, emitting synthetic ignore",
			"idempotency_key", req.IdempotencyKey)
		w.post(ctx, req, synthetic) // synthetic ignore — no order submission
		return
	}

	// 2. Call provider with a 60s timeout.
	dCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	decision, err := w.cfg.Provider.Decide(dCtx, req)
	if err != nil {
		w.cfg.Log.Error("agent_worker: provider error, skipping",
			"idempotency_key", req.IdempotencyKey,
			"err", err)
		return
	}

	// 3. POST decision to portfolio-api for persistence + scoring.
	posted := w.post(ctx, req, decision)

	// 4. Submit order when execution is enabled and decision persisted.
	if posted {
		w.maybeSubmitOrder(ctx, req, decision)
	}
}

// mutexFor returns (creating if necessary) the per-symbol mutex for a request.
// The key is the symbol prefix from the signal, or "daily" for daily triggers.
func (w *Worker) mutexFor(req DecisionRequest) *sync.Mutex {
	key := "daily"
	if req.TriggerKind == TriggerSignal && req.Signal != nil {
		key = strings.ToUpper(req.Signal.Symbol)
	}
	v, _ := w.mutexes.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

type decisionPostBody struct {
	Request  DecisionRequest `json:"request"`
	Decision *Decision       `json:"decision"`
}

// post persists the decision to portfolio-api and returns true on success.
func (w *Worker) post(ctx context.Context, req DecisionRequest, decision *Decision) bool {
	body, err := json.Marshal(decisionPostBody{Request: req, Decision: decision})
	if err != nil {
		w.cfg.Log.Error("agent_worker: marshal post body", "err", err)
		return false
	}

	url := strings.TrimRight(w.cfg.PortfolioAPIURL, "/") + "/internal/agent-decisions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		w.cfg.Log.Error("agent_worker: build request", "err", err)
		return false
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Key", w.cfg.InternalAPIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		w.cfg.Log.Error("agent_worker: post decision", "err", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		w.cfg.Log.Warn("agent_worker: unexpected status posting decision",
			"status", resp.StatusCode,
			"idempotency_key", req.IdempotencyKey)
		return false
	}

	w.cfg.Log.Info("agent_worker: decision posted",
		"idempotency_key", req.IdempotencyKey,
		"action", decision.Action,
		"model", decision.Model,
		"cost_cents", decision.CostCents,
		"latency_ms", decision.LatencyMS,
		"status", resp.StatusCode)

	if err := validateProviderDecision(decision); err != nil {
		w.cfg.Log.Warn("agent_worker: decision validation warning", "err", err)
	}
	return true
}

func validateProviderDecision(d *Decision) error {
	switch d.Action {
	case ActionBuy, ActionSell, ActionIgnore:
	default:
		return fmt.Errorf("unknown action %q", d.Action)
	}
	return nil
}
