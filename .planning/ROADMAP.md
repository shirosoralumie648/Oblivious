# Roadmap: Oblivious

## Milestones

- ✅ **v08 Product Completeness** — Phase 25 through Phase 30 (completed 2026-05-29)
- ✅ **v07 Production Operations** — Phase 21 through Phase 24 (completed 2026-05-28)
- ✅ **v06 Billing And Marketplace Operations** — Phase 17 through Phase 20 (completed 2026-05-28)
- ✅ **v05 Relay Billing Completeness** — Phase 13 through Phase 16 (completed 2026-05-28)
- ✅ **v04 Commercial Foundation** — Phase 9 through Phase 12 (completed 2026-05-28)
- ✅ **v03.3 Mainline Consolidation** — Phase 5 through Phase 8 plus 999.1/999.2 follow-ups (completed 2026-05-27)
- ✅ **v03.2 Quality and Release** — Phase 4 (shipped 2026-05-14; Docker compose runtime validated 2026-05-12)
- ✅ **v03.1 Admin and Marketplace UI** — Phase 03.1 (shipped 2026-05-02)
- ✅ **Foundation through Admin/Marketplace Backend** — Phase 1, Phase 2, Phase 3a (completed 2026-04-27 through 2026-04-29)

## Current Status

Milestone v08 has been initialized from `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`.

**Next workflow step:** commercial complete objective is closed for the current repository-local evidence model; prepare normal commit/release review if desired.

## Current Milestone: v08 Product Completeness — Complete

**Goal:** Remove remaining MVP and placeholder customer-facing behavior so the platform matches the commercial SaaS promise across MCP, Agent, Knowledge, Chat, Admin, Marketplace, docs, onboarding, pricing, operator guides, and final end-to-end journeys.

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| Phase 25 | MCP Tool Commercial Behavior | PROD-01 | Complete |
| Phase 26 | Durable Agent Workflows | PROD-02 | Complete |
| Phase 27 | Knowledge Product Promise Alignment | PROD-03 | Complete |
| Phase 28 | Commercial UX and Journey Hardening | PROD-04 | Complete |
| Phase 29 | Public Docs Onboarding Pricing and Operator Guides | PROD-05 | Complete |
| Phase 30 | End-to-End Commercial Journey and Final Audit | PROD-06, AUDIT-01 | Complete |

### Phase 25: MCP Tool Commercial Behavior

**Goal:** Remove default-enabled placeholder MCP builtin behavior by making safe built-ins real and disabling or gating unsafe/provider-dependent built-ins from commercial use unless explicitly configured.

**Requirements:** PROD-01

**Success criteria:**
1. `calculator` returns deterministic parsed calculation results for supported arithmetic and rejects malformed or unsafe expressions without placeholder text.
2. `datetime` remains a real built-in with stable timezone/format behavior covered by tests.
3. `web_search` is not exposed or executable by default unless a real provider-backed configuration is present; disabled behavior must be explicit in API/tool metadata and product copy.
4. `http_request` is not exposed or executable by default for commercial Agents unless an explicit tenant-safe outbound policy exists; disabled behavior must not perform network I/O.
5. Agent tool execution and tool-definition generation cannot surface disabled commercial built-ins as enabled customer-facing tools.
6. Docs and quality gates prove no enabled default builtin returns customer-facing placeholder output.

**Planning evidence:**
- Context: `.planning/phases/25-mcp-tool-commercial-behavior/25-CONTEXT.md`
- Plan: `.planning/phases/25-mcp-tool-commercial-behavior/25-01-PLAN.md`

**Completion evidence:**
- Verification: `.planning/phases/25-mcp-tool-commercial-behavior/25-VERIFICATION.md`
- Summary: `.planning/phases/25-mcp-tool-commercial-behavior/25-01-SUMMARY.md`
- Code: `src/server/internal/mcp/builtin.go`, `src/server/internal/agent/executor.go`, `src/server/internal/agent/service.go`
- Tests: `src/server/internal/mcp/builtin_test.go`, `src/server/internal/agent/service_test.go`
- Docs/gates: `docs/API.md`, `docs/release/commercial-gates.md`, `scripts/verify-quality-gates.sh`

**Verified:**
- `cd src/server && go test ./internal/mcp ./internal/agent -run 'Builtin|Commercial|Tool|Calculator|WebSearch|HTTPRequest|Disabled' -count=1`
- `bash scripts/check.sh docs`
- `git diff --check`

**Boundary:** Phase 25 closes only `PROD-01`. Durable Agent workflow state, Knowledge RAG/copy alignment, broader UX hardening, public docs, end-to-end journeys, and final commercial audit remain Phase 26 through Phase 30 work.

### Phase 26: Durable Agent Workflows

**Goal:** Make Agent tool workflows durable and observable by introducing persisted run/tool-run state, approval gates, memory injection evidence, and failure/retry semantics instead of relying only on transient loop execution plus `agent_messages`.

**Requirements:** PROD-02

**Success criteria:**
1. Agent message execution creates an `agent_runs` record scoped by organization, conversation, agent, user, status, request metadata, memory usage, iteration count, and terminal error/final message references.
2. Each tool call creates an `agent_tool_runs` record with tool name/type, call ID, arguments, status, approval state, attempt count, result/error, timestamps, and organization scope.
3. Tools can require approval through Agent tool configuration; approval-required tool runs pause without executing and can later be approved or rejected through service/API methods.
4. Failed tool runs persist failure status and can be retried from the durable run record without duplicating successful tool results.
5. Memory injection evidence records whether memory was requested, whether memory search ran, and the number of injected results for each Agent run.
6. Runtime status APIs or service methods expose current run/tool-run state for UI and operator inspection.
7. DB-backed tests prove cross-tenant run/tool-run access is denied.

**Planning evidence:**
- Context: `.planning/phases/26-durable-agent-workflows/26-CONTEXT.md`
- Plan: `.planning/phases/26-durable-agent-workflows/26-01-PLAN.md`

**Completion evidence:**
- Verification: `.planning/phases/26-durable-agent-workflows/26-VERIFICATION.md`
- Summary: `.planning/phases/26-durable-agent-workflows/26-01-SUMMARY.md`
- Code: `src/server/migrations/0031_agent_workflow_runs.sql`, `src/server/internal/agent/store.go`, `src/server/internal/agent/runner.go`, `src/server/internal/agent/service.go`, `src/server/internal/http/agent_handler.go`, `src/server/internal/http/router.go`
- Tests: `src/server/internal/agent/store_test.go`, `src/server/internal/agent/service_test.go`, `src/server/internal/http/agent_handler_test.go`
- Docs/gates: `docs/API.md`, `docs/release/commercial-gates.md`, `scripts/verify-quality-gates.sh`

