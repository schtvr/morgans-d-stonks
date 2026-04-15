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
