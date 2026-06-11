#!/bin/bash
set -e

psql "$DATABASE_URL" <<SQL
CREATE TABLE IF NOT EXISTS agents (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  config JSONB,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_runs (
  id SERIAL PRIMARY KEY,
  agent_id INTEGER REFERENCES agents(id),
  status VARCHAR(50),
  result JSONB,
  started_at TIMESTAMP DEFAULT NOW(),
  completed_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_runs_agent_id ON agent_runs(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_runs_status ON agent_runs(status);
SQL

echo "Migration complete"
