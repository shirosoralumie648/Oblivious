# Repository Rescan - 2026-06-16

## Current Truth

- Branch: `main`.
- Current commit: `1c52194ac7bd1592d28b3c067b0d2d4152e5643f` (`test(agent): cover llm memory consolidation`).
- Remote parity: `HEAD == origin/main`.
- Working tree at scan start: clean after the Agent memory consolidation slice was committed and pushed.
- The previous `2026-06-15` rescan report is now an older baseline; it referenced earlier implementation heads and does not include the latest Agent planning/tool and memory-policy evidence slices.
- The project is still not complete against the four `docs/superpowers/specs/2026-06-04-*` specs.
- Current matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate remains `99/100`: repository-local evidence is broad, but final completion is still gated by target/live proof and a strict no-skip release run.

## What Changed Since The Last Rescan

- Agent action IDs now fail closed when snake_case and camelCase aliases conflict for tool-run and plan-step actions.
- PlanningEngine tool steps now have direct ReAct handoff proof.
- MCP auth-token PostgreSQL evidence and strict verifier coverage were tightened.
- Agent gRPC generated-client coverage and Workflow/Task generated-client proof remain in the checked-in evidence set.
- The newest pushed slice proves LLM-assisted long-term memory extraction composes with `memory_key_consolidate`: generated keyed facts update the existing long-term memory in place instead of creating duplicates.

## Repository Inventory

- First-party tracked files:
  - `src`: 993
  - `docs`: 92
  - `scripts`: 37
  - `deploy`: 42
  - `.planning`: 210
  - other tracked files: 27
  - total tracked files: 1401
- Server internal shape:
  - `src/server/internal`: 590 tracked files.
  - largest active domains: `relay` 110, `http` 104, `mcp` 43, `admin` 39, `agent` 34, `workflow` 31, `knowledge` 30, `observability` 28, `channel` 27, `migration` 24, `marketplace` 16.
- Test inventory:
  - Go test files: 230
  - Web component/API test files: 67
  - Web Playwright specs: 16
  - Web E2E fixture files: 16
- Migration inventory:
  - PostgreSQL SQL migration files: 106.
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
- `scripts/verify-commercial-completion.sh` is strict by default. It requires `TEST_DATABASE_URL`, deploy validation, Kubernetes validation, backup/restore smoke, docs/security gates, web TypeScript, full Go suites, DB-backed evidence profiles, and diff hygiene.
- A run with `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true` remains partial local evidence only and cannot count as final readiness.

## Current Blockers

These are still outside repository-local proof and keep the final status at `99/100`:

- Target Kubernetes validation with reachable cluster access and a filled untracked `OBLIVIOUS_K8S_SECRET_FILE`.
- Target deployment failover/recovery evidence, including real platform restart, scale, and failover behavior.
- Live billing and Marketplace provider rails for checkout, refund, payout, and reconciliation.
- Target Agent/Workflow/Task gRPC reachability using generated clients against running service endpoints.
- Target secret audit for deployed provider/channel/workflow/runtime secrets.
- Target or CI `TEST_DATABASE_URL` runs broad enough to count as final release evidence, not only disposable local profile proof.
- A final `scripts/verify-commercial-completion.sh` run with no environment skips.

## TODO And Placeholder Scan

The active TODO/stub scan did not expose a new broad implementation gap. The remaining matches are known categories:

- Future Relay surfaces intentionally marked `DisabledInProduction` in `docs/release/relay-route-table.md`.
- Stale or alternate `src/server/internal/relay/handler_new` TODOs that are not the active runtime handler path.
- Test stubs and generated gRPC `Unimplemented*` boilerplate.
- Placeholder-only release docs, config examples, and UI input placeholders.
- `scripts/migrate-service-template.sh` template TODO.

## Local Follow-Up Candidates

These are useful local slices, but they do not replace target/live final proof:

1. Agent plan-step `dependsOn` index drift after insert/move/delete.
   - Risk: dependencies store step indexes, so draft reordering can point dependencies at the wrong step or a missing step.
   - Likely files: `src/server/internal/agent/service.go`, `src/server/internal/agent/service_test.go`, optional HTTP handler tests.
2. Marketplace publisher delete/uninstall settlement audit.
   - Risk: deleting published agents or uninstalling paid installs could damage order/settlement audit evidence if SQL cascade behavior is not guarded.
   - Likely files: `src/server/internal/marketplace/store.go`, `src/server/internal/marketplace/service.go`, `src/server/internal/marketplace/service_test.go`.
3. Scheduled task failure re-claim behavior.
   - Risk: failed due runs may remain immediately claimable if failure handling does not advance `next_run_at`, causing repeated failure loops.
   - Likely files: `src/server/internal/schedule/service.go`, `src/server/internal/schedule/store.go`, `src/server/internal/schedule/worker_test.go`, `src/server/internal/schedule/store_test.go`.

## Commands Run For This Rescan

```bash
git status --short --branch
git rev-parse HEAD origin/main
git log --oneline --decorate -12
find . \( -path './.git' -o -path './node_modules' -o -path './src/web/node_modules' -o -path './.tmp' -o -path './reference' \) -prune -o -name AGENTS.md -print
git ls-files | awk '...tracked file distribution...'
git ls-files src/server/internal | awk '...server domain counts...'
git ls-files 'src/server/**/*_test.go' | wc -l
git ls-files 'src/web/src/**/*.test.ts' 'src/web/src/**/*.test.tsx' | wc -l
git ls-files 'src/web/e2e/*.spec.ts' | wc -l
git ls-files 'src/web/e2e/fixtures/*' | wc -l
git ls-files 'src/server/migrations/*.sql' | wc -l
git ls-files 'src/server/migrations/*.sql' | sort | tail -12
rg -o '\| (Proven|Partial|Gap|Unverified) \|' docs/reports/2026-06-07-fusion-spec-completion-matrix.md | sort | uniq -c
rg -n '\| Proven \|' docs/reports/2026-06-07-fusion-spec-completion-matrix.md
rg -n '\| Partial \|' docs/reports/2026-06-07-fusion-spec-completion-matrix.md
rg -n '^#|^##|^###' docs/superpowers/specs/2026-06-04-*.md
rg -n '^run_[a-z0-9_]+_profile\(\)|run_all_profiles|case "\$profile"' scripts/verify-commercial-db-evidence.sh
rg -n 'target|Kubernetes|secret|payment|provider|failover|gRPC|grpc|no-skip|live|target-environment|cluster|COMMERCIAL_COMPLETION_RUN_K8S' docs/release/commercial-completion-audit.md docs/release/fusion-spec-evidence-pack.md docs/reports/2026-06-07-fusion-spec-completion-matrix.md scripts/verify-commercial-completion.sh scripts/k8s-validate.sh
rg -n 'TODO|FIXME|XXX|stub|placeholder|Unimplemented|DisabledInProduction' src scripts deploy docs/release docs/reports -g '!src/web/test-results/**' -g '!src/web/playwright-report/**' -g '!**/*.pb.go' -g '!docs/reports/archive/**'
sed -n '1,180p' scripts/verify-commercial-completion.sh
sed -n '1,140p' docs/release/fusion-spec-evidence-pack.md
```

## Verification For This Report

This report is documentation-only. It should be verified with:

```bash
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
git diff --check
```
