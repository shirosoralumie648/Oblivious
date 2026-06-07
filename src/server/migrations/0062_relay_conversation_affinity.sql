CREATE TABLE IF NOT EXISTS relay_conversation_affinity (
    conversation_id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_relay_conversation_affinity_channel_id
    ON relay_conversation_affinity(channel_id);
