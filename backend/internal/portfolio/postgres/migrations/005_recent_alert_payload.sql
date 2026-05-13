ALTER TABLE recent_alerts
ADD COLUMN IF NOT EXISTS payload_json JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_recent_alerts_payload_schema
    ON recent_alerts ((payload_json->>'schemaVersion'));
