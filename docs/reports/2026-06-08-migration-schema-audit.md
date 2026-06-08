# Migration Schema Audit - 2026-06-08

This audit covers the repository-owned migration contract for the 2026-06-04 fusion specs, especially Part 3 database schema, deployment database configuration, and migration strategy requirements.

## Evidence Added

- `src/server/cmd/migrate/main.go` rejects checked-in or runtime `.sql` files whose names do not match `NNNN_description.sql`.
- `src/server/cmd/migrate/main_test.go` covers malformed migration filenames with `TestLoadMigrationFilesRejectsMalformedNames`.
- `scripts/verify-migration-contract.sh` statically checks every PostgreSQL migration in `src/server/migrations` before docs gates pass.
- `scripts/check.sh docs` now runs `scripts/verify-migration-contract.sh`, and `scripts/verify-quality-gates.sh` asserts the migration contract gate remains wired.

## Historical Duplicate Prefix Boundary

The current tree has accepted historical duplicate prefix pairs from `0013-0022`:

- `0013_channels.sql`, `0013_gateway_tables.sql`
- `0014_agents.sql`, `0014_relay_enhanced.sql`
- `0015_mcp_servers.sql`, `0015_workflow_enhanced.sql`
- `0016_knowledge_enhanced.sql`, `0016_pgvector.sql`
- `0017_agent_enhanced.sql`, `0017_quotas.sql`
- `0018_channel_tables.sql`, `0018_user_preferences_ext.sql`
- `0019_admin_role.sql`, `0019_task_tables.sql`
- `0020_marketplace_enhanced.sql`, `0020_memory_hnsw.sql`
- `0021_billing_enhanced.sql`, `0021_plan_extensions.sql`
- `0022_audit_logs.sql`, `0022_observability_tables.sql`

Do not rename historical migration files in-place. Their full filenames are already recorded as schema ledger versions, so renaming them would strand existing deployments or require a separate compatibility migration plan. The contract gate treats these exact pairs as accepted historical duplicate prefixes and fails if any new duplicate prefix appears or if the accepted set drifts.

## Current Completion Assessment

This slice moves the Database schema and migrations row forward by making migration ordering inputs explicit and preventing future malformed or ambiguous file additions. It does not by itself prove full Part 3 schema coverage or SQL idempotence for every table family.

Remaining repository-owned work:

- Audit Part 3 table families against actual migrations and service stores.
- Run DB-backed migration replay when `TEST_DATABASE_URL` is available.
- Add focused idempotence checks for high-risk table families that still rely on direct migration application in package tests.
- Decide whether historical duplicate prefixes need a future forward-only ledger compatibility migration, instead of in-place renames.
