-- Durable, route-attempt-bound quota compensation state for late Relay
-- readiness denials.  The key columns intentionally hold SHA-256 digests only:
-- raw route attempts and caller idempotency values never enter this table.

CREATE TABLE IF NOT EXISTS relay_quota_compensation_jobs (
    job_key_digest TEXT PRIMARY KEY,
    organization_scope_key_digest TEXT NOT NULL DEFAULT '',
    api_token_scope_key_digest TEXT NOT NULL DEFAULT '',
    organization_id TEXT NOT NULL DEFAULT '',
    billing_session_id TEXT NOT NULL DEFAULT '',
    api_token_id TEXT NOT NULL DEFAULT '',
    amount NUMERIC(15,6) NOT NULL DEFAULT 0,
    organization_required BOOLEAN NOT NULL DEFAULT FALSE,
    organization_completed BOOLEAN NOT NULL DEFAULT FALSE,
    api_token_required BOOLEAN NOT NULL DEFAULT FALSE,
    api_token_completed BOOLEAN NOT NULL DEFAULT FALSE,
    organization_error_code TEXT NOT NULL DEFAULT '',
    api_token_error_code TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    locked_at TIMESTAMPTZ,
    locked_by TEXT NOT NULL DEFAULT '',
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (organization_required OR api_token_required),
    CHECK (NOT organization_required OR organization_scope_key_digest <> ''),
    CHECK (NOT api_token_required OR api_token_scope_key_digest <> ''),
    CHECK (status IN ('pending', 'processing', 'succeeded', 'failed')),
    CHECK (organization_error_code IN ('', 'refund_failed', 'mark_failed', 'retry_failed', 'persistence_failed', 'unknown')),
    CHECK (api_token_error_code IN ('', 'refund_failed', 'mark_failed', 'retry_failed', 'persistence_failed', 'unknown'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_relay_quota_compensation_jobs_organization_scope
    ON relay_quota_compensation_jobs(organization_scope_key_digest)
    WHERE organization_scope_key_digest <> '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_relay_quota_compensation_jobs_api_token_scope
    ON relay_quota_compensation_jobs(api_token_scope_key_digest)
    WHERE api_token_scope_key_digest <> '';

CREATE INDEX IF NOT EXISTS idx_relay_quota_compensation_jobs_due
    ON relay_quota_compensation_jobs(available_at ASC, created_at ASC, job_key_digest ASC)
    WHERE status IN ('pending', 'failed', 'processing');

CREATE TABLE IF NOT EXISTS relay_api_token_quota_refund_receipts (
    scope_key_digest TEXT PRIMARY KEY,
    api_token_id TEXT NOT NULL,
    amount NUMERIC(15,6) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (scope_key_digest, api_token_id, amount),
    CHECK (scope_key_digest <> ''),
    CHECK (api_token_id <> ''),
    CHECK (amount > 0)
);
