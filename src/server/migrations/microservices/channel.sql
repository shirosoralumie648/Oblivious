-- Channel microservice schema

CREATE TABLE IF NOT EXISTS channel_configs (
    channel_id TEXT PRIMARY KEY,
    guild_id TEXT NOT NULL,
    channel_type TEXT NOT NULL,
    permissions JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS channel_messages (
    message_id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channel_configs(channel_id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_channel_configs_guild ON channel_configs(guild_id);
CREATE INDEX IF NOT EXISTS idx_channel_messages_channel ON channel_messages(channel_id);
CREATE INDEX IF NOT EXISTS idx_channel_messages_created ON channel_messages(created_at DESC);
