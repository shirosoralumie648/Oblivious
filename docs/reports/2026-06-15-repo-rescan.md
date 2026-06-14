# Repository Rescan - 2026-06-15

## Current Truth

- Branch: `main`; this follow-up scan starts from pushed commit `0e61dcc test(frontend): prove domestic topup refund flow`.
- Refresh base: `HEAD` and `origin/main` both resolved to `0e61dcc6f961e247bcf992e46b9ff67cfdda21ef`; the worktree was clean at scan time.
- The project is still **not complete** against the four 2026-06-04 fusion specs.
- The current completion matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate after this scan is **97/100**. Admin Relay channel API-key, Observability alert-provider config, Observability alert/recovery SQL persistence, Publishing channel config, Workflow definition/version/execution-snapshot/node-execution secret-like fields, Agent Memories browser CRUD/import-export, Agent memory store PostgreSQL persistence, Agent gRPC registration and planning continue/adjust plus plan-step action service-adapter proof, Billing provider lifecycle PostgreSQL transitions, Admin usage analytics daily aggregate PostgreSQL proof, Marketplace governance/review PostgreSQL proof, Admin Billing operator payout/refund browser proof, Admin commercial configuration browser proof, Marketplace publisher/My Agents browser proof, Admin route manifest dispatch proof, durable Agent planning completion evidence, Agent plan adjustment browser proof, Admin Reviews browser moderation/governance proof, and Admin Alerts browser alert-management proof are now covered with repository-local proof, but target-environment workflow telemetry, target secret audits, deployment validation, payment/provider live rails, platform failover, live moderation/notification operations, deployed gRPC/client compatibility, and final no-skip release readiness remain open.

## What Changed In This Rescan

- `scripts/verify-commercial-db-evidence.sh observability-alert-recovery-persistence` now supplies no-skip disposable PostgreSQL proof for Observability alert routing rules, alert lifecycle and escalation, alert-state filters, notification throttle state, recovery cooldown reuse, and repeated delivery-batch history.
- `scripts/verify-commercial-db-evidence.sh admin-usage-analytics-db` now supplies no-skip disposable PostgreSQL proof for Admin usage daily aggregate refresh/query and zero-total-token fallback for raw analytics plus usage-log listing.
- `scripts/verify-commercial-db-evidence.sh marketplace-governance-review` now supplies no-skip disposable PostgreSQL proof for Marketplace automated review, takedown/appeal/reinstate, abuse reports, publisher notifications, needs-changes review state, review SLA enforcement, and real HTTP route persistence.
- `scripts/verify-commercial-db-evidence.sh agent-runtime-memory` now includes Agent memory store PostgreSQL create/list/filter/cross-tenant evidence in addition to durable run, plan-step, approval config, execution-mode, and memory-policy persistence.
- `scripts/verify-commercial-db-evidence.sh all` now includes the Observability alert/recovery persistence, Admin usage analytics DB, Marketplace governance/review, and expanded Agent memory store profiles, so final commercial DB evidence cannot omit those state/accounting/governance/runtime-memory paths.
- These repository slices improve Observability, Security, API contract, Agent, Frontend shell, and Release readiness evidence, but they do not reclassify any matrix row to Proven because target deployment, live provider rails, live moderation/notification operations, platform failover, deployed gRPC/client compatibility, and final no-skip release proof remain open.
- Agent Memories now has browser-level Workspace proof for `/memories`: active navigation, search filter propagation, export query propagation, blob download-link rendering, user-managed create/update/delete, JSON import, and memory-count state updates.
- Agent gRPC now has package-level service-adapter proof for planning run continuation and adjustment plus plan-step approve/execute/skip/retry actions, including authenticated `auth.Session` forwarding, refreshed run detail, cross-run plan-step rejection, and approval-boundary `FailedPrecondition` mapping.
- Admin Billing now has browser-level Admin-shell proof for `/admin/billing`: payout paid confirmation, payout failure with operator reason evidence, payout/top-up filter query propagation, Stripe top-up refund payload/state propagation, and domestic Alipay CNY top-up refund payload propagation without Stripe charge-ID evidence.
- Admin Reviews now has browser-level Admin-shell proof for `/admin/reviews`: active Review Queue navigation, pending-review commercial/SLA context, SLA enforcement, approve/reject/needs-changes decisions, abuse-report resolve/dismiss triage, and takedown/reinstate governance payloads.
- Admin Alerts now has browser-level Admin-shell proof for `/admin/alerts`: alert filter query propagation, recovery-action evidence, delivery-history inspection, acknowledge/resolve state mutation, severity routing updates, Slack webhook provider creation, and provider-test feedback.
- `scripts/verify-commercial-db-evidence.sh billing-provider-lifecycle` now supplies no-skip disposable PostgreSQL proof for Stripe/shared checkout, invoice, subscription, and refund lifecycle transitions, and the `all` profile includes it.
- Admin Relay channel API keys now use a shared AES-GCM `secretbox` codec before writing `channels.api_key_encrypted`.
- `OBLIVIOUS_SECRET_ENCRYPTION_KEY` is the preferred deployment key, with `SESSION_SECRET` fallback for compatibility; local, Docker, Kubernetes, and architecture env docs now list the variable.
- Admin channel create/update protects API keys at rest, and Admin provider probes decrypt the stored key before calling upstream.
- Relay runtime channel create/update protects API keys at rest, and runtime list/get/pool loading decrypts protected keys before adapters use them.
- Legacy unprefixed plaintext channel rows remain readable so existing deployments do not lose runtime credentials before rotation.
- Observability alert-provider config secrets now use the same shared AES-GCM `secretbox` codec before JSONB persistence, and SQL store reads decrypt before HTTP redaction, marker-preserving updates, provider tests, and alert delivery sink construction.
- Publishing channel config secrets now use the same shared AES-GCM `secretbox` codec before JSONB persistence, and SQL store reads decrypt before HTTP redaction, marker-preserving updates, signed public webhooks, and retry/send runtime use.
- Workflow definitions, workflow versions, execution snapshots, and workflow node execution input/output/error/context payloads now use the same shared AES-GCM `secretbox` codec for secret-like keys before JSONB persistence. SQL store reads decrypt before runtime use; HTTP responses redact workflow definitions, node executions, and debug snapshots.
- This is repository-local PostgreSQL at-rest encryption proof for Admin Relay channel API keys, Observability alert-provider config secrets, Publishing channel config secrets, and Workflow secret-like definition/runtime payloads. Target-environment secret audits remain open.

