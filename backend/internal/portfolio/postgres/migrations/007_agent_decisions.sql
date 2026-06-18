CREATE TABLE IF NOT EXISTS agent_decisions (
    id BIGSERIAL PRIMARY KEY,
    trigger_kind TEXT NOT NULL CHECK (trigger_kind IN ('signal','daily')),
    trigger_at TIMESTAMPTZ NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    symbol TEXT NOT NULL DEFAULT '',
    signal_event_id BIGINT REFERENCES lab_signal_events(id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('buy','sell','ignore')),
    confidence DOUBLE PRECISION NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    rationale TEXT NOT NULL DEFAULT '',
    sizing_hint_notional DOUBLE PRECISION,
    model TEXT NOT NULL DEFAULT '',
    prompt_version TEXT NOT NULL DEFAULT '',
    latency_ms BIGINT NOT NULL DEFAULT 0,
    cost_cents BIGINT NOT NULL DEFAULT 0,
    request_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    tool_calls_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_decisions_trigger_at
    ON agent_decisions (trigger_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_agent_decisions_symbol_trigger_at
    ON agent_decisions (symbol, trigger_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_decisions_action_trigger_at
    ON agent_decisions (action, trigger_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_decisions_cost_day
    ON agent_decisions (((trigger_at AT TIME ZONE 'UTC')::date));
