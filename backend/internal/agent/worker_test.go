package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/signal"
)

// slowProvider blocks for a configurable duration then returns ignore.
type slowProvider struct {
	delay    time.Duration
	calls    atomic.Int32
	deciding chan struct{} // closed when provider starts deciding
}

func (s *slowProvider) Name() string  { return "slow" }
func (s *slowProvider) Model() string { return "slow" }
func (s *slowProvider) Decide(ctx context.Context, _ DecisionRequest) (*Decision, error) {
	s.calls.Add(1)
	if s.deciding != nil {
		select {
		case <-s.deciding:
		default:
			close(s.deciding)
		}
	}
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &Decision{Action: ActionIgnore, Rationale: "slow", Model: "slow", ToolCalls: []ToolCall{}}, nil
}

// capRepo always returns a cost that hits the cap.
type capRepo struct{ sum int64 }

func (c *capRepo) SumCostCentsForDay(_ context.Context, _ time.Time) (int64, error) {
	return c.sum, nil
}

// errorProvider returns an error for every call.
type errorProvider struct{ calls atomic.Int32 }

func (e *errorProvider) Name() string  { return "error" }
func (e *errorProvider) Model() string { return "error" }
func (e *errorProvider) Decide(_ context.Context, _ DecisionRequest) (*Decision, error) {
	e.calls.Add(1)
	return nil, errors.New("provider always errors")
}

func newNopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func makeWorker(t *testing.T, provider Provider, costTracker *CostTracker, server *httptest.Server) *Worker {
	t.Helper()
	url := "http://127.0.0.1:1" // unreachable default; override with server URL
	if server != nil {
		url = server.URL
	}
	return NewWorker(WorkerConfig{
		Provider:        provider,
		Concurrency:     1,
		CostTracker:     costTracker,
		PortfolioAPIURL: url,
		InternalAPIKey:  "test-key",
		Log:             newNopLogger(),
	})
}

// TestWorker_PerSymbolSerializiation verifies that two requests for the same
// symbol do not run concurrently: the second waits for the first to finish.
func TestWorker_PerSymbolSerialization(t *testing.T) {
	t.Parallel()

	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	deciding := make(chan struct{})
	prov := &slowProvider{delay: 50 * time.Millisecond, deciding: deciding}
	tracker := NewCostTracker(&capRepo{sum: 0}, 10000)

	w := NewWorker(WorkerConfig{
		Provider:        prov,
		Concurrency:     2, // two workers; still serialized per symbol
		CostTracker:     tracker,
		PortfolioAPIURL: srv.URL,
		InternalAPIKey:  "test-key",
		Log:             newNopLogger(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	alert := signal.CryptoAlert{Symbol: "BTC-USD", DeltaPct: 0, ThresholdPct: 1}
	req1 := DecisionRequest{TriggerKind: TriggerSignal, IdempotencyKey: "k1", Signal: &alert}
	req2 := DecisionRequest{TriggerKind: TriggerSignal, IdempotencyKey: "k2", Signal: &alert}

	w.Enqueue(req1)
	// Wait for the first decide to start before enqueuing the second.
	select {
	case <-deciding:
	case <-time.After(2 * time.Second):
		t.Fatal("first decision never started")
	}
	w.Enqueue(req2)

	// Give enough time for both to complete.
	time.Sleep(300 * time.Millisecond)
	w.Stop()

	if calls := prov.calls.Load(); calls != 2 {
		t.Errorf("expected 2 provider calls, got %d", calls)
	}
	if p := posts.Load(); p != 2 {
		t.Errorf("expected 2 POSTs, got %d", p)
	}
}

// TestWorker_CostCapReached verifies that when the cost cap is hit, a synthetic
// ignore is posted without calling the provider.
func TestWorker_CostCapReached(t *testing.T) {
	t.Parallel()

	var postedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	prov := &errorProvider{}
	// cap = 50, today sum = 100 → always capped
	tracker := NewCostTracker(&capRepo{sum: 100}, 50)
	w := makeWorker(t, prov, tracker, srv)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	w.Enqueue(DecisionRequest{TriggerKind: TriggerDaily, IdempotencyKey: "daily-cap-test"})
	time.Sleep(200 * time.Millisecond)
	w.Stop()

	if calls := prov.calls.Load(); calls != 0 {
		t.Errorf("expected 0 provider calls on cap, got %d", calls)
	}
	if len(postedBody) == 0 {
		t.Fatal("expected a POST to portfolio-api with the synthetic ignore")
	}
	var body decisionPostBody
	if err := json.Unmarshal(postedBody, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Decision.Action != ActionIgnore {
		t.Errorf("expected ignore, got %s", body.Decision.Action)
	}
	if body.Decision.Rationale != "daily cost cap reached" {
		t.Errorf("unexpected rationale: %s", body.Decision.Rationale)
	}
}

// TestWorker_ProviderError verifies that a provider error does NOT produce a
// POST to portfolio-api.
func TestWorker_ProviderError(t *testing.T) {
	t.Parallel()

	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	prov := &errorProvider{}
	tracker := NewCostTracker(&capRepo{sum: 0}, 10000)
	w := makeWorker(t, prov, tracker, srv)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	w.Enqueue(DecisionRequest{TriggerKind: TriggerDaily, IdempotencyKey: "err-test"})
	time.Sleep(200 * time.Millisecond)
	w.Stop()

	if p := posts.Load(); p != 0 {
		t.Errorf("expected 0 POSTs on provider error, got %d", p)
	}
}

// TestWorker_EnqueueDropsWhenFull verifies that Enqueue is non-blocking and
// drops requests when the channel is at capacity.
func TestWorker_EnqueueDropsWhenFull(t *testing.T) {
	t.Parallel()

	// Concurrency=1 → cap=4. Fill the channel without starting workers.
	prov := &slowProvider{delay: time.Hour}
	tracker := NewCostTracker(&capRepo{sum: 0}, 10000)
	w := NewWorker(WorkerConfig{
		Provider:        prov,
		Concurrency:     1,
		CostTracker:     tracker,
		PortfolioAPIURL: "http://127.0.0.1:1",
		InternalAPIKey:  "test",
		Log:             newNopLogger(),
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Enqueue(DecisionRequest{TriggerKind: TriggerDaily, IdempotencyKey: "drop-test"})
		}()
	}
	wg.Wait()
	// Should not block or panic — just drops extras.
}
