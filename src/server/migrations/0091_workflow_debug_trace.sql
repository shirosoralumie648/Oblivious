-- Durable Workflow debug trace and variable snapshots.

CREATE TABLE IF NOT EXISTS workflow_debug_trace_entries (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  execution_id TEXT NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
  node_id TEXT NOT NULL DEFAULT '',
  node_type TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  attempt INTEGER NOT NULL DEFAULT 0,
  input JSONB NOT NULL DEFAULT '{}',
  output JSONB NOT NULL DEFAULT '{}',
  error JSONB NOT NULL DEFAULT '{}',
  context JSONB NOT NULL DEFAULT '{}',
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  duration_ms INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS workflow_debug_trace_entries_org_execution_created_idx
  ON workflow_debug_trace_entries (organization_id, execution_id, created_at ASC, id ASC);

CREATE TABLE IF NOT EXISTS workflow_debug_variable_snapshots (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  execution_id TEXT NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
  input JSONB NOT NULL DEFAULT '{}',
  context JSONB NOT NULL DEFAULT '{}',
  node_outputs JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS workflow_debug_variable_snapshots_org_execution_created_idx
  ON workflow_debug_variable_snapshots (organization_id, execution_id, created_at DESC, id DESC);
