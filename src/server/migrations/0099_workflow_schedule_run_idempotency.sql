-- Durable Workflow schedule trigger idempotency by scheduled task run.

CREATE UNIQUE INDEX IF NOT EXISTS idx_workflow_executions_schedule_run_idempotency
    ON workflow_executions (
        organization_id,
        workflow_id,
        (context->'trigger'->>'scheduledTaskRunId')
    )
    WHERE context->'trigger'->>'type' = 'schedule'
      AND COALESCE(context->'trigger'->>'scheduledTaskRunId', '') <> '';
