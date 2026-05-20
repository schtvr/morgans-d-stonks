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

// ListSnapshotsSince returns snapshots with taken_at >= since, oldest first.
func (r *Repository) ListSnapshotsSince(ctx context.Context, since time.Time, limit int) ([]portfolio.SnapshotRecord, error) {
	if limit <= 0 {
		limit = 5000
	}
	if limit > 10000 {
		limit = 10000
	}
	const q = `SELECT taken_at, data FROM snapshots WHERE taken_at >= $1 ORDER BY taken_at ASC LIMIT $2`
	rows, err := r.pool.Query(ctx, q, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]portfolio.SnapshotRecord, 0)
	for rows.Next() {
		var rec portfolio.SnapshotRecord
		if err := rows.Scan(&rec.TakenAt, &rec.Data); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
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

// InsertAgentDecision persists a new decision. On duplicate idempotency_key the
// existing row is returned without error — callers must not treat a duplicate as a
// failure.
func (r *Repository) InsertAgentDecision(ctx context.Context, d portfolio.AgentDecision) (*portfolio.AgentDecision, error) {
	const q = `
INSERT INTO agent_decisions (
    trigger_kind, trigger_at, idempotency_key, symbol, signal_event_id, action,
    confidence, rationale, sizing_hint_notional, model, prompt_version,
    latency_ms, cost_cents, request_json, response_json, tool_calls_json
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15::jsonb, $16::jsonb)
ON CONFLICT (idempotency_key) DO NOTHING
RETURNING id, trigger_kind, trigger_at, idempotency_key, symbol, signal_event_id, action,
          confidence, rationale, sizing_hint_notional, model, prompt_version,
          latency_ms, cost_cents, request_json, response_json, tool_calls_json, created_at`
	row := r.pool.QueryRow(ctx, q,
		d.TriggerKind,
		d.TriggerAt,
		d.IdempotencyKey,
		d.Symbol,
		d.SignalEventID,
		d.Action,
		d.Confidence,
		d.Rationale,
		d.SizingHintNotional,
		d.Model,
		d.PromptVersion,
		d.LatencyMS,
		d.CostCents,
		jsonOrEmpty(d.RequestJSON),
		jsonOrEmpty(d.ResponseJSON),
		jsonOrEmptyArray(d.ToolCallsJSON),
	)
	out, err := scanAgentDecision(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Duplicate idempotency key — fetch and return the existing row.
			return r.GetAgentDecisionByIdempotencyKey(ctx, d.IdempotencyKey)
		}
		return nil, fmt.Errorf("InsertAgentDecision: %w", err)
	}
	return out, nil
}

// GetAgentDecision returns one agent decision by primary key.
func (r *Repository) GetAgentDecision(ctx context.Context, id int64) (*portfolio.AgentDecision, error) {
	const q = `
SELECT id, trigger_kind, trigger_at, idempotency_key, symbol, signal_event_id, action,
       confidence, rationale, sizing_hint_notional, model, prompt_version,
       latency_ms, cost_cents, request_json, response_json, tool_calls_json, created_at
FROM agent_decisions
WHERE id = $1`
	out, err := scanAgentDecision(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		return nil, fmt.Errorf("GetAgentDecision id=%d: %w", id, err)
	}
	return out, nil
}

// GetAgentDecisionByIdempotencyKey returns one agent decision by idempotency key.
func (r *Repository) GetAgentDecisionByIdempotencyKey(ctx context.Context, key string) (*portfolio.AgentDecision, error) {
	const q = `
SELECT id, trigger_kind, trigger_at, idempotency_key, symbol, signal_event_id, action,
       confidence, rationale, sizing_hint_notional, model, prompt_version,
       latency_ms, cost_cents, request_json, response_json, tool_calls_json, created_at
FROM agent_decisions
WHERE idempotency_key = $1`
	out, err := scanAgentDecision(r.pool.QueryRow(ctx, q, key))
	if err != nil {
		return nil, fmt.Errorf("GetAgentDecisionByIdempotencyKey: %w", err)
	}
	return out, nil
}

