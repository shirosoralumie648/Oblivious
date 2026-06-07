-- PostgreSQL daily cost aggregates for Billing 6.3 usage analytics.

CREATE TABLE IF NOT EXISTS usage_daily_aggregates (
    usage_date DATE NOT NULL,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    feature_type TEXT NOT NULL DEFAULT 'unknown',
    channel_id TEXT NOT NULL DEFAULT 'unknown',
    provider TEXT NOT NULL DEFAULT 'unknown',
    status TEXT NOT NULL DEFAULT 'unknown',
    request_count INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    total_cost NUMERIC(15,6) NOT NULL DEFAULT 0,
    channel_cost NUMERIC(15,6) NOT NULL DEFAULT 0,
    refreshed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (usage_date, organization_id, user_id, model_id, feature_type, channel_id, provider, status)
);

CREATE INDEX IF NOT EXISTS idx_usage_daily_aggregates_org_date
    ON usage_daily_aggregates(organization_id, usage_date DESC);

CREATE INDEX IF NOT EXISTS idx_usage_daily_aggregates_user_date
    ON usage_daily_aggregates(user_id, usage_date DESC);

CREATE INDEX IF NOT EXISTS idx_usage_daily_aggregates_model_date
    ON usage_daily_aggregates(model_id, usage_date DESC);

CREATE INDEX IF NOT EXISTS idx_usage_daily_aggregates_feature_date
    ON usage_daily_aggregates(feature_type, usage_date DESC);
