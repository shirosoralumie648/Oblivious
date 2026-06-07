-- Persist Agent tool risk policy decisions with durable tool runs.

ALTER TABLE IF EXISTS agent_tool_runs
  ADD COLUMN IF NOT EXISTS risk_level TEXT NOT NULL DEFAULT '';