// ListAgentDecisions returns decisions matching the filter, newest first.
func (r *Repository) ListAgentDecisions(ctx context.Context, filter portfolio.AgentDecisionFilter) ([]portfolio.AgentDecision, error) {
	limit := clampLimit(filter.Limit, 50, 200)
	const q = `
SELECT id, trigger_kind, trigger_at, idempotency_key, symbol, signal_event_id, action,
       confidence, rationale, sizing_hint_notional, model, prompt_version,
       latency_ms, cost_cents, request_json, response_json, tool_calls_json, created_at
FROM agent_decisions
WHERE ($2 = '' OR symbol = $2)
  AND ($3 = '' OR action = $3)
  AND ($4::timestamptz IS NULL OR trigger_at >= $4)
  AND ($5::timestamptz IS NULL OR trigger_at <= $5)
ORDER BY trigger_at DESC, id DESC
LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit,
		strings.TrimSpace(filter.Symbol),
		strings.TrimSpace(filter.Action),
		filter.From,
		filter.To,
	)
	if err != nil {
		return nil, fmt.Errorf("ListAgentDecisions: %w", err)
	}
	defer rows.Close()
	out := make([]portfolio.AgentDecision, 0, limit)
	for rows.Next() {
		item, err := scanAgentDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

// CountDecisionsForSymbolSince returns the number of decisions for a symbol since a given time.
func (r *Repository) CountDecisionsForSymbolSince(ctx context.Context, symbol string, since time.Time) (int, error) {
	const q = `SELECT COUNT(*) FROM agent_decisions WHERE symbol = $1 AND trigger_at >= $2`
	var n int
	if err := r.pool.QueryRow(ctx, q, symbol, since).Scan(&n); err != nil {
		return 0, fmt.Errorf("CountDecisionsForSymbolSince: %w", err)
	}
	return n, nil
}

// SumCostCentsForDay returns the total agent cost in cents for the given calendar day (UTC).
func (r *Repository) SumCostCentsForDay(ctx context.Context, day time.Time) (int64, error) {
	const q = `
SELECT COALESCE(SUM(cost_cents), 0)
FROM agent_decisions
WHERE (trigger_at AT TIME ZONE 'UTC')::date = ($1 AT TIME ZONE 'UTC')::date`
	var total int64
	if err := r.pool.QueryRow(ctx, q, day).Scan(&total); err != nil {
		return 0, fmt.Errorf("SumCostCentsForDay: %w", err)
	}
	return total, nil
}

// ListAgentCostDaily returns per-day cost aggregates for the last N days, newest first.
func (r *Repository) ListAgentCostDaily(ctx context.Context, days int) ([]portfolio.AgentCostPoint, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	const q = `
SELECT (trigger_at AT TIME ZONE 'UTC')::date::text AS day,
       SUM(cost_cents) AS cost_cents,
       COUNT(*) AS decisions
FROM agent_decisions
WHERE trigger_at >= now() - ($1 || ' days')::interval
GROUP BY day
ORDER BY day DESC`
	rows, err := r.pool.Query(ctx, q, days)
	if err != nil {
		return nil, fmt.Errorf("ListAgentCostDaily: %w", err)
	}
	defer rows.Close()
	out := make([]portfolio.AgentCostPoint, 0, days)
	for rows.Next() {
		var pt portfolio.AgentCostPoint
		if err := rows.Scan(&pt.Day, &pt.CostCents, &pt.Decisions); err != nil {
			return nil, err
		}
		out = append(out, pt)
	}
	return out, rows.Err()
}

// InsertAgentDecisionOutcome stores one scored outcome. Duplicate (decision_id, horizon)
// is silently ignored — the unique constraint is the idempotency guard.
func (r *Repository) InsertAgentDecisionOutcome(ctx context.Context, o portfolio.AgentDecisionOutcome) (*portfolio.AgentDecisionOutcome, error) {
	const q = `
INSERT INTO agent_decision_outcomes (
    decision_id, horizon, price_at_decision, price_at_horizon,
    symbol_return_pct, btc_return_pct, realized_return_pct, excess_return_pct,
    fees_modeled_pct
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (decision_id, horizon) DO NOTHING
RETURNING id, decision_id, horizon, price_at_decision, price_at_horizon,
          symbol_return_pct, btc_return_pct, realized_return_pct, excess_return_pct,
          fees_modeled_pct, scored_at`
	row := r.pool.QueryRow(ctx, q,
		o.DecisionID,
		o.Horizon,
		o.PriceAtDecision,
		o.PriceAtHorizon,
		o.SymbolReturnPct,
		o.BTCReturnPct,
		o.RealizedReturnPct,
		o.ExcessReturnPct,
		o.FeesModeledPct,
	)
	out, err := scanAgentDecisionOutcome(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Already scored — fetch and return the existing row.
			const sel = `
SELECT id, decision_id, horizon, price_at_decision, price_at_horizon,
       symbol_return_pct, btc_return_pct, realized_return_pct, excess_return_pct,
       fees_modeled_pct, scored_at
FROM agent_decision_outcomes
WHERE decision_id = $1 AND horizon = $2`
			existing, serr := scanAgentDecisionOutcome(r.pool.QueryRow(ctx, sel, o.DecisionID, o.Horizon))
			if serr != nil {
				return nil, fmt.Errorf("InsertAgentDecisionOutcome (fetch existing): %w", serr)
			}
			return existing, nil
		}
		return nil, fmt.Errorf("InsertAgentDecisionOutcome: %w", err)
	}
	return out, nil
}

// ListUnscoredDecisionHorizons returns (decision, horizon) pairs where the horizon
// deadline has passed but no outcome row exists yet.
func (r *Repository) ListUnscoredDecisionHorizons(ctx context.Context, now time.Time, limit int) ([]portfolio.UnscoredHorizon, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	// Cross-join decisions × all 4 horizons; exclude already-scored pairs; filter
	// where now >= trigger_at + horizon_duration.
	const q = `
WITH horizons(horizon, duration) AS (
    VALUES
        ('1h',  '1 hour'::interval),
        ('24h', '24 hours'::interval),
        ('7d',  '7 days'::interval),
        ('14d', '14 days'::interval)
)
SELECT d.id, d.symbol, d.trigger_at, h.horizon, d.action
FROM agent_decisions d
CROSS JOIN horizons h
WHERE $1::timestamptz >= d.trigger_at + h.duration
  AND NOT EXISTS (
      SELECT 1 FROM agent_decision_outcomes o
      WHERE o.decision_id = d.id AND o.horizon = h.horizon
  )
ORDER BY d.trigger_at ASC, d.id ASC
LIMIT $2`
	rows, err := r.pool.Query(ctx, q, now, limit)
	if err != nil {
		return nil, fmt.Errorf("ListUnscoredDecisionHorizons: %w", err)
	}
	defer rows.Close()
	out := make([]portfolio.UnscoredHorizon, 0, limit)
	for rows.Next() {
		var h portfolio.UnscoredHorizon
		if err := rows.Scan(&h.DecisionID, &h.Symbol, &h.TriggerAt, &h.Horizon, &h.Action); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ListAgentDecisionOutcomes returns outcomes matching the filter.
func (r *Repository) ListAgentDecisionOutcomes(ctx context.Context, filter portfolio.AgentDecisionOutcomeFilter) ([]portfolio.AgentDecisionOutcome, error) {
	const q = `
SELECT id, decision_id, horizon, price_at_decision, price_at_horizon,
       symbol_return_pct, btc_return_pct, realized_return_pct, excess_return_pct,
       fees_modeled_pct, scored_at
FROM agent_decision_outcomes
WHERE ($1 = '' OR horizon = $1)
  AND (array_length($2::bigint[], 1) IS NULL OR decision_id = ANY($2::bigint[]))
ORDER BY scored_at DESC`
	rows, err := r.pool.Query(ctx, q,
		strings.TrimSpace(filter.Horizon),
		filter.DecisionIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("ListAgentDecisionOutcomes: %w", err)
	}
	defer rows.Close()
	out := make([]portfolio.AgentDecisionOutcome, 0)
	for rows.Next() {
		item, err := scanAgentDecisionOutcome(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

// ListBenchmarkDaily returns per-day aggregate returns for buy/sell decisions at the
// given horizon, for the last N days.
func (r *Repository) ListBenchmarkDaily(ctx context.Context, horizon string, days int) ([]portfolio.AgentBenchmarkPoint, error) {
	if days <= 0 {
		days = 14
	}
	if days > 90 {
		days = 90
	}
	const q = `
SELECT
    (d.trigger_at AT TIME ZONE 'UTC')::date AS day,
    COALESCE(AVG(CASE WHEN d.action IN ('buy','sell') THEN o.realized_return_pct END), 0) AS realized_return_pct,
    COALESCE(AVG(CASE WHEN d.action IN ('buy','sell') THEN o.btc_return_pct END), 0)      AS btc_return_pct,
    COALESCE(AVG(CASE WHEN d.action IN ('buy','sell') THEN o.excess_return_pct END), 0)   AS excess_return_pct,
    COUNT(CASE WHEN d.action IN ('buy','sell') THEN 1 END)                                AS decision_count,
    COUNT(CASE WHEN d.action = 'ignore' THEN 1 END)                                       AS ignore_count
FROM agent_decision_outcomes o
JOIN agent_decisions d ON d.id = o.decision_id
WHERE o.horizon = $1
  AND d.trigger_at >= now() - ($2 || ' days')::interval
GROUP BY day
ORDER BY day DESC`
	rows, err := r.pool.Query(ctx, q, horizon, days)
	if err != nil {
		return nil, fmt.Errorf("ListBenchmarkDaily: %w", err)
	}
	defer rows.Close()
	out := make([]portfolio.AgentBenchmarkPoint, 0, days)
	for rows.Next() {
		var pt portfolio.AgentBenchmarkPoint
		var day string
		if err := rows.Scan(&day, &pt.RealizedReturnPct, &pt.BTCReturnPct, &pt.ExcessReturnPct, &pt.DecisionCount, &pt.IgnoreCount); err != nil {
			return nil, err
		}
		if t, err := time.Parse("2006-01-02", day); err == nil {
			pt.AsOf = t
		}
		out = append(out, pt)
	}
	return out, rows.Err()
}

// scanAgentDecision reads one agent_decisions row from a pgx row scanner.
func scanAgentDecision(row rowScanner) (*portfolio.AgentDecision, error) {
	var d portfolio.AgentDecision
	var requestJSON, responseJSON, toolCallsJSON []byte
	err := row.Scan(
		&d.ID,
		&d.TriggerKind,
		&d.TriggerAt,
		&d.IdempotencyKey,
		&d.Symbol,
		&d.SignalEventID,
		&d.Action,
		&d.Confidence,
		&d.Rationale,
		&d.SizingHintNotional,
		&d.Model,
		&d.PromptVersion,
		&d.LatencyMS,
		&d.CostCents,
		&requestJSON,
		&responseJSON,
		&toolCallsJSON,
		&d.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	d.RequestJSON = json.RawMessage(requestJSON)
	d.ResponseJSON = json.RawMessage(responseJSON)
	d.ToolCallsJSON = json.RawMessage(toolCallsJSON)
	return &d, nil
}

// scanAgentDecisionOutcome reads one agent_decision_outcomes row.
func scanAgentDecisionOutcome(row rowScanner) (*portfolio.AgentDecisionOutcome, error) {
	var o portfolio.AgentDecisionOutcome
	err := row.Scan(
		&o.ID,
		&o.DecisionID,
		&o.Horizon,
		&o.PriceAtDecision,
		&o.PriceAtHorizon,
		&o.SymbolReturnPct,
		&o.BTCReturnPct,
		&o.RealizedReturnPct,
		&o.ExcessReturnPct,
		&o.FeesModeledPct,
		&o.ScoredAt,
	)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// jsonOrEmpty returns the JSON bytes as-is, falling back to an empty object.
func jsonOrEmpty(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	return raw
}

// jsonOrEmptyArray returns the JSON bytes as-is, falling back to an empty array.
func jsonOrEmptyArray(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte(`[]`)
	}
	return raw
}

var _ portfolio.Repository = (*Repository)(nil)
