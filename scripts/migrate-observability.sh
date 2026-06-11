#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

source "$SCRIPT_DIR/db-functions.sh"

check_psql

echo "Migrating alert_configs table..."

psql "$DATABASE_URL" <<SQL
CREATE TABLE IF NOT EXISTS alert_configs (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  metric VARCHAR(255) NOT NULL,
  condition VARCHAR(50) NOT NULL,
  threshold NUMERIC NOT NULL,
  severity VARCHAR(50) NOT NULL,
  enabled BOOLEAN DEFAULT true,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_alert_configs_metric ON alert_configs(metric);
CREATE INDEX IF NOT EXISTS idx_alert_configs_enabled ON alert_configs(enabled);
SQL

echo "Migration completed successfully."
