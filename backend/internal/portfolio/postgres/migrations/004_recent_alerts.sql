CREATE TABLE IF NOT EXISTS recent_alerts (
    id BIGSERIAL PRIMARY KEY,
    type TEXT NOT NULL DEFAULT 'crypto_alert',
    symbol TEXT NOT NULL,
    product_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    current_price DOUBLE PRECISION NOT NULL,
    previous_price DOUBLE PRECISION,
    delta_amount DOUBLE PRECISION,
    delta_pct DOUBLE PRECISION NOT NULL,
    threshold_pct DOUBLE PRECISION NOT NULL,
    quantity DOUBLE PRECISION,
    avg_cost DOUBLE PRECISION,
    cost_basis DOUBLE PRECISION,
    unrealized_pl DOUBLE PRECISION,
    unrealized_pl_pct DOUBLE PRECISION,
    fired_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_recent_alerts_fired_at
    ON recent_alerts (fired_at DESC, id DESC);