**Verified:**
- `cd src/server && TEST_DATABASE_URL='postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable' OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/agent ./internal/http -run 'AgentRun|ToolRun|Approval|Retry|MemoryEvidence|Tenant|Durable' -count=1`
- `bash scripts/check.sh docs`
- `git diff --check`

**Boundary:** Phase 26 closes only `PROD-02`. Knowledge RAG/copy alignment, broader UX placeholder audits, public docs, end-to-end journeys, and final commercial audit remain Phase 27 through Phase 30 work.

### Phase 27: Knowledge Product Promise Alignment

**Goal:** Align Knowledge with the commercial product promise by implementing embedding-backed RAG retrieval with source citations instead of relying on text/snippet search.

**Requirements:** PROD-03

**Success criteria:**
1. Knowledge document create/update indexes chunks with embeddings produced through the configured Relay embedder.
2. `knowledge_document_chunks` stores embedding vectors, embedding model metadata, index timestamps, and supports vector search through pgvector.
3. Retrieval embeds the query and ranks indexed chunks by vector similarity under organization scope.
4. Retrieval results include source citation fields: document ID/title, chunk ID, chunk index, similarity, retrieval method, and snippet.
5. The workspace Knowledge page renders source citations for retrieval results.
6. Docs and quality gates prevent customer-facing RAG claims unless embedding-backed retrieval and source citations are present.

**Planning evidence:**
- Context: `.planning/phases/27-knowledge-product-promise-alignment/27-CONTEXT.md`
- Plan: `.planning/phases/27-knowledge-product-promise-alignment/27-01-PLAN.md`

**Completion evidence:**
- Verification: `.planning/phases/27-knowledge-product-promise-alignment/27-VERIFICATION.md`
- Summary: `.planning/phases/27-knowledge-product-promise-alignment/27-01-SUMMARY.md`
- Code: `src/server/migrations/0032_knowledge_rag_index.sql`, `src/server/internal/knowledge/service.go`, `src/server/internal/knowledge/store.go`, `src/server/internal/http/router.go`, `src/web/src/routes/workspace/KnowledgePage.tsx`
- Tests: `src/server/internal/knowledge/service_test.go`, `src/server/internal/knowledge/store_test.go`, `src/server/internal/http/knowledge_handler_test.go`, `src/web/src/routes/workspace/KnowledgePage.test.tsx`
- Docs/gates: `docs/API.md`, `docs/release/commercial-gates.md`, `scripts/verify-quality-gates.sh`

