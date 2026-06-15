# Repository Rescan - 2026-06-16

## Current Truth

- Branch: `main`.
- Scan base commit: `0694b44ba95e11f48aff568516b473da54683dc3` (`test(quota): aggregate lifecycle db evidence`).
- Remote parity at scan start: `HEAD == origin/main`.
- Working tree at scan start: dirty with the target live evidence manifest verifier slice in progress.
- The earlier same-day scans at `1c52194`, `53aaca0`, `d4fdc37`, `75ff216`, `98a1683`, `c6d9b34`, `5969e4b`, `1e7c6ef`, and `e222967` are now older baselines for this report; they did not include the latest quota lifecycle aggregation or target/live evidence manifest verifier gate.
- The project is still not complete against the four `docs/superpowers/specs/2026-06-04-*` specs.
- Current matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate remains `99/100`: repository-local evidence is broad, but final completion is still gated by target/live proof and a strict no-skip release run.

## What Changed Since The Last Rescan

- Agent action IDs now fail closed when snake_case and camelCase aliases conflict for tool-run and plan-step actions.
- PlanningEngine tool steps now have direct ReAct handoff proof.
- MCP auth-token PostgreSQL evidence and strict verifier coverage were tightened.
- Agent gRPC generated-client coverage and Workflow/Task generated-client proof remain in the checked-in evidence set.
- The prior pushed memory slice proves LLM-assisted long-term memory extraction composes with `memory_key_consolidate`: generated keyed facts update the existing long-term memory in place instead of creating duplicates.
- Manual Agent plan-step draft insert/move/delete now preserves `dependsOn` references by logical step. Deleting a draft step that another step depends on fails closed instead of silently loosening the plan.
- Marketplace paid audit rows are now protected from publisher hard-delete, direct SQL cascade, and buyer uninstall regressions. `DeleteAgent` rejects hard deletion once marketplace order audit evidence exists for the agent in the publisher organization, and migration `0082_marketplace_audit_retention.sql` rebuilds paid audit foreign keys with non-cascading delete semantics.
- Scheduled-task due-worker failures now advance `next_run_at` through a dedicated failure path, so a failed claimed run cannot be immediately reclaimed while its old due time remains in the past.
- Scheduled Task HTTP routes now have active-organization isolation proof for run-history, status-update, and run-now paths. Cross-organization requests return 404, do not leak owner run evidence, do not mutate the owner task, and do not create extra owner runs.
- Admin top-up refund operator evidence now has no-skip PostgreSQL route proof: duplicate Admin Stripe refund submissions persist one refund row and one lifecycle transition with provider charge/payment-intent evidence, while missing Stripe charge/payment-intent evidence returns 400 without mutating refund, lifecycle, payment-intent, top-up, or quota state.
- Quota SQL isolation now aggregates quota lifecycle proof across quota store, HTTP route, Admin, and Stripe lifecycle tests in one no-skip PostgreSQL profile, including user-scoped balance mode, request-cap fallback, top-up no-credit-before-webhook, direct top-up rejection, Admin refund quota reversal, and Stripe top-up/refund accounting.
- Target/live release evidence now has a machine-readable manifest verifier in progress. The strict commercial verifier requires `COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true` for final readiness, and the manifest validator checks exact commit, strict no-skip verifier result, deployment/backup/migration evidence, Kubernetes validation/rollout/failover evidence, live provider checkout/refund/payout/reconciliation evidence, Agent/Workflow/Task generated-client gRPC smoke evidence, target secret audit, workflow telemetry `successRate >= 0.99`, no skip-like fields, and no embedded secret material.

## Repository Inventory

- First-party tracked files after this target-evidence verifier slice:
  - `src`: 994
  - `docs`: 93
  - `scripts`: 38
  - `deploy`: 42
  - `.planning`: 210
  - other tracked files: 27
  - total tracked files: 1404
- Server internal shape:
  - `src/server/internal`: 590 tracked files.
  - largest active domains: `relay` 110, `http` 104, `mcp` 43, `admin` 39, `agent` 34, `workflow` 31, `knowledge` 30, `observability` 28, `channel` 27, `migration` 24, `marketplace` 16.
- Test inventory:
  - Go test files: 230
  - Web component/API test files: 67
  - Web Playwright specs: 16
  - Web E2E fixture files: 16
- Migration inventory:
  - Top-level PostgreSQL SQL migration files: 94.
  - ClickHouse SQL migration files: 1.
  - Microservice split SQL migration files: 12.
  - Microservice split migrations exist for `admin`, `agent`, `billing`, `channel`, `chat`, `gateway`, `marketplace`, `observability`, `rag`, `relay`, `task`, and `workflow`.