## Repository Inventory

- First-party tracked file distribution after this follow-up scan:
  - `src`: 987 files
  - `.planning`: 210 files
  - `docs`: 92 files
  - `scripts`: 37 files
  - `deploy`: 42 files
- Server shape remains:
  - `src/server/internal`: 590 tracked files
  - `src/server/migrations`: 106 SQL migration files
  - largest active server domains remain `relay`, `http`, `mcp`, `admin`, `agent`, `workflow`, `knowledge`, `observability`, `channel`, `migration`, and `marketplace`
- Web shape remains:
  - `src/web/src/routes`: 80 tracked files
  - `src/web/src/features`: 51 tracked files
  - route families are mostly `workspace`, `admin`, `console`, `marketing`, and `marketplace`
- Test inventory after this slice:
  - Go test files: 230
  - Web component/API test files: 67
  - Web Playwright specs: 13 specs
  - Web E2E fixture files: 13 files
- Latest checked-in top-level migration remains `src/server/migrations/0081_admin_relay_channel_organization_scope.sql`.
- Project-local `AGENTS.md`: none found under the first-party scan scope; dependency caches and nested `reference/*` repositories are excluded.

## Completion Matrix Snapshot

Proven rows remain:

- API gateway and relay
- Knowledge base and RAG
- Multi-channel publishing
- Database schema and migrations

Partial rows remain:

- Workflow engine
- Agent system
- Billing and monetization
- Marketplace ecosystem
- Frontend shell and core pages
- Observability metrics, logs, alerts, recovery
- API contract
- Deployment and operations
- Security and tenant isolation
- Migration strategy and release readiness

