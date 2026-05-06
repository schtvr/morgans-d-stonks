package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

// Repository implements portfolio.Repository using Postgres.
type Repository struct {
	pool *pgxpool.Pool
}

// New connects and returns a repository.
func New(ctx context.Context, databaseURL string) (*Repository, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	r := &Repository{pool: pool}
	return r, nil
}

// RunMigrations applies embedded SQL migrations.
func (r *Repository) RunMigrations(ctx context.Context) error {
	return applyMigrations(ctx, r.pool)
}

// Close releases the pool.
func (r *Repository) Close() {
	r.pool.Close()
}

// UpsertSnapshot inserts or replaces a snapshot keyed by taken_at (rounded by caller).
func (r *Repository) UpsertSnapshot(ctx context.Context, takenAt time.Time, payload []byte) error {
	const q = `
INSERT INTO snapshots (taken_at, data)
VALUES ($1, $2::jsonb)
ON CONFLICT (taken_at) DO UPDATE SET data = EXCLUDED.data`
	_, err := r.pool.Exec(ctx, q, takenAt, payload)
	return err
}

// LatestSnapshot returns the most recent snapshot.
func (r *Repository) LatestSnapshot(ctx context.Context) (time.Time, []byte, error) {
	const q = `SELECT taken_at, data FROM snapshots ORDER BY taken_at DESC LIMIT 1`
	var takenAt time.Time
	var data []byte
	err := r.pool.QueryRow(ctx, q).Scan(&takenAt, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil, pgx.ErrNoRows
	}
	if err != nil {
		return time.Time{}, nil, err
	}
	return takenAt, data, nil
}

