-- Durable workflow execution transition audit.

CREATE TABLE IF NOT EXISTS workflow_execution_events (
  id TEXT PRIMARY KEY,
  execution_id TEXT NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
  organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  from_status TEXT NOT NULL DEFAULT '',
  to_status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (event_type IN ('created', 'status_changed')),
  CHECK (to_status <> '')
);

CREATE INDEX IF NOT EXISTS workflow_execution_events_org_execution_created_idx
  ON workflow_execution_events (organization_id, execution_id, created_at ASC, id ASC);

INSERT INTO workflow_execution_events (
  id,
  execution_id,
  organization_id,
  event_type,
  from_status,
  to_status,
  created_at
)
SELECT
  'wevt_' || md5(e.organization_id || ':' || e.id || ':' || e.status || ':' || e.created_at::TEXT),
  e.id,
  e.organization_id,
  'created',
  '',
  e.status,
  e.created_at
FROM workflow_executions e
ON CONFLICT (id) DO NOTHING;
