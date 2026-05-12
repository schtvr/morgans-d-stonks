package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/schtvr/morgans-d-stonks/internal/trading"
)

// Repository implements trading.Repository using Postgres.
type Repository struct {
	pool *pgxpool.Pool
}

// New connects to Postgres and returns a repository.
func New(ctx context.Context, databaseURL string) (*Repository, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Repository{pool: pool}, nil
}

// RunMigrations applies embedded SQL migrations.
func (r *Repository) RunMigrations(ctx context.Context) error {
	return applyMigrations(ctx, r.pool)
}

// Close releases the pool.
func (r *Repository) Close() {
	r.pool.Close()
}

// CreateOrder inserts a new order row.
func (r *Repository) CreateOrder(ctx context.Context, order trading.Order) error {
	const q = `
INSERT INTO orders (
	id, symbol, side, quantity, limit_price, notional, status, reason, idempotency_key, request_hash, provider, provider_order_id, response_json, created_at, updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14, $15
)`
	_, err := r.pool.Exec(ctx, q,
		order.ID, order.Symbol, string(order.Side), order.Quantity, order.LimitPrice, order.Notional, string(order.Status),
		order.Reason, order.IdempotencyKey, order.RequestHash, order.Provider, order.ProviderOrderID, order.ResponseJSON,
		order.CreatedAt, order.UpdatedAt,
	)
	return err
}

// GetOrder fetches an order by id.
func (r *Repository) GetOrder(ctx context.Context, id string) (*trading.Order, error) {
	const q = `
SELECT id, symbol, side, quantity, limit_price, notional, status, reason, idempotency_key, request_hash, response_json, provider, provider_order_id, created_at, updated_at
FROM orders WHERE id = $1`
	var o trading.Order
	var side, status string
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&o.ID, &o.Symbol, &side, &o.Quantity, &o.LimitPrice, &o.Notional, &status, &o.Reason, &o.IdempotencyKey, &o.RequestHash, &o.ResponseJSON, &o.Provider, &o.ProviderOrderID, &o.CreatedAt, &o.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, trading.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	o.Side = trading.OrderSide(side)
	o.Status = trading.OrderStatus(status)
	return &o, nil
}

// GetOrderByIdempotencyKey fetches a replayable order.
func (r *Repository) GetOrderByIdempotencyKey(ctx context.Context, key string) (*trading.Order, error) {
	const q = `
SELECT id, symbol, side, quantity, limit_price, notional, status, reason, idempotency_key, request_hash, response_json, provider, provider_order_id, created_at, updated_at
FROM orders WHERE idempotency_key = $1`
	var o trading.Order
	var side, status string
	err := r.pool.QueryRow(ctx, q, key).Scan(
		&o.ID, &o.Symbol, &side, &o.Quantity, &o.LimitPrice, &o.Notional, &status, &o.Reason, &o.IdempotencyKey, &o.RequestHash, &o.ResponseJSON, &o.Provider, &o.ProviderOrderID, &o.CreatedAt, &o.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, trading.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	o.Side = trading.OrderSide(side)
	o.Status = trading.OrderStatus(status)
	return &o, nil
}

// ListOpenOrders returns orders still eligible for reconciliation.
func (r *Repository) ListOpenOrders(ctx context.Context) ([]trading.Order, error) {
	const q = `
SELECT id, symbol, side, quantity, limit_price, notional, status, reason, idempotency_key, request_hash, response_json, provider, provider_order_id, created_at, updated_at
FROM orders WHERE status IN ('new','accepted','partially_filled') ORDER BY created_at ASC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]trading.Order, 0)
	for rows.Next() {
		var o trading.Order
		var side, status string
		if err := rows.Scan(&o.ID, &o.Symbol, &side, &o.Quantity, &o.LimitPrice, &o.Notional, &status, &o.Reason, &o.IdempotencyKey, &o.RequestHash, &o.ResponseJSON, &o.Provider, &o.ProviderOrderID, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		o.Side = trading.OrderSide(side)
		o.Status = trading.OrderStatus(status)
		out = append(out, o)
	}
	return out, rows.Err()
}

// UpdateOrderStatus updates the mutable order row state.
func (r *Repository) UpdateOrderStatus(ctx context.Context, id string, status trading.OrderStatus, reason string, updatedAt time.Time) error {
	const q = `UPDATE orders SET status = $2, reason = $3, updated_at = $4 WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id, string(status), reason, updatedAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return trading.ErrOrderNotFound
	}
	return nil
}

// SetProviderOrderID stores the broker-side order id.
func (r *Repository) SetProviderOrderID(ctx context.Context, id string, providerOrderID string) error {
	const q = `UPDATE orders SET provider_order_id = $2 WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id, providerOrderID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return trading.ErrOrderNotFound
	}
	return nil
}

// SaveResponseJSON persists a replay-safe response blob.
func (r *Repository) SaveResponseJSON(ctx context.Context, id string, payload []byte) error {
	const q = `UPDATE orders SET response_json = $2::jsonb WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id, payload)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return trading.ErrOrderNotFound
	}
	return nil
}

// AppendOrderEvent inserts an immutable event row.
func (r *Repository) AppendOrderEvent(ctx context.Context, event trading.OrderEvent) error {
	const q = `
INSERT INTO order_events (order_id, event_type, from_status, to_status, reason, payload, created_at)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)`
	_, err := r.pool.Exec(ctx, q, event.OrderID, event.EventType, string(event.FromStatus), string(event.ToStatus), event.Reason, event.Payload, event.CreatedAt)
	return err
}

// RecordFill appends a fill row.
func (r *Repository) RecordFill(ctx context.Context, fill trading.Fill) error {
	const q = `
INSERT INTO fills (order_id, price, quantity, fee, currency, executed_at)
VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.pool.Exec(ctx, q, fill.OrderID, fill.Price, fill.Quantity, fill.Fee, fill.Currency, fill.ExecutedAt)
	return err
}

// RecordReconciliation appends a reconciliation row.
func (r *Repository) RecordReconciliation(ctx context.Context, rec trading.Reconciliation) error {
	const q = `
INSERT INTO reconciliation (order_id, expected_status, observed_status, drift, action, details, checked_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.pool.Exec(ctx, q, rec.OrderID, string(rec.ExpectedStatus), string(rec.ObservedStatus), rec.Drift, rec.Action, rec.Details, rec.CheckedAt)
	return err
}

var _ trading.Repository = (*Repository)(nil)
