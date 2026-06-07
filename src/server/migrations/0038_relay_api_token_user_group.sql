-- Relay API tokens can pin external /v1 requests to a routing group.

ALTER TABLE IF EXISTS relay_api_tokens
    ADD COLUMN IF NOT EXISTS user_group TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_relay_api_tokens_user_group
    ON relay_api_tokens(organization_id, user_group, status);
