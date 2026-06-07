ALTER TABLE messages
ADD COLUMN IF NOT EXISTS bookmarked BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS message_shares (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (message_id)
);

ALTER TABLE message_shares
ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS message_shares_conversation_id_idx ON message_shares (conversation_id);
CREATE INDEX IF NOT EXISTS message_shares_organization_id_idx ON message_shares (organization_id);
CREATE INDEX IF NOT EXISTS message_shares_expires_at_idx ON message_shares (expires_at);

CREATE TABLE IF NOT EXISTS conversation_shares (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    start_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL,
    end_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL,
    url TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS conversation_shares_conversation_id_idx ON conversation_shares (conversation_id);
CREATE INDEX IF NOT EXISTS conversation_shares_organization_id_idx ON conversation_shares (organization_id);
CREATE INDEX IF NOT EXISTS conversation_shares_expires_at_idx ON conversation_shares (expires_at);