**Verified:**
- `cd src/server && TEST_DATABASE_URL='postgres://oblivious:oblivious@127.0.0.1:32771/oblivious_test?sslmode=disable' OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/knowledge ./internal/http -run 'Knowledge|RAG|Retrieve|Citation|Source|Tenant' -count=1`
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test -- KnowledgePage --runInBand`
- `bash scripts/check.sh docs`
- `git diff --check`

**Boundary:** Phase 27 closes only `PROD-03`. Commercial UX hardening, public docs/onboarding/pricing/operator guides, end-to-end commercial journeys, and final completion audit remain Phase 28 through Phase 30 work.

### Phase 28: Commercial UX and Journey Hardening

**Goal:** Harden the active Chat, Agent/SOLO, Knowledge, Admin, and Marketplace customer journeys so they expose quota, Relay, budget, authorization, review, settlement, loading, empty, and recoverable error boundaries instead of presenting enabled fake-ready commercial behavior.

**Requirements:** PROD-04

**Success criteria:**
1. Chat shows retryable workspace-load errors, preserves drafts on send failures, exposes quota/Relay failure text, and removes duplicate SOLO handoff headings.
2. Agent/SOLO shows commercial run readiness with status, budget, authorization scope, knowledge scope, enabled/blocked tools, approval boundaries, and retry recovery context.
3. Marketplace detail, publish, review, install, and uninstall flows show paid/free/review/settlement boundaries and visible action errors without false success.
4. Admin dashboard, billing, and review surfaces prove commercial operation coverage across channels, routes, plans, billing, users, audit, and review queue.
5. Quality gates assert Phase 28 planning artifacts and active-route anti-fake-copy boundaries while preserving the no-final-readiness boundary.

**Planning evidence:**
- Context: `.planning/phases/28-commercial-ux-and-journey-hardening/28-CONTEXT.md`
- Plan: `.planning/phases/28-commercial-ux-and-journey-hardening/28-01-PLAN.md`

**Completion evidence:**
- Verification: `.planning/phases/28-commercial-ux-and-journey-hardening/28-VERIFICATION.md`
- Summary: `.planning/phases/28-commercial-ux-and-journey-hardening/28-01-SUMMARY.md`
- Code: `src/web/src/routes/workspace/ChatPage.tsx`, `src/web/src/routes/workspace/SoloPage.tsx`, `src/web/src/routes/workspace/KnowledgePage.tsx`, `src/web/src/routes/marketplace/MarketplaceAgentDetailPage.tsx`, `src/web/src/routes/marketplace/MarketplacePublishPage.tsx`, `src/web/src/routes/marketplace/MarketplaceMyAgentsPage.tsx`, `src/web/src/routes/admin/AdminHomePage.tsx`, `src/web/src/routes/admin/AdminBillingPage.tsx`, `src/web/src/routes/admin/AdminReviewsPage.tsx`
- Tests: `ChatPage.behavior.test.tsx`, `SoloPage.test.tsx`, `KnowledgePage.test.tsx`, `MarketplacePage.test.tsx`, `AdminHomePage.test.tsx`, `AdminBillingPage.test.tsx`, `AdminReviewsPage.test.tsx`

**Verified:**
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test -- AdminHomePage AdminBillingPage AdminReviewsPage --runInBand`
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test -- ChatPage SoloPage KnowledgePage MarketplacePage AdminHomePage AdminBillingPage AdminReviewsPage --runInBand`
- `bash scripts/check.sh docs`
- `git diff --check`

**Status:** Complete.

**Boundary:** Phase 28 closes only `PROD-04`. Public docs/onboarding/pricing/operator guides, final end-to-end journeys, `AUDIT-01`, Product Completeness Gate, and final commercial readiness remain Phase 29 and Phase 30 work.

### Phase 29: Public Docs Onboarding Pricing and Operator Guides

**Goal:** Align public docs, onboarding, pricing, and operator guides with implemented tenant, Relay, billing, Marketplace, operations, and product behavior.

**Requirements:** PROD-05

**Success criteria:**
1. README presents the current commercial multi-tenant AI SaaS platform, Relay invariant, quick start, documentation index, and no-final-readiness boundary.
2. Public product overview maps Chat, Agent/SOLO, Knowledge RAG, Relay, Admin, Marketplace, billing, tenant isolation, and operations to implemented behavior.
3. Onboarding guide maps customer, organization/admin, publisher, and operator setup steps without requiring committed live secrets.
4. Pricing guide explains subscription, top-up, quota, invoice, refund, Relay usage settlement, and Marketplace settlement without inventing production price points.
5. Operator guide links the verified deploy, backup, restore, observability, release, rollback, incident, and disaster recovery paths.
6. API and architecture contracts no longer carry stale v03.3, text-retrieval, pre-v05, or SOLO MVP wording.
7. Quality gates assert Phase 29 docs and stale-doc scan coverage while preserving the Phase 30 and `AUDIT-01` boundary.

**Planning evidence:**
- Context: `.planning/phases/29-public-docs-onboarding-pricing-and-operator-guides/29-CONTEXT.md`
- Plan: `.planning/phases/29-public-docs-onboarding-pricing-and-operator-guides/29-01-PLAN.md`

**Completion evidence:**
- Verification: `.planning/phases/29-public-docs-onboarding-pricing-and-operator-guides/29-VERIFICATION.md`
- Summary: `.planning/phases/29-public-docs-onboarding-pricing-and-operator-guides/29-01-SUMMARY.md`
- Docs: `README.md`, `docs/product/public-overview.md`, `docs/product/onboarding.md`, `docs/product/pricing.md`, `docs/product/operator-guide.md`, `docs/API.md`, `docs/architecture/current-system-contracts.md`, `docs/release/commercial-gates.md`
- Gates: `scripts/verify-quality-gates.sh`

**Verified:**
- `bash scripts/check.sh docs`
- `rg -n "v03.3|text matching|SOLO.*MVP|must receive final settlement/refund proof before v05 closes|release-candidate mainline" README.md docs/API.md docs/architecture/current-system-contracts.md docs/product docs/release/commercial-gates.md`
- `git diff --check`

**Status:** Complete.

**Boundary:** Phase 29 closes only `PROD-05`. End-to-end commercial journeys, `AUDIT-01`, Product Completeness Gate, and final commercial readiness remain Phase 30 work.

### Phase 30: End-to-End Commercial Journey and Final Audit

**Goal:** Prove the final commercial journey and completion audit for the full multi-tenant AI SaaS objective.

**Requirements:** PROD-06, AUDIT-01

**Success criteria:**
1. DB-backed backend journey proves signup/session, organization/tenant scope, provider/channel and route configuration, subscription/top-up lifecycle, Chat, Agent, Knowledge, Marketplace, Admin billing, and shared organization scope.
2. Browser journey proves the routed customer/operator experience across onboarding, Chat, Knowledge, SOLO/Agent, Marketplace, Admin, and billing context.
3. `scripts/verify-commercial-completion.sh` orchestrates docs, Relay security, targeted frontend tests, Playwright commercial journey, DB-backed backend journey, deployment validation, and backup/restore smoke.
4. Deployment validation and backup/restore smoke are strict final-readiness requirements; environment skips may be recorded but cannot close final readiness.
5. `docs/release/commercial-completion-audit.md` maps every commercial gate and every explicit user objective surface to files, commands, environment class, pass/fail status, skipped checks, and residual risk.
6. `30-VERIFICATION.md` records fresh command output and refuses Product Completeness Gate closure if any required check fails or is skipped.

**Planning evidence:**
- Context: `.planning/phases/30-end-to-end-commercial-journey-and-final-audit/30-CONTEXT.md`
- Plan: `.planning/phases/30-end-to-end-commercial-journey-and-final-audit/30-01-PLAN.md`

**Completion evidence:**
- Backend journey: `src/server/internal/http/commercial_journey_test.go`
- Browser journey: `src/web/e2e/commercial-journey.spec.ts`
- Verifier: `scripts/verify-commercial-completion.sh`
- Audit: `docs/release/commercial-completion-audit.md`
- Verification: `.planning/phases/30-end-to-end-commercial-journey-and-final-audit/30-VERIFICATION.md`
- Summary: `.planning/phases/30-end-to-end-commercial-journey-and-final-audit/30-01-SUMMARY.md`

**Verification targets:**
- `bash scripts/verify-commercial-completion.sh`
- `bash scripts/check.sh docs`
- `git diff --check`

**Verified:**
- `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome TEST_DATABASE_URL='postgres://oblivious:oblivious@127.0.0.1:32771/oblivious_test?sslmode=disable' COMMERCIAL_COMPLETION_RUN_DEPLOY=true COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 PG_CLIENT_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/verify-commercial-completion.sh`
- `bash scripts/check.sh docs`
- `git diff --check`

**Status:** Complete.

**Boundary:** Phase 30 is complete because strict final verification passed without hidden environment skips. Future readiness claims must rerun the strict verifier without `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true`.

## Archived Milestone: v07 Production Operations — Complete

**Goal:** Prove the platform is operable by a production team: orchestration starts the actual stack, migrations run safely, `/healthz` plus app/Relay paths smoke, tenant data can be backed up/restored, observability surfaces exist, and release/rollback/incident/DR runbooks are verified.

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| Phase 21 | Production Orchestration Runtime Proof | OPS-01, OPS-02 | Complete |
| Phase 22 | Backup Restore and Migration Recovery | OPS-03 | Complete |
| Phase 23 | Observability Alerts Dashboards and SLOs | OPS-04, OPS-05 | Complete |
| Phase 24 | Release Rollback Incident DR and v07 Closeout | OPS-02, OPS-06, DOC-06 | Complete |

### Phase 21: Production Orchestration Runtime Proof

**Goal:** Make production orchestration validation executable before v07 runbooks close: migration-aware compose validation, Kubernetes validation, app/Relay smoke, and restricted-network evidence.

**Requirements:** OPS-01, OPS-02

**Success criteria:**
1. Docker compose validation runs the migration binary against the compose database before declaring the stack healthy.
2. Runtime smoke probes `/healthz`, `/metrics`, one app API route, and one Relay route without live provider keys; app/Relay route checks must prove the route is mounted and not silently 404.
3. Kubernetes validation script validates/applies namespace, secret, config, Postgres, Redis, server, and web manifests, waits for rollouts, port-forwards the service, and runs the same smoke probe.
4. Missing `kubectl`/cluster tooling exits with a clear non-success status and instructions; it cannot be recorded as Kubernetes proof.
5. Normal and restricted-network validation commands are documented with exact environment overrides and evidence targets.

**Completion evidence:**
- Context: `.planning/phases/21-production-orchestration-runtime-proof/21-CONTEXT.md`
- Plan: `.planning/phases/21-production-orchestration-runtime-proof/21-01-PLAN.md`
- Verification: `.planning/phases/21-production-orchestration-runtime-proof/21-VERIFICATION.md`
- Summary: `.planning/phases/21-production-orchestration-runtime-proof/21-01-SUMMARY.md`

**Verified:**
- `DOCKER_BUILDKIT=1 docker build -f Dockerfile.postgres-pgvector -t oblivious-postgres-pgvector:pg16 .`
- Local pgvector container: `CREATE EXTENSION vector` and vector distance query passed.
- `cd src/server && go test ./internal/relay -run TestNewRelayRegistersCommercialChatRoute -count=1`
- `OBLIVIOUS_SERVER_HOST_PORT=18080 OBLIVIOUS_WEB_HOST_PORT=14173 OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh`
- `env -u OBLIVIOUS_K8S_SECRET_FILE bash scripts/k8s-validate.sh` failed with missing `kubectl`, proving the Kubernetes path does not silently pass when tooling is absent.

**Boundaries:**
- Backup/restore smoke was closed in Phase 22.
- Observability alerts, dashboards, and SLOs were closed in Phase 23.
- Release/rollback/incident/DR closeout was closed in Phase 24.
- v08 product completeness and final commercial readiness remain required.

### Phase 22: Backup Restore and Migration Recovery

**Goal:** Prove PostgreSQL tenant data and migration ledger state can be backed up and restored into a fresh database.

**Requirements:** OPS-03

**Success criteria:**
1. Backup script exports tenant-relevant PostgreSQL data without committing secrets or dumps.
2. Restore script loads into a fresh database and verifies `schema_migrations` integrity.
3. Smoke fixture proves organization, membership, quota/billing, Marketplace, and audit rows survive backup/restore.
4. Runbook documents operator prerequisites, retention expectations, encryption boundary, and failure handling.

**Planning evidence:**
- Context: `.planning/phases/22-backup-restore-and-migration-recovery/22-CONTEXT.md`
- Plan: `.planning/phases/22-backup-restore-and-migration-recovery/22-01-PLAN.md`

**Completion evidence:**
- Verification: `.planning/phases/22-backup-restore-and-migration-recovery/22-VERIFICATION.md`
- Summary: `.planning/phases/22-backup-restore-and-migration-recovery/22-01-SUMMARY.md`

**Verified:**
- `env -u BACKUP_DATABASE_URL -u DATABASE_URL bash scripts/backup-postgres.sh` failed with the expected prerequisite message.
- `env -u BACKUP_FILE RESTORE_DATABASE_URL=postgres://example:example@127.0.0.1:1/example bash scripts/restore-postgres.sh` failed with the expected prerequisite message.
- `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh` passed with 30 migration ledger checks and commercial tenant fixture verification.
- `bash scripts/check.sh docs`
- `git diff --check`

