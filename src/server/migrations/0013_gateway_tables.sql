-- Gateway rate limiting and circuit breaker state tables.

CREATE TABLE IF NOT EXISTS rate_limit_counters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    scope_id TEXT NOT NULL DEFAULT '',
    window_seconds INT NOT NULL DEFAULT 60,
    counter BIGINT NOT NULL DEFAULT 0,
    window_start TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (scope IN ('org', 'user', 'api_token', 'ip')),
    CHECK (window_seconds > 0),
    CHECK (counter >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_rate_limit_counters_org_scope_window
    ON rate_limit_counters(organization_id, scope, scope_id, window_seconds, window_start);

CREATE INDEX IF NOT EXISTS idx_rate_limit_counters_org_updated
    ON rate_limit_counters(organization_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS circuit_breaker_state (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'closed',
    failure_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    threshold INTEGER NOT NULL DEFAULT 5,
    cooldown_seconds INTEGER NOT NULL DEFAULT 30,
    half_open_max_calls INTEGER NOT NULL DEFAULT 3,
    half_open_calls INTEGER NOT NULL DEFAULT 0,
    last_failure_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    opened_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (state IN ('closed', 'open', 'half_open')),
    CHECK (failure_count >= 0),
    CHECK (success_count >= 0),
    CHECK (threshold > 0),
    CHECK (cooldown_seconds > 0),
    CHECK (half_open_max_calls > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_circuit_breaker_state_org_channel_provider
    ON circuit_breaker_state(organization_id, channel_id, provider);

CREATE INDEX IF NOT EXISTS idx_circuit_breaker_state_org_state
    ON circuit_breaker_state(organization_id, state);
