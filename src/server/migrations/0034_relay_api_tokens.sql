-- Relay customer API tokens for production /v1/* access.

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

CREATE INDEX IF NOT EXISTS idx_relay_api_tokens_org_status
    ON relay_api_tokens(organization_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_relay_api_tokens_user_status
    ON relay_api_tokens(user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_relay_api_tokens_prefix
    ON relay_api_tokens(token_prefix);