### Phase 23: Observability Alerts Dashboards and SLOs

**Goal:** Make production failures visible and actionable across HTTP, Relay, billing, jobs, and provider integrations.

**Requirements:** OPS-04, OPS-05

**Success criteria:**
1. Structured log fields cover request ID, tenant, user, route, status, latency, Relay route class, billing session, and provider/channel where applicable.
2. Metrics and tracing hooks cover HTTP, Relay, billing lifecycle, Marketplace settlement, background jobs, and provider failures.
3. Alert rules exist for Relay outage, quota settlement failure, webhook failure, migration failure, high provider error rate, and tenant isolation incidents.
4. Dashboards and SLO docs map each alert to metrics, thresholds, owner, and runbook.

**Planning evidence:**
- Context: `.planning/phases/23-observability-alerts-dashboards-and-slos/23-CONTEXT.md`
- Plan: `.planning/phases/23-observability-alerts-dashboards-and-slos/23-01-PLAN.md`

**Completion evidence:**
- Verification: `.planning/phases/23-observability-alerts-dashboards-and-slos/23-VERIFICATION.md`
- Summary: `.planning/phases/23-observability-alerts-dashboards-and-slos/23-01-SUMMARY.md`
- Code evidence: `src/server/internal/observability`, HTTP middleware, Relay policy/router, metrics package, Stripe lifecycle/webhook, quota service, Marketplace settlement, task service, and migration command instrumentation.
- Operator artifacts: `deploy/observability/prometheus-alerts.yaml`, `deploy/observability/grafana-dashboard.json`, and `docs/release/observability-slos.md`.
- Verified commands:
  - `cd src/server && go test ./internal/observability -count=1`
  - `cd src/server && go test ./internal/http ./internal/metrics -run 'Observability|Logging|Metrics|Request' -count=1`
  - `cd src/server && go test ./internal/relay ./internal/relay/handler -run 'Observability|RouteDecision|ProviderFailure' -count=1`
  - `cd src/server && go test ./internal/stripe ./internal/quota ./internal/marketplace ./internal/task ./cmd/migrate -run 'Observability|Metrics|Failure|Webhook|Settlement|Migration' -count=1`
  - `cd src/server && go test ./internal/observability ./internal/http ./internal/metrics ./internal/relay ./internal/relay/handler ./internal/stripe ./internal/quota ./internal/marketplace ./internal/task ./cmd/migrate -count=1`
  - `bash scripts/check.sh docs`
  - `git diff --check`

**Boundary:** Phase 23 closes only OPS-04 and OPS-05. Phase 24 owns OPS-02 final release-path evidence, OPS-06, DOC-06, v07 closeout, and the no-final-commercial-readiness boundary.

### Phase 24: Release Rollback Incident DR and v07 Closeout

**Goal:** Close v07 with verified operations evidence while leaving v08 product completeness visible as required future work.

**Requirements:** OPS-02, OPS-06, DOC-06

**Success criteria:**
1. Release and rollback runbooks are verified against deployment validation, migration status, smoke checks, and rollback commands.
2. Incident and disaster recovery runbooks link to alerts, dashboards, backup/restore, communication, and evidence capture steps.
3. v07 verification records exact commands, environment class, runtime smoke, skipped checks, known residual debt, and the reason final commercial readiness remains unclaimed until v08.
4. `.planning/milestones/v07-*` snapshots archive v07 completion only after all OPS requirements pass.

**Planning evidence:**
- Context: `.planning/phases/24-release-rollback-incident-dr-and-v07-closeout/24-CONTEXT.md`
- Plan: `.planning/phases/24-release-rollback-incident-dr-and-v07-closeout/24-01-PLAN.md`

**Completion evidence:**
- Verification: `.planning/phases/24-release-rollback-incident-dr-and-v07-closeout/24-VERIFICATION.md`
- Summary: `.planning/phases/24-release-rollback-incident-dr-and-v07-closeout/24-01-SUMMARY.md`
- Runbooks: `docs/release/release-rollback-runbook.md`, `docs/release/incident-response-runbook.md`, `docs/release/disaster-recovery-runbook.md`
- Evidence map: `docs/release/v07-operations-evidence.md`
- Verified:
  - Bare default `timeout 900 bash scripts/deploy-validate.sh` passed after `Dockerfile.server` reused `/go/pkg/mod` in build steps and default image tags were locally available.
  - Restricted/fallback compose deployment passed with migration and runtime smoke.
  - `env -u OBLIVIOUS_K8S_SECRET_FILE bash scripts/k8s-validate.sh` failed with missing `kubectl`, proving the Kubernetes path does not silently pass when tooling is absent.
  - `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh` passed.
  - `bash scripts/check.sh docs` and `git diff --check` passed.
