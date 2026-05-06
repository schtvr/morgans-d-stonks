CREATE TABLE IF NOT EXISTS followed_symbols (
    symbol      TEXT PRIMARY KEY,
    source      TEXT NOT NULL DEFAULT 'manual',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_followed_symbols_source_created_at ON followed_symbols (source, created_at DESC);

CREATE TABLE IF NOT EXISTS followed_symbol_state (
    singleton   BOOLEAN PRIMARY KEY DEFAULT TRUE,
    seeded_at   TIMESTAMPTZ,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO followed_symbol_state (singleton, seeded_at)
VALUES (TRUE, NULL)
ON CONFLICT (singleton) DO NOTHING;
