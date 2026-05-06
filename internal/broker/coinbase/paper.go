package coinbase

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

// PaperBroker simulates deterministic Coinbase execution for paper trading.
type PaperBroker struct {
	mu     sync.Mutex
	next   int64
	mode   string
	orders map[string]broker.Order
}

// NewPaperExecution returns a paper execution broker.
func NewPaperExecution() broker.ExecutionBroker {
	return &PaperBroker{
		mode:   paperMode(),
		orders: map[string]broker.Order{},
	}
}

func paperMode() string {
	v := strings.TrimSpace(os.Getenv("COINBASE_PAPER_FILL_MODE"))
	if v == "" {
		return "fill"
	}
	return strings.ToLower(v)
}

// PlaceOrder records a deterministic simulated order.
func (p *PaperBroker) PlaceOrder(ctx context.Context, intent broker.OrderIntent) (*broker.Order, error) {
	_ = ctx
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next++
	id := fmt.Sprintf("paper-%06d", p.next)
	status := "filled"
	switch p.mode {
	case "accept":
		status = "accepted"
	case "partial":
		status = "partially_filled"
	case "reject":
		status = "rejected"
	}
	order := broker.Order{
		ID:        id,
		Symbol:    intent.Symbol,
		Status:    status,
		CreatedAt: time.Now().UTC(),
	}
	p.orders[id] = order
	return &order, nil
}

// CancelOrder marks an existing order canceled if it is not final.
func (p *PaperBroker) CancelOrder(ctx context.Context, orderID string) error {
	_ = ctx
	p.mu.Lock()
	defer p.mu.Unlock()
	order, ok := p.orders[orderID]
	if !ok {
		return fmt.Errorf("coinbase paper: order %s not found", orderID)
	}
	if order.Status == "filled" || order.Status == "canceled" || order.Status == "rejected" {
		return nil
	}
	order.Status = "canceled"
	p.orders[orderID] = order
	return nil
}

var _ broker.ExecutionBroker = (*PaperBroker)(nil)
