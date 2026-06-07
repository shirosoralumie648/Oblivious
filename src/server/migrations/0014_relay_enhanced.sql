-- Relay enhanced tables: semantic cache, channel affinity, real-time metrics.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS relay_semantic_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cache_scope TEXT NOT NULL CHECK (cache_scope IN ('global', 'org')),
    organization_id TEXT,
    model_id TEXT NOT NULL,
    query_hash TEXT NOT NULL,
    query_text TEXT NOT NULL DEFAULT '',
    query_embedding vector(1536),
    response JSONB NOT NULL,
    hit_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    CHECK (
        (cache_scope = 'global' AND organization_id IS NULL)
        OR (cache_scope = 'org' AND organization_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_relay_semantic_cache_scope_key
    ON relay_semantic_cache(cache_scope, COALESCE(organization_id, ''), model_id, query_hash);

CREATE INDEX IF NOT EXISTS idx_relay_semantic_cache_expires_at
    ON relay_semantic_cache(expires_at);

CREATE INDEX IF NOT EXISTS idx_relay_semantic_cache_embedding
    ON relay_semantic_cache
    USING hnsw (query_embedding vector_cosine_ops)
    WHERE query_embedding IS NOT NULL;

CREATE TABLE IF NOT EXISTS relay_channel_affinity (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id TEXT,
    model_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    affinity_score DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    total_requests BIGINT NOT NULL DEFAULT 0,
    total_successes BIGINT NOT NULL DEFAULT 0,
    avg_latency_ms DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (total_requests >= 0),
    CHECK (total_successes >= 0),
    CHECK (avg_latency_ms >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_relay_channel_affinity_org_user_model_channel
    ON relay_channel_affinity(organization_id, COALESCE(user_id, ''), model_id, channel_id);

CREATE INDEX IF NOT EXISTS idx_relay_channel_affinity_org_model_score
    ON relay_channel_affinity(organization_id, model_id, affinity_score DESC);

CREATE TABLE IF NOT EXISTS relay_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    model_id TEXT NOT NULL DEFAULT '',
    metric_type TEXT NOT NULL,
    value DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    window_start TIMESTAMPTZ NOT NULL,
    window_seconds INT NOT NULL DEFAULT 60,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (metric_type IN ('latency_p50', 'latency_p95', 'latency_p99', 'throughput', 'error_rate', 'token_rate')),
    CHECK (window_seconds > 0)
);

CREATE INDEX IF NOT EXISTS idx_relay_metrics_org_channel_type_window
    ON relay_metrics(organization_id, channel_id, metric_type, window_start DESC);

CREATE INDEX IF NOT EXISTS idx_relay_metrics_org_provider_type_window
    ON relay_metrics(organization_id, provider, metric_type, window_start DESC);

CREATE INDEX IF NOT EXISTS idx_relay_metrics_org_window
    ON relay_metrics(organization_id, window_start DESC);
