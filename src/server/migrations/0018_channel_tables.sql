-- Channel tables: configs and message logs.

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

CREATE TABLE IF NOT EXISTS channel_messages (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channel_configs(id) ON DELETE CASCADE,
    conversation_id TEXT,
    direction TEXT NOT NULL,
    raw_message JSONB NOT NULL,
    transformed_message JSONB,
    transform_success BOOLEAN NOT NULL DEFAULT false,
    transform_error TEXT,
    status TEXT NOT NULL DEFAULT 'recorded',
    retry_count INTEGER NOT NULL DEFAULT 0,
    failure_reason TEXT,
    next_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (direction IN ('inbound', 'outbound')),
    CHECK (status IN ('recorded', 'retry_pending', 'permanent_failure')),
    CHECK (retry_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_channel_configs_organization_id
    ON channel_configs(organization_id);

CREATE INDEX IF NOT EXISTS idx_channel_configs_organization_status
    ON channel_configs(organization_id, status);

CREATE INDEX IF NOT EXISTS idx_channel_messages_channel_id
    ON channel_messages(channel_id);

CREATE INDEX IF NOT EXISTS idx_channel_messages_conversation_id
    ON channel_messages(conversation_id);

CREATE INDEX IF NOT EXISTS idx_channel_messages_channel_created_at
    ON channel_messages(channel_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_channel_messages_retry_queue
    ON channel_messages(status, next_retry_at)
    WHERE status = 'retry_pending';
