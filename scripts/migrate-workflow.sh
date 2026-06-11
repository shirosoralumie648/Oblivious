#!/bin/bash
set -e

DB_PATH="${DB_PATH:-./oblivious.db}"

sqlite3 "$DB_PATH" <<'SQL'
CREATE TABLE IF NOT EXISTS workflows (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  config TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_runs (
  id TEXT PRIMARY KEY,
  workflow_id TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  completed_at INTEGER,
  result TEXT,
  FOREIGN KEY (workflow_id) REFERENCES workflows(id)
);

CREATE INDEX IF NOT EXISTS idx_workflow_runs_workflow_id ON workflow_runs(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_runs_status ON workflow_runs(status);
SQL

echo "Workflow tables migrated successfully"
