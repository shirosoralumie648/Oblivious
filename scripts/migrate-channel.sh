#!/bin/bash
set -e

DB_PATH="${1:-./data/oblivious.db}"

sqlite3 "$DB_PATH" <<'SQL'
CREATE TABLE IF NOT EXISTS channel_configs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_id TEXT NOT NULL UNIQUE,
  config TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS update_channel_configs_timestamp
AFTER UPDATE ON channel_configs
BEGIN
  UPDATE channel_configs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
SQL

echo "Migration complete: channel_configs table created"