- First-party `AGENTS.md`: none found after excluding `.git`, `node_modules`, `.tmp`, `src/web/node_modules`, and `reference`.

## Spec Surface

The active completion target is still the four June 4 specs:

- `2026-06-04-complete-fusion-design.md`: Gateway/Relay, Workflow, Knowledge/RAG.
- `2026-06-04-complete-fusion-design-part2.md`: Agent, Billing, Marketplace, Publishing channels, frontend, roadmap.
- `2026-06-04-complete-fusion-design-part3.md`: schema, API, Kubernetes deployment, monitoring, security, migration, success criteria.
- `2026-06-04-functional-logic-details.md`: detailed logic for Relay, Workflow, RAG, Agent, Publishing, Billing, Frontend UI, Marketplace, Observability.

## Completion Matrix Snapshot

Proven rows:

- API gateway and relay.
- Knowledge base and RAG.
- Multi-channel publishing.
- Database schema and migrations.

Partial rows:

- Workflow engine.
- Agent system.
- Billing and monetization.
- Marketplace ecosystem.
- Frontend shell and core pages.
- Observability metrics, logs, alerts, recovery.
- API contract.
- Deployment and operations.
- Security and tenant isolation.
- Migration strategy and release readiness.

No rows are currently marked `Gap` or `Unverified`.

## Evidence Boundary

- `docs/release/fusion-spec-evidence-pack.md` still states that the pack is not a final completion claim; any `Partial` row remains open until row-specific proof is recorded and rerun on the target environment where required.
- `scripts/verify-target-release-evidence.sh` now provides the external target/live evidence manifest gate for proof that cannot be collected from repository-local tests.
- `scripts/verify-commercial-db-evidence.sh` currently exposes 24 focused profiles plus the `all` aggregator:
  - `backend-journey`
  - `marketplace-money-movement`
  - `marketplace-governance-review`
  - `marketplace-recommendation-search`
  - `marketplace-template-routes`
  - `billing-checkout-topup-http`
  - `billing-provider-lifecycle`
  - `admin-usage-analytics-db`
  - `app-stateful-routes`
  - `tenant-membership-lifecycle`
  - `tenant-cross-surface`
  - `secret-response-safety`
  - `agent-runtime-memory`
  - `scheduled-task-runtime`
  - `auth-security-persistence`
  - `relay-file-mapping-tenant-ownership`
  - `relay-runtime-channel-isolation`
  - `workflow-sql-isolation`
  - `publishing-channel-isolation`
  - `admin-relay-channel-isolation`
  - `admin-relay-read-isolation`
  - `observability-alert-recovery-persistence`
  - `quota-sql-isolation`
  - `core-sql-persistence`
- `scripts/verify-commercial-completion.sh` is strict by default. It requires `TEST_DATABASE_URL`, deploy validation, Kubernetes validation, backup/restore smoke, target live evidence manifest validation, docs/security gates, web TypeScript, full Go suites, DB-backed evidence profiles, and diff hygiene.
- A run with `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true` remains partial local evidence only and cannot count as final readiness.

## Current Blockers

These are still outside repository-local proof and keep the final status at `99/100`:

- Target Kubernetes validation with reachable cluster access and a filled untracked `OBLIVIOUS_K8S_SECRET_FILE`.
- Target deployment failover/recovery evidence, including real platform restart, scale, and failover behavior.
- Live billing and Marketplace provider rails for checkout, refund, payout, and reconciliation.
- Target Agent/Workflow/Task gRPC reachability using generated clients against running service endpoints.
- Target secret audit for deployed provider/channel/workflow/runtime secrets.
- Target or CI `TEST_DATABASE_URL` runs broad enough to count as final release evidence, not only disposable local profile proof.
- A final `scripts/verify-commercial-completion.sh` run with no environment skips and `COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true` pointing at an external, secret-free target evidence manifest for the exact release commit.

## TODO And Placeholder Scan

The active TODO/stub scan did not expose a new broad implementation gap. The remaining matches are known categories:

- Future Relay surfaces intentionally marked `DisabledInProduction` in `docs/release/relay-route-table.md`.
- Stale or alternate `src/server/internal/relay/handler_new` TODOs that are not the active runtime handler path.
- Test stubs and generated gRPC `Unimplemented*` boilerplate.
- Placeholder-only release docs, config examples, and UI input placeholders.
- `scripts/migrate-service-template.sh` template TODO.

