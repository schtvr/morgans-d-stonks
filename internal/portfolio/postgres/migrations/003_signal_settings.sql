CREATE TABLE IF NOT EXISTS signal_settings (
    singleton             BOOLEAN PRIMARY KEY DEFAULT TRUE,
    move_threshold_pct    NUMERIC(8, 4) NOT NULL DEFAULT 1.0,
    cooldown              INTERVAL NOT NULL DEFAULT '15 minutes',
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO signal_settings (singleton, move_threshold_pct, cooldown)
VALUES (TRUE, 1.0, '15 minutes')
ON CONFLICT (singleton) DO NOTHING;
