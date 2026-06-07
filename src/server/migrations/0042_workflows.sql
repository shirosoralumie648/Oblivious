-- Workflow definition and execution persistence foundation

CREATE TABLE IF NOT EXISTS workflows (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    version INTEGER NOT NULL DEFAULT 1,
    definition JSONB NOT NULL DEFAULT '{}',
    variables JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('draft', 'published', 'archived')),
    CHECK (version >= 1)
);

CREATE INDEX IF NOT EXISTS idx_workflows_org_updated
    ON workflows(organization_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_workflows_org_status_updated
    ON workflows(organization_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS workflow_versions (
    workflow_id TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    definition JSONB NOT NULL DEFAULT '{}',
    variables JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workflow_id, version),
    CHECK (status IN ('draft', 'published', 'archived')),
    CHECK (version >= 1)
);

CREATE INDEX IF NOT EXISTS idx_workflow_versions_org_workflow_version
    ON workflow_versions(organization_id, workflow_id, version DESC);
CREATE INDEX IF NOT EXISTS idx_workflow_versions_org_status_version
    ON workflow_versions(organization_id, workflow_id, status, version DESC);

CREATE TABLE IF NOT EXISTS workflow_executions (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    workflow_version INTEGER NOT NULL DEFAULT 1,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    input JSONB NOT NULL DEFAULT '{}',
    output JSONB NOT NULL DEFAULT '{}',
    error JSONB NOT NULL DEFAULT '{}',
    context JSONB NOT NULL DEFAULT '{}',
    workflow_snapshot JSONB NOT NULL DEFAULT '{}',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('queued', 'running', 'paused', 'succeeded', 'completed', 'failed', 'cancelled', 'partial_success', 'timeout', 'max_iterations')),
    CHECK (workflow_version >= 1),
    CHECK (duration_ms >= 0)
);

CREATE INDEX IF NOT EXISTS idx_workflow_executions_org_workflow_started
    ON workflow_executions(organization_id, workflow_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_org_status_started
    ON workflow_executions(organization_id, status, started_at DESC);

CREATE TABLE IF NOT EXISTS workflow_node_executions (
    id TEXT PRIMARY KEY,
    execution_id TEXT NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL,
    node_type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 0,
    input JSONB NOT NULL DEFAULT '{}',
    output JSONB NOT NULL DEFAULT '{}',
    error JSONB NOT NULL DEFAULT '{}',
    context JSONB NOT NULL DEFAULT '{}',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('pending', 'running', 'retrying', 'succeeded', 'completed', 'failed', 'skipped')),
    CHECK (attempt >= 0),
    CHECK (duration_ms >= 0)
);

CREATE INDEX IF NOT EXISTS idx_workflow_node_executions_org_execution_created
    ON workflow_node_executions(organization_id, execution_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_workflow_node_executions_org_node_status
    ON workflow_node_executions(organization_id, node_id, status);
