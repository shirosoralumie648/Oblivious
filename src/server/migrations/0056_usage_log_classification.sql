-- Classify usage records by user-facing feature and quota accounting mode.

ALTER TABLE IF EXISTS usage_records
    ADD COLUMN IF NOT EXISTS feature_type TEXT,
    ADD COLUMN IF NOT EXISTS quota_mode TEXT;

CREATE INDEX IF NOT EXISTS idx_usage_records_feature_created
    ON usage_records(feature_type, created_at DESC)
    WHERE feature_type IS NOT NULL AND feature_type <> '';

CREATE INDEX IF NOT EXISTS idx_usage_records_quota_mode_created
    ON usage_records(quota_mode, created_at DESC)
    WHERE quota_mode IS NOT NULL AND quota_mode <> '';
