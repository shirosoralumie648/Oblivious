-- Relay semantic cache storage skeleton.
-- The core is currently in-memory; this table captures the durable shape for
-- a future store without wiring cache into live relay handlers yet.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
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
