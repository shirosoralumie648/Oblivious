# Repository Rescan - 2026-06-15

## Current Truth

- Branch: `main`; this follow-up rescan starts from `6757a81 test(db): add core sql persistence evidence profile` and includes the current MCP Servers and Workflow advanced-controls built-browser evidence slices.
- The earlier runtime route-surface scan baseline was `26fe3be test(api): guard runtime route surface parity`. This report may be committed after the scanned implementation head; report-only refresh commits do not change the completion matrix or feature evidence.
- The project is still **not complete** against the four 2026-06-04 fusion specs.
- The current completion matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate after this scan is **98/100**. Admin Relay channel API-key, Observability alert-provider config, Observability alert/recovery SQL persistence, Publishing channel config, Workflow definition/version/execution-snapshot/node-execution secret-like fields, Workflow-to-Agent planning-control resume integration, Agent Memories browser CRUD/import-export, MCP Servers browser lifecycle/tool-execution proof, Workflow advanced-controls browser proof, Agent memory store PostgreSQL persistence, Agent gRPC registration and planning continue/adjust plus plan-step action and token-budget resume service-adapter proof, Workflow/Task gRPC generated-client dispatch and fail-closed boundary proof, Billing checkout/top-up HTTP PostgreSQL proof, Billing provider lifecycle PostgreSQL transitions, Admin usage analytics daily aggregate PostgreSQL proof, Marketplace governance/review PostgreSQL proof, Marketplace recommendation metadata OpenAPI/browser proof plus recommended-search PostgreSQL proof, Marketplace template route PostgreSQL proof, Chat/Publishing/Relay core SQL persistence proof, Admin Organization real-router PostgreSQL lifecycle proof, Admin Users quota allocation browser/API and real-router PostgreSQL/audit proof, Admin Billing operator payout/refund browser proof, Admin commercial configuration browser proof, Marketplace publisher/My Agents browser proof, Admin route manifest dispatch proof, runtime API route-surface reverse parity proof, durable Agent planning completion evidence, Agent plan adjustment browser proof, Admin Reviews browser moderation/governance proof, Admin Alerts browser alert-management proof, Scheduled Tasks built-app browser proof, and Kubernetes Secret/ConfigMap static contract proof are now covered with repository-local proof, but target-environment workflow telemetry, target secret audits, deployment validation/Kubernetes proof, payment/provider live rails, platform failover, live moderation/notification operations, deployed gRPC/client compatibility, and final no-skip release readiness remain open.

## What Changed In This Rescan

