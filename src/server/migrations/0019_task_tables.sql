-- Task tables: scheduled tasks and task execution records.

CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    cron_expression TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (btrim(name) <> ''),
    CHECK (target_type IN ('workflow', 'agent')),
    CHECK (btrim(cron_expression) <> '')
);

CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_org_enabled_created
    ON scheduled_tasks(organization_id, enabled, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_org_enabled_next_run
    ON scheduled_tasks(organization_id, enabled, next_run_at ASC);
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_org_target
    ON scheduled_tasks(organization_id, target_type, target_id);

CREATE TABLE IF NOT EXISTS task_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scheduled_task_id TEXT NOT NULL REFERENCES scheduled_tasks(id) ON DELETE CASCADE,
    trigger_type TEXT NOT NULL DEFAULT 'cron',
    status TEXT NOT NULL DEFAULT 'queued',
    input JSONB NOT NULL DEFAULT '{}',
    output JSONB NOT NULL DEFAULT '{}',
    error TEXT NOT NULL DEFAULT '',
    attempt INTEGER NOT NULL DEFAULT 1,
    max_retries INTEGER NOT NULL DEFAULT 3,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (trigger_type IN ('cron', 'manual', 'api', 'workflow_trigger')),
    CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled', 'timeout')),
    CHECK (attempt >= 1),
    CHECK (max_retries >= 0),
    CHECK (duration_ms >= 0)
);

CREATE INDEX IF NOT EXISTS idx_task_executions_org_task_created
    ON task_executions(organization_id, scheduled_task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_executions_org_status_created
    ON task_executions(organization_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_executions_org_trigger_type
    ON task_executions(organization_id, trigger_type, created_at DESC);