// ListFollowedSymbols returns the current crypto watchlist.
func (r *Repository) ListFollowedSymbols(ctx context.Context) ([]portfolio.FollowedSymbol, error) {
	const q = `
SELECT symbol, source, created_at, updated_at
FROM followed_symbols
ORDER BY created_at ASC, symbol ASC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]portfolio.FollowedSymbol, 0)
	for rows.Next() {
		var item portfolio.FollowedSymbol
		if err := rows.Scan(&item.Symbol, &item.Source, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// UpsertFollowedSymbol adds or refreshes a watched crypto symbol.
func (r *Repository) UpsertFollowedSymbol(ctx context.Context, symbol, source string) error {
	const q = `
INSERT INTO followed_symbols (symbol, source, created_at, updated_at)
VALUES ($1, $2, now(), now())
ON CONFLICT (symbol) DO UPDATE SET
    source = EXCLUDED.source,
    updated_at = now()`
	_, err := r.pool.Exec(ctx, q, symbol, source)
	return err
}

// RemoveFollowedSymbol deletes a watched symbol.
func (r *Repository) RemoveFollowedSymbol(ctx context.Context, symbol string) error {
	const q = `DELETE FROM followed_symbols WHERE symbol = $1`
	_, err := r.pool.Exec(ctx, q, symbol)
	return err
}

// FollowedSymbolsSeeded reports whether the one-time seed has already been applied.
func (r *Repository) FollowedSymbolsSeeded(ctx context.Context) (bool, error) {
	const q = `SELECT seeded_at IS NOT NULL FROM followed_symbol_state WHERE singleton = TRUE`
	var seeded bool
	err := r.pool.QueryRow(ctx, q).Scan(&seeded)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return seeded, err
}

// MarkFollowedSymbolsSeeded records that the one-time seed has been applied.
func (r *Repository) MarkFollowedSymbolsSeeded(ctx context.Context, seededAt time.Time) error {
	const q = `
INSERT INTO followed_symbol_state (singleton, seeded_at, updated_at)
VALUES (TRUE, $1, now())
ON CONFLICT (singleton) DO UPDATE SET
    seeded_at = EXCLUDED.seeded_at,
    updated_at = now()`
	_, err := r.pool.Exec(ctx, q, seededAt)
	return err
}

// GetSignalSettings returns the persisted crypto alert settings.
func (r *Repository) GetSignalSettings(ctx context.Context) (*portfolio.SignalSettings, error) {
	const q = `
SELECT move_threshold_pct, cooldown, updated_at
FROM signal_settings
WHERE singleton = TRUE`
	var settings portfolio.SignalSettings
	if err := r.pool.QueryRow(ctx, q).Scan(&settings.MoveThresholdPct, &settings.Cooldown, &settings.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, err
	}
	return &settings, nil
}

// UpdateSignalSettings overwrites the persisted crypto alert settings.
func (r *Repository) UpdateSignalSettings(ctx context.Context, req portfolio.SignalSettingsRequest) error {
	const q = `
INSERT INTO signal_settings (singleton, move_threshold_pct, cooldown, updated_at)
VALUES (TRUE, $1, $2::interval, now())
ON CONFLICT (singleton) DO UPDATE SET
    move_threshold_pct = EXCLUDED.move_threshold_pct,
    cooldown = EXCLUDED.cooldown,
    updated_at = now()`
	_, err := r.pool.Exec(ctx, q, req.MoveThresholdPct, req.Cooldown)
	return err
}

// ListRecentAlerts returns the newest fired alerts for the dashboard.
func (r *Repository) ListRecentAlerts(ctx context.Context, limit int) ([]portfolio.RecentAlert, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	const q = `
SELECT id, type, symbol, product_id, source, current_price, previous_price, delta_amount, delta_pct, threshold_pct,
       quantity, avg_cost, cost_basis, unrealized_pl, unrealized_pl_pct, fired_at, created_at
FROM recent_alerts
ORDER BY fired_at DESC, id DESC
LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]portfolio.RecentAlert, 0, limit)
	for rows.Next() {
		var item portfolio.RecentAlert
		if err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Symbol,
			&item.ProductID,
			&item.Source,
			&item.CurrentPrice,
			&item.PreviousPrice,
			&item.DeltaAmount,
			&item.DeltaPct,
			&item.ThresholdPct,
			&item.Quantity,
			&item.AvgCost,
			&item.CostBasis,
			&item.UnrealizedPL,
			&item.UnrealizedPLPct,
			&item.FiredAt,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// InsertRecentAlert stores a fired alert for dashboard history.
func (r *Repository) InsertRecentAlert(ctx context.Context, alert portfolio.RecentAlert) error {
	const q = `
INSERT INTO recent_alerts (
    type, symbol, product_id, source, current_price, previous_price, delta_amount, delta_pct, threshold_pct,
    quantity, avg_cost, cost_basis, unrealized_pl, unrealized_pl_pct, fired_at, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, now())`
	_, err := r.pool.Exec(
		ctx,
		q,
		alert.Type,
		alert.Symbol,
		alert.ProductID,
		alert.Source,
		alert.CurrentPrice,
		alert.PreviousPrice,
		alert.DeltaAmount,
		alert.DeltaPct,
		alert.ThresholdPct,
		alert.Quantity,
		alert.AvgCost,
		alert.CostBasis,
		alert.UnrealizedPL,
		alert.UnrealizedPLPct,
		alert.FiredAt,
	)
	return err
}

// CreateSession stores an opaque session token.
func (r *Repository) CreateSession(ctx context.Context, token, username string, expiresAt time.Time) error {
	const q = `INSERT INTO sessions (token, username, expires_at) VALUES ($1, $2, $3)`
	_, err := r.pool.Exec(ctx, q, token, username, expiresAt)
	return err
}

// SessionUser returns the username for a valid, unexpired session.
func (r *Repository) SessionUser(ctx context.Context, token string) (string, error) {
	const q = `SELECT username FROM sessions WHERE token = $1 AND expires_at >= now()`
	var user string
	err := r.pool.QueryRow(ctx, q, token).Scan(&user)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", pgx.ErrNoRows
	}
	return user, err
}

// DeleteSession removes a session (logout).
func (r *Repository) DeleteSession(ctx context.Context, token string) error {
	const q = `DELETE FROM sessions WHERE token = $1`
	_, err := r.pool.Exec(ctx, q, token)
	return err
}

var _ portfolio.Repository = (*Repository)(nil)
