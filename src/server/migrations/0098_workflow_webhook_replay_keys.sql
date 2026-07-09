-- Durable Workflow trigger replay prevention.

CREATE TABLE IF NOT EXISTS workflow_webhook_replay_keys (
  replay_key TEXT PRIMARY KEY,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS workflow_webhook_replay_keys_expires_idx
  ON workflow_webhook_replay_keys (expires_at);
