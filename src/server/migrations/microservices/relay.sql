-- Relay Service Database Schema
-- Tables: channels, channel_configs, model_routes, model_channel_weights, relay_api_tokens,
--         relay_semantic_cache, relay_metrics, relay_pricing_settings, relay_file_mappings,
--         relay_conversation_affinity, relay_channel_affinity

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS vector;

-- ============================================================================
-- Core Relay Tables
-- ============================================================================

-- Relay channels and routing configuration
CREATE TABLE IF NOT EXISTS channels (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    base_url TEXT NOT NULL DEFAULT 'https://api.openai.com',
    api_key_encrypted TEXT NOT NULL,
    models TEXT[] NOT NULL DEFAULT '{}',
    rpm_limit INTEGER DEFAULT 1000,
    tpm_limit INTEGER DEFAULT 100000,
    cb_threshold INTEGER DEFAULT 5,
    cb_timeout INTEGER DEFAULT 30,
    health_check_strategy TEXT DEFAULT 'models_api',
    probe_model TEXT,
    probe_prompt TEXT,
    strategy TEXT DEFAULT 'weighted',
    priority INTEGER DEFAULT 0,
    estimated_cost_per_1k DOUBLE PRECISION NOT NULL DEFAULT 0,
    cost_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_channels_provider ON channels(provider);
CREATE INDEX idx_channels_enabled ON channels(enabled);

-- Model routing configuration
CREATE TABLE IF NOT EXISTS model_routes (
    id TEXT PRIMARY KEY,
    model TEXT NOT NULL UNIQUE,
    strategy TEXT DEFAULT 'weighted',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_model_routes_model ON model_routes(model);

-- Model-channel weight mappings
CREATE TABLE IF NOT EXISTS model_channel_weights (
    id TEXT PRIMARY KEY,
    route_id TEXT NOT NULL REFERENCES model_routes(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    weight INTEGER DEFAULT 100,
    priority INTEGER DEFAULT 0,
    enabled BOOLEAN DEFAULT true,
    UNIQUE(route_id, channel_id)
);

CREATE INDEX idx_model_channel_weights_route_id ON model_channel_weights(route_id);
CREATE INDEX idx_model_channel_weights_channel_id ON model_channel_weights(channel_id);

-- Channel configurations per organization
CREATE TABLE IF NOT EXISTS channel_configs (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('active', 'degraded', 'disabled'))
);

CREATE INDEX idx_channel_configs_organization_id ON channel_configs(organization_id);
CREATE INDEX idx_channel_configs_organization_status ON channel_configs(organization_id, status);

-- ============================================================================
-- API Token Management
-- ============================================================================

-- Relay customer API tokens for production /v1/* access
CREATE TABLE IF NOT EXISTS relay_api_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    model_limits_enabled BOOLEAN NOT NULL DEFAULT false,
    model_limits TEXT[] NOT NULL DEFAULT '{}',
    quota_limit NUMERIC(18, 6),
    used_quota NUMERIC(18, 6) NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    CHECK (status IN ('active', 'revoked'))
);

CREATE INDEX idx_relay_api_tokens_org_status ON relay_api_tokens(organization_id, status, created_at DESC);
CREATE INDEX idx_relay_api_tokens_user_status ON relay_api_tokens(user_id, status, created_at DESC);
CREATE INDEX idx_relay_api_tokens_prefix ON relay_api_tokens(token_prefix);

-- ============================================================================
-- Semantic Cache
-- ============================================================================

-- Semantic cache with vector embeddings
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

CREATE UNIQUE INDEX idx_relay_semantic_cache_scope_key
    ON relay_semantic_cache(cache_scope, COALESCE(organization_id, ''), model_id, query_hash);
CREATE INDEX idx_relay_semantic_cache_expires_at ON relay_semantic_cache(expires_at);
CREATE INDEX idx_relay_semantic_cache_embedding
    ON relay_semantic_cache USING hnsw (query_embedding vector_cosine_ops)
    WHERE query_embedding IS NOT NULL;

-- ============================================================================
-- Affinity & Metrics
-- ============================================================================

-- Channel affinity tracking per org/user/model
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

CREATE UNIQUE INDEX idx_relay_channel_affinity_org_user_model_channel
    ON relay_channel_affinity(organization_id, COALESCE(user_id, ''), model_id, channel_id);
CREATE INDEX idx_relay_channel_affinity_org_model_score
    ON relay_channel_affinity(organization_id, model_id, affinity_score DESC);

-- Conversation-channel affinity for session stickiness
CREATE TABLE IF NOT EXISTS relay_conversation_affinity (
    conversation_id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_relay_conversation_affinity_channel_id ON relay_conversation_affinity(channel_id);

-- Real-time relay metrics
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

CREATE INDEX idx_relay_metrics_org_channel_type_window
    ON relay_metrics(organization_id, channel_id, metric_type, window_start DESC);
CREATE INDEX idx_relay_metrics_org_provider_type_window
    ON relay_metrics(organization_id, provider, metric_type, window_start DESC);
CREATE INDEX idx_relay_metrics_org_window ON relay_metrics(organization_id, window_start DESC);

-- ============================================================================
-- Pricing & File Management
-- ============================================================================

-- Persistent model/group multiplier settings for relay billing
CREATE TABLE IF NOT EXISTS relay_pricing_settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Relay file upload tenant ownership mapping
CREATE TABLE IF NOT EXISTS relay_file_mappings (
    local_file_id TEXT PRIMARY KEY,
    openai_file_id TEXT NOT NULL UNIQUE,
    local_path TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    user_id TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_relay_file_mappings_org_created ON relay_file_mappings(organization_id, created_at DESC);
CREATE INDEX idx_relay_file_mappings_user_created ON relay_file_mappings(user_id, created_at DESC);
CREATE INDEX idx_relay_file_mappings_request_id ON relay_file_mappings(request_id) WHERE request_id <> '';