## Local Follow-Up Candidates

Closed after this rescan:

- Marketplace publisher delete/uninstall settlement audit.
  - `src/server/internal/marketplace/store.go` now rejects publisher hard delete once order audit evidence exists, `src/server/migrations/0082_marketplace_audit_retention.sql` rebuilds paid audit foreign keys with non-cascading delete semantics, and `scripts/verify-commercial-db-evidence.sh marketplace-money-movement` now includes PostgreSQL proof that direct SQL delete attempts are blocked and buyer uninstall preserves paid order/settlement rows while clearing `marketplace_orders.install_id`.
- Scheduled task failure re-claim behavior.
  - `src/server/internal/schedule/worker.go` now sends due-worker failures through `FailScheduledTaskRun`, `src/server/internal/schedule/store.go` atomically marks the claimed run `failed` and advances `scheduled_tasks.next_run_at`, and `scripts/verify-commercial-db-evidence.sh scheduled-task-runtime` now includes PostgreSQL proof that the failed due task is not immediately claimed again.
- Scheduled Task HTTP cross-tenant isolation.
  - `src/server/internal/schedule/service.go` now verifies task ownership before listing run history, `src/server/internal/http/schedule_handler.go` returns 404 for missing/cross-organization run-history requests, and `scripts/verify-commercial-db-evidence.sh tenant-cross-surface` now includes PostgreSQL proof for cross-organization list-runs, status-update, and run-now denial without leakage or mutation.
- Admin top-up refund operator-evidence DB proof.
  - `src/server/internal/http/admin_billing_handler_test.go` now asserts persisted `billing_refunds` provider charge/payment-intent/top-up evidence, the exact operator lifecycle transition, idempotent duplicate submission behavior, and a real-router rejection path that preserves refund/lifecycle ledgers, payment intent, top-up order, and quota state. `scripts/verify-commercial-db-evidence.sh marketplace-money-movement` now includes the rejection-preservation test.
- Quota lifecycle DB evidence aggregation.
  - `scripts/verify-commercial-db-evidence.sh quota-sql-isolation` now runs quota SQL store lifecycle/isolation tests, quota/Admin/Billing HTTP route tests, and Stripe top-up/refund lifecycle balance-accounting tests in one no-skip PostgreSQL profile.
- Target/live release evidence manifest gate.
  - `scripts/verify-target-release-evidence.sh` now validates the external JSON evidence package required for live provider rails, deployed gRPC reachability, target secret audit, Kubernetes failover, workflow telemetry, and strict no-skip verifier proof, while rejecting embedded secret material.

The remaining follow-up is target/live evidence collection; it does not have a repository-only substitute:

1. Target/live release evidence collection.
   - Risk: repository-local proof is broad but still cannot replace real Kubernetes, provider rails, deployed gRPC reachability, target secret audit, and a strict no-skip release run.
   - Likely files: release evidence attachments under `docs/release/` after target environment runs complete.

## Commands Run For This Rescan

