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
| Marketplace | `published_agents`, `agent_installs`, `agent_reviews`, `marketplace_orders`, `marketplace_settlements`, `marketplace_templates` |
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

Verification:

- `bash scripts/verify-migration-replay.sh`
- Result: first run applied 88 migrations, second run applied 0 and skipped 88, and `schema_migrations` reported 88 migrations recorded.

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

This slice moves the Database schema and migrations row forward by making migration ordering inputs explicit, preventing future malformed or ambiguous file additions, requiring checked-in migration evidence for every Part 3 core schema family, and proving a fresh PostgreSQL migration replay plus ledger skip rerun in the current Docker-capable environment. It does not by itself prove semantic idempotence for every data backfill path under non-empty legacy production datasets.

Remaining repository-owned work:

- Add focused idempotence checks for high-risk table families that still rely on direct migration application in package tests.
- Decide whether historical duplicate prefixes need a future forward-only ledger compatibility migration, instead of in-place renames.
