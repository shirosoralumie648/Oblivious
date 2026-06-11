#!/bin/bash
set -e

DB_FILE="${1:-./data/oblivious.db}"

sqlite3 "$DB_FILE" <<'SQL'
CREATE TABLE IF NOT EXISTS scheduled_tasks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  schedule TEXT NOT NULL,
  command TEXT NOT NULL,
  enabled INTEGER DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
SQL

echo "Migration complete: scheduled_tasks table created"
