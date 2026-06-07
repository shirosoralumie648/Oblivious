ALTER TABLE conversations
ADD COLUMN IF NOT EXISTS parent_id TEXT REFERENCES conversations(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS conversations_parent_id_idx ON conversations(parent_id);
