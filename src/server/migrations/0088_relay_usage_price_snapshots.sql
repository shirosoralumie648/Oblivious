-- Immutable relay pricing snapshots on usage records for billing audit and reconciliation.

ALTER TABLE usage_records
  ADD COLUMN IF NOT EXISTS price_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS price_currency TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS price_source TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS price_effective_from TIMESTAMPTZ;