This scan does not reclassify any Partial row to Proven. Admin Relay API-key, Observability alert-provider, Observability alert/recovery persistence, Publishing channel, Workflow at-rest encryption, Agent Memories browser proof, Agent memory store DB proof, Agent gRPC planning service-adapter proof, Billing provider lifecycle proof, Admin usage analytics DB proof, Marketplace governance/review DB proof, Admin Billing operator browser proof, Admin Reviews browser proof, and Admin Alerts browser proof improve Relay/Publishing/Workflow, Agent, Frontend, Billing, Marketplace, Observability, API, Security, operations/env, and release evidence, but broader target-environment telemetry, target secret audits, live provider proof, platform failover, live notification-provider proof, deployed gRPC/client compatibility, and final release proof remain open.

## Verification Evidence And Current Scan

Current `0e61dcc` refresh commands:

```bash
pwd
git status --short --branch
git rev-parse HEAD origin/main
git log --oneline -12 --decorate
git ls-files | awk '...inventory counters...'
git ls-files src/server/internal | awk '...domain counters...'
git ls-files src/server/migrations/*.sql | sort | tail -12
find . \( -path './node_modules' -o -path './src/web/node_modules' -o -path './.tmp' -o -path './reference' -o -path './.git' \) -prune -o -name AGENTS.md -print
awk '...top-level completion matrix rows...' docs/reports/2026-06-07-fusion-spec-completion-matrix.md
rg -n "^## |^### |^# " docs/superpowers/specs/2026-06-04-*.md
rg -n "TODO|FIXME|XXX|stub|placeholder|Unimplemented|DisabledInProduction" src scripts deploy docs/release docs/reports -g '!src/web/test-results/**' -g '!src/web/playwright-report/**' -g '!**/*.pb.go' -g '!docs/reports/archive/**'
rg -n "topup_browser|providerPaymentIntent|alipay|wechatpay|refund" src/web/e2e/admin-billing-operator.spec.ts src/web/e2e/fixtures/adminBillingOperator.ts src/web/src/routes/admin/AdminBillingPage.tsx src/web/src/routes/admin/AdminBillingPage.test.tsx
```

Previously recorded follow-up verification commands:

```bash
pwd
git status --short --branch
git rev-parse HEAD origin/main
git log --oneline -8 --decorate
git ls-files | awk '...inventory counters...'
git ls-files src/server/internal | awk '...domain counters...'
git ls-files src/server/migrations/*.sql | sort | tail -12
find . \( -path '*/node_modules/*' -o -path './.tmp/*' -o -path './reference/*' -o -path './.git/*' \) -prune -o -name AGENTS.md -print
rg -n "^## |^### |^# " docs/superpowers/specs/2026-06-04-*.md
sed -n '21,34p' docs/reports/2026-06-07-fusion-spec-completion-matrix.md | rg -c "\| Proven \|"
sed -n '21,34p' docs/reports/2026-06-07-fusion-spec-completion-matrix.md | rg -c "\| Partial \|"
rg -n "^run_[a-z0-9_]+_profile\(\)|run_all_profiles|case \"\$profile\"" scripts/verify-commercial-db-evidence.sh
rg -n "TODO|FIXME|XXX|stub|placeholder|Unimplemented|DisabledInProduction" src scripts deploy docs/release docs/reports -g '!src/web/test-results/**' -g '!src/web/playwright-report/**' -g '!**/*.pb.go' -g '!docs/reports/archive/**'
rg -n "TestAgentMemoryStorePersistsAndFiltersMemories|func TestAgentMemoryStore" src/server/internal/agent/store_test.go src/server/internal/agent/service_test.go
bash -n scripts/verify-commercial-db-evidence.sh
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh observability-alert-recovery-persistence
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh admin-usage-analytics-db
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-governance-review
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh agent-runtime-memory
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./pkg/agent -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/agent ./internal/http -run 'TestService(ContinuePlanningRun|RetryPlanStep|AdjustPlanSteps|StartPlanningRun|ExecutePlanStep)|TestAgentRunsHandler(ContinuePlan|PlanStepActions|RetryPlanStep)|TestRegisterAgentRunRoutesDispatches(ContinuePlan|AdjustPlan)' -count=1 -v
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/admin-reviews.spec.ts --project=chromium
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web test src/routes/admin/AdminReviewsPage.test.tsx src/features/admin/api.test.ts -- --runInBand
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/admin-alerts.spec.ts --project=chromium
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web test src/routes/admin/AdminAlertsPage.test.tsx src/features/admin/api.test.ts -- --runInBand
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-governance-review
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh observability-alert-recovery-persistence
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
git diff --check
```

