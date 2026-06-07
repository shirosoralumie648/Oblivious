-- Billing enhanced tables: concurrency limits and token rate limits.

CREATE TABLE IF NOT EXISTS concurrency_limits (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    max_concurrent_requests INT NOT NULL DEFAULT 5,
    current_concurrent INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (max_concurrent_requests >= 0),
    CHECK (current_concurrent >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_concurrency_limits_org_scope
    ON concurrency_limits(organization_id)
    WHERE user_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_concurrency_limits_user_scope
    ON concurrency_limits(organization_id, user_id)
    WHERE user_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS token_rate_limits (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    window_seconds INT NOT NULL DEFAULT 60,
    max_tokens_per_window BIGINT NOT NULL,
    current_window_start TIMESTAMPTZ,
    current_window_tokens BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (window_seconds > 0),
    CHECK (max_tokens_per_window >= 0),
    CHECK (current_window_tokens >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_token_rate_limits_org_scope
    ON token_rate_limits(organization_id)
    WHERE user_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_token_rate_limits_user_scope
    ON token_rate_limits(organization_id, user_id)
    WHERE user_id IS NOT NULL;