- Kubernetes Secret/ConfigMap static contract is now covered by `scripts/verify-deployment-operations-contract.sh`: it scans every tracked `deploy/kubernetes/*.yaml` file for `secretKeyRef`, `secretRef`, `configMapKeyRef`, and `configMapRef` entries, requires references to resolve to tracked `secret.example.yaml` or `configmap.yaml` contracts, and treats `ingress.yaml` `oblivious-tls` as a cert-manager-managed TLS Secret exception.
- `deploy/kubernetes/secret.example.yaml` now documents uppercase `DB_URL_CHAT`, `DB_URL_MARKETPLACE`, and `DB_URL_OBSERVABILITY`; Chat, Marketplace, Observability, and Workflow manifests reference those uppercase keys; Relay ConfigMap references use uppercase tracked keys; and the microservice deployment manifests load `oblivious-config` plus `oblivious-secrets` through `envFrom`.
- `docs/release/rc-checklist.md`, `docs/release/deployment-runtime-remediation.md`, `docs/release/fusion-spec-evidence-pack.md`, and `scripts/verify-quality-gates.sh` now clarify that target Kubernetes smoke uses `OBLIVIOUS_K8S_SECRET_FILE=/path/outside/git/secret.yaml bash scripts/k8s-validate.sh` and the script's explicit release allowlist, while other microservice manifests remain static/reference contracts until they are explicitly added to target smoke.
- `scripts/verify-commercial-db-evidence.sh observability-alert-recovery-persistence` now supplies no-skip disposable PostgreSQL proof for Observability alert routing rules, alert lifecycle and escalation, alert-state filters, notification throttle state, recovery cooldown reuse, and repeated delivery-batch history.
- `scripts/verify-commercial-db-evidence.sh admin-usage-analytics-db` now supplies no-skip disposable PostgreSQL proof for Admin usage daily aggregate refresh/query and zero-total-token fallback for raw analytics plus usage-log listing.
- `scripts/verify-commercial-db-evidence.sh marketplace-governance-review` now supplies no-skip disposable PostgreSQL proof for Marketplace automated review, takedown/appeal/reinstate, abuse reports, publisher notifications, needs-changes review state, review SLA enforcement, and real HTTP route persistence.
- `scripts/verify-commercial-db-evidence.sh marketplace-recommendation-search` now supplies no-skip disposable PostgreSQL proof for Marketplace recommended search content matching, deterministic exploration, ranking signals, collaborative filtering, and governance demotion.
- `scripts/verify-commercial-db-evidence.sh marketplace-template-routes` now supplies no-skip disposable PostgreSQL proof for Marketplace template create/list/detail/install HTTP routes.
- `scripts/verify-commercial-db-evidence.sh billing-checkout-topup-http` now supplies no-skip disposable PostgreSQL proof for Billing checkout/top-up HTTP routes, Stripe webhook signature/ledger/subscription retry behavior, and domestic top-up checkout/refund webhooks.
- `scripts/verify-commercial-db-evidence.sh agent-runtime-memory` now includes Agent memory store PostgreSQL create/list/filter/cross-tenant evidence in addition to durable run, plan-step, approval config, execution-mode, and memory-policy persistence.
- `scripts/verify-commercial-db-evidence.sh core-sql-persistence` now supplies no-skip disposable PostgreSQL proof for Chat share/fork/config/attachment/citation/persona/bookmark persistence, Publishing channel config/retry/manual-failover/archive persistence, and Relay semantic-cache pgvector persistence.
- `scripts/verify-commercial-db-evidence.sh all` now includes the Observability alert/recovery persistence, Admin usage analytics DB, Marketplace governance/review, Marketplace recommendation search, Marketplace template route, Billing checkout/top-up HTTP, expanded Agent memory store, and core SQL persistence profiles, so final commercial DB evidence cannot omit those state, accounting, governance, recommendation, template, billing, runtime-memory, Chat, Publishing, or Relay semantic-cache paths.
- `scripts/verify-commercial-db-evidence.sh tenant-membership-lifecycle` now also includes `TestAdminOrganizationRoutesPersistWithPostgres`, which drives real Admin organization HTTP create/list/detail/update/archive/member routes against disposable PostgreSQL and verifies SQL `created_by_user_id`, metadata, owner membership, updated status, and `archived_at` evidence.
- `scripts/verify-commercial-db-evidence.sh quota-sql-isolation` now also includes `TestAdminUserQuotaRoutePersistsWithPostgres`, which drives real Admin user quota `PATCH /api/v1/admin/users/{userId}` with signed admin cookie plus CSRF against disposable PostgreSQL and verifies SQL user-scoped quota persistence plus `user.quota.update` audit evidence.
- `src/web/e2e/scheduled-tasks.spec.ts` now supplies built-app Playwright proof for `/scheduled-tasks`: active Workspace navigation, schedule create payload trimming, disabled creation, enable status payload, run-now ordering after enable, recent-run listing, and mobile no-horizontal-overflow evidence.
- `src/web/e2e/mcp-servers.spec.ts` now supplies built-app Playwright proof for `/mcp-servers`: active Workspace navigation, tenant-safe local MCP catalog rendering, remote server connect/disconnect/diagnose/list-tools controls, invalid JSON guard, valid tool execution payload, create/delete lifecycle, and raw auth-token non-disclosure after creation.
- `src/web/e2e/workflows.spec.ts` now supplies built-app Playwright proof for Workflow advanced controls on `/workflows`: version history loading, rollback, branch create/publish/merge lifecycle, resource-check payload propagation, and paused-failure edited-input retry decision payloads. The fixture fails closed on unexpected API paths, payload drift, and out-of-order lifecycle calls.
- Marketplace recommended search metadata is now contract-gated, browser-proven, and PostgreSQL-proven: `docs/api/openapi.yaml` documents `MarketplacePublishedAgent.recommendation`, `scripts/verify-openapi-contract.sh` requires the `MarketplaceRecommendationMetadata` score/reason schema, `src/web/e2e/admin-marketplace.spec.ts` proves the built Marketplace browse page renders the recommended card label, `92% match`, and explanation reason while the fixture fails closed unless search preserves `sort=recommended`, and `scripts/verify-commercial-db-evidence.sh marketplace-recommendation-search` drives the DB-backed recommended search ranking cases without accepting skips.
- Workflow/Task gRPC now has generated-client compatibility proof through real registered servers. `src/server/pkg/workflow` fails closed with `FailedPrecondition` when no Workflow service is configured, and its bufconn test drives generated `WorkflowServiceClient.Execute` through `grpc.Server`. `src/server/pkg/task` fails closed when no scheduler is configured, and its bufconn test drives generated `TaskServiceClient.Schedule` and `Cancel` through `grpc.Server`.
- Runtime API route registration now has reverse route-surface parity proof: `TestRouteSurfaceRuntimeAPIRoutesAreDocumentedInManifestWithoutDatabase` parses active router registrations from `router.go` and called `routes_*.go` registrar files, then fails when a literal `/api/` `mux.Handle` or `mux.HandleFunc` registration lacks exact or prefix coverage in `docs/api/route-surface-manifest.json`.
- These repository slices improve Observability, Security, API contract, Agent, Frontend shell, and Release readiness evidence, but they do not reclassify any matrix row to Proven because target deployment, live provider rails, live moderation/notification operations, platform failover, deployed gRPC/client compatibility, and final no-skip release proof remain open.
- This pass closes the newly identified local DB evidence-profile opportunity by adding `core-sql-persistence`, closes the MCP Servers built-browser contract candidate, and closes the Workflow advanced-controls browser workflow candidate. Remaining local follow-up from the read-only frontend/API scans is mainly Console Notifications browser/API contract proof, while release-boundary drift remains target-bound: a new completion claim still needs the surrounding full Go/test/database/diff commands, deployment proof, Kubernetes proof, backup/restore, and strict no-skip verifier evidence recorded separately.
- After the Workflow advanced-controls browser slice, the next local proof candidate from the read-only frontend/API scans is Console Notifications browser contract.
- Agent Memories now has browser-level Workspace proof for `/memories`: active navigation, search filter propagation, export query propagation, blob download-link rendering, user-managed create/update/delete, JSON import, and memory-count state updates.
- Agent gRPC now has package-level service-adapter proof for planning run continuation and adjustment plus plan-step approve/execute/skip/retry actions, including authenticated `auth.Session` forwarding, refreshed run detail, cross-run plan-step rejection, and approval-boundary `FailedPrecondition` mapping.
- Agent gRPC now also has package-level service-adapter proof for token-budget resume through `ContinueBudget`: bounded `token_budget` validation, forwarding into `ContinueRunWithTokenBudget`, refreshed run detail reload, ReAct pending-tool evidence, planning plan-step evidence, and standalone generated-client bufconn dispatch.
- Workflow now wires the real Agent node executor into the default router/server workflow service after Agent service construction. Pending Workflow `agent` nodes can resume Agent planning control actions for continue, adjust-plan, token-budget continuation, and plan-step approve/execute/skip/retry, with organization/user/workspace scope forwarded and refreshed Agent run detail carried into downstream node output.
- Admin Users now has user-level quota allocation browser proof: `AdminUser` responses expose `quotaBalance`, SQL list/detail reads hydrate it from user-scope quota balance, the frontend API sends `PATCH /api/v1/admin/users/{userId}` with `{ balance }`, and `/admin/users` renders, edits, submits, and refreshes quota balance inside the Admin shell.
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
  - `src`: 989 files
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
  - Web Playwright specs: 15 specs
  - Web E2E fixture files: 15 files
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