Previously recorded implementation verification commands for the checked-in evidence set:

```bash
git status --short --branch
git log --oneline -8
git ls-files | awk '...inventory counters...'
find . \( -path '*/node_modules/*' -o -path './.tmp/*' -o -path './reference/*' -o -path './.git/*' \) -prune -o -name AGENTS.md -print
rg -n "TODO|FIXME|XXX|stub|placeholder|Unimplemented|DisabledInProduction" src scripts deploy docs/release docs/reports -g '!src/web/test-results/**' -g '!src/web/playwright-report/**'
rg -n "api_key_encrypted|secretbox|ENCRYPTION_KEY|SESSION_SECRET" src/server/internal/{admin,relay,mcp,http,workflow,observability,channel} docs/release docs/reports scripts/verify-commercial-db-evidence.sh -g '*.go' -g '*.md' -g '*.sh'
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/secretbox -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/workflow -count=1
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/channel -count=1
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/observability -count=1
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run '^TestWorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers$' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run '^TestPublishingChannelHTTPRouteRedactsSQLStoreConfigSecretsAndPreservesMarkers$' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run '^TestObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted$' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/admin ./internal/relay ./internal/http -run 'Test(RelayChannelProbeUsesAPIKeyAndReturnsModels|RelayStoreProtectsChannelAPIKeyAtRestAndHydratesRuntimeKey|AdminRelayChannelHTTPRouteRedactsSQLStoreAPIKeysAndPreservesMarkers)$' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh secret-response-safety
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh relay-runtime-channel-isolation
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh admin-relay-channel-isolation
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh billing-provider-lifecycle
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh admin-usage-analytics-db
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-governance-review
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh all
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web test src/features/agents/memoriesApi.test.ts src/routes/workspace/AgentMemoriesPage.test.tsx -- --runInBand
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec vitest run src/routes/admin/AdminBillingPage.test.tsx src/features/admin/api.test.ts
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/agent-memories.spec.ts --project=chromium
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/admin-billing-operator.spec.ts --project=chromium
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/admin-reviews.spec.ts --project=chromium
git diff --check
```

Result:

