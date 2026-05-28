# PostgreSQL Backup And Restore Runbook

This runbook defines the v07 `OPS-03` recovery path for Oblivious. It proves PostgreSQL tenant data can be exported, restored into a fresh database, and checked against the append-only `schema_migrations` ledger.

## Prerequisites

- `bash`, `sha256sum`, Docker, and Go for the automated smoke path.
- Host `pg_dump`, `pg_restore`, and `psql`, or Docker access to a PostgreSQL image that contains those client tools.
- A pgvector-capable PostgreSQL image for disposable smoke databases. Use `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16` when the local Phase 21 fallback image is available.
- Never commit database URLs, passwords, backup dumps, manifests with secrets, kubeconfig material, provider keys, or Stripe secrets.

## Backup

Use `BACKUP_DATABASE_URL` for the source database and keep generated files under `.tmp/backups` or another ignored/operator-controlled directory.

```bash
BACKUP_DATABASE_URL=postgres://USER:REPLACE_ME@HOST:5432/oblivious?sslmode=require \
  BACKUP_DIR=.tmp/backups \
  bash scripts/backup-postgres.sh
```

The script writes:

- `<basename>.dump`: `pg_dump --format=custom --no-owner --no-privileges`
- `<basename>.manifest`: timestamp, dump filename, byte size, SHA-256 checksum, sanitized database URL class, and client mode

If host PostgreSQL client tools are missing, set `PG_CLIENT_IMAGE` or `OBLIVIOUS_POSTGRES_IMAGE` so Docker can provide `pg_dump`:

```bash
PG_CLIENT_IMAGE=postgres:16 \
  BACKUP_DATABASE_URL=postgres://USER:REPLACE_ME@HOST:5432/oblivious?sslmode=require \
  bash scripts/backup-postgres.sh
```

## Restore

Restore into a fresh database first. Production overwrite requires an explicit maintenance window, separate operator approval, and a verified rollback path.

```bash
RESTORE_DATABASE_URL=postgres://USER:REPLACE_ME@RESTORE_HOST:5432/oblivious_restore?sslmode=require \
  BACKUP_FILE=.tmp/backups/oblivious-YYYYmmdd-HHMMSS.dump \
  bash scripts/restore-postgres.sh
```

The restore script runs `pg_restore --clean --if-exists --no-owner --no-privileges`, then verifies every `src/server/migrations/*.sql` filename and SHA-256 checksum against `schema_migrations`. It also fails if extra migration ledger rows exist.

## Automated Smoke

The default smoke starts disposable source and restore databases with Docker, applies migrations, seeds commercial tenant data, backs up the source, restores into the fresh target, and verifies:

- `schema_migrations`
- `users`, `organizations`, `organization_memberships`
- `quotas`, `billing_sessions`, `payment_intents`, `stripe_webhook_events`, `billing_lifecycle_events`, `billing_invoices`, `billing_refunds`
- `published_agents`, `agent_versions`, `agent_installs`, `marketplace_orders`, `marketplace_settlements`, `marketplace_payouts`, `marketplace_governance_events`, `marketplace_abuse_reports`
- `audit_logs`

```bash
OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 \
  bash scripts/backup-restore-smoke.sh
```

To use externally provisioned disposable databases instead:

```bash
BACKUP_SMOKE_SOURCE_DATABASE_URL=postgres://USER:REPLACE_ME@SOURCE_HOST:5432/oblivious_source?sslmode=require \
  BACKUP_SMOKE_RESTORE_DATABASE_URL=postgres://USER:REPLACE_ME@RESTORE_HOST:5432/oblivious_restore?sslmode=require \
  bash scripts/backup-restore-smoke.sh
```

The restore target must be fresh unless `BACKUP_RESTORE_ALLOW_NON_EMPTY=true` is set for manual debugging. Do not use that override for release evidence.

## Retention

- Keep at least daily backups for the active production retention window defined by the deployment owner.
- Take a pre-release backup before migrations, deployment validation, or manual data repair.
- Keep backup manifests with evidence, but store dumps in encrypted operator-controlled storage outside git.
- Record retention policy, storage class, and restore test date in release evidence.

## Encryption Boundary

The scripts create local dump files only. Encryption is the operator responsibility before off-host transfer. Use encrypted disks, encrypted object storage, or an explicit encryption step before copying dumps outside the trusted host.

## Failure Handling

- `pg_dump` failure: do not delete the last known good backup; capture command output and retry only after validating database connectivity and disk capacity.
- `pg_restore` failure: discard the restore target and retry into a fresh database after fixing the error.
- `schema_migrations` mismatch: treat the restore as invalid; compare the dump source commit and current migration files before using restored data.
- Fixture mismatch in `backup-restore-smoke`: treat `OPS-03` as not proven and inspect the named table category from the smoke output.
- Missing Docker or pgvector image: record the exact non-success output and use the Phase 21 fallback image path before declaring blocked.

## Evidence Capture

For `OPS-03`, record:

- Exact command.
- Environment class: local disposable Docker, local external disposable Postgres, CI service Postgres, or production-like staging.
- Backup dump manifest path, not the dump contents.
- Restore target class.
- `schema_migrations` count/checksum result.
- Commercial fixture categories verified.
- Skipped checks and accepted residual risk.

Phase 22 evidence does not close v07 Operations Gate by itself. Observability, alerts, dashboards, SLOs, release/rollback, incident response, disaster recovery closeout, v08 product completeness, and final commercial readiness remain separate required work.
