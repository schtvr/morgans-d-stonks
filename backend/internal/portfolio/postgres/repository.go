package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
SELECT move_threshold_pct, extract(epoch from cooldown)::bigint, updated_at
FROM signal_settings
WHERE singleton = TRUE`
	var settings portfolio.SignalSettings
	var cooldownSecs int64
	if err := r.pool.QueryRow(ctx, q).Scan(&settings.MoveThresholdPct, &cooldownSecs, &settings.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, err
	}
	settings.Cooldown = fmt.Sprintf("%ds", cooldownSecs)
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
       quantity, avg_cost, cost_basis, unrealized_pl, unrealized_pl_pct, fired_at, created_at, payload_json
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
			&item.PayloadJSON,
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
    quantity, avg_cost, cost_basis, unrealized_pl, unrealized_pl_pct, fired_at, created_at, payload_json
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, now(), $16::jsonb)`
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
		payloadJSONBytes(alert.PayloadJSON),
	)
	return err
}

// InsertLabSignalEvent records a qualifying crypto signal for The Lab.
func (r *Repository) InsertLabSignalEvent(ctx context.Context, alert portfolio.RecentAlert) (*portfolio.LabSignalEvent, error) {
	const q = `
INSERT INTO lab_signal_events (
    type, symbol, product_id, source, current_price, previous_price, delta_amount, delta_pct, threshold_pct,
    fired_at, payload_json, discord_status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, 'signal_only')
RETURNING id, type, symbol, product_id, source, current_price, previous_price, delta_amount, delta_pct,
          threshold_pct, fired_at, payload_json, discord_status, created_at`
	var item portfolio.LabSignalEvent
	err := r.pool.QueryRow(
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
		alert.FiredAt,
		payloadJSONBytes(alert.PayloadJSON),
	).Scan(
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
		&item.FiredAt,
		&item.PayloadJSON,
		&item.DiscordStatus,
		&item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ListLabSignalEvents returns newest Lab signals with simple filters.
func (r *Repository) ListLabSignalEvents(ctx context.Context, filter portfolio.LabSignalFilter) ([]portfolio.LabSignalEvent, error) {
	limit := clampLimit(filter.Limit, 50, 200)
	const q = `
SELECT id, type, symbol, product_id, source, current_price, previous_price, delta_amount, delta_pct,
       threshold_pct, fired_at, payload_json, discord_status, created_at
FROM lab_signal_events
WHERE ($2 = '' OR symbol = $2)
  AND ($3::timestamptz IS NULL OR fired_at >= $3)
  AND ($4::timestamptz IS NULL OR fired_at <= $4)
ORDER BY fired_at DESC, id DESC
LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit, strings.TrimSpace(filter.Symbol), filter.From, filter.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]portfolio.LabSignalEvent, 0, limit)
	for rows.Next() {
		item, err := scanLabSignalEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// GetLabSignalEvent returns one durable Lab signal.
func (r *Repository) GetLabSignalEvent(ctx context.Context, id int64) (*portfolio.LabSignalEvent, error) {
	const q = `
SELECT id, type, symbol, product_id, source, current_price, previous_price, delta_amount, delta_pct,
       threshold_pct, fired_at, payload_json, discord_status, created_at
FROM lab_signal_events
WHERE id = $1`
	rows, err := r.pool.Query(ctx, q, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, pgx.ErrNoRows
	}
	item, err := scanLabSignalEvent(rows)
	if err != nil {
		return nil, err
	}
	return &item, rows.Err()
}

// UpsertLabOpenClawRun stores a Lab-visible OpenClaw attempt.
func (r *Repository) UpsertLabOpenClawRun(ctx context.Context, run portfolio.LabOpenClawRun) error {
	const q = `
INSERT INTO lab_openclaw_runs (
    request_id, signal_event_id, status, attempts, analysis, recommendation, confidence, tool_names,
    error_text, request_hash, response_hash, full_request_json, full_response_json, started_at, completed_at, updated_at
)
VALUES ($1, $2, $3, GREATEST($4, 1), $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13::jsonb, $14, $15, now())
ON CONFLICT (request_id) DO UPDATE SET
    status = EXCLUDED.status,
    attempts = GREATEST(lab_openclaw_runs.attempts, EXCLUDED.attempts),
    analysis = EXCLUDED.analysis,
    recommendation = EXCLUDED.recommendation,
    confidence = EXCLUDED.confidence,
    tool_names = EXCLUDED.tool_names,
    error_text = EXCLUDED.error_text,
    request_hash = EXCLUDED.request_hash,
    response_hash = EXCLUDED.response_hash,
    full_request_json = COALESCE(EXCLUDED.full_request_json, lab_openclaw_runs.full_request_json),
    full_response_json = COALESCE(EXCLUDED.full_response_json, lab_openclaw_runs.full_response_json),
    started_at = COALESCE(EXCLUDED.started_at, lab_openclaw_runs.started_at),
    completed_at = EXCLUDED.completed_at,
    updated_at = now()`
	_, err := r.pool.Exec(
		ctx,
		q,
		run.RequestID,
		run.SignalID,
		run.Status,
		run.Attempts,
		run.Analysis,
		run.Recommendation,
		run.Confidence,
		run.ToolNames,
		run.ErrorText,
		run.RequestHash,
		run.ResponseHash,
		nullableJSONBytes(run.RequestJSON),
		nullableJSONBytes(run.ResponseJSON),
		run.StartedAt,
		run.CompletedAt,
	)
	return err
}

func (r *Repository) ListLabOpenClawRuns(ctx context.Context, filter portfolio.LabRunFilter) ([]portfolio.LabOpenClawRun, error) {
	limit := clampLimit(filter.Limit, 50, 200)
	const q = `
SELECT request_id, signal_event_id, status, attempts, analysis, recommendation, confidence, tool_names, error_text,
       request_hash, response_hash, full_request_json, full_response_json, started_at, completed_at, created_at, updated_at
FROM lab_openclaw_runs
WHERE ($2 = '' OR status = $2)
  AND ($3 = 0 OR signal_event_id = $3)
ORDER BY updated_at DESC, created_at DESC
LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit, strings.TrimSpace(filter.Status), filter.SignalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]portfolio.LabOpenClawRun, 0, limit)
	for rows.Next() {
		item, err := scanLabOpenClawRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) GetLabOpenClawRun(ctx context.Context, requestID string) (*portfolio.LabOpenClawRun, error) {
	const q = `
SELECT request_id, signal_event_id, status, attempts, analysis, recommendation, confidence, tool_names, error_text,
       request_hash, response_hash, full_request_json, full_response_json, started_at, completed_at, created_at, updated_at
FROM lab_openclaw_runs
WHERE request_id = $1`
	rows, err := r.pool.Query(ctx, q, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, pgx.ErrNoRows
	}
	item, err := scanLabOpenClawRun(rows)
	if err != nil {
		return nil, err
	}
	return &item, rows.Err()
}

func (r *Repository) InsertLabNote(ctx context.Context, note portfolio.LabNoteRequest) (*portfolio.LabNote, error) {
	const q = `
INSERT INTO lab_notes (signal_event_id, request_id, body)
VALUES ($1, NULLIF($2, ''), $3)
RETURNING id, signal_event_id, COALESCE(request_id, ''), body, created_at`
	var out portfolio.LabNote
	err := r.pool.QueryRow(ctx, q, note.SignalID, strings.TrimSpace(note.RequestID), strings.TrimSpace(note.Body)).Scan(
		&out.ID,
		&out.SignalID,
		&out.RequestID,
		&out.Body,
		&out.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Repository) ListLabTelemetry(ctx context.Context, symbol, window string) ([]portfolio.LabTelemetryPoint, error) {
	const q = `
SELECT date_trunc('minute', fired_at) AS bucket, symbol, avg(current_price), avg(delta_pct), avg(threshold_pct), count(*)
FROM lab_signal_events
WHERE fired_at >= now() - $1::interval
  AND ($2 = '' OR symbol = $2)
GROUP BY bucket, symbol
ORDER BY bucket ASC, symbol ASC`
	rows, err := r.pool.Query(ctx, q, window, strings.TrimSpace(symbol))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]portfolio.LabTelemetryPoint, 0)
	for rows.Next() {
		var item portfolio.LabTelemetryPoint
		if err := rows.Scan(&item.Bucket, &item.Symbol, &item.CurrentPrice, &item.DeltaPct, &item.ThresholdPct, &item.SignalCount); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) GetLabControlState(ctx context.Context) (*portfolio.LabControlState, error) {
	const q = `
SELECT openclaw_paused, circuit_open, circuit_reason, updated_at
FROM lab_control_state
WHERE singleton = TRUE`
	var out portfolio.LabControlState
	if err := r.pool.QueryRow(ctx, q).Scan(&out.OpenClawPaused, &out.CircuitOpen, &out.CircuitReason, &out.UpdatedAt); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Repository) UpdateLabControlState(ctx context.Context, control portfolio.LabControlState) (*portfolio.LabControlState, error) {
	const q = `
INSERT INTO lab_control_state (singleton, openclaw_paused, circuit_open, circuit_reason, updated_at)
VALUES (TRUE, $1, $2, $3, now())
ON CONFLICT (singleton) DO UPDATE SET
    openclaw_paused = EXCLUDED.openclaw_paused,
    circuit_open = EXCLUDED.circuit_open,
    circuit_reason = EXCLUDED.circuit_reason,
    updated_at = now()
RETURNING openclaw_paused, circuit_open, circuit_reason, updated_at`
	var out portfolio.LabControlState
	err := r.pool.QueryRow(ctx, q, control.OpenClawPaused, control.CircuitOpen, control.CircuitReason).Scan(
		&out.OpenClawPaused,
		&out.CircuitOpen,
		&out.CircuitReason,
		&out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Repository) InsertSignalSettingsVersion(ctx context.Context, req portfolio.SignalSettingsRequest, reason string) (*portfolio.SignalSettingsVersion, error) {
	const q = `
INSERT INTO signal_settings_versions (move_threshold_pct, cooldown, reason)
VALUES ($1, $2, $3)
RETURNING id, move_threshold_pct, cooldown, reason, created_at`
	var out portfolio.SignalSettingsVersion
	err := r.pool.QueryRow(ctx, q, req.MoveThresholdPct, req.Cooldown, strings.TrimSpace(reason)).Scan(
		&out.ID,
		&out.MoveThresholdPct,
		&out.Cooldown,
		&out.Reason,
		&out.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Repository) ListSignalSettingsVersions(ctx context.Context, limit int) ([]portfolio.SignalSettingsVersion, error) {
	limit = clampLimit(limit, 10, 50)
	const q = `
SELECT id, move_threshold_pct, cooldown, reason, created_at
FROM signal_settings_versions
ORDER BY created_at DESC, id DESC
LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]portfolio.SignalSettingsVersion, 0, limit)
	for rows.Next() {
		var item portfolio.SignalSettingsVersion
		if err := rows.Scan(&item.ID, &item.MoveThresholdPct, &item.Cooldown, &item.Reason, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) RevertSignalSettings(ctx context.Context, versionID int64) (*portfolio.SignalSettings, error) {
	const lookup = `SELECT move_threshold_pct, cooldown FROM signal_settings_versions WHERE id = $1`
	var req portfolio.SignalSettingsRequest
	if err := r.pool.QueryRow(ctx, lookup, versionID).Scan(&req.MoveThresholdPct, &req.Cooldown); err != nil {
		return nil, err
	}
	if _, err := r.InsertSignalSettingsVersion(ctx, req, fmt.Sprintf("revert:%d", versionID)); err != nil {
		return nil, err
	}
	if err := r.UpdateSignalSettings(ctx, req); err != nil {
		return nil, err
	}
	return r.GetSignalSettings(ctx)
}

func (r *Repository) CompactLabOpenClawPayloads(ctx context.Context, olderThan time.Time) error {
	const q = `
UPDATE lab_openclaw_runs
SET full_request_json = NULL,
    full_response_json = NULL,
    updated_at = now()
WHERE updated_at < $1
  AND (full_request_json IS NOT NULL OR full_response_json IS NOT NULL)`
	_, err := r.pool.Exec(ctx, q, olderThan)
	return err
}

func payloadJSONBytes(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	return raw
}

func nullableJSONBytes(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLabSignalEvent(row rowScanner) (portfolio.LabSignalEvent, error) {
	var item portfolio.LabSignalEvent
	err := row.Scan(
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
		&item.FiredAt,
		&item.PayloadJSON,
		&item.DiscordStatus,
		&item.CreatedAt,
	)
	return item, err
}

func scanLabOpenClawRun(row rowScanner) (portfolio.LabOpenClawRun, error) {
	var item portfolio.LabOpenClawRun
	var requestJSON []byte
	var responseJSON []byte
	err := row.Scan(
		&item.RequestID,
		&item.SignalID,
		&item.Status,
		&item.Attempts,
		&item.Analysis,
		&item.Recommendation,
		&item.Confidence,
		&item.ToolNames,
		&item.ErrorText,
		&item.RequestHash,
		&item.ResponseHash,
		&requestJSON,
		&responseJSON,
		&item.StartedAt,
		&item.CompletedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return portfolio.LabOpenClawRun{}, err
	}
	item.RequestJSON = json.RawMessage(requestJSON)
	item.ResponseJSON = json.RawMessage(responseJSON)
	return item, nil
}

func clampLimit(limit, fallback, max int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > max {
		return max
	}
	return limit
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
