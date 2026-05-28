# Phase 22 Verification: Backup Restore and Migration Recovery

**Status:** Complete for `OPS-03` PostgreSQL backup/restore smoke.
**Updated:** 2026-05-28

## Scope

Phase 22 proves PostgreSQL backup export, restore into a fresh database, migration ledger integrity verification, and commercial tenant data survival for organization, membership, quota/billing, Marketplace, and audit rows.

This evidence does not close observability, alerts, dashboards, release/rollback, incident response, disaster recovery closeout, v08 product completeness, or final commercial readiness.

## Evidence

| Command | Result | Evidence |
| --- | --- | --- |
| `env -u BACKUP_DATABASE_URL -u DATABASE_URL bash scripts/backup-postgres.sh` | Expected fail, exit 2 | Backup script fails with `[backup-postgres] BACKUP_DATABASE_URL or DATABASE_URL is required`, proving it does not silently choose a database. |
| `env -u BACKUP_FILE RESTORE_DATABASE_URL=postgres://example:example@127.0.0.1:1/example bash scripts/restore-postgres.sh` | Expected fail, exit 2 | Restore script fails with `[restore-postgres] BACKUP_FILE is required` before attempting a network connection. |
| `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh` before setting migrate env | Expected fail, exit 1 | Migration runner required `SESSION_SECRET`; the smoke was fixed to pass a non-secret placeholder for migration-only execution. |
| `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh` before checksum query fix | Expected fail, exit 1 | Restore reached checksum verification but `psql -c` did not expand `:'version'`; restore script now uses explicit SQL string escaping for checked-in migration filenames. |
| `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh` | Pass | Started disposable source/restore pgvector PostgreSQL databases, applied 30 migrations, seeded Phase 22 commercial fixture, wrote custom dump and manifest under `.tmp/backups`, restored into a fresh target, verified 30 `schema_migrations` rows/checksums, and verified all fixture categories. |

## Restored Fixture Categories

The passing smoke verified:

- `schema_migrations`: 30 checked-in migration filenames/checksums.
- Tenant identity: 3 `users`, 2 `organizations`, 3 `organization_memberships`.
- Quota/billing: 2 `quotas`, 1 `billing_sessions`, 3 `payment_intents`, 1 `stripe_webhook_events`, 1 `billing_lifecycle_events`, 1 `billing_invoices`, 1 `billing_refunds`.
- Marketplace: 1 `published_agents`, 1 `marketplace_orders`, 1 `marketplace_settlements`, 1 `marketplace_payouts`, 1 `marketplace_governance_events`, 1 `marketplace_abuse_reports`.
- Audit: 1 `audit_logs` row with `organization_id = phase22_org_publisher`.

## Closed In Phase 22

- `OPS-03`: Complete. PostgreSQL backup/restore runbook and automated smoke prove tenant-commercial data can be backed up and restored into a fresh database with migration ledger integrity.

## Residual Work

- `OPS-02` remains partially open for Phase 24 release-path evidence.
- `OPS-04` and `OPS-05` observability, alert, dashboard, and SLO evidence remain Phase 23.
- `OPS-06` and `DOC-06` release/rollback/incident/DR runbook and v07 closeout evidence remain Phase 24.
- Kubernetes cluster proof remains unavailable until `kubectl` and a reachable cluster are provided.
- v08 Product Completeness and final commercial readiness remain open.