This scan does not reclassify any Partial row to Proven. Admin Relay API-key, Observability alert-provider, Observability alert/recovery persistence, Publishing channel, Workflow at-rest encryption, Workflow-to-Agent planning-control resume integration, Workflow/Task gRPC generated-client dispatch proof, Workflow advanced-controls browser proof, Agent Memories browser proof, MCP Servers browser proof, Agent memory store DB proof, Agent gRPC planning and token-budget resume service-adapter proof, Billing provider lifecycle proof, Admin usage analytics DB proof, Chat/Publishing/Relay core SQL persistence proof, Admin Users quota allocation browser/API plus real-router PostgreSQL/audit proof, Admin Organization HTTP PostgreSQL lifecycle proof, Marketplace governance/review DB proof, Marketplace recommendation metadata OpenAPI/browser proof plus recommended-search DB proof, Admin Billing operator browser proof, Admin Reviews browser proof, Admin Alerts browser proof, Scheduled Tasks browser proof, and Kubernetes Secret/ConfigMap static contract proof improve Relay/Publishing/Workflow, Agent, Frontend, Billing, Marketplace, Observability, API, Security, operations/env, and release evidence, but broader target-environment telemetry, target secret audits, live provider proof, platform failover, live notification-provider proof, deployed gRPC/client compatibility, and final release proof remain open.

## Verification Evidence And Current Scan

Current `3f4a53c` baseline rescan commands, the current Kubernetes Secret/ConfigMap contract proof command, and already-pushed implementation verification commands:

