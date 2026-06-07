ALTER TABLE observability_alert_delivery_attempts
ADD COLUMN IF NOT EXISTS provider_id TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS provider_kind TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_observability_alert_delivery_attempts_provider
    ON observability_alert_delivery_attempts(provider_id, provider_kind, attempted_at DESC);
