# Phase 22 Summary: Backup Restore and Migration Recovery

**Status:** Complete
**Completed:** 2026-05-28
**Requirements:** `OPS-03`

## Delivered

- Added `scripts/backup-postgres.sh` for custom-format PostgreSQL dumps with non-secret manifests under `.tmp/backups` by default.
- Added `scripts/restore-postgres.sh` for `pg_restore` plus `schema_migrations` filename/checksum verification against `src/server/migrations/*.sql`.
- Added `scripts/backup-restore-smoke.sh` for disposable pgvector PostgreSQL source/restore databases, migration application, commercial fixture seeding, backup, restore, and fixture verification.
- Added `docs/release/backup-restore-runbook.md` covering prerequisites, backup, restore, smoke, retention, encryption boundary, failure handling, and evidence capture.
- Updated release docs and quality gates so backup/restore artifacts are covered by `bash scripts/check.sh docs`.

## Verification

- `env -u BACKUP_DATABASE_URL -u DATABASE_URL bash scripts/backup-postgres.sh` failed with the expected prerequisite message.
- `env -u BACKUP_FILE RESTORE_DATABASE_URL=postgres://example:example@127.0.0.1:1/example bash scripts/restore-postgres.sh` failed with the expected prerequisite message.
- `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh` passed.
- `bash scripts/check.sh docs` passed.
- `git diff --check` passed.

## Evidence

Detailed command results are recorded in `.planning/phases/22-backup-restore-and-migration-recovery/22-VERIFICATION.md`.

## Boundary

Phase 22 closes only `OPS-03`. v07 remains active because observability, alerting, release/rollback, incident response, disaster recovery closeout, and `DOC-06` are still open. v08 Product Completeness and final commercial readiness remain required.
