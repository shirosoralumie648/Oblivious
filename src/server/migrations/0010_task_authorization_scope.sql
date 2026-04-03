ALTER TABLE tasks
  ADD COLUMN IF NOT EXISTS authorization_scope TEXT NOT NULL DEFAULT 'workspace_tools';