- Boundary:
  - Fresh Docker Hub daemon pulls and live Kubernetes/Prometheus/Grafana/OTel/error-tracking vendor deployment remain environment-specific checks.
  - v08 Product Completeness and the final commercial audit remain required.

## Archived Milestone: v06 Billing And Marketplace Operations — Complete

**Goal:** Complete commercial money movement and Marketplace governance: Stripe checkout/webhooks, subscription lifecycle, invoices, refunds, failed-payment states, plan changes, top-ups, billing admin evidence, publisher settlement, platform fees, payout state, refund impact, and moderation workflows.

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| Phase 17 | Stripe Payment Authority and Webhook Ledger | PAY-01, PAY-02 | Complete |
| Phase 18 | Subscription Invoice Top-up Refund State Machine | PAY-03 | Complete |
| Phase 19 | Marketplace Settlement and Governance | MARKET-03, MARKET-04 | Complete |
| Phase 20 | Billing Admin Evidence and v06 Closeout | ADMIN-BILL-01, DOC-05 | Complete |

### Phase 17: Stripe Payment Authority and Webhook Ledger

**Goal:** Mount Stripe checkout and webhook routes in the running server, make checkout tenant-aware and testable without live Stripe calls, and record webhook events in a dedicated idempotent ledger after raw-body signature verification.

**Requirements:** PAY-01, PAY-02

**Success criteria:**
1. `POST /api/v1/billing/checkout` requires an authenticated tenant session and creates a payment intent/checkout record containing organization ID, user ID, package ID, checkout kind, amount, and provider checkout session ID.
2. Checkout creation uses an interface so route tests can use a fake Stripe client; live Stripe API keys are not required for automated tests.
3. `POST /api/v1/billing/stripe/webhook` is mounted as a public endpoint but rejects missing or invalid Stripe signatures before writing provider state.
4. Signed webhook fixture tests prove Stripe event IDs are recorded exactly once in a provider webhook ledger with event type, processing status, tenant metadata, payload, and error details.
5. Existing quota top-up and subscription mutation behavior is not silently marked paid before a verified payment event; full lifecycle application remains Phase 18.

**Likely verification:**
- `cd src/server && go test ./internal/stripe -run 'Webhook|Checkout|Ledger' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook' -count=1`
- `bash scripts/check.sh docs`

**Planning evidence:**
- Context: `.planning/phases/17-stripe-payment-authority-and-webhook-ledger/17-CONTEXT.md`
- Plan: `.planning/phases/17-stripe-payment-authority-and-webhook-ledger/17-01-PLAN.md`

**Completion evidence:**
- Summary: `.planning/phases/17-stripe-payment-authority-and-webhook-ledger/17-01-SUMMARY.md`
- Focused tests: `cd src/server && go test ./internal/stripe ./internal/config -count=1`
- DB-backed route tests: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook' -count=1`
- Broader package check: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe ./internal/http ./internal/config ./internal/quota -count=1`
- Docs gate: `bash scripts/check.sh docs`
- Broad quality gate: `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`
- Diff hygiene: `git diff --check`

### Phase 18: Subscription Invoice Top-up Refund State Machine

**Goal:** Apply verified Stripe events to subscription, invoice, top-up, failed-payment, plan-change, and refund state through an auditable, idempotent lifecycle service.

**Requirements:** PAY-03

**Success criteria:**
1. `checkout.session.completed` for subscription checkout completes the local payment intent, creates or updates an organization-scoped subscription, updates user plan assignment, and records one lifecycle transition.
2. `checkout.session.completed` for top-up checkout marks the top-up paid and credits tenant quota exactly once; no direct paid top-up flow can credit quota without verified payment evidence.
3. `invoice.paid` and `invoice.payment_failed` upsert invoice state and update subscription active/past-due or failed-payment state through append-only transitions.
4. `customer.subscription.updated` and `customer.subscription.deleted` preserve provider subscription IDs, period fields, cancel-at-period-end state, plan changes, and cancellation history.
5. Refund events create refund records, update payment intent refund state, reverse paid top-up effects once where applicable, and leave Marketplace refund impact for Phase 19.

**Likely verification:**
- `cd src/server && go test ./internal/stripe -run 'Lifecycle|CheckoutCompleted|Topup|Invoice|Refund|Subscription' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook|Topup|Invoice|Refund|Subscription' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe ./internal/http ./internal/quota -count=1`
- `bash scripts/check.sh docs`
- `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`
- `git diff --check`

**Planning evidence:**
- Context: `.planning/phases/18-subscription-invoice-topup-refund-state-machine/18-CONTEXT.md`
- Plan: `.planning/phases/18-subscription-invoice-topup-refund-state-machine/18-01-PLAN.md`

**Completion evidence:**
- Summary: `.planning/phases/18-subscription-invoice-topup-refund-state-machine/18-01-SUMMARY.md`
- Focused lifecycle tests: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe -run 'Lifecycle|CheckoutCompleted|Topup|Invoice|Refund|Subscription' -count=1`
- DB-backed route tests: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook|Topup|Invoice|Refund|Subscription' -count=1`
- Broader package check: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe ./internal/http ./internal/quota -count=1`
- Docs gate: `bash scripts/check.sh docs`
- Broad quality gate: `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`
- Diff hygiene: `git diff --check`

**Boundaries:**
- Marketplace settlement, platform fees, payout state, and refund impact were left to Phase 19 and are now complete.
- Admin billing inspection remains Phase 20.
- v07 production operations and v08 product completeness remain required.

### Phase 19: Marketplace Settlement and Governance

**Goal:** Model paid Marketplace order/settlement/payout state and add moderation/abuse governance before paid Marketplace operation is enabled.

**Requirements:** MARKET-03, MARKET-04

**Success criteria:**
1. Paid Marketplace installs create pending orders and checkout/payment intent state without installing the agent before verified payment evidence.
2. Verified `checkout.session.completed` events for Marketplace installs create one install, one paid order, and one settlement with gross amount, platform fee, publisher net, payout state, and append-only lifecycle/governance evidence.
3. Refund events update Marketplace order refund state and adjust/reverse settlement state exactly once.
4. Admin takedown, publisher appeal, admin reinstate, abuse report, and abuse resolution/dismissal workflows are recorded as governance events.
5. Publisher stats expose settlement-backed gross, platform fee, net, refund, pending, available, payout-pending, and paid-out amounts.

**Likely verification:**
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/marketplace -run 'Settlement|Governance|Abuse|Payout|PublisherStats' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Marketplace.*(Paid|Settlement|Refund|Takedown|Appeal|Abuse|PublisherStats)|Stripe.*Marketplace' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/marketplace ./internal/stripe ./internal/http -count=1`
- `bash scripts/check.sh docs`
- `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`
- `git diff --check`

