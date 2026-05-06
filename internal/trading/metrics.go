package trading

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Metrics stores a small Prometheus-compatible snapshot for trading flows.
type Metrics struct {
	mu                    sync.Mutex
	orderCreatesTotal     uint64
	orderRejectsTotal     uint64
	placementCount        uint64
	placementLatencyTotal time.Duration
	reconLagCount         uint64
	reconLagTotal         time.Duration
}

// IncOrderCreate increments the order create counter.
func (m *Metrics) IncOrderCreate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orderCreatesTotal++
}

// IncOrderReject increments the order reject counter.
func (m *Metrics) IncOrderReject() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orderRejectsTotal++
}

// ObservePlacementLatency records placement latency.
func (m *Metrics) ObservePlacementLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.placementCount++
	m.placementLatencyTotal += d
}

// ObserveReconciliationLag records reconciliation lag.
func (m *Metrics) ObserveReconciliationLag(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconLagCount++
	m.reconLagTotal += d
}

// ServeHTTP emits a Prometheus-compatible text snapshot.
func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r
	m.mu.Lock()
	defer m.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "# HELP trading_order_creates_total Total trading order create requests.\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_order_creates_total counter\ntrading_order_creates_total %d\n", m.orderCreatesTotal)
	_, _ = fmt.Fprintf(w, "# HELP trading_order_rejects_total Total rejected trading orders.\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_order_rejects_total counter\ntrading_order_rejects_total %d\n", m.orderRejectsTotal)
	_, _ = fmt.Fprintf(w, "# HELP trading_placement_latency_seconds Order placement latency.\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_placement_latency_seconds summary\ntrading_placement_latency_seconds_sum %.6f\ntrading_placement_latency_seconds_count %d\n", m.placementLatencyTotal.Seconds(), m.placementCount)
	_, _ = fmt.Fprintf(w, "# HELP trading_reconciliation_lag_seconds Reconciliation lag for open orders.\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_reconciliation_lag_seconds summary\ntrading_reconciliation_lag_seconds_sum %.6f\ntrading_reconciliation_lag_seconds_count %d\n", m.reconLagTotal.Seconds(), m.reconLagCount)
}
