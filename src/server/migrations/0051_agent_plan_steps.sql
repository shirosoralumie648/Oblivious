-- Durable Agent planning steps for approval and execution loops.

CREATE TABLE IF NOT EXISTS agent_plan_steps (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    run_id TEXT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    step_index INTEGER NOT NULL,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    approval_status TEXT NOT NULL DEFAULT 'not_required',
    tool_name TEXT NOT NULL DEFAULT '',
    input JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (step_index > 0),
    CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    UNIQUE (organization_id, run_id, step_index)
);

CREATE INDEX IF NOT EXISTS idx_agent_plan_steps_org_run_index
    ON agent_plan_steps(organization_id, run_id, step_index ASC);

