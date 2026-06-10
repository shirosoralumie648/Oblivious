# Migration Schema Audit - 2026-06-08

This audit covers the repository-owned migration contract for the 2026-06-04 fusion specs, especially Part 3 database schema, deployment database configuration, and migration strategy requirements.

## Evidence Added

- `src/server/cmd/migrate/main.go` rejects checked-in or runtime `.sql` files whose names do not match `NNNN_description.sql`.
- `src/server/cmd/migrate/main_test.go` covers malformed migration filenames with `TestLoadMigrationFilesRejectsMalformedNames`.
- `scripts/verify-migration-contract.sh` statically checks every PostgreSQL migration in `src/server/migrations` before docs gates pass.
- `scripts/verify-schema-coverage.sh` statically checks Part 3 core schema family coverage across `src/server/migrations` and `src/server/migrations/clickhouse`.
- `scripts/verify-migration-replay.sh` starts a temporary pgvector PostgreSQL database when no `MIGRATION_REPLAY_DATABASE_URL` or `TEST_DATABASE_URL` is set, applies all migrations, reruns the migrator, and verifies the `schema_migrations` count.
- `scripts/check.sh docs` now runs `scripts/verify-migration-contract.sh`, and `scripts/verify-quality-gates.sh` asserts the migration contract gate remains wired.

## Part 3 Core Schema Family Coverage

`scripts/verify-schema-coverage.sh` maps the Part 3 core schema families to migration evidence:

| Part 3 core schema family | Required migration evidence |
| --- | --- |
| User and organization | `organizations`, `users`, `organization_memberships` |
| Chat | `conversations`, `messages`, `personas`, conversation `parent_id` |
| Relay and gateway | `channels`, `rate_limit_counters`, `relay_semantic_cache`, `relay_metrics` |
| Workflow | `workflows`, `workflow_executions`, `workflow_node_executions`, `workflow_versions` |
| Knowledge and RAG | `knowledge_bases`, `knowledge_documents`, `knowledge_document_chunks`, `vector` extension |
| Agent | `agent_runs`, `agent_tool_runs`, `agent_memories`, `agent_plan_steps` |
| Billing | `subscriptions`, `payment_intents`, `quotas`, `concurrency_limits`, `token_rate_limits`, billing lifecycle/invoice/refund tables |
| Marketplace | `published_agents`, `agent_installs`, `agent_reviews`, `marketplace_orders`, `marketplace_settlements`, `marketplace_templates`, `published_agents.category_id` NOT NULL and `categories(id)` FK |
| Channel | `channel_configs`, `channel_messages` |
| Task | `scheduled_tasks`, `task_executions` |
| Observability | ClickHouse `request_logs`, PostgreSQL alert state and delivery attempt tables |

This is a static coverage gate. It proves that each Part 3 core schema family has checked-in migration evidence, including the ClickHouse observability table. It does not prove that a fresh PostgreSQL instance can replay all migrations in this environment.

## Full Migration Replay

Fresh PostgreSQL replay initially exposed two ordering defects:

- `0013_gateway_tables.sql` referenced `organizations` before the original tenant foundation migration created it.
- `0016_knowledge_enhanced.sql` created organization-scoped indexes on `knowledge_bases`, but the table already existed from the v03 migration without the enhanced columns.

The fix is forward-only and avoids renaming historical files:

- `0000_tenant_prerequisites.sql` creates the minimal `organizations` prerequisite table before service migrations that reference it.
- `0016_000_knowledge_enhanced_prerequisites.sql` backfills enhanced `knowledge_bases` columns before `0016_knowledge_enhanced.sql` creates organization-scoped indexes.
- `0076_organization_created_by_fk.sql` adds the `organizations.created_by_user_id` foreign key after `users` is guaranteed to exist.
- `0079_marketplace_category_id_fk.sql` normalizes legacy Marketplace category slugs, blank values, and unknown values to stable `categories.id` values before enforcing the `published_agents.category_id` NOT NULL foreign key.

Verification:

