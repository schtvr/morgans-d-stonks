CREATE TABLE IF NOT EXISTS agent_decision_outcomes (
    id BIGSERIAL PRIMARY KEY,
    decision_id BIGINT NOT NULL REFERENCES agent_decisions(id) ON DELETE CASCADE,
    horizon TEXT NOT NULL CHECK (horizon IN ('1h','24h','7d','14d')),
    price_at_decision DOUBLE PRECISION,
    price_at_horizon DOUBLE PRECISION,
    -- symbol_return_pct: actual price/NAV move over horizon. Populated for all
    -- actions including ignore (opportunity-cost display). NULL only if deferred.
    symbol_return_pct DOUBLE PRECISION,
    -- btc_return_pct: raw BTC return over horizon. 0.6% fee deducted only at
    -- horizon='14d'. Other horizons: raw return (no fee drag).
    btc_return_pct DOUBLE PRECISION NOT NULL DEFAULT 0,
    -- realized_return_pct: directional return for buy/sell. NULL for ignore and
    -- daily triggers — shadow mode does not claim P&L on inaction.
    realized_return_pct DOUBLE PRECISION,
    -- excess_return_pct: realized - btc. NULL when realized is NULL.
    -- Excluded from headline metric for ignore/daily decisions.
    excess_return_pct DOUBLE PRECISION,
    fees_modeled_pct DOUBLE PRECISION NOT NULL DEFAULT 0,
    scored_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (decision_id, horizon)
);

CREATE INDEX IF NOT EXISTS idx_agent_decision_outcomes_decision
    ON agent_decision_outcomes (decision_id);
CREATE INDEX IF NOT EXISTS idx_agent_decision_outcomes_horizon_scored
    ON agent_decision_outcomes (horizon, scored_at DESC);
