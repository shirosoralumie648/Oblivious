# Repository Rescan - 2026-06-15

## Current Truth

- Branch: `main`; this browser-proof refresh starts from pushed commit `1786c0c docs: refresh repo rescan status`.
- Worktree status at refresh scan start: clean and in sync with `origin/main`.
- The project is still **not complete** against the four 2026-06-04 fusion specs.
- The current completion matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate after this implementation scan is **90/100**. Admin Relay channel API-key, Observability alert-provider config, Publishing channel config, Workflow definition/version/execution-snapshot/node-execution secret-like fields, Agent Memories browser CRUD/import-export, Billing provider lifecycle PostgreSQL transitions, and Admin Billing operator payout/refund browser proof are now covered with repository-local proof, but target-environment workflow telemetry, target secret audits, deployment validation, payment/provider live rails, and final no-skip release readiness remain open.

## What Changed In This Rescan

- Agent Memories now has browser-level Workspace proof for `/memories`: active navigation, search filter propagation, export query propagation, blob download-link rendering, user-managed create/update/delete, JSON import, and memory-count state updates.
- Admin Billing now has browser-level Admin-shell proof for `/admin/billing`: payout paid confirmation, payout failure with operator reason evidence, payout/top-up filter query propagation, and Stripe top-up refund payload/state propagation.
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

- First-party tracked file distribution after this browser-proof slice:
  - `src`: 979 files
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
  - Go test files: 228
  - Web component/API test files: 67
  - Web Playwright specs: 10 specs
  - Web E2E fixture files: 10 files
- Latest checked-in top-level migration remains `src/server/migrations/0081_admin_relay_channel_organization_scope.sql`.
- Project-local `AGENTS.md`: none at the main repo root or under first-party source; dependency caches and nested `reference/*` repositories are excluded from this first-party scan.

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

This scan does not reclassify any Partial row to Proven. Admin Relay API-key, Observability alert-provider, Publishing channel, Workflow at-rest encryption, Agent Memories browser proof, Billing provider lifecycle proof, and Admin Billing operator browser proof improve Relay/Publishing/Workflow, Agent, Frontend, Billing, Marketplace, API, Security, operations/env, and release evidence, but broader target-environment telemetry, target secret audits, live provider proof, and final release proof remain open.