**Planning evidence:**
- Context: `.planning/phases/19-marketplace-settlement-and-governance/19-CONTEXT.md`
- Plan: `.planning/phases/19-marketplace-settlement-and-governance/19-01-PLAN.md`

**Boundaries:**
- Admin billing inspection remains Phase 20.
- External payout provider execution is not enabled in Phase 19.
- v07 production operations and v08 product completeness remain required.

**Completion evidence:**
- Summary: `.planning/phases/19-marketplace-settlement-and-governance/19-01-SUMMARY.md`
- RED route tests: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Marketplace.*(Takedown|Appeal|Abuse|PublisherStats)|StripeRefundUpdatesMarketplaceSettlementOnce' -count=1` failed on missing governance routes and missing Marketplace settlement refund impact before implementation.
- Focused DB-backed verification: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test -p 1 ./internal/marketplace ./internal/http -run 'Settlement|Governance|Abuse|Payout|PublisherStats|Marketplace.*(Paid|Settlement|Refund|Takedown|Appeal|Abuse|PublisherStats)|Stripe.*Marketplace' -count=1`
- Broader DB-backed package check: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test -p 1 ./internal/marketplace ./internal/stripe ./internal/http -count=1`
- Docs gate: `bash scripts/check.sh docs`
- Broad quality gate: `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`
- Diff hygiene: `git diff --check`

### Phase 20: Billing Admin Evidence and v06 Closeout

**Goal:** Add read-only Admin billing inspection APIs/UI and close v06 with reproducible evidence while leaving v07/v08 visible as required future work.

**Requirements:** ADMIN-BILL-01, DOC-05

**Success criteria:**
1. Admin-only routes expose billing sessions, payment intents, webhook events, subscriptions, top-ups, invoices, refunds, Marketplace settlements, and payout state.
2. Inspection queries are read-only and do not mutate payment, quota, settlement, or payout state.
3. Admin Billing UI exposes every required surface with summary metrics, filters, and tabular records.
4. v06 closeout evidence maps PAY-01, PAY-02, PAY-03, MARKET-03, MARKET-04, ADMIN-BILL-01, and DOC-05 to files, tests, and runtime/database proof.
5. v07 Production Operations and v08 Product Completeness remain open before final commercial readiness.

**Planning evidence:**
- Context: `.planning/phases/20-billing-admin-evidence-and-v06-closeout/20-CONTEXT.md`
- Plan: `.planning/phases/20-billing-admin-evidence-and-v06-closeout/20-01-PLAN.md`

**Completion evidence:**
- Summary: `.planning/phases/20-billing-admin-evidence-and-v06-closeout/20-01-SUMMARY.md`
- Verification: `.planning/phases/20-billing-admin-evidence-and-v06-closeout/20-VERIFICATION.md`
- Backend focused test: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test -p 1 ./internal/http -run 'AdminBilling|BillingAdmin' -count=1`
- Backend broader package check: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test -p 1 ./internal/admin ./internal/http ./internal/stripe ./internal/marketplace -count=1`
- Frontend focused tests: `cd src/web && npx vitest run src/features/admin/api.test.ts src/routes/admin/AdminBillingPage.test.tsx src/features/layouts/AdminSidebar.test.tsx src/app/router.test.tsx`
- Docs gate: `bash scripts/check.sh docs`
- Broad quality gate: `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`
- Diff hygiene: `git diff --check`

## Archived Milestone: v05 Relay Billing Completeness — Complete

**Goal:** Make the Relay invariant true for every commercial AI surface: every `/v1/*` endpoint is classified, unsupported production behavior fails closed, supported behavior has auth/rate-limit/billing/audit semantics, and provider-bypass checks prove app services cannot call upstream LLM providers outside Relay.

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| Phase 13 | Relay Endpoint Authority and Production Fail-Closed | RELAY-08, RELAY-09 | Complete |
| Phase 14 | Relay Provider Bypass and Cost-Abuse Guardrails | RELAY-10, RELAY-11 | Complete |
| Phase 15 | Relay Billing Settlement and Refund Semantics | BILL-01, BILL-02 | Complete |
| Phase 16 | Relay Authority Evidence and v05 Closeout | DOC-04 | Complete |

### Phase 13: Relay Endpoint Authority and Production Fail-Closed

**Goal:** Create the authoritative route policy registry for all registered `/v1/*` endpoints and enforce production fail-closed behavior for disabled or partial endpoints before any provider call.

**Requirements:** RELAY-08, RELAY-09

**Success criteria:**
1. Policy tests prove every route from `src/server/internal/relay/handler/router.go` has exactly one commercial class.
2. Supported routes are explicitly marked as billed commercial surfaces; disabled routes list the reason and owning future work.
3. Production mode rejects disabled or partial routes before invoking native, passthrough, or file-proxy handlers.
4. `docs/API.md` and `docs/release/commercial-gates.md` expose the route table and fail-closed contract.
5. Local non-production behavior remains usable for implementation/testing where existing tests depend on route registration.

