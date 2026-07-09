-- Relay pricing catalog import, approval, and audit history.

CREATE TABLE IF NOT EXISTS relay_pricing_catalog_imports (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL,
  source TEXT NOT NULL,
  source_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'approved', 'rejected')),
  notes TEXT NOT NULL DEFAULT '',
  deactivate_missing BOOLEAN NOT NULL DEFAULT false,
  imported_by TEXT NOT NULL DEFAULT '',
  imported_by_email TEXT NOT NULL DEFAULT '',
  approved_by TEXT NOT NULL DEFAULT '',
  approved_by_email TEXT NOT NULL DEFAULT '',
  entries JSONB NOT NULL DEFAULT '[]'::jsonb,
  diff JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  approved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_relay_pricing_catalog_imports_status_created
  ON relay_pricing_catalog_imports (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_relay_pricing_catalog_imports_provider_source
  ON relay_pricing_catalog_imports (provider, source, created_at DESC);

CREATE TABLE IF NOT EXISTS relay_pricing_catalog_events (
  id TEXT PRIMARY KEY,
  import_id TEXT REFERENCES relay_pricing_catalog_imports(id) ON DELETE SET NULL,
  pricing_entry_id TEXT,
  action TEXT NOT NULL
    CHECK (action IN ('import_created', 'approved', 'rejected', 'entry_added', 'entry_updated', 'entry_deactivated')),
  actor_id TEXT NOT NULL DEFAULT '',
  actor_email TEXT NOT NULL DEFAULT '',
  before JSONB NOT NULL DEFAULT '{}'::jsonb,
  after JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_relay_pricing_catalog_events_import_created
  ON relay_pricing_catalog_events (import_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_relay_pricing_catalog_events_pricing_entry
  ON relay_pricing_catalog_events (pricing_entry_id, created_at DESC);
