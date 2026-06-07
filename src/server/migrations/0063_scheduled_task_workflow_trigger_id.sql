-- Stable identity for workflow-definition-backed scheduled tasks.

ALTER TABLE scheduled_tasks
    ADD COLUMN IF NOT EXISTS workflow_trigger_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_scheduled_tasks_workflow_trigger_unique
    ON scheduled_tasks(organization_id, target_type, target_id, workflow_trigger_id)
    WHERE target_type = 'workflow' AND workflow_trigger_id IS NOT NULL;