**Completion evidence:**
- Implementation commit: `3b9d4dd`
- Summary: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-01-SUMMARY.md`
- Route table: `docs/release/relay-route-table.md`
- Focused tests: `cd src/server && go test ./internal/relay/handler -count=1`
- Broader package check: `cd src/server && go test ./internal/relay/... ./internal/http -count=1`
- Docs gate: `bash scripts/check.sh docs`

**Planning evidence:**
- Context: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-CONTEXT.md`
- Plan: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-01-PLAN.md`

### Phase 14: Relay Provider Bypass and Cost-Abuse Guardrails

**Goal:** Prove production app services can only reach upstream LLM providers through Relay and attach auth, tenant, rate-limit, and audit policy to supported endpoint classes.

**Requirements:** RELAY-10, RELAY-11

**Success criteria:**
1. CI bypass checks fail if non-Relay packages import provider SDKs, instantiate direct provider clients, or hard-code direct provider URLs for AI calls.
2. Relay external/public entry points require an authenticated tenant/API identity appropriate to the endpoint class.
3. Supported endpoint classes have rate-limit policies that prevent cost-abuse before provider calls.
4. Relay audit events capture request identity, organization, endpoint class, policy result, channel, and failure reason.
5. App-internal Chat, Agent, and Knowledge embedding paths keep using Relay metadata and trusted internal headers.

**Likely verification:**
- `bash scripts/check.sh relay-security`
- `cd src/server && go test ./internal/relay ./internal/http -run 'Auth|RateLimit|Audit|Bypass' -count=1`
- Targeted `rg` checks proving direct provider URLs are limited to Relay/channel adapters and docs/examples.

**Completion evidence:**
- Summary: `.planning/phases/14-relay-provider-bypass-and-cost-abuse-guardrails/14-01-SUMMARY.md`
- Relay security gate: `bash scripts/check.sh relay-security`
- Focused policy tests: `cd src/server && go test ./internal/relay/handler -run 'SupportedRoutePolicies|ProductionSupportedRoutesRequireTrustedIdentity|ProductionSupportedRoutesAttachTrustedIdentityAndAudit|FailClosed|RoutePolicy' -count=1`
- App metadata tests: `cd src/server && go test ./internal/chat ./internal/memory -run 'HTTPReplyGenerator|RelayEmbedder|RelayIdentity' -count=1`
- Broader relay/http check: `cd src/server && go test ./internal/relay/... ./internal/http -count=1`
- Docs gate: `bash scripts/check.sh docs`

**Planning evidence:**
- Context: `.planning/phases/14-relay-provider-bypass-and-cost-abuse-guardrails/14-CONTEXT.md`
- Plan: `.planning/phases/14-relay-provider-bypass-and-cost-abuse-guardrails/14-01-PLAN.md`

### Phase 15: Relay Billing Settlement and Refund Semantics

**Goal:** Make supported Relay calls charge quota exactly once and refund correctly across success, upstream failure, retry, streaming abort, and async/disabled endpoint behavior.

**Requirements:** BILL-01, BILL-02

**Success criteria:**
1. Billing sessions are scoped by organization, user, endpoint/API type, model, channel, and idempotency key.
2. Successful supported calls pre-authorize and settle exactly once per idempotency key.
3. Upstream errors, client aborts, and unsupported production rejections refund or avoid charge consistently.
4. Streaming/realtime, file, batch, and async flows either have tested settlement semantics or are production-disabled with documented reason.
5. Regression tests cover native, passthrough/file-proxy-disabled, and streaming paths.

**Likely verification:**
- `cd src/server && go test ./internal/relay ./internal/quota -run 'Billing|Settlement|Refund|Idempotency|Streaming' -count=1`
- DB-backed server integration tests for Relay billing and quota ledger behavior.

**Completion evidence:**
- Summary: `.planning/phases/15-relay-billing-settlement-and-refund-semantics/15-01-SUMMARY.md`
- Focused billing lifecycle tests: `cd src/server && go test ./internal/relay -run 'DefaultPricingCovers|RouteWithBilling|BillingHook_Duplicate.*FreshSession' -count=1`
- Provider usage and route policy tests: `cd src/server && go test ./internal/relay/channel -run 'EstimateUsage' -count=1` and `cd src/server && go test ./internal/relay/handler -run 'ProviderResponseFromHTTP|ResponsesStreaming|BillingSettlementPolicy|RoutePoliciesDeclareBilling' -count=1`
- Broader relay/http check: `cd src/server && go test ./internal/relay/... ./internal/http -count=1`
- Docs and Relay security gates: `bash scripts/check.sh docs` and `bash scripts/check.sh relay-security`

### Phase 16: Relay Authority Evidence and v05 Closeout

**Goal:** Close v05 with reproducible evidence while keeping v06-v08 visible as required future commercial work.

**Requirements:** DOC-04

**Success criteria:**
1. `docs/release/relay-route-table.md` or equivalent documents every `/v1/*` route class, auth policy, rate-limit policy, billing policy, audit behavior, and production status.
2. `docs/release/commercial-gates.md` marks the Relay Authority Gate evidence as complete only after Phase 13-16 verification passes.
3. v05 verification records exact commands, environment class, DB migration status, passed checks, skipped checks, and residual v06-v08 work.
4. `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/STATE.md`, and `.planning/PROJECT.md` close v05 without claiming final commercial readiness.

**Likely verification:**
- `bash scripts/check.sh all`
- `bash scripts/test.sh all` with DB-backed coverage enabled
- Targeted `rg` checks for route table and commercial-gate references.

**Completion evidence:**
- Summary: `.planning/phases/16-relay-authority-evidence-and-v05-closeout/16-01-SUMMARY.md`
- Verification: `.planning/phases/16-relay-authority-evidence-and-v05-closeout/16-VERIFICATION.md`
- Route table: `docs/release/relay-route-table.md`
- Commercial gate: `docs/release/commercial-gates.md`
- Docs gate: `bash scripts/check.sh docs`
- Relay security gate: `bash scripts/check.sh relay-security`
- Focused Relay/http tests: `cd src/server && go test ./internal/relay/... ./internal/http -count=1`
- DB-backed all tests: `GOPROXY=... GOSUMDB=... TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true bash scripts/test.sh all`
- Broad checks: `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`

## Traceability

| Requirement | Phase | Coverage |
|-------------|-------|----------|
| OPS-01 | Phase 21 | Complete — equivalent compose orchestration starts stack, applies migrations, and smokes health/metrics/app/Relay paths |
| OPS-02 | Phase 21, Phase 24 | Complete — restricted/fallback smoke and bare default-command smoke passed; fresh Docker Hub daemon pull remains environment-specific |
| OPS-03 | Phase 22 | Complete — PostgreSQL backup/restore smoke and migration ledger integrity |
| OPS-04 | Phase 23 | Complete — structured logs, metrics, tracing, and error tracking coverage |
| OPS-05 | Phase 23 | Complete — alert rules, dashboards, and SLO definitions |
| OPS-06 | Phase 24 | Complete — release, rollback, incident, and DR runbooks |
| DOC-06 | Phase 24 | Complete — v07 verification and closeout evidence |
| PROD-01 | Phase 25 | Complete — default MCP built-ins are real or disabled from commercial use |
| PROD-02 | Phase 26 | Complete — durable Agent workflow runs, tool runs, approvals, retry/failure evidence, memory evidence, and tenant-scoped status APIs |
| PROD-03 | Phase 27 | Complete — Relay embedding-backed Knowledge RAG with pgvector retrieval and source citations |
| PROD-04 | Phase 28 | Complete — commercial UX and journey hardening |
| PROD-05 | Phase 29 | Complete — public docs, onboarding, pricing, and operator guides |
| PROD-06 | Phase 30 | Complete — end-to-end commercial journeys, deploy, backup, and restore verified |
| AUDIT-01 | Phase 30 | Complete — final commercial completion audit and strict evidence recorded |
| RELAY-08 | Phase 13 | Complete — route policy registry and complete `/v1/*` classification |
| RELAY-09 | Phase 13 | Complete — production fail-closed behavior for disabled/partial endpoints |
| RELAY-10 | Phase 14 | Complete — provider-bypass CI checks |
| RELAY-11 | Phase 14 | Complete — endpoint auth/rate-limit/audit semantics |
| BILL-01 | Phase 15 | Complete — quota pre-authorization, exactly-once settlement, and refund behavior |
| BILL-02 | Phase 15 | Complete — streaming/realtime/file/batch/async settlement or production disablement |
| DOC-04 | Phase 16 | Complete — v05 route table, evidence, and closeout |
| PAY-01 | Phase 17 | Complete — Stripe checkout route authority and tenant-aware payment intents |
| PAY-02 | Phase 17 | Complete — Stripe webhook signature verification and idempotent webhook ledger |
| PAY-03 | Phase 18 | Complete — subscription, invoice, failed-payment, plan-change, top-up, and refund lifecycle transitions |
| MARKET-03 | Phase 19 | Complete — publisher revenue, platform fee, payout state, and refund impact |
| MARKET-04 | Phase 19 | Complete — moderation and abuse workflows |
| ADMIN-BILL-01 | Phase 20 | Complete — admin inspection for billing and settlement evidence |
| DOC-05 | Phase 20 | Complete — v06 verification and closeout evidence |

## Archived Milestone Details

<details>
<summary>✅ v04 Commercial Foundation — COMPLETE 2026-05-28</summary>

**Goal:** Establish the SaaS tenant, security, migration, and CI foundation required before commercial Relay billing, Marketplace settlement, production operations, and final product completeness work.

**Requirements:** TENANT-01, TENANT-02, TENANT-03, TENANT-04, TENANT-05, SEC-01, SEC-02, SEC-03, MIGR-01, CI-01, DOC-03.

**Delivered:**
- Phase 9 organization tenant model and append-only migration ledger.
- Phase 10 auditable memberships, roles, invitations, ownership transfer, CSRF, rate limits, password policy, and session rotation.
- Phase 11 tenant scope across Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, and Marketplace publisher data with DB-backed cross-tenant denial tests.
- Phase 12 DB-backed CI required mode and commercial gate documentation.

**Archive snapshots:**
- `.planning/milestones/v04-ROADMAP.md`
- `.planning/milestones/v04-REQUIREMENTS.md`
- `.planning/milestones/v04-STATE.md`

</details>

<details>
<summary>✅ v03.3 Mainline Consolidation — COMPLETE 2026-05-27</summary>

**Goal:** Make the current uncommitted mainline work coherent, verified, documented, and ready for clean commits.

**Requirements:** CONS-01, ROUTE-01, CHAT-06, AUTH-01, DEPLOY-02, DOC-02, VERIFY-01.

**Delivered:**
- Phase 5 dirty worktree triage and commit-boundary inventory.
- Phase 6 backend route/service/auth/Relay hardening.
- Phase 7 frontend, E2E, CI, Docker, and deployment gate alignment.
- Phase 8 contract docs and release verification.
- Phase 999.1 missing Phase 01 summary reconstruction.
- Phase 999.2 obsolete workspace MarketplacePage cleanup and living requirements close policy verification.

**Archive snapshots:**
- `.planning/milestones/v03.3-ROADMAP.md`
- `.planning/milestones/v03.3-REQUIREMENTS.md`
- `.planning/milestones/v03.3-STATE.md`

</details>

<details>
<summary>✅ v03.2 Quality and Release — SHIPPED 2026-05-14</summary>

**Goal:** 补齐集成测试、E2E、API 文档和部署发布能力。

**Requirements:** TEST-01, TEST-02, DOC-01, DEPLOY-01.

**Primary verification:**
- `bash scripts/check.sh all`
- `bash scripts/test.sh all`
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e`
- `docker compose config`
- `OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_POSTGRES_IMAGE=docker.m.daocloud.io/pgvector/pgvector:pg16 OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh`

**Archive:** `.planning/milestones/v03.2-ROADMAP.md`

</details>

<details>
<summary>✅ v03.1 Admin and Marketplace UI — SHIPPED 2026-05-02</summary>

**Goal:** 实现 Admin 管理面板 UI 和 Marketplace 前端页面（8 Admin + 4 Marketplace 页面合同）。

**Requirements:** ADMIN-04, MARKET-02.

**Verification:** Go handler suite, 12 focused Vitest files / 32 tests, and `tsc --noEmit` passed.

**Archive:** `.planning/milestones/v03.1-ROADMAP.md`

</details>

<details>
<summary>✅ Foundation through Admin/Marketplace Backend — COMPLETED 2026-04-27 to 2026-04-29</summary>

- [x] Phase 1 Relay/Chat/Agent/MCP foundation — RELAY-01~07, CHAT-01~05, AGENT-01~10, MCP-01~07.
- [x] Phase 2 Agent 与 Memory 增强 — EXEC-01~03, MEM-01~03, QUOTA-01.
- [x] Phase 3a Admin 与 Marketplace 后端 — ADMIN-01~03, MARKET-01.

</details>

## Progress

| Milestone | Scope | Plans | Requirements | Status | Completed |
|-----------|-------|-------|--------------|--------|-----------|
| v08 Product Completeness | Phases 25-30 | 6/6 plans complete | 7/7 requirements complete | Complete | 2026-05-29 |
| v07 Production Operations | Phases 21-24 | 4/4 plans complete | 7/7 requirements complete | Complete | 2026-05-28 |
| v06 Billing And Marketplace Operations | Phases 17-20 | 4/4 plans complete | 7/7 requirements complete | Complete | 2026-05-28 |
| v05 Relay Billing Completeness | Phases 13-16 | 4/4 plans complete | 7/7 requirements complete | Complete | 2026-05-28 |
| v04 Commercial Foundation | Phases 9-12 | 4/4 plans complete | 11/11 requirements complete | Complete | 2026-05-28 |
| v03.3 Mainline Consolidation | Phases 5-8 plus backlog 999.1 and 999.2 | 12/12 steps complete | 7/7 requirements complete | Complete | 2026-05-27 |
| v03.2 Quality and Release | Phase 4 | 4/4 | TEST-01, TEST-02, DOC-01, DEPLOY-01 | Shipped | 2026-05-14 |
| v03.1 Admin and Marketplace UI | Phase 03.1 | 7/7 | ADMIN-04, MARKET-02 | Shipped | 2026-05-02 |
| Foundation through Backend | Phases 1, 2, 3a | Historical | RELAY, CHAT, AGENT, MCP, MEM, EXEC, QUOTA, ADMIN, MARKET | Complete | 2026-04-29 |

---
*Roadmap updated: 2026-05-29 after completing Phase 30 End-to-End Commercial Journey and Final Audit*
