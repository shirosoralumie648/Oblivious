-- Relay file upload tenant ownership mapping.
-- POST /v1/files stores a local copy and the upstream OpenAI file id; this
-- table is the durable evidence needed before any file passthrough is enabled.

CREATE TABLE IF NOT EXISTS relay_file_mappings (
    local_file_id TEXT PRIMARY KEY,
    openai_file_id TEXT NOT NULL UNIQUE,
    local_path TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    user_id TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_relay_file_mappings_org_created
    ON relay_file_mappings(organization_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_relay_file_mappings_user_created
    ON relay_file_mappings(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_relay_file_mappings_request_id
    ON relay_file_mappings(request_id)
    WHERE request_id <> '';
