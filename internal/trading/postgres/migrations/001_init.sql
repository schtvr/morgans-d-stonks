CREATE TABLE IF NOT EXISTS orders (
    id                TEXT PRIMARY KEY,
    symbol            TEXT NOT NULL,
    side              TEXT NOT NULL,
    quantity          NUMERIC(24, 12) NOT NULL,
    limit_price       NUMERIC(24, 12) NOT NULL DEFAULT 0,
    notional          NUMERIC(24, 12) NOT NULL DEFAULT 0,
    status            TEXT NOT NULL,
    reason            TEXT NOT NULL DEFAULT '',
    idempotency_key   TEXT NOT NULL UNIQUE,
    request_hash      TEXT NOT NULL,
    provider          TEXT NOT NULL DEFAULT '',
    provider_order_id TEXT NOT NULL DEFAULT '',
    response_json     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_orders_status_created_at ON orders (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_symbol_created_at ON orders (symbol, created_at DESC);

CREATE TABLE IF NOT EXISTS order_events (
    id           BIGSERIAL PRIMARY KEY,
    order_id     TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    event_type   TEXT NOT NULL,
    from_status  TEXT NOT NULL DEFAULT '',
    to_status    TEXT NOT NULL DEFAULT '',
    reason       TEXT NOT NULL DEFAULT '',
    payload      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_order_events_order_id_created_at ON order_events (order_id, created_at DESC);

CREATE TABLE IF NOT EXISTS fills (
    id           BIGSERIAL PRIMARY KEY,
    order_id     TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    price        NUMERIC(24, 12) NOT NULL,
    quantity     NUMERIC(24, 12) NOT NULL,
    fee          NUMERIC(24, 12) NOT NULL DEFAULT 0,
    currency     TEXT NOT NULL DEFAULT 'USD',
    executed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_fills_order_id_executed_at ON fills (order_id, executed_at DESC);

CREATE TABLE IF NOT EXISTS reconciliation (
    id              BIGSERIAL PRIMARY KEY,
    order_id        TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    expected_status TEXT NOT NULL DEFAULT '',
    observed_status TEXT NOT NULL DEFAULT '',
    drift           BOOLEAN NOT NULL DEFAULT false,
    action          TEXT NOT NULL DEFAULT '',
    details         TEXT NOT NULL DEFAULT '',
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_reconciliation_order_id_checked_at ON reconciliation (order_id, checked_at DESC);

CREATE OR REPLACE FUNCTION trading_prevent_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'immutable table % cannot be modified', TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trading_prevent_order_events_update ON order_events;
CREATE TRIGGER trading_prevent_order_events_update
BEFORE UPDATE OR DELETE ON order_events
FOR EACH ROW EXECUTE FUNCTION trading_prevent_mutation();

DROP TRIGGER IF EXISTS trading_prevent_fills_update ON fills;
CREATE TRIGGER trading_prevent_fills_update
BEFORE UPDATE OR DELETE ON fills
FOR EACH ROW EXECUTE FUNCTION trading_prevent_mutation();

DROP TRIGGER IF EXISTS trading_prevent_reconciliation_update ON reconciliation;
CREATE TRIGGER trading_prevent_reconciliation_update
BEFORE UPDATE OR DELETE ON reconciliation
FOR EACH ROW EXECUTE FUNCTION trading_prevent_mutation();

