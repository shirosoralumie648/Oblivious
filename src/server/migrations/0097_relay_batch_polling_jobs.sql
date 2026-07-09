-- Durable Relay Batch polling jobs.
-- POST /v1/batch must persist a local task after upstream batch creation so
-- completion polling, settlement/refund, usage capture, and audit can resume
-- after process restart.

CREATE TABLE IF NOT EXISTS relay_batch_polling_jobs (
    batch_id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    organization_id TEXT NOT NULL DEFAULT '',
    api_token_id TEXT NOT NULL DEFAULT '',
    feature_type TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    api_type TEXT NOT NULL,
    billing_session_id TEXT NOT NULL DEFAULT '',
    preauthorized_amount DECIMAL(15,6) NOT NULL DEFAULT 0,
    token_preauthorized_amount DECIMAL(15,6) NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    error TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
    locked_at TIMESTAMPTZ,
    locked_by TEXT NOT NULL DEFAULT '',
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'dead_letter'))
);

CREATE INDEX IF NOT EXISTS idx_relay_batch_polling_jobs_due
    ON relay_batch_polling_jobs(available_at ASC, created_at ASC, batch_id ASC)
    WHERE status IN ('pending', 'failed', 'processing');

CREATE INDEX IF NOT EXISTS idx_relay_batch_polling_jobs_request_id
    ON relay_batch_polling_jobs(request_id)
    WHERE request_id <> '';

CREATE INDEX IF NOT EXISTS idx_relay_batch_polling_jobs_status_updated
    ON relay_batch_polling_jobs(status, updated_at DESC);