- `pwd` showed `/media/shirosora/4A183E5C183E46EB/codestorage/Oblivious`.
- `git status --short --branch` at this follow-up start showed `main...origin/main` at `0e61dcc` with a clean worktree before this report refresh.
- `git rev-parse HEAD origin/main` returned `0e61dcc6f961e247bcf992e46b9ff67cfdda21ef` for both refs before this report refresh.
- The top-level matrix count remains 4 `Proven` and 10 `Partial`; the `Gap` and `Unverified` counts remain 0.
- The current first-party inventory counters after this slice are: `src=987`, `docs=92`, `scripts=37`, `deploy=42`, `.planning=210`, Go test files `230`, web component/API test files `67`, Playwright specs `13`, and Playwright fixtures `13`.
- The current server-domain leaders are still `relay`, `http`, `mcp`, `admin`, `agent`, `workflow`, `knowledge`, `observability`, `channel`, `migration`, and `marketplace`.
- The latest checked-in top-level migration remains `src/server/migrations/0081_admin_relay_channel_organization_scope.sql`.
- The current first-party AGENTS scan found no `AGENTS.md` in the scanned first-party tree.
- `scripts/verify-commercial-db-evidence.sh` now exposes `backend-journey`, `marketplace-money-movement`, `marketplace-governance-review`, `billing-provider-lifecycle`, `admin-usage-analytics-db`, `app-stateful-routes`, `tenant-membership-lifecycle`, `tenant-cross-surface`, `secret-response-safety`, `agent-runtime-memory`, `scheduled-task-runtime`, `auth-security-persistence`, `relay-file-mapping-tenant-ownership`, `relay-runtime-channel-isolation`, `workflow-sql-isolation`, `publishing-channel-isolation`, `admin-relay-channel-isolation`, `admin-relay-read-isolation`, `observability-alert-recovery-persistence`, and `quota-sql-isolation`.
- `bash -n scripts/verify-commercial-db-evidence.sh` passed.
- `scripts/verify-commercial-db-evidence.sh agent-runtime-memory` now includes `TestAgentMemoryStorePersistsAndFiltersMemories`, so Agent memory store create/list/filter/cross-tenant behavior is guarded by no-skip PostgreSQL evidence.
- `scripts/verify-commercial-db-evidence.sh observability-alert-recovery-persistence` passed with disposable pgvector PostgreSQL and skipped tests: none. It ran `TestSQLAlertRoutingRuleStorePersistsRoutingRules`, `TestSQLAlertStateStorePersistsAlertLifecycleAndEscalation`, `TestSQLAlertStateStoreListsAlertStatesWithFilters`, `TestSQLAlertStateStorePersistsNotificationThrottleAndRecoveryCooldown`, and `TestSQLAlertStateStoreRecordsRepeatedDeliveryBatchesForSameAlert`.
- `scripts/verify-commercial-db-evidence.sh admin-usage-analytics-db` passed with disposable pgvector PostgreSQL and skipped tests: none. It ran `TestSQLStoreUsageDailyAggregatesPostgresRefreshAndAnalytics`, `TestSQLStoreUsageAnalyticsRawRecordsFallsBackFromZeroTotalTokens`, and `TestSQLStoreListUsageLogsFallsBackFromZeroTotalTokens`.
- `scripts/verify-commercial-db-evidence.sh marketplace-governance-review` passed with disposable pgvector PostgreSQL and skipped tests: none. It ran Marketplace governance/automated-review persistence tests for takedown, appeal, reinstate, abuse-report lifecycle/listing/notification, automated review pass/reject, and needs-changes; it also ran HTTP route tests for review SLA enforcement, takedown/appeal/reinstate, abuse-report lifecycle/listing, publish-time automated review governance, and admin needs-changes.
- `scripts/verify-commercial-db-evidence.sh agent-runtime-memory` passed with disposable pgvector PostgreSQL and skipped tests: none. It ran Agent durable run lifecycle, structured plan-step persistence/update, approval/tool-risk config persistence, default execution-mode persistence, long-term memory policy persistence, and Agent memory store persistence/filtering/cross-tenant isolation.
- `go test ./pkg/agent -count=1 -v` passed with new Agent gRPC tests for `ContinuePlan`, `AdjustPlan`, plan-step approve/execute/skip/retry action forwarding, refreshed run detail mapping, request validation, nil-runtime failure, `auth.Session` forwarding into the internal Agent service, cross-run plan-step rejection, and approval-boundary error mapping.
- `go test ./internal/agent ./internal/http -run 'TestService(ContinuePlanningRun|RetryPlanStep|AdjustPlanSteps|StartPlanningRun|ExecutePlanStep)|TestAgentRunsHandler(ContinuePlan|PlanStepActions|RetryPlanStep)|TestRegisterAgentRunRoutesDispatches(ContinuePlan|AdjustPlan)' -count=1 -v` passed, confirming the existing planning service and HTTP semantics still match the gRPC adapter assumptions.
- `go test ./internal/secretbox -count=1 -v` passed.
- `go test ./internal/workflow -count=1` passed.
- `go test ./internal/channel -count=1` passed.
- `go test ./internal/observability -count=1` passed.
- The direct Workflow HTTP package command compiled; the DB-backed test skipped locally without `TEST_DATABASE_URL`, which is expected for that direct command.
- The direct Publishing HTTP package command compiled; the DB-backed test skipped locally without `TEST_DATABASE_URL`, which is expected for that direct command.
- The direct Observability HTTP package command compiled; the DB-backed test skipped locally without `TEST_DATABASE_URL`, which is expected for that direct command.
- The package-level Admin/Relay/HTTP command compiled the changed packages; DB-backed tests skipped locally without `TEST_DATABASE_URL`, which is expected for that direct command.
- `scripts/verify-commercial-db-evidence.sh secret-response-safety` passed with disposable pgvector PostgreSQL and skipped tests: none.
- `scripts/verify-commercial-db-evidence.sh relay-runtime-channel-isolation` passed with disposable pgvector PostgreSQL and skipped tests: none, including the new runtime at-rest encryption test.
- `scripts/verify-commercial-db-evidence.sh admin-relay-channel-isolation` passed with disposable pgvector PostgreSQL and skipped tests: none.
- `scripts/verify-commercial-db-evidence.sh billing-provider-lifecycle` passed with disposable pgvector PostgreSQL and skipped tests: none.
- `scripts/verify-commercial-db-evidence.sh all` passed with disposable pgvector PostgreSQL and skipped tests: none, including the Billing provider lifecycle, Observability alert/recovery persistence, Admin usage analytics DB, and Marketplace governance/review profiles.
- `bash scripts/check.sh docs` passed.
- `pnpm --dir src/web test src/features/agents/memoriesApi.test.ts src/routes/workspace/AgentMemoriesPage.test.tsx -- --runInBand` passed.
- `pnpm --dir src/web exec vitest run src/routes/admin/AdminBillingPage.test.tsx src/features/admin/api.test.ts` passed.
- `pnpm --dir src/web exec tsc --noEmit` passed.
- `pnpm --dir src/web exec playwright test e2e/agent-memories.spec.ts --project=chromium` passed with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome`.
- `pnpm --dir src/web exec playwright test e2e/admin-billing-operator.spec.ts --project=chromium` passed with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome`.
- `pnpm --dir src/web test src/routes/admin/AdminReviewsPage.test.tsx src/features/admin/api.test.ts -- --runInBand` passed.
- `pnpm --dir src/web exec playwright test e2e/admin-reviews.spec.ts --project=chromium` passed with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome`. It ran five browser tests covering Review Queue navigation, review SLA enforcement, approve/reject/needs-changes decisions, abuse-report resolve/dismiss, and takedown/reinstate governance actions.
- `pnpm --dir src/web test src/routes/admin/AdminAlertsPage.test.tsx src/features/admin/api.test.ts -- --runInBand` passed with 2 files and 37 tests.
- `pnpm --dir src/web exec playwright test e2e/admin-alerts.spec.ts --project=chromium` passed with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome`. It ran three browser tests covering alert filter query propagation, delivery/recovery inspection, acknowledge/resolve state changes, notification routing updates, Slack webhook provider creation, and provider-test feedback.
- `git diff --check` passed.
- The previously recorded first-party AGENTS scan found no main-root or first-party source `AGENTS.md`.
- The TODO/stub scan did not reveal a new broad implementation gap. Active first-party matches remain the known release-boundary items from the June 14 scan: disabled future Relay surfaces, generated gRPC `Unimplemented*` boilerplate, test stubs, placeholder-only release docs/config examples, and the service-template migration TODO.
- The secret-storage scan now confirms MCP auth tokens, Admin Relay channel API keys, Observability alert-provider config secrets, Publishing channel config secrets, and Workflow definition/version/execution-snapshot/node-execution secret-like fields have repository-owned reversible at-rest protection. Target-environment secret audits remain the open secret-storage proof boundary.

