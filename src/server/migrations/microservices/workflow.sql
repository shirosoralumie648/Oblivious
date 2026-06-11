-- Workflow Microservice Schema
-- Tables: workflows, workflow_runs, workflow_runtime_state, workflow_node_executions

CREATE TABLE IF NOT EXISTS workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    definition JSONB NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_workflows_created_by ON workflows(created_by);
CREATE INDEX idx_workflows_is_active ON workflows(is_active);

CREATE TABLE IF NOT EXISTS workflow_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    started_by UUID NOT NULL,
    input JSONB,
    output JSONB,
    error TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_workflow_runs_workflow_id ON workflow_runs(workflow_id);
CREATE INDEX idx_workflow_runs_status ON workflow_runs(status);
CREATE INDEX idx_workflow_runs_started_by ON workflow_runs(started_by);
CREATE INDEX idx_workflow_runs_started_at ON workflow_runs(started_at);

CREATE TABLE IF NOT EXISTS workflow_runtime_state (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    current_node VARCHAR(255),
    variables JSONB NOT NULL DEFAULT '{}',
    context JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_workflow_runtime_state_run_id ON workflow_runtime_state(run_id);

CREATE TABLE IF NOT EXISTS workflow_node_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    node_id VARCHAR(255) NOT NULL,
    node_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'skipped')),
    input JSONB,
    output JSONB,
    error TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER
);

CREATE INDEX idx_workflow_node_executions_run_id ON workflow_node_executions(run_id);
CREATE INDEX idx_workflow_node_executions_node_id ON workflow_node_executions(node_id);
CREATE INDEX idx_workflow_node_executions_status ON workflow_node_executions(status);
