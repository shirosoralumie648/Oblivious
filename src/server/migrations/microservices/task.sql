-- Task Microservice Schema
-- Tables: scheduled_tasks, task_executions

CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    schedule_type VARCHAR(50) NOT NULL CHECK (schedule_type IN ('cron', 'interval', 'once')),
    schedule_config JSONB NOT NULL,
    task_type VARCHAR(100) NOT NULL,
    task_payload JSONB,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_by UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    next_run_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_scheduled_tasks_is_enabled ON scheduled_tasks(is_enabled);
CREATE INDEX idx_scheduled_tasks_next_run_at ON scheduled_tasks(next_run_at);
CREATE INDEX idx_scheduled_tasks_created_by ON scheduled_tasks(created_by);

CREATE TABLE IF NOT EXISTS task_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES scheduled_tasks(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER,
    result JSONB,
    error TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_task_executions_task_id ON task_executions(task_id);
CREATE INDEX idx_task_executions_status ON task_executions(status);
CREATE INDEX idx_task_executions_started_at ON task_executions(started_at);