## Notable Scan Findings

- This follow-up starts from `0e61dcc`; previous scan text that referenced earlier slice starts such as `2f53af9`, `1a83cff`, `9630167`, `8e4f9fd`, `d7a91f0`, `0655515`, `79d6000`, or `b2fc74b` is stale after the Agent memory DB evidence, Admin Reviews browser-proof, Admin Alerts browser-proof, Agent gRPC planning-boundary, runtime registration, Admin commercial config, Marketplace publisher, durable planning completion, Admin manifest dispatch, Agent plan-adjustment browser, and domestic Admin Billing top-up refund browser slices.
- Existing DB-backed tenant-isolation evidence remains stronger than target-environment evidence; this slice adds DB-backed at-rest encryption proof for the Workflow definition/runtime secret path on top of the prior Admin Relay, Observability, and Publishing secret paths.
- Observability alert/routing SQL persistence is now a first-class commercial DB evidence profile. The profile rejects skips and empty regex matches while proving routing rules, alert lifecycle/escalation, alert-state filters, notification throttling, recovery cooldown reuse, and repeated delivery-batch history against PostgreSQL.
- Admin usage analytics daily aggregate SQL persistence is now a first-class commercial DB evidence profile. The profile rejects skips and empty regex matches while proving daily aggregate refresh/query and zero-total-token fallback for both raw analytics and usage-log listing against PostgreSQL.
- `src/server/internal/http/admin_channel_secret_response_test.go` now pairs response redaction with direct SQL ciphertext assertions and an upstream probe assertion that the decrypted rotated key is usable.
- `src/server/internal/relay/store_test.go` now proves Relay runtime persistence writes protected channel API keys, hydrates raw keys for runtime callers, and preserves legacy plaintext compatibility.
- `src/server/internal/http/observability_alert_handler_test.go` now pairs Observability response redaction with direct SQL ciphertext assertions and a SQL-backed `/test` assertion that the decrypted webhook URL remains usable.
- `src/server/internal/http/channel_handler_test.go` now pairs Publishing response redaction with direct SQL ciphertext assertions and a SQL-backed signed webhook assertion that the decrypted channel secret remains usable.
- `src/server/internal/http/workflow_secret_response_test.go` now pairs Workflow response redaction with direct SQL ciphertext assertions for definitions, versions, execution snapshots, and workflow node executions, plus a SQL-backed signed webhook assertion that the decrypted workflow secret remains usable.
- Response safety and at-rest encryption remain separate. Admin Relay channel API keys, Observability alert-provider config secrets, Publishing channel config secrets, and Workflow definition/runtime secret-like payloads now have both response safety and at-rest encryption proof in repository-local PostgreSQL.
- Agent Memories now has a built-app Playwright proof for the manual memory-management workflow. The fixture fails closed if browser search/export filters or create/update/import/delete payloads drift from the Workspace UI controls.
- Agent memory store persistence is now included in the no-skip DB profile. `TestAgentMemoryStorePersistsAndFiltersMemories` proves user-managed and long-term memory creation, metadata persistence, type/query filtering, and cross-tenant empty-list behavior against PostgreSQL.
- Agent gRPC now exposes planning continuation/adjustment and plan-step approve/execute/skip/retry through generated proto bindings and `src/server/pkg/agent`. The adapter does not bypass the internal planning service; it forwards authenticated sessions, validates run/step ownership, returns refreshed run detail after single-step actions, and reuses existing approval-boundary error semantics.
- Admin Billing now has a built-app Playwright proof for operator money-movement actions. The fixture fails closed if browser payout/top-up filters, payout paid/failed payloads, Stripe refund evidence, or domestic Alipay `currency=cny` refund payloads drift from the Admin Billing UI controls.
- The domestic Admin Billing browser proof now records an Alipay top-up refund with provider payment-intent evidence while the fixture rejects any Stripe charge-ID payload, narrowing the local browser gap for domestic payment operator refunds.
- Admin Reviews now has a built-app Playwright proof for moderation and governance actions. The fixture fails closed if browser review filters, SLA enforcement queries, approve/reject/needs-changes payloads, abuse-report resolve/dismiss payloads, or takedown/reinstate governance reasons drift from the Admin Reviews UI controls.
- Admin Alerts now has a built-app Playwright proof for operational alert management. The fixture fails closed if browser alert filters, routing payloads, Slack provider creation payloads, acknowledge/resolve requests, provider tests, recovery-action rendering, or delivery-history inspection drift from the Admin Alerts UI controls.
- Billing provider lifecycle DB tests are now first-class commercial evidence. The profile rejects skips and empty regex matches while proving subscription checkout, top-up checkout, invoice paid/payment-failed, subscription update/delete, and refund/quota reversal transitions against PostgreSQL.

## Recommended Next Slices

1. Local: do one remaining-row audit pass for any other non-target-bound gaps still hidden inside the `Partial` rows before starting target-environment verification.
2. Target environment: rerun the strict commercial verifier with deploy and backup/restore enabled before renewing any final readiness claim.
3. Target environment: extend Observability/recovery proof into true OOM/crash restart, scale, and failover evidence; run configured provider/payment/workflow secret audits, live provider rail checks, and deployed gRPC/client compatibility checks.

## Boundary

This report is a repository rescan and evidence checkpoint, not a final completion claim. Rows marked `Partial` in `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` remain open until row-specific proof is recorded and rerun in the required environment.
