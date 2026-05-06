package trading

import (
	"context"
	"testing"
	"time"
)

type serviceRepo struct {
	orders map[string]Order
}

func (r *serviceRepo) RunMigrations(context.Context) error { return nil }
func (r *serviceRepo) Close()                              {}
func (r *serviceRepo) CreateOrder(_ context.Context, order Order) error {
	if r.orders == nil {
		r.orders = map[string]Order{}
	}
	r.orders[order.ID] = order
	return nil
}
func (r *serviceRepo) GetOrder(_ context.Context, id string) (*Order, error) {
	if o, ok := r.orders[id]; ok {
		return &o, nil
	}
	return nil, ErrOrderNotFound
}
func (r *serviceRepo) GetOrderByIdempotencyKey(_ context.Context, key string) (*Order, error) {
	for _, o := range r.orders {
		if o.IdempotencyKey == key {
			copy := o
			return &copy, nil
		}
	}
	return nil, ErrOrderNotFound
}
func (r *serviceRepo) ListOpenOrders(context.Context) ([]Order, error) { return nil, nil }
func (r *serviceRepo) UpdateOrderStatus(context.Context, string, OrderStatus, string, time.Time) error {
	return nil
}
func (r *serviceRepo) SetProviderOrderID(context.Context, string, string) error { return nil }
func (r *serviceRepo) SaveResponseJSON(_ context.Context, id string, payload []byte) error {
	o := r.orders[id]
	o.ResponseJSON = append([]byte(nil), payload...)
	r.orders[id] = o
	return nil
}
func (r *serviceRepo) AppendOrderEvent(context.Context, OrderEvent) error { return nil }
func (r *serviceRepo) RecordFill(context.Context, Fill) error             { return nil }
func (r *serviceRepo) RecordReconciliation(context.Context, Reconciliation) error {
	return nil
}

func TestServiceCreateReplayAndHashValidation(t *testing.T) {
	repo := &serviceRepo{}
	svc := NewService(repo, Policy{MaxNotional: 100, AllowedSymbols: []string{"BTC-USD"}})
	req := OrderRequest{Symbol: "BTC-USD", Side: OrderSideBuy, Quantity: 1, LimitPrice: 50, IdempotencyKey: "abc", RequestHash: "hash-1"}
	resp1, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp1.Order.Status != OrderStatusAccepted {
		t.Fatalf("unexpected first status: %+v", resp1.Order)
	}
	resp2, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Order.ID != resp1.Order.ID {
		t.Fatalf("expected replay to return same order, got %s and %s", resp1.Order.ID, resp2.Order.ID)
	}
	req.RequestHash = "hash-2"
	if _, err := svc.Create(context.Background(), req); err == nil {
		t.Fatal("expected request hash mismatch to fail")
	}
}
