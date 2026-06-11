#!/bin/bash
set -e

psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<-EOSQL
CREATE TABLE IF NOT EXISTS marketplace_agents (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    version VARCHAR(50),
    author VARCHAR(255),
    price DECIMAL(10,2) DEFAULT 0.00,
    downloads INTEGER DEFAULT 0,
    rating DECIMAL(3,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_marketplace_agents_name ON marketplace_agents(name);
CREATE INDEX IF NOT EXISTS idx_marketplace_agents_author ON marketplace_agents(author);
EOSQL

echo "marketplace_agents table migrated successfully"
