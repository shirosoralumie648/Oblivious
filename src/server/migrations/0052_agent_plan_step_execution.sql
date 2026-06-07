-- Minimal execution state for durable Agent planning steps.

ALTER TABLE agent_plan_steps
    ADD COLUMN IF NOT EXISTS result_content TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

ALTER TABLE agent_plan_steps
    DROP CONSTRAINT IF EXISTS agent_plan_steps_status_check;

ALTER TABLE agent_plan_steps
    ADD CONSTRAINT agent_plan_steps_status_check
    CHECK (status IN ('pending', 'approved', 'running', 'completed', 'failed', 'skipped'));
