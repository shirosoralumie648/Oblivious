-- Scheduled task foundation
-- Data model only; execution is intentionally handled outside this migration.

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

CREATE TABLE IF NOT EXISTS scheduled_task_runs (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scheduled_task_id TEXT NOT NULL REFERENCES scheduled_tasks(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_scheduled_task_runs_org_task_created
    ON scheduled_task_runs(organization_id, scheduled_task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_scheduled_task_runs_org_status_created
    ON scheduled_task_runs(organization_id, status, created_at DESC);