- `bash scripts/verify-migration-replay.sh`
- Result: first run applied 91 migrations, second run applied 0 and skipped 91, and `schema_migrations` reported 91 migrations recorded.

## Legacy Tenant Backfill Fixture

`TestApplyMigrationsBackfillsLegacyTenantScopeData` covers the high-risk non-empty legacy migration path:

1. Apply migrations through `0026_membership_auth_security.sql`.
2. Seed legacy user/workspace/session/chat/message/config/binding, Knowledge, Agent, memory, MCP, usage/quota/billing/subscription/top-up, Marketplace, and audit rows without `organization_id`.
3. Apply the full migration directory.
4. Assert all tenant-scoped legacy tables have non-null `organization_id`.
5. Rerun the full migrator and assert no migrations are reapplied.

Verification:

- `TEST_DATABASE_URL=<temporary pgvector PostgreSQL> go test ./cmd/migrate -run TestApplyMigrationsBackfillsLegacyTenantScopeData -count=1 -v`
- Result: passed.

## Marketplace Category ID Backfill Fixture

`TestApplyMigrationsBackfillsMarketplaceCategoryIDs` covers the Marketplace-specific legacy data-shape introduced before publishing required stable category IDs:

1. Apply migrations through `0078_marketplace_order_failed_status.sql`.
2. Seed published agents whose `category_id` values contain a legacy slug, a blank string, an unknown value, and an already-valid category ID.
3. Apply the full migration directory, including `0079_marketplace_category_id_fk.sql`.
4. Assert the legacy slug, blank value, and unknown value are backfilled to a valid productivity category ID while the already-valid ID is preserved.
5. Assert no invalid category references remain, the `published_agents_category_id_fkey` constraint exists, `published_agents.category_id` is NOT NULL, invalid inserts fail, null inserts fail, and a second full migrator run applies zero migrations.

Verification:

- `TEST_DATABASE_URL=<temporary pgvector PostgreSQL> go test ./cmd/migrate -run TestApplyMigrationsBackfillsMarketplaceCategoryIDs -count=1 -v`
- Local no-database runs compile the test but skip the DB-backed assertions when `TEST_DATABASE_URL` is unset.

## Historical Duplicate Prefix Boundary

The current tree has accepted historical duplicate prefix pairs from `0013-0022`:

- `0013_channels.sql`, `0013_gateway_tables.sql`
- `0014_agents.sql`, `0014_relay_enhanced.sql`
- `0015_mcp_servers.sql`, `0015_workflow_enhanced.sql`
- `0016_000_knowledge_enhanced_prerequisites.sql`, `0016_knowledge_enhanced.sql`, `0016_pgvector.sql`
- `0017_agent_enhanced.sql`, `0017_quotas.sql`
- `0018_channel_tables.sql`, `0018_user_preferences_ext.sql`
- `0019_admin_role.sql`, `0019_task_tables.sql`
- `0020_marketplace_enhanced.sql`, `0020_memory_hnsw.sql`
- `0021_billing_enhanced.sql`, `0021_plan_extensions.sql`
- `0022_audit_logs.sql`, `0022_observability_tables.sql`

Do not rename historical migration files in-place. Their full filenames are already recorded as schema ledger versions, so renaming them would strand existing deployments or require a separate compatibility migration plan. The contract gate treats these exact pairs as accepted historical duplicate prefixes and fails if any new duplicate prefix appears or if the accepted set drifts.

## Current Completion Assessment

This slice moves the Database schema and migrations row forward by making migration ordering inputs explicit, preventing future malformed or ambiguous file additions, requiring checked-in migration evidence for every Part 3 core schema family, proving a fresh PostgreSQL migration replay plus ledger skip rerun in the current Docker-capable environment, covering the highest-risk non-empty legacy tenant backfill path with a DB-backed fixture, and enforcing Marketplace category IDs at the database layer after a compatibility backfill. It does not by itself prove every possible production data-shape variant.

Remaining repository-owned work:

- Decide whether historical duplicate prefixes need a future forward-only ledger compatibility migration, instead of in-place renames.