## Verification Run During This Rescan

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
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh all
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web test src/features/agents/memoriesApi.test.ts src/routes/workspace/AgentMemoriesPage.test.tsx -- --runInBand
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec vitest run src/routes/admin/AdminBillingPage.test.tsx src/features/admin/api.test.ts
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/agent-memories.spec.ts --project=chromium
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/admin-billing-operator.spec.ts --project=chromium
git diff --check
```

Result:

- `git status --short --branch` showed `main...origin/main` before the Admin Billing browser-proof files were added.
- The current first-party inventory counters are: `src=979`, `docs=92`, `scripts=37`, `deploy=42`, `.planning=210`, Go test files `228`, web component/API test files `67`, Playwright specs `10`, and Playwright fixtures `10`.
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
- `scripts/verify-commercial-db-evidence.sh all` passed with disposable pgvector PostgreSQL and skipped tests: none, including the new Billing provider lifecycle profile.
- `bash scripts/check.sh docs` passed.
- `pnpm --dir src/web test src/features/agents/memoriesApi.test.ts src/routes/workspace/AgentMemoriesPage.test.tsx -- --runInBand` passed.
- `pnpm --dir src/web exec vitest run src/routes/admin/AdminBillingPage.test.tsx src/features/admin/api.test.ts` passed.
- `pnpm --dir src/web exec tsc --noEmit` passed.
- `pnpm --dir src/web exec playwright test e2e/agent-memories.spec.ts --project=chromium` passed with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome`.
- `pnpm --dir src/web exec playwright test e2e/admin-billing-operator.spec.ts --project=chromium` passed with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome`.
- `git diff --check` passed.
- The first-party AGENTS scan found no main-root or first-party source `AGENTS.md`.
- The TODO/stub scan did not reveal a new broad implementation gap. Active first-party matches remain the known release-boundary items from the June 14 scan: disabled future Relay surfaces, generated gRPC `Unimplemented*` boilerplate, test stubs, placeholder-only release docs/config examples, and the service-template migration TODO.
- The secret-storage scan now confirms MCP auth tokens, Admin Relay channel API keys, Observability alert-provider config secrets, Publishing channel config secrets, and Workflow definition/version/execution-snapshot/node-execution secret-like fields have repository-owned reversible at-rest protection. Target-environment secret audits remain the open secret-storage proof boundary.

## Notable Scan Findings

- The current live worktree is clean at `03b0a60`; the pre-slice checkpoint before Billing provider lifecycle evidence was `08ff744`.
- Existing DB-backed tenant-isolation evidence remains stronger than target-environment evidence; this slice adds DB-backed at-rest encryption proof for the Workflow definition/runtime secret path on top of the prior Admin Relay, Observability, and Publishing secret paths.
- `src/server/internal/http/admin_channel_secret_response_test.go` now pairs response redaction with direct SQL ciphertext assertions and an upstream probe assertion that the decrypted rotated key is usable.
- `src/server/internal/relay/store_test.go` now proves Relay runtime persistence writes protected channel API keys, hydrates raw keys for runtime callers, and preserves legacy plaintext compatibility.
- `src/server/internal/http/observability_alert_handler_test.go` now pairs Observability response redaction with direct SQL ciphertext assertions and a SQL-backed `/test` assertion that the decrypted webhook URL remains usable.
- `src/server/internal/http/channel_handler_test.go` now pairs Publishing response redaction with direct SQL ciphertext assertions and a SQL-backed signed webhook assertion that the decrypted channel secret remains usable.
- `src/server/internal/http/workflow_secret_response_test.go` now pairs Workflow response redaction with direct SQL ciphertext assertions for definitions, versions, execution snapshots, and workflow node executions, plus a SQL-backed signed webhook assertion that the decrypted workflow secret remains usable.
- Response safety and at-rest encryption remain separate. Admin Relay channel API keys, Observability alert-provider config secrets, Publishing channel config secrets, and Workflow definition/runtime secret-like payloads now have both response safety and at-rest encryption proof in repository-local PostgreSQL.
- Agent Memories now has a built-app Playwright proof for the manual memory-management workflow. The fixture fails closed if browser search/export filters or create/update/import/delete payloads drift from the Workspace UI controls.
- Admin Billing now has a built-app Playwright proof for operator money-movement actions. The fixture fails closed if browser payout/top-up filters, payout paid/failed payloads, or top-up refund provider evidence drift from the Admin Billing UI controls.
- Billing provider lifecycle DB tests are now first-class commercial evidence. The profile rejects skips and empty regex matches while proving subscription checkout, top-up checkout, invoice paid/payment-failed, subscription update/delete, and refund/quota reversal transitions against PostgreSQL.

## Recommended Next Slices

1. Rerun the strict commercial verifier on target infrastructure with deploy and backup/restore enabled before renewing any final readiness claim.
2. Extend Observability/recovery proof from repository-owned panic recovery into target-environment OOM/crash restart, scale, and failover evidence.
3. Run target-environment secret audits against configured provider/payment/workflow secrets, not only repository-local disposable PostgreSQL.
4. Keep Deployment and release readiness open until Kubernetes, backup/restore, migration replay, and provider/payment rails have target-environment proof.
5. Continue browser/E2E proof for the next uncovered high-value journey only where it narrows a specific Partial row.

## Boundary

This report is a repository rescan and evidence checkpoint, not a final completion claim. Rows marked `Partial` in `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` remain open until row-specific proof is recorded and rerun in the required environment.
