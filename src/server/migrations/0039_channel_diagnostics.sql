-- Persist latest channel probe diagnostics for admin operations.

ALTER TABLE IF EXISTS channels
    ADD COLUMN IF NOT EXISTS last_balance_amount DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS last_balance_currency TEXT,
    ADD COLUMN IF NOT EXISTS last_balance_source TEXT,
    ADD COLUMN IF NOT EXISTS last_balance_error TEXT,
    ADD COLUMN IF NOT EXISTS last_health_status TEXT NOT NULL DEFAULT 'offline',
    ADD COLUMN IF NOT EXISTS last_health_message TEXT,
    ADD COLUMN IF NOT EXISTS last_health_checked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_latency_ms BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_probe_error TEXT;

CREATE INDEX IF NOT EXISTS idx_channels_last_health_status
    ON channels(last_health_status);
