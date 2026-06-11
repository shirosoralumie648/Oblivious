#!/bin/bash
set -e

DB_URL="${DATABASE_URL}"

psql "$DB_URL" <<SQL
CREATE TABLE IF NOT EXISTS subscriptions (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  plan VARCHAR(50) NOT NULL,
  status VARCHAR(20) NOT NULL,
  started_at TIMESTAMP NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS invoices (
  id SERIAL PRIMARY KEY,
  subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
  amount DECIMAL(10,2) NOT NULL,
  status VARCHAR(20) NOT NULL,
  issued_at TIMESTAMP NOT NULL DEFAULT NOW(),
  paid_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_invoices_subscription_id ON invoices(subscription_id);
SQL

echo "Migration completed"
