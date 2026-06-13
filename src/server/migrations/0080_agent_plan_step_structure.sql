-- Structured durable Agent planning metadata.

ALTER TABLE agent_plan_steps
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS depends_on JSONB NOT NULL DEFAULT '[]';