```bash
pwd
git status --short --branch
git rev-parse HEAD origin/main
git log --oneline --decorate -8
git ls-files | awk '...inventory counters...'
git ls-files src/server/internal | awk '...domain counters...'
git ls-files 'src/server/**/*_test.go' | wc -l
git ls-files 'src/web/src/**/*.test.ts' 'src/web/src/**/*.test.tsx' | wc -l
git ls-files 'src/web/e2e/*.spec.ts' | wc -l
git ls-files 'src/web/e2e/fixtures/*' | wc -l
git ls-files src/server/migrations/*.sql | sort | tail -12
find . \( -path './node_modules' -o -path './src/web/node_modules' -o -path './.tmp' -o -path './reference' -o -path './.git' \) -prune -o -name AGENTS.md -print
awk '...top-level completion matrix rows...' docs/reports/2026-06-07-fusion-spec-completion-matrix.md
find src -maxdepth 4 -type f \( -name 'go.mod' -o -name 'package.json' -o -name 'vite.config.*' -o -name 'tsconfig.json' -o -name 'playwright.config.*' \) | sort
find scripts -maxdepth 2 -type f | sort
rg -n "^## |^### |^# " docs/superpowers/specs/2026-06-04-*.md
rg -n "^run_[a-z0-9_]+_profile\(\)|run_all_profiles|case \"\$profile\"" scripts/verify-commercial-db-evidence.sh
rg -n "TODO|FIXME|XXX|stub|placeholder|Unimplemented|DisabledInProduction" src scripts deploy docs/release docs/reports -g '!src/web/test-results/**' -g '!src/web/playwright-report/**' -g '!**/*.pb.go' -g '!docs/reports/archive/**'
rg -n "recommendation|Recommended|recommended|topRated|popular" src/server/internal/marketplace src/server/internal/http docs/api/openapi.yaml src/web/src/features/marketplace src/web/src/routes/marketplace src/web/e2e -S
rg -n "scheduled-tasks|ScheduledTasks|scheduledTasks|Schedule" src/web/src src/web/e2e docs/reports/2026-06-07-fusion-spec-completion-matrix.md docs/release/fusion-spec-evidence-pack.md -S
rg -n "quotaBalance|updateUserQuota|Quota Balance|user quota allocation|AdminUserQuotaUpdateRequest" docs/api/openapi.yaml src/server/internal/admin src/server/internal/http src/web/src src/web/e2e -S
git ls-files deploy/kubernetes/*.yaml | sort
if rg -n -w -e 'db-url-chat' -e 'db-url-marketplace' -e 'session-secret' -e 'oblivious-secret' -e 'db-credentials' -e 'observability-url' -e 'relay_default_model' -e 'relay_rate_limit_backend' -e 'relay_semantic_cache_backend' deploy/kubernetes; then exit 1; else echo legacy-drift-scan:ok; fi
bash scripts/verify-deployment-operations-contract.sh
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run 'TestRouteSurface(Manifest|RuntimeAPIRoutes)' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/admin ./internal/http -run 'Test(UpdateUserQuota|AdminHandlerUpdateUserQuota)' -count=1 -v
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec vitest run src/features/admin/api.test.ts src/routes/admin/AdminUsersPage.test.tsx
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/admin-commercial-config.spec.ts --project=chromium
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh billing-checkout-topup-http
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace -run 'Test(SearchAgentsRecommended|AddRecommendationMetadata|BuildOrderByRecommended)' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-recommendation-search
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-template-routes
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh core-sql-persistence
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run 'TestMarketplace(SearchFilterIncludesRequesterScopeWhenSessionExists|SearchFilterKeepsAnonymousRequesterScopeEmpty|HandlerPublicBrowseAndSessionGuards)' -count=1 -v
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec vitest run src/features/marketplace/api.test.ts src/routes/marketplace/MarketplacePage.test.tsx
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/admin-marketplace.spec.ts --project=chromium
bash scripts/verify-openapi-contract.sh
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./pkg/workflow ./pkg/task -count=1 -v
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
git diff --check
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
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./cmd/agent -count=1 -v
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
- `git status --short --branch` at the start of this MCP follow-up showed `main...origin/main` plus only `src/web/e2e/fixtures/mcpServers.ts` and `src/web/e2e/mcp-servers.spec.ts` as untracked files.
- `git rev-parse HEAD origin/main` returned `6757a81` for both refs before this slice was committed.
- `git log --oneline --decorate -8` showed `6757a81 test(db): add core sql persistence evidence profile`, `da004e6 test(k8s): guard manifest secret contract`, `3f4a53c test(release): gate commercial verifier k8s readiness`, `c87e167 test(billing): prove checkout topup HTTP persistence`, `3271af0 test(marketplace): prove template route persistence`, `ddb6c9d test(marketplace): prove recommendation search persistence`, `dec7f88 test(grpc): prove workflow task client dispatch`, and `bbe89d1 test(marketplace): prove recommendation metadata contract`.
- The top-level matrix count remains 4 `Proven` and 10 `Partial`; the `Gap` and `Unverified` counts remain 0.
- The current first-party inventory counters after this slice are: `src=989`, `docs=92`, `scripts=37`, `deploy=42`, `.planning=210`, Go test files `230`, web component/API test files `67`, Playwright specs `15`, and Playwright fixtures `15`.
- The current server-domain leaders are still `relay`, `http`, `mcp`, `admin`, `agent`, `workflow`, `knowledge`, `observability`, `channel`, `migration`, and `marketplace`.
- The latest checked-in top-level migration remains `src/server/migrations/0081_admin_relay_channel_organization_scope.sql`.
- The current first-party AGENTS scan found no `AGENTS.md` in the scanned first-party tree.
- `git ls-files deploy/kubernetes/*.yaml | sort` confirmed the tracked Kubernetes contract surface spans 27 YAML files, including the release-smoke files plus microservice, infra, and reference manifests.
- The exact-word Kubernetes drift scan for `db-url-chat`, `db-url-marketplace`, `session-secret`, `oblivious-secret`, `db-credentials`, `observability-url`, `relay_default_model`, `relay_rate_limit_backend`, and `relay_semantic_cache_backend` returned `legacy-drift-scan:ok`, so no old lowercase or legacy Secret/ConfigMap references remain in `deploy/kubernetes`.
- `bash scripts/verify-deployment-operations-contract.sh` and `COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs` both passed after the static Secret/ConfigMap reference scan was added and the release-quality gate was switched to the external-secret Kubernetes smoke entry.
- `scripts/verify-commercial-db-evidence.sh` now exposes `backend-journey`, `marketplace-money-movement`, `marketplace-governance-review`, `marketplace-recommendation-search`, `marketplace-template-routes`, `billing-checkout-topup-http`, `billing-provider-lifecycle`, `admin-usage-analytics-db`, `app-stateful-routes`, `tenant-membership-lifecycle`, `tenant-cross-surface`, `secret-response-safety`, `agent-runtime-memory`, `scheduled-task-runtime`, `core-sql-persistence`, `auth-security-persistence`, `relay-file-mapping-tenant-ownership`, `relay-runtime-channel-isolation`, `workflow-sql-isolation`, `publishing-channel-isolation`, `admin-relay-channel-isolation`, `admin-relay-read-isolation`, `observability-alert-recovery-persistence`, and `quota-sql-isolation`.
- The current local DB evidence-profile opportunity is now represented by `core-sql-persistence`; remaining local follow-up from the fresh scan is Console Notifications browser/API contract proof.
- `bash -n scripts/verify-commercial-db-evidence.sh` passed.
- `scripts/verify-commercial-db-evidence.sh tenant-membership-lifecycle` passed with disposable pgvector PostgreSQL and skipped tests: none after adding `TestAdminOrganizationRoutesPersistWithPostgres`. It now runs Tenant SQL organization/membership tests plus HTTP tests for default organization/session scope, legacy login scope, organization selection, invitation revoke rejection, session security on membership changes, member list/ownership/removal flows, and Admin organization create/list/detail/update/member/archive persistence.
- `scripts/verify-commercial-db-evidence.sh agent-runtime-memory` now includes `TestAgentMemoryStorePersistsAndFiltersMemories`, so Agent memory store create/list/filter/cross-tenant behavior is guarded by no-skip PostgreSQL evidence.
- `scripts/verify-commercial-db-evidence.sh observability-alert-recovery-persistence` passed with disposable pgvector PostgreSQL and skipped tests: none. It ran `TestSQLAlertRoutingRuleStorePersistsRoutingRules`, `TestSQLAlertStateStorePersistsAlertLifecycleAndEscalation`, `TestSQLAlertStateStoreListsAlertStatesWithFilters`, `TestSQLAlertStateStorePersistsNotificationThrottleAndRecoveryCooldown`, and `TestSQLAlertStateStoreRecordsRepeatedDeliveryBatchesForSameAlert`.
- `scripts/verify-commercial-db-evidence.sh admin-usage-analytics-db` passed with disposable pgvector PostgreSQL and skipped tests: none. It ran `TestSQLStoreUsageDailyAggregatesPostgresRefreshAndAnalytics`, `TestSQLStoreUsageAnalyticsRawRecordsFallsBackFromZeroTotalTokens`, and `TestSQLStoreListUsageLogsFallsBackFromZeroTotalTokens`.
- `scripts/verify-commercial-db-evidence.sh marketplace-governance-review` passed with disposable pgvector PostgreSQL and skipped tests: none. It ran Marketplace governance/automated-review persistence tests for takedown, appeal, reinstate, abuse-report lifecycle/listing/notification, automated review pass/reject, and needs-changes; it also ran HTTP route tests for review SLA enforcement, takedown/appeal/reinstate, abuse-report lifecycle/listing, publish-time automated review governance, and admin needs-changes.
- `scripts/verify-commercial-db-evidence.sh agent-runtime-memory` passed with disposable pgvector PostgreSQL and skipped tests: none. It ran Agent durable run lifecycle, structured plan-step persistence/update, approval/tool-risk config persistence, default execution-mode persistence, long-term memory policy persistence, and Agent memory store persistence/filtering/cross-tenant isolation.
- `go test ./pkg/agent -count=1 -v` passed with new Agent gRPC tests for `ContinueBudget`, `ContinuePlan`, `AdjustPlan`, plan-step approve/execute/skip/retry action forwarding, refreshed run detail mapping, request validation, nil-runtime failure, `auth.Session` forwarding into the internal Agent service, cross-run plan-step rejection, and approval-boundary error mapping.
- `go test ./cmd/agent -count=1 -v` passed with generated-client bufconn dispatch for both `ContinuePlan` and `ContinueBudget` through the registered standalone Agent gRPC service.
- `go test ./pkg/workflow ./pkg/task -count=1 -v` passed with Workflow generated-client `Execute` bufconn dispatch, Task generated-client schedule/cancel bufconn dispatch, and fail-closed missing Workflow service / Task scheduler boundaries.
- `go test ./internal/workflow -run 'TestAgentNodeExecutor|TestServiceResumeExecution.*Agent' -count=1 -v` passed with Workflow Agent node coverage for pending approval, token-budget pause, continue/adjust/budget/plan-step resume action routing, and downstream node output after plan-step and budget continuations.
- `go test ./internal/http -run 'TestWorkflowAgentServiceAdapter|TestRegisterWorkflowAgentExecutorRunsAgentNode|TestRouteSurfaceWiresConfiguredWorkflowSystemLimits' -count=1 -v` passed with HTTP adapter coverage for Agent planning control methods plus default Workflow Agent executor registration in router/server.
- `go test ./internal/http -run 'TestRouteSurface(Manifest|RuntimeAPIRoutes)' -count=1 -v` passed, including the new reverse parity test that parses active runtime `/api/` registrations and fails if any registration lacks route-surface manifest coverage.
- `go test ./internal/admin ./internal/http -run 'Test(UpdateUserQuota|AdminHandlerUpdateUserQuota)' -count=1 -v` passed with Admin quota service validation/audit coverage and HTTP handler response coverage for `quotaBalance`.
- `scripts/verify-commercial-db-evidence.sh quota-sql-isolation` passed with disposable pgvector PostgreSQL and skipped tests: none after adding `TestAdminUserQuotaRoutePersistsWithPostgres`. It now runs Quota SQL organization-scope tests plus HTTP tests for active-organization quota reads and Admin user quota route persistence/audit evidence.
- `pnpm --dir src/web exec playwright test e2e/scheduled-tasks.spec.ts --project=chromium` passed with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome`. It ran two browser tests covering `/scheduled-tasks` create/enable/run-now/recent-runs behavior and mobile no-horizontal-overflow evidence.
- `pnpm --dir src/web exec playwright test e2e/mcp-servers.spec.ts --project=chromium` passed with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome`. It ran the built-browser `/mcp-servers` journey covering Workspace navigation, local MCP catalog rendering, remote server connect/disconnect/diagnose/list-tools, invalid JSON guard, valid tool execution, server create/delete lifecycle, and raw auth-token non-disclosure.
- `pnpm --dir src/web exec vitest run src/features/mcp/mcpServersApi.test.ts src/routes/workspace/McpServersPage.test.tsx src/app/router.test.tsx` passed with API wrapper, page-level MCP lifecycle, and real-router Workspace shell coverage.
- `go test ./internal/http -run 'TestRouteSurfaceMCP(ReadRoutesRequireSessionWithoutDatabase|MutationsRejectCookieWithoutCSRFWithoutDatabase)' -count=1 -v` passed with DB-free MCP read-session and mutation-CSRF route-surface coverage.
- `bash scripts/verify-openapi-contract.sh` passed with MCP response secrecy, request schema, and security contract coverage still intact.
- `pnpm --dir src/web exec playwright test e2e/workflows.spec.ts --project=chromium` passed with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome`. It ran three browser tests, including the new Workflow advanced-controls journey covering version history, rollback, branch create/publish/merge, resource-check, and paused-failure edited-input retry controls.
- `pnpm --dir src/web exec vitest run src/routes/workspace/WorkflowsPage.test.tsx src/features/workflows/workflowsApi.test.ts src/app/router.test.tsx` passed with 3 files and 142 tests.
- `go test ./internal/http -run 'TestRegisterWorkflowRoutesDispatches(VersionHistoryAndRollback|CreateWorkflowBranch|BranchPublishAndMerge|ResourceCheck|PausedFailureDecision)|TestWorkflowHandler(ListWorkflowVersionsReturnsHistory|RollbackWorkflowPassesVersion|CreateWorkflowBranchPassesExperimentRequest|PublishWorkflowBranchPassesRequest|MergeWorkflowBranchPassesRequest|CheckResourceLimitsPassesUsage|CheckResourceLimitsReturnsUpdatedExecutionWhenLimitIsReached|WorkflowDecisionSupportsPausedFailureUserActions)' -count=1 -v` passed with route dispatch and handler payload coverage for Workflow version, rollback, branch, resource-check, and paused-failure decision APIs.
- `pnpm --dir src/web test src/features/scheduledTasks/scheduledTasksApi.test.ts src/routes/workspace/ScheduledTasksPage.test.tsx -- --runInBand` passed with 2 files and 11 tests.
- `scripts/verify-commercial-db-evidence.sh scheduled-task-runtime` passed with disposable pgvector PostgreSQL and skipped tests: none. It ran Scheduled Task SQL runtime tests plus HTTP route and Workflow trigger sync persistence tests.
- `scripts/verify-commercial-db-evidence.sh billing-checkout-topup-http` passed with disposable pgvector PostgreSQL and skipped tests: none. It runs real-router Billing checkout/top-up HTTP tests for session guards, tenant payment-intent persistence, explicit Stripe and domestic provider routing, unconfigured provider fail-closed behavior before artifacts, checkout-creator failure cleanup, direct top-up rejection, Stripe webhook signature/ledger/subscription retry behavior, and domestic top-up checkout/refund webhooks.
- `scripts/verify-commercial-completion.sh` now includes dependency security, web TypeScript, and target-gated Kubernetes validation in the strict verifier path. `bash -n scripts/verify-commercial-completion.sh`, `bash -n scripts/verify-quality-gates.sh`, and `bash scripts/verify-commercial-completion.sh --help` passed; the help output documents `COMMERCIAL_COMPLETION_RUN_K8S=true`, `OBLIVIOUS_K8S_SECRET_FILE`, and the fact that `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true` is partial local evidence only.
- `COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh security` passed for the newly orchestrated dependency-security gate. `pnpm audit` reported no known npm vulnerabilities, and `govulncheck v1.3.0` reported the code is affected by 0 vulnerabilities.
- `COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit` passed for the newly orchestrated web TypeScript gate.
- `go test ./internal/marketplace -run 'Test(SearchAgentsRecommended|AddRecommendationMetadata|BuildOrderByRecommended)' -count=1 -v` passed for DB-free recommendation ordering and explanation tests; `scripts/verify-commercial-db-evidence.sh marketplace-recommendation-search` now separately passes the DB-backed `SearchAgentsRecommended*` tests against disposable pgvector PostgreSQL with skipped tests: none.
- `scripts/verify-commercial-db-evidence.sh marketplace-template-routes` passed with disposable pgvector PostgreSQL and skipped tests: none. It runs `TestMarketplaceTemplateRoutesCreateListDetailAndInstall`, covering authenticated template creation, public template listing/filtering, public template detail, and authenticated install through the real HTTP router.
- `scripts/verify-commercial-db-evidence.sh core-sql-persistence` passed with disposable pgvector PostgreSQL and skipped tests: none. It runs Chat SQL tests for message/conversation sharing, scoped fork, config, attachments, citations, personas, boundary copies, and bookmark flags; Publishing channel SQL tests for config, message logs, retry claiming, manual failover, and archive retention; and Relay semantic-cache SQL tests for pgvector similarity lookup, persisted entries, and hit counts.
- `go test ./internal/http -run 'TestMarketplace(SearchFilterIncludesRequesterScopeWhenSessionExists|SearchFilterKeepsAnonymousRequesterScopeEmpty|HandlerPublicBrowseAndSessionGuards)' -count=1 -v` passed, confirming Marketplace public search/session-boundary behavior still dispatches through the real route surface.
- `pnpm --dir src/web exec vitest run src/features/marketplace/api.test.ts src/routes/marketplace/MarketplacePage.test.tsx` passed with 2 files and 40 tests, including recommendation metadata preservation and page-level recommended-card rendering.
- `pnpm --dir src/web exec playwright test e2e/admin-marketplace.spec.ts --project=chromium` passed with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome`. It ran five browser tests covering Admin navigation, Marketplace browse/detail/install, paid checkout-provider continuation, publisher publish/My Agents, and My Agents mutation contracts; the browse test now proves `Recommendation for Release Helper` renders `Recommended`, `92% match`, and the recommendation reason.
- `pnpm --dir src/web exec vitest run src/features/admin/api.test.ts src/routes/admin/AdminUsersPage.test.tsx` passed with frontend API `PATCH /api/v1/admin/users/{userId}` payload coverage and Admin Users drawer quota allocation coverage.
- `pnpm --dir src/web exec tsc --noEmit` passed after the Admin Users quota allocation API/type/page changes.
- `pnpm --dir src/web exec playwright test e2e/admin-commercial-config.spec.ts --project=chromium` passed with browser proof that `/admin/users` renders existing quota balance, sends the quota allocation PATCH payload, and refreshes the updated balance inside the Admin shell.
- `bash scripts/verify-openapi-contract.sh` passed after adding `quotaBalance` to the `AdminUser` response schema.
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
- `scripts/verify-commercial-db-evidence.sh all` passed before the later `core-sql-persistence` profile was added, including the Billing provider lifecycle, Observability alert/recovery persistence, Admin usage analytics DB, and Marketplace governance/review profiles. The new profile was verified directly above; final release readiness should rerun `all` after this wiring.
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
- The TODO/stub scan did not reveal a new broad implementation gap. Active first-party matches remain known release-boundary items: disabled future Relay surfaces, stale/alternate `src/server/internal/relay/handler_new` TODOs, generated gRPC `Unimplemented*` boilerplate, test stubs, placeholder-only release docs/config examples, UI input placeholders, and the service-template migration TODO.
- The secret-storage scan now confirms MCP auth tokens, Admin Relay channel API keys, Observability alert-provider config secrets, Publishing channel config secrets, and Workflow definition/version/execution-snapshot/node-execution secret-like fields have repository-owned reversible at-rest protection. Target-environment secret audits remain the open secret-storage proof boundary.

## Notable Scan Findings

- Previous scan text that referenced earlier slice starts such as `f1aa0ab`, `6db760c`, `099cc56`, `0e61dcc`, `2f53af9`, `1a83cff`, `9630167`, `8e4f9fd`, `d7a91f0`, `0655515`, `79d6000`, `b2fc74b`, or `93c5086` is stale after the Agent memory DB evidence, Admin Reviews browser-proof, Admin Alerts browser-proof, Agent gRPC planning-boundary and token-budget resume, Workflow-to-Agent planning-control resume integration, Workflow advanced-controls browser proof, Admin Users quota allocation browser/API plus real-router PostgreSQL proof, Admin Organization HTTP PostgreSQL proof, runtime registration, Admin commercial config, Marketplace publisher, durable planning completion, Admin manifest dispatch, Agent plan-adjustment browser, domestic Admin Billing top-up refund browser slices, runtime route-surface parity, quota allocation persistence, Scheduled Tasks browser proof, MCP Servers browser proof, Marketplace recommendation metadata contract/browser proof, Kubernetes Secret/ConfigMap contract proof, and core SQL persistence profile.
- The freshest local follow-up order from the read-only frontend/API scans is now Console Notifications browser contract. This is a local evidence-depth opportunity, not a blocker for the Workflow advanced-controls browser proof slice.
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
- MCP Servers now has a built-app Playwright proof for the Workspace remote-server lifecycle. The fixture fails closed if browser create payloads, lifecycle ordering, tool-execution payloads, delete ordering, or raw-token non-disclosure drift from the `/api/v1/app/mcp-servers` contract.
- Agent memory store persistence is now included in the no-skip DB profile. `TestAgentMemoryStorePersistsAndFiltersMemories` proves user-managed and long-term memory creation, metadata persistence, type/query filtering, and cross-tenant empty-list behavior against PostgreSQL.
- Agent gRPC now exposes planning continuation/adjustment and plan-step approve/execute/skip/retry through generated proto bindings and `src/server/pkg/agent`. The adapter does not bypass the internal planning service; it forwards authenticated sessions, validates run/step ownership, returns refreshed run detail after single-step actions, and reuses existing approval-boundary error semantics.
- Workflow and Task gRPC now have repository-local generated-client dispatch proof through registered `grpc.Server` instances. This narrows the gRPC compatibility gap for package-level service boundaries, but it still does not prove deployed endpoint reachability, target-network client compatibility, or production service discovery.
- Admin Users quota allocation now has browser/API plus real-router PostgreSQL proof. The fixture fails closed if the browser omits `PATCH /api/v1/admin/users/user_browser_entitlement` with `{ "balance": 2500 }`; `TestAdminUserQuotaRoutePersistsWithPostgres` proves the signed Admin route writes a user-scoped quota row and `user.quota.update` audit evidence against disposable PostgreSQL. This narrows the Admin user-level quota allocation route/persistence gap; live quota enforcement and target billing proof remain environment-bound.
- Admin Billing now has a built-app Playwright proof for operator money-movement actions. The fixture fails closed if browser payout/top-up filters, payout paid/failed payloads, Stripe refund evidence, or domestic Alipay `currency=cny` refund payloads drift from the Admin Billing UI controls.
- The domestic Admin Billing browser proof now records an Alipay top-up refund with provider payment-intent evidence while the fixture rejects any Stripe charge-ID payload, narrowing the local browser gap for domestic payment operator refunds.
- Admin Reviews now has a built-app Playwright proof for moderation and governance actions. The fixture fails closed if browser review filters, SLA enforcement queries, approve/reject/needs-changes payloads, abuse-report resolve/dismiss payloads, or takedown/reinstate governance reasons drift from the Admin Reviews UI controls.
- Admin Alerts now has a built-app Playwright proof for operational alert management. The fixture fails closed if browser alert filters, routing payloads, Slack provider creation payloads, acknowledge/resolve requests, provider tests, recovery-action rendering, or delivery-history inspection drift from the Admin Alerts UI controls.
- Scheduled Tasks now has a built-app Playwright proof for the Workspace schedule-control path. The fixture fails closed if schedule creation, enable/disable status mutation, run-now ordering, or recent-run listing drifts from the `/api/v1/scheduled-tasks` contract; the mobile test also guards document-level horizontal overflow on `/scheduled-tasks`.
- Workflow advanced controls now have a built-app Playwright proof in the Workspace shell. The fixture fails closed if version history, rollback, branch create/publish/merge, resource-check, or paused-failure edited-retry payloads drift from the `/api/v1/workflows` contract or execute out of order.
- Billing provider lifecycle DB tests are now first-class commercial evidence. The profile rejects skips and empty regex matches while proving subscription checkout, top-up checkout, invoice paid/payment-failed, subscription update/delete, and refund/quota reversal transitions against PostgreSQL.
- The release verifier boundary is still narrower than the fusion evidence pack's full command list: `scripts/verify-commercial-completion.sh` now runs docs, Relay security, dependency security, frontend, browser, DB profiles, deploy validation, target-gated Kubernetes validation, and backup/restore, while the broader `go test`, `TEST_DATABASE_URL` full-suite, and `git diff --check` checklist in the evidence pack still must be recorded separately for a fresh release claim.
- Kubernetes validation remains target-bound and partly contract-bound. The deployment rescan found that several microservice manifests are outside the current `k8s-validate.sh` smoke path and some Secret references do not derive cleanly from `deploy/kubernetes/secret.example.yaml`; either the Secret contract should be normalized locally or those older manifests should be explicitly marked non-release.

## Recommended Next Slices

1. Console Notifications browser contract: add built-browser proof for unread-count, mark-all-read, single mark-read, and delete UI/API behavior.
2. Target environment: rerun strict release validation with deploy, Kubernetes, backup/restore, failover, live provider/payment/notification rails, target secret audits, and deployed gRPC/client compatibility before any final completion claim.

## Boundary

This report is a repository rescan and evidence checkpoint, not a final completion claim. Rows marked `Partial` in `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` remain open until row-specific proof is recorded and rerun in the required environment.
