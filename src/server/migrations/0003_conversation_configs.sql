CREATE TABLE IF NOT EXISTS conversation_configs (
    conversation_id TEXT PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    model_id TEXT NOT NULL DEFAULT 'demo-reply',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
