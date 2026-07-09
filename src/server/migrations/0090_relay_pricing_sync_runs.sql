-- Relay pricing provider-source sync and reconciliation run ledger.

CREATE TABLE IF NOT EXISTS relay_pricing_sync_runs (
  id TEXT PRIMARY KEY,
  job TEXT NOT NULL
    CHECK (job IN ('freshness', 'reconciliation')),
  provider TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  source_ref TEXT NOT NULL DEFAULT '',
  source_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL
    CHECK (status IN ('pending_import', 'unchanged', 'succeeded', 'issues_found', 'failed')),
  import_id TEXT REFERENCES relay_pricing_catalog_imports(id) ON DELETE SET NULL,
  entry_count INTEGER NOT NULL DEFAULT 0 CHECK (entry_count >= 0),
  skipped_count INTEGER NOT NULL DEFAULT 0 CHECK (skipped_count >= 0),
  checked_records INTEGER NOT NULL DEFAULT 0 CHECK (checked_records >= 0),
  issue_count INTEGER NOT NULL DEFAULT 0 CHECK (issue_count >= 0),
  error TEXT NOT NULL DEFAULT '',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_relay_pricing_sync_runs_job_status_finished
  ON relay_pricing_sync_runs (job, status, finished_at DESC);

CREATE INDEX IF NOT EXISTS idx_relay_pricing_sync_runs_provider_source_finished
  ON relay_pricing_sync_runs (provider, source, finished_at DESC);

CREATE INDEX IF NOT EXISTS idx_relay_pricing_sync_runs_import
  ON relay_pricing_sync_runs (import_id);
