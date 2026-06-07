-- Relay request-level usage logging for external gateway/API-token traffic.
-- Chat-originated app usage keeps using the existing conversation/workspace fields.

ALTER TABLE IF EXISTS usage_records
    ALTER COLUMN workspace_id DROP NOT NULL;

ALTER TABLE IF EXISTS usage_records
    ADD COLUMN IF NOT EXISTS api_type TEXT,
    ADD COLUMN IF NOT EXISTS channel_id TEXT,
    ADD COLUMN IF NOT EXISTS provider TEXT,
    ADD COLUMN IF NOT EXISTS api_token_id TEXT,
    ADD COLUMN IF NOT EXISTS status TEXT,
    ADD COLUMN IF NOT EXISTS status_code INTEGER,
    ADD COLUMN IF NOT EXISTS latency_ms INTEGER,
    ADD COLUMN IF NOT EXISTS cost NUMERIC(15,6) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS channel_cost NUMERIC(15,6) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS request_id TEXT,
    ADD COLUMN IF NOT EXISTS error_code TEXT,
    ADD COLUMN IF NOT EXISTS total_tokens INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_usage_records_request_id
    ON usage_records(request_id)
    WHERE request_id IS NOT NULL AND request_id <> '';

CREATE INDEX IF NOT EXISTS idx_usage_records_api_token_created
    ON usage_records(api_token_id, created_at DESC)
    WHERE api_token_id IS NOT NULL AND api_token_id <> '';

CREATE INDEX IF NOT EXISTS idx_usage_records_channel_created
    ON usage_records(channel_id, created_at DESC)
    WHERE channel_id IS NOT NULL AND channel_id <> '';

CREATE INDEX IF NOT EXISTS idx_usage_records_status_created
    ON usage_records(status, created_at DESC)
    WHERE status IS NOT NULL AND status <> '';
