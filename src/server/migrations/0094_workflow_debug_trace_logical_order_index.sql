-- Keep Workflow debug trace reads ordered by logical node time without
-- forcing per-execution in-memory sorts once traces grow.
CREATE INDEX IF NOT EXISTS workflow_debug_trace_entries_org_execution_logical_created_idx
  ON workflow_debug_trace_entries (
    organization_id,
    execution_id,
    (COALESCE(started_at, created_at)) ASC,
    created_at ASC,
    id ASC
  );
