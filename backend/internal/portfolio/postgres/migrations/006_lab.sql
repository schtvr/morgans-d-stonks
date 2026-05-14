CREATE TABLE IF NOT EXISTS lab_signal_events (
    id BIGSERIAL PRIMARY KEY,
    source_alert_id BIGINT,
    type TEXT NOT NULL DEFAULT 'crypto_price_move',
    symbol TEXT NOT NULL,
    product_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    current_price DOUBLE PRECISION NOT NULL,
    previous_price DOUBLE PRECISION,
    delta_amount DOUBLE PRECISION,
    delta_pct DOUBLE PRECISION NOT NULL,
    threshold_pct DOUBLE PRECISION NOT NULL,
    fired_at TIMESTAMPTZ NOT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    discord_status TEXT NOT NULL DEFAULT 'signal_only',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_lab_signal_events_fired_at
    ON lab_signal_events (fired_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_lab_signal_events_symbol_fired_at
    ON lab_signal_events (symbol, fired_at DESC);

CREATE TABLE IF NOT EXISTS lab_openclaw_runs (
    request_id TEXT PRIMARY KEY,
    signal_event_id BIGINT NOT NULL REFERENCES lab_signal_events(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 1,
    analysis TEXT NOT NULL DEFAULT '',
    recommendation TEXT NOT NULL DEFAULT '',
    confidence DOUBLE PRECISION,
    tool_names TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    error_text TEXT NOT NULL DEFAULT '',
    request_hash TEXT NOT NULL DEFAULT '',
    response_hash TEXT NOT NULL DEFAULT '',
    full_request_json JSONB,
    full_response_json JSONB,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_lab_openclaw_runs_signal_event_id
    ON lab_openclaw_runs (signal_event_id);

CREATE INDEX IF NOT EXISTS idx_lab_openclaw_runs_status_updated_at
    ON lab_openclaw_runs (status, updated_at DESC);

CREATE TABLE IF NOT EXISTS lab_notes (
    id BIGSERIAL PRIMARY KEY,
    signal_event_id BIGINT REFERENCES lab_signal_events(id) ON DELETE CASCADE,
    request_id TEXT REFERENCES lab_openclaw_runs(request_id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (signal_event_id IS NOT NULL OR request_id IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_lab_notes_signal_event_id
    ON lab_notes (signal_event_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_lab_notes_request_id
    ON lab_notes (request_id, created_at DESC);

CREATE TABLE IF NOT EXISTS signal_settings_versions (
    id BIGSERIAL PRIMARY KEY,
    move_threshold_pct DOUBLE PRECISION NOT NULL,
    cooldown TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_signal_settings_versions_created_at
    ON signal_settings_versions (created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS lab_control_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
    openclaw_paused BOOLEAN NOT NULL DEFAULT FALSE,
    circuit_open BOOLEAN NOT NULL DEFAULT FALSE,
    circuit_reason TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (singleton = TRUE)
);

INSERT INTO lab_control_state (singleton)
VALUES (TRUE)
ON CONFLICT (singleton) DO NOTHING;
