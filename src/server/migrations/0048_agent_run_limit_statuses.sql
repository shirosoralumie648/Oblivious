-- Allow Agent run limit stops to be queried as first-class terminal states.

ALTER TABLE IF EXISTS agent_runs
  DROP CONSTRAINT IF EXISTS agent_runs_status_check;

ALTER TABLE IF EXISTS agent_runs
  ADD CONSTRAINT agent_runs_status_check
  CHECK (status IN (
    'running',
    'pending_approval',
    'completed',
    'failed',
    'max_iterations_reached',
    'token_budget_exceeded'
  ));
