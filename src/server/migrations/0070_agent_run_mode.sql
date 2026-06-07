ALTER TABLE IF EXISTS agent_runs
    ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT 'react';

ALTER TABLE IF EXISTS agent_runs
    DROP CONSTRAINT IF EXISTS agent_runs_mode_check;

ALTER TABLE IF EXISTS agent_runs
    ADD CONSTRAINT agent_runs_mode_check
    CHECK (mode IN ('react', 'planning'));