```bash
git status --short --branch
git rev-parse HEAD origin/main
git log --oneline --decorate -12
git log --oneline --decorate -8
find . \( -path './.git' -o -path './node_modules' -o -path './src/web/node_modules' -o -path './.tmp' -o -path './reference' \) -prune -o -name AGENTS.md -print
find . -maxdepth 3 -type f \( -name 'package.json' -o -name 'go.mod' -o -name 'pnpm-lock.yaml' -o -name 'Makefile' -o -name 'Dockerfile*' -o -name 'docker-compose*.yml' -o -name 'AGENTS.md' \) | sort
git ls-files | awk '...tracked file distribution...'
git ls-files src/server/internal | awk '...server domain counts...'
git ls-files 'src/server/**/*_test.go' | wc -l
git ls-files 'src/web/src/**/*.test.ts' 'src/web/src/**/*.test.tsx' | wc -l
git ls-files 'src/web/e2e/*.spec.ts' | wc -l
git ls-files 'src/web/e2e/fixtures/*' | wc -l
git ls-files src/server/migrations | awk '...top-level/clickhouse/microservices migration counts...'
git ls-files src/server/internal/relay/migrations/*.sql | wc -l
git ls-files 'src/server/**/*.sql' | sed -n '1,220p'
rg -o '\| (Proven|Partial|Gap|Unverified) \|' docs/reports/2026-06-07-fusion-spec-completion-matrix.md | sort | uniq -c
rg -n '\| Proven \|' docs/reports/2026-06-07-fusion-spec-completion-matrix.md
rg -n '\| Partial \|' docs/reports/2026-06-07-fusion-spec-completion-matrix.md
rg -n '^#|^##|^###' docs/superpowers/specs/2026-06-04-*.md
rg -n '^run_[a-z0-9_]+_profile\(\)|run_all_profiles|case "\$profile"' scripts/verify-commercial-db-evidence.sh
rg -n 'target|Kubernetes|secret|payment|provider|failover|gRPC|grpc|no-skip|live|target-environment|cluster|COMMERCIAL_COMPLETION_RUN_K8S' docs/release/commercial-completion-audit.md docs/release/fusion-spec-evidence-pack.md docs/reports/2026-06-07-fusion-spec-completion-matrix.md scripts/verify-commercial-completion.sh scripts/k8s-validate.sh
rg -n 'COMMERCIAL_COMPLETION_RUN_DEPLOY|COMMERCIAL_COMPLETION_RUN_K8S|COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE|COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE|verify-target-release-evidence|target live evidence' docs/release docs/reports scripts -S
rg -n 'TODO|FIXME|XXX|stub|placeholder|Unimplemented|DisabledInProduction' src scripts deploy docs/release docs/reports -g '!src/web/test-results/**' -g '!src/web/playwright-report/**' -g '!**/*.pb.go' -g '!docs/reports/archive/**'
sed -n '1,340p' scripts/verify-target-release-evidence.sh
sed -n '1,180p' docs/release/commercial-completion-audit.md
git diff --stat
git diff -- scripts/verify-commercial-completion.sh scripts/verify-fusion-evidence-pack.sh scripts/verify-quality-gates.sh scripts/verify-target-release-evidence.sh docs/release/commercial-gates.md docs/release/fusion-spec-evidence-pack.md docs/release/rc-checklist.md docs/release/commercial-completion-audit.md docs/reports/2026-06-07-fusion-spec-completion-matrix.md docs/reports/2026-06-16-repo-rescan.md
nl -ba src/server/internal/marketplace/store.go | sed -n '330,390p;740,790p'
nl -ba src/server/migrations/0030_marketplace_settlement_governance.sql | sed -n '1,180p'
rg -n 'marketplace_audit_retention|AuditRetention|DeleteWithPaidOrder|BuyerUninstallPreserves|scheduled_task_store_pattern' scripts/verify-commercial-db-evidence.sh src/server/internal/marketplace/settlement_test.go src/server/internal/marketplace/store.go src/server/migrations/0082_marketplace_audit_retention.sql src/server/internal/schedule/worker_test.go src/server/internal/schedule/store_test.go
nl -ba src/server/internal/schedule/worker.go | sed -n '1,260p'
nl -ba src/server/internal/schedule/store.go | sed -n '250,520p'
nl -ba src/server/internal/schedule/worker_test.go | sed -n '229,420p;520,760p'
nl -ba src/server/internal/schedule/store_test.go | sed -n '360,620p'
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/schedule -count=1
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh scheduled-task-runtime
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh tenant-cross-surface
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/schedule -run 'TestListScheduledTaskRunsUsesOrganizationScope' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run '^TestAdminBilling(RecordsTopupRefundAndAdjustsQuota|RejectsTopupRefundWithoutOperatorEvidenceAndPreservesLedger)$' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-money-movement
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh quota-sql-isolation
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
bash -n scripts/verify-commercial-db-evidence.sh
bash -n scripts/verify-target-release-evidence.sh scripts/verify-commercial-completion.sh scripts/verify-fusion-evidence-pack.sh scripts/verify-quality-gates.sh
bash scripts/verify-target-release-evidence.sh --help
tmpdir=$(mktemp -d)
# temp valid/invalid target evidence manifests outside git:
#   valid manifest passed for current HEAD
#   missing path failed
#   commit mismatch failed
#   non-live provider failed
#   non-empty skippedChecks failed
#   embedded secret material failed
git diff --check
sed -n '1,180p' scripts/verify-commercial-completion.sh
sed -n '1,140p' docs/release/fusion-spec-evidence-pack.md
```

## Verification For This Report

This report is documentation-only. It should be verified with:

```bash
bash -n scripts/verify-target-release-evidence.sh scripts/verify-commercial-completion.sh scripts/verify-fusion-evidence-pack.sh scripts/verify-quality-gates.sh
bash scripts/verify-target-release-evidence.sh --help
target evidence manifest temp valid/invalid checks
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
git diff --check
```
