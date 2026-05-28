# Phase 22 Context: Backup Restore and Migration Recovery

## Milestone

v07 Production Operations.

## Why This Phase Exists

Phase 21 proved the application stack can be built, migrated, started, and smoked through an equivalent Docker compose production path. That still does not prove the platform is recoverable. A commercial multi-tenant SaaS cannot claim operations readiness unless PostgreSQL tenant data can be exported, restored into a fresh database, and checked against the append-only migration ledger.

This phase closes `OPS-03` only. It does not close observability, alerting, release/rollback, incident response, disaster recovery closeout, v08 product completeness, or final commercial readiness.

## Requirements

- **OPS-03:** Backup and restore runbooks plus automated smoke prove PostgreSQL tenant data can be backed up and restored into a fresh database with migration ledger integrity.

## Current Evidence And Gaps

Existing assets:

- `src/server/cmd/migrate/main.go` creates and verifies `schema_migrations` rows by filename and SHA-256 checksum.
- `src/server/migrations/*.sql` currently define the migration ledger, organization tenants, memberships, tenant-scoped core domains, quota/billing tables, Stripe ledger tables, Marketplace order/settlement/governance tables, and audit logs.
- `docker-compose.yml` now uses a pgvector-capable PostgreSQL image by default and Phase 21 validated a local pgvector fallback image for restricted-network environments.
- `scripts/deploy-validate.sh` runs `/usr/local/bin/oblivious-migrate` before runtime smoke.
- `.gitignore` ignores `.tmp/`, which is the right default location for generated local backup artifacts.

Current gaps:

- There is no operator-facing PostgreSQL backup script.
- There is no restore script that refuses unsafe inputs and verifies `schema_migrations` after restore.
- There is no automated backup/restore smoke that seeds commercial tenant data and restores it into a fresh database.
- There is no backup/restore runbook that defines prerequisites, retention, encryption boundary, failure handling, and evidence capture.
- Existing release docs mention deployment smoke but not PostgreSQL recovery evidence.

## Recovery Data Fixture Contract

The Phase 22 smoke fixture must prove the commercial state from v04-v06 survives backup/restore. At minimum, it should create and verify rows across these categories:

- Tenant identity: `users`, `organizations`, `organization_memberships`, and optionally `organization_invitations`.
- Quota and billing: `quotas`, `billing_sessions`, `packages`, `subscriptions`, `topup_orders`, `payment_intents`, `stripe_webhook_events`, `billing_lifecycle_events`, `billing_invoices`, and `billing_refunds`.
- Marketplace: `published_agents`, `agent_versions`, `agent_installs`, `marketplace_orders`, `marketplace_settlements`, `marketplace_payouts`, `marketplace_governance_events`, and `marketplace_abuse_reports`.
- Audit: `audit_logs` with `organization_id` populated after the tenant-scope migration.
- Migration ledger: `schema_migrations` count and checksums must match the checked-in migration files after restore.

The fixture should use deterministic IDs prefixed with `phase22_` so the verification queries can be specific and so a failed smoke is easy to inspect.

## Backup And Restore Design

Phase 22 should add three executable scripts:

- `scripts/backup-postgres.sh` exports a custom-format PostgreSQL dump with `pg_dump --format=custom --no-owner --no-privileges`, writes it under `.tmp/backups` by default, and writes a small manifest that excludes secrets.
- `scripts/restore-postgres.sh` restores a chosen dump into the target database with `pg_restore --clean --if-exists --no-owner --no-privileges`, then verifies `schema_migrations` against `src/server/migrations/*.sql`.
- `scripts/backup-restore-smoke.sh` creates or uses disposable source and restore databases, applies migrations to the source, inserts the commercial fixture, backs it up, restores into a fresh target, and verifies fixture rows plus migration ledger integrity.

PostgreSQL client tools should be resolved pragmatically: use host `pg_dump`, `pg_restore`, and `psql` when available; otherwise use Docker with `PG_CLIENT_IMAGE` or `OBLIVIOUS_POSTGRES_IMAGE` so hosts that already validated the Phase 21 pgvector image can still run the recovery smoke.

The smoke can support two modes:

1. Environment-provided databases through `BACKUP_SMOKE_SOURCE_DATABASE_URL` and `BACKUP_SMOKE_RESTORE_DATABASE_URL`.
2. Docker-managed disposable PostgreSQL containers when URLs are not provided, using `OBLIVIOUS_POSTGRES_IMAGE` and the Phase 21 local pgvector fallback path for restricted-network hosts.

The smoke must not commit dump files, secrets, database passwords, kubeconfig material, or runtime logs.

## Runbook Requirements

Create `docs/release/backup-restore-runbook.md` with:

- Required tools: host `pg_dump`, `pg_restore`, and `psql`, or Docker access to a PostgreSQL image that contains those client tools.
- Required environment variables and safe examples that use placeholders only.
- Backup storage expectations: generated dumps belong outside git, preferably encrypted at the storage layer or before off-host transfer.
- Restore boundary: restore into a fresh database first; production overwrite requires an explicit operator maintenance window and separate approval.
- Retention expectations: define minimum daily and pre-release backups without hard-coding a vendor-specific storage service.
- Failure handling: failed `pg_dump`, failed `pg_restore`, checksum mismatch, missing migration ledger rows, fixture mismatch, and partial restore cleanup.
- Evidence capture: exact command, environment class, dump manifest path, restore target class, migration ledger result, fixture verification result, skipped checks, and residual risk.

## Verification Targets

Focused RED/GREEN:

- `bash scripts/backup-postgres.sh` without a database URL should fail with a clear prerequisite message.
- `bash scripts/restore-postgres.sh` without `BACKUP_FILE` should fail with a clear prerequisite message.
- `bash scripts/backup-restore-smoke.sh` should fail if the restore target already contains user tables unless explicitly configured for an unsafe manual debug mode.

Broader phase verification:

- `bash scripts/backup-restore-smoke.sh`
- `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh` when remote pgvector pulls are blocked and the local fallback image exists.
- `bash scripts/check.sh docs`
- `git diff --check`

## Residual Risk After This Phase

Phase 22 should close only PostgreSQL backup/restore smoke and migration recovery evidence. The product remains non-commercial-complete until Phase 23 proves logs, metrics, tracing, alerts, dashboards, and SLOs; Phase 24 closes release/rollback/incident/DR evidence; and v08 completes customer-facing product behavior and final commercial journeys.
