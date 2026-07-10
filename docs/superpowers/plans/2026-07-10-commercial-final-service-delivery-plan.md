# Commercial Final Service Delivery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver functional parity for all 17 independent services, prove the commercial golden journeys without fixture-only routes, and pass race, load, chaos, security, recovery, and final no-skip release gates.

**Architecture:** Execute service work in dependency-ordered batches with at most six concurrent writers and one worktree per service. Every service owns its process, private database, migrations, generated contracts, deployment, SLO, alerts, recovery evidence, and target proof; shared contracts and release files remain single-writer domains.

**Tech Stack:** Go 1.25, PostgreSQL, Kafka, ClickHouse, Qdrant, Redis/Valkey, S3-compatible object storage, React/TypeScript, Playwright, gRPC, OpenAPI, Helm/Kubernetes, k6, chaos tooling, existing commercial evidence framework.

---

## Batch Schedule

| Batch | Concurrent writers | Services | Exit condition |
|---|---:|---|---|
| F1 Foundation | 5 | `identity-access`, `api-gateway`, `event-contract-platform`, `platform-ops`, `observability-audit` | Independent process/database/deployment and complete audit chain |
| F2 AI and financial authority | 5 | `relay`, `knowledge-rag`, `tool-mcp`, `sandbox`, `billing-payment` | Measured usage, reservation/settlement, RAG, tools, and sandbox form one real chain |
| F3 Product runtime | 4 | `chat`, `agent`, `workflow`, `channel` | Product paths no longer depend on aggregate business handlers |
| F4 Post-runtime | 2 then 1 | `task-scheduler`, `marketplace`, then `admin-console` | Admin can operate and audit the preceding 16 services |
| F5 NFR/profile | 6 | four profiles plus performance/chaos and security/recovery lanes | One digest passes every non-functional gate |
| F6 RC | 1 writer, 2 reviewers | release integration | Final no-skip verifier exits `0` |

## Service Worktree Contract

Each service worktree may modify only the exact roots in this matrix plus tests and a service runbook beneath those roots:

| Service | Command | Owned implementation | Proto | Migration | Image | Helm template |
|---|---|---|---|---|---|---|
| `identity-access` | `src/server/cmd/identity-access/` | `src/server/internal/identityaccess/` | `api/proto/oblivious/identityaccess/` | `src/server/migrations/services/identity-access/` | `deploy/docker/Dockerfile.identity-access` | `deploy/helm/oblivious/templates/services/identity-access.yaml` |
| `api-gateway` | `src/server/cmd/gateway/` | `src/server/internal/gateway/` | `api/proto/oblivious/apigateway/` | `src/server/migrations/services/api-gateway/` | `deploy/docker/Dockerfile.gateway` | `deploy/helm/oblivious/templates/services/api-gateway.yaml` |
| `event-contract-platform` | `src/server/cmd/event-contract-platform/` | `src/server/internal/eventcontract/` | `api/proto/oblivious/eventcontractplatform/` | `src/server/migrations/services/event-contract-platform/` | `deploy/docker/Dockerfile.event-contract-platform` | `deploy/helm/oblivious/templates/services/event-contract-platform.yaml` |
| `platform-ops` | `src/server/cmd/platform-ops/` | `src/server/internal/platformops/` | `api/proto/oblivious/platformops/` | `src/server/migrations/services/platform-ops/` | `deploy/docker/Dockerfile.platform-ops` | `deploy/helm/oblivious/templates/services/platform-ops.yaml` |
| `observability-audit` | `src/server/cmd/observability-audit/` | `src/server/internal/observability/` | `api/proto/oblivious/observabilityaudit/` | `src/server/migrations/services/observability-audit/` | `deploy/docker/Dockerfile.observability-audit` | `deploy/helm/oblivious/templates/services/observability-audit.yaml` |
| `relay` | `src/server/cmd/relay/` | `src/server/internal/relay/` | `api/proto/oblivious/relay/` | `src/server/migrations/services/relay/` | `deploy/docker/Dockerfile.relay` | `deploy/helm/oblivious/templates/services/relay.yaml` |
| `knowledge-rag` | `src/server/cmd/knowledge-rag/` | `src/server/internal/knowledge/`, `src/server/internal/rag/` | `api/proto/oblivious/knowledgerag/` | `src/server/migrations/services/knowledge-rag/` | `deploy/docker/Dockerfile.knowledge-rag` | `deploy/helm/oblivious/templates/services/knowledge-rag.yaml` |
| `tool-mcp` | `src/server/cmd/tool-mcp/` | `src/server/internal/mcp/` | `api/proto/oblivious/toolmcp/` | `src/server/migrations/services/tool-mcp/` | `deploy/docker/Dockerfile.tool-mcp` | `deploy/helm/oblivious/templates/services/tool-mcp.yaml` |
| `sandbox` | `src/server/cmd/sandbox/` | `src/server/internal/sandbox/` | `api/proto/oblivious/sandbox/` | `src/server/migrations/services/sandbox/` | `deploy/docker/Dockerfile.sandbox` | `deploy/helm/oblivious/templates/services/sandbox.yaml` |
| `chat` | `src/server/cmd/chat/` | `src/server/internal/chat/` | `api/proto/oblivious/chat/` | `src/server/migrations/services/chat/` | `deploy/docker/Dockerfile.chat` | `deploy/helm/oblivious/templates/services/chat.yaml` |
| `agent` | `src/server/cmd/agent/` | `src/server/internal/agent/` | `api/proto/oblivious/agent/` | `src/server/migrations/services/agent/` | `deploy/docker/Dockerfile.agent` | `deploy/helm/oblivious/templates/services/agent.yaml` |
| `workflow` | `src/server/cmd/workflow/` | `src/server/internal/workflow/` | `api/proto/oblivious/workflow/` | `src/server/migrations/services/workflow/` | `deploy/docker/Dockerfile.workflow` | `deploy/helm/oblivious/templates/services/workflow.yaml` |
| `task-scheduler` | `src/server/cmd/task-scheduler/` | `src/server/internal/task/`, `src/server/internal/schedule/` | `api/proto/oblivious/taskscheduler/` | `src/server/migrations/services/task-scheduler/` | `deploy/docker/Dockerfile.task-scheduler` | `deploy/helm/oblivious/templates/services/task-scheduler.yaml` |
| `channel` | `src/server/cmd/channel/` | `src/server/internal/channel/` | `api/proto/oblivious/channel/` | `src/server/migrations/services/channel/` | `deploy/docker/Dockerfile.channel` | `deploy/helm/oblivious/templates/services/channel.yaml` |
| `billing-payment` | `src/server/cmd/billing-payment/` | `src/server/internal/billing/`, `src/server/internal/payment/`, `src/server/internal/quota/`, `src/server/internal/usage/`, `src/server/internal/stripe/` | `api/proto/oblivious/billingpayment/` | `src/server/migrations/services/billing-payment/` | `deploy/docker/Dockerfile.billing-payment` | `deploy/helm/oblivious/templates/services/billing-payment.yaml` |
| `marketplace` | `src/server/cmd/marketplace/` | `src/server/internal/marketplace/` | `api/proto/oblivious/marketplace/` | `src/server/migrations/services/marketplace/` | `deploy/docker/Dockerfile.marketplace` | `deploy/helm/oblivious/templates/services/marketplace.yaml` |
| `admin-console` | `src/server/cmd/admin-console/` | `src/server/internal/admin/`, `src/server/internal/console/` | `api/proto/oblivious/adminconsole/` | `src/server/migrations/services/admin-console/` | `deploy/docker/Dockerfile.admin-console` | `deploy/helm/oblivious/templates/services/admin-console.yaml` |

Service workers must not modify shared OpenAPI/Proto/event sources, contract baselines, ownership manifests, Helm common values, dependency locks, CI workflows, or commercial release scripts without an approved lease.

## Task 0: Add Service Test Runners

**Files:**

- Create: `scripts/test-service.sh`
- Create: `scripts/test-service-batch.sh`
- Create: `scripts/test-service-fixtures.sh`

- [ ] **Step 1: Implement `scripts/test-service.sh`**

The script accepts one approved service ID and `--race` or `--contract`, loads exact Go package roots from `docs/governance/service-ownership.json`, requires `TEST_DATABASE_URL`, and refuses legacy IDs such as `gateway`, `rag`, `billing`, `task`, and `admin`.

- [ ] **Step 2: Implement `scripts/test-service-batch.sh`**

Batch mappings are fixed: `foundation`, `authority`, `product`, and `post-runtime`. The script runs `test-service.sh` serially for database-backed tests and returns the first failure.

- [ ] **Step 3: Add runner fixture tests**

Verify accepted service IDs, rejected legacy IDs, missing database URL, race flag forwarding, contract flag forwarding, and exact package selection.

- [ ] **Step 4: Run and commit**

```bash
bash scripts/test-service-fixtures.sh
git add scripts/test-service.sh scripts/test-service-batch.sh \
  scripts/test-service-fixtures.sh
git commit -m "test: add service and batch test runners"
```

## Common Service DoD

Every service must provide:

1. Independent process, health, readiness, graceful shutdown, and capacity settings.
2. Independent database name, role, credential, migration ledger, backup, and restore proof.
3. Tenant isolation, authentication, authorization, service identity, and negative cross-tenant tests.
4. Idempotency, timeout, retry policy, circuit behavior, outbox/inbox, DLQ, and operator redrive.
5. Generated provider and consumer contracts with compatibility tests.
6. Structured logs, metrics, traces, audit events, SLO, alerts, dashboards, and runbook.
7. Unit, component, contract, database, integration, race, and target-environment tests.
8. Upgrade, rollback or forward-fix, backup/restore, and threat model evidence.

## Standard TDD Loop

Every service slice follows these steps:

- [ ] **Step 1: Write the provider contract failure test**

The test calls the generated server interface and asserts the approved success and error model.

- [ ] **Step 2: Run the provider test and confirm failure**

```bash
bash scripts/test-service.sh identity-access --contract
```

Expected: failure identifies the missing behavior, route, migration, or adapter.

- [ ] **Step 3: Write the consumer contract failure test**

The consumer test uses the generated client; direct internal-package imports across services are forbidden.

- [ ] **Step 4: Implement the smallest vertical slice**

The slice includes API/gRPC handler, domain state transition, private migration, outbox event, inbox idempotency, telemetry, and recovery behavior.

- [ ] **Step 5: Run service tests with race and database enforcement**

```bash
TEST_DATABASE_URL="$SERVICE_TEST_DATABASE_URL" \
OBLIVIOUS_REQUIRE_TEST_DATABASE=true \
bash scripts/test-service.sh identity-access --race
```

Expected: exit `0`, no race, no database skip, and no fixture-only success route.

- [ ] **Step 6: Run contract and profile render checks**

```bash
bash scripts/check.sh contracts
helm template oblivious deploy/helm/oblivious \
  -f deploy/profiles/self-hosted.yaml \
  | kubeconform -strict -summary
```

Expected: contract baseline remains compatible and the service renders successfully.

- [ ] **Step 7: Commit the service slice**

Use one service-specific contract commit, such as `contract(identity-access): freeze access APIs`, followed by one service-specific implementation commit, such as `feat(identity-access): add independent access runtime`.

## Task 1: Deliver Foundation Services

### `identity-access`

**Current sources:** `src/server/internal/auth/`, `src/server/internal/tenant/`, user/session routes in `src/server/internal/http/`.

**Target files:** `src/server/cmd/identity-access/`, `src/server/internal/identityaccess/`, `src/server/migrations/services/identity-access/`, `deploy/docker/Dockerfile.identity-access`, service contract and Helm template.

- [ ] Extract users, organizations, memberships, invitations, sessions, API credentials, MFA, OIDC, SAML, SCIM, RBAC/ABAC, and service identity into the owned service.
- [ ] Add tests for invitation replay, session revocation, cross-tenant membership access, role downgrade, MFA challenge expiry, and service-token audience.
- [ ] Prove all other services trust signed identity context and cannot query identity tables.

### `event-contract-platform`

**Current sources:** `src/server/pkg/event/`, Kafka configuration, release evidence handlers.

**Target files:** `src/server/cmd/event-contract-platform/`, `src/server/internal/eventcontract/`, `src/server/migrations/services/event-contract-platform/`, image, contract, and Helm template.

- [ ] Implement schema registry metadata, topic catalog API, compatibility result storage, DLQ inventory, and redrive requests.
- [ ] Add tests for incompatible schema rejection, duplicate event suppression, handler failure without offset commit, DLQ authorization, and redrive identity preservation.
- [ ] Prove the platform manages contracts but never owns domain event state.

### `observability-audit`

**Current sources:** `src/server/cmd/observability/`, `src/server/internal/observability/`, ClickHouse and alert assets.

**Target files:** rename service entry to `src/server/cmd/observability-audit/`, isolate audit/incident/SLO persistence and deployment.

- [ ] Persist request logs, traces, audit records, incidents, alert state, and SLO evidence without mutating domain truth.
- [ ] Add tests for immutable audit append, tenant-scoped query, alert delivery retry, recovery action audit, and ClickHouse outage degradation.
- [ ] Prove request, trace, usage, reservation, and ledger IDs join across J02.

### `platform-ops`

**Current sources:** deployment scripts, recovery scripts, release handlers, and evidence collectors.

**Target files:** `src/server/cmd/platform-ops/`, `src/server/internal/platformops/`, private database, image, contract, and Helm template.

- [ ] Implement release manifest, deployment, upgrade, rollback, backup, restore, DR exercise, compliance result, and evidence-reference state machines.
- [ ] Add tests for duplicate deployment request, failed backup blocking upgrade, expired adapter certification, and immutable release digest.
- [ ] Prove Air-gap does not require the hosted control plane.

### `api-gateway`

**Current sources:** `src/server/cmd/gateway/`, `src/server/internal/gateway/`.

**Target files:** retain paths but replace health-shell routing with generated-client routing to all 16 internal services.

- [ ] Implement trusted identity propagation, HTTP/SSE/WebSocket proxying, per-route timeout, rate limit, circuit breaker, mTLS identity, health removal, and request correlation.
- [ ] Add tests for forged identity headers, streaming cancellation, WebSocket origin/auth, unhealthy route removal, idempotency-key forwarding, and service error mapping.
- [ ] Prove no product request is handled by the aggregate server.

**Foundation verification:**

```bash
TEST_DATABASE_URL="$INTEGRATION_DATABASE_URL" \
  bash scripts/test-service-batch.sh foundation --race
pnpm --dir src/web exec playwright test \
  src/web/e2e/target/j01-admin-configuration.spec.ts
```

Expected: five services run independently; J01 creates provider/payment/channel configuration and an immutable audit record.

## Task 2: Deliver AI Runtime And Financial Authority

### `relay`

- [ ] Keep Relay as the only service holding shared AI provider credentials and performing billable provider calls.
- [ ] Complete Chat Completions, Responses, Embeddings, Image/Audio, Files, Batch, and Realtime lifecycle behavior.
- [ ] Add tests for provider timeout/429, streaming cancel, Realtime disconnect, Batch retry, File cleanup, pricing snapshot, exact measured usage, and provider reconciliation.

### `billing-payment`

**Current sources:** `src/server/internal/billing/`, `payment/`, `quota/`, `usage/`, `stripe/`, and billing routes.

- [ ] Implement catalog, rating, quota reserve/commit/release, order/payment/refund/dispute, invoice/tax, immutable double-entry ledger, and reconciliation.
- [ ] Add Stripe, Alipay, and WeChat Pay adapter contracts with signed webhook validation and idempotent state transitions.
- [ ] Add tests proving Billing consumes Relay `usage_id` and never estimates provider usage.

### `knowledge-rag`

**Current sources:** `src/server/internal/knowledge/`, `src/server/internal/rag/`, `src/server/cmd/rag/`.

- [ ] Start durable ingestion and index workers in the production service entrypoint.
- [ ] Implement object originals, parser isolation, OCR, chunking, Relay embeddings, Qdrant indexing, hybrid retrieval, reranking, citations, document versioning, evaluation, deletion propagation, retry, and DLQ.
- [ ] Add tests for worker restart, drain, stale vector filtering, raw parser replay, version replacement, and delete completeness.

### `tool-mcp`

**Current sources:** `src/server/internal/mcp/`, agent tool metadata and routes.

- [ ] Implement MCP registry, versioned schemas, tenant credential references, OAuth, risk policy, approval, timeout, cancellation, egress policy, health, audit, and cost attribution.
- [ ] Add tests for credential isolation, OAuth expiry, approval race, remote timeout, network denial, result-size limit, and disabled-tool fail-closed behavior.

### `sandbox`

**Current sources:** `src/server/internal/workflow/sandbox/` and Agent Docker runner wiring.

- [ ] Make Sandbox the sole custom-code execution authority for Agent and Workflow.
- [ ] Implement non-root isolation, default-deny networking, resource quotas, cancellation, worker lease, logs, artifacts, malware controls, retention, and capacity reporting.
- [ ] Add tests for process escape attempt, network denial, timeout, cancellation, worker crash, artifact checksum, and retention deletion.

**Authority-chain verification:**

```bash
TEST_DATABASE_URL="$INTEGRATION_DATABASE_URL" \
  bash scripts/test-service-batch.sh authority --race
(cd src/server && go test -count=3 ./test/integration/golden \
  -run 'TestJ03RealtimeBatchFile|TestJ04RAGIngestionRetrieval')
```

Expected: Realtime/Batch/File settle once; RAG produces durable cited retrieval; every billable request joins Relay usage and Billing ledger.

## Task 3: Deliver Product Runtime Services

### `chat`

- [ ] Implement multi-turn messages, edit/retry/branch, attachments, multimodal input, Knowledge/Tool binding, citations, sharing, real provider streaming, cancellation, failure atomicity, and usage display.
- [ ] Add browser tests for Browser to Gateway to Chat to Relay to Billing with matching request, trace, usage, reservation, and ledger IDs.

### `agent`

- [ ] Implement versioned definitions, durable run/plan/tool/memory state, approvals, budgets, structured streaming, checkpoint resume, Sandbox tools, artifacts, cancellation, and human takeover.
- [ ] Add tests for approval timeout, duplicate tool result, resume after worker restart, budget exhaustion, cancellation, artifact access, and takeover audit.

### `workflow`

- [ ] Implement versioned typed graphs, triggers, durable execution, retry/failure branches, pause/resume/cancel, snapshots, debugger, compensation, worker leases, replay, and retention.
- [ ] Add tests for lease expiry, duplicate dispatch, database restart, compensation order, pause/resume identity, snapshot restore, and retention pruning.

### `channel`

- [ ] Certify Feishu, DingTalk, Slack, and Telegram for V1 using OAuth/credential references, signed webhook validation, inbound/outbound message lifecycle, receipt, retry, DLQ, rate limits, archive, and channel quota.
- [ ] Add tests for replayed webhook, signature failure, duplicate receipt, provider rate limit, dead-letter redrive, and tenant credential isolation.

**Product runtime verification:**

```bash
pnpm --dir src/web exec playwright test \
  src/web/e2e/target/j02-chat-relay-billing.spec.ts \
  src/web/e2e/target/j05-agent-sandbox-resume.spec.ts \
  src/web/e2e/target/j06-workflow-recovery.spec.ts
```

Expected: all journeys pass three consecutive runs without fixture routes or retry masking.

## Task 4: Deliver Scheduler, Marketplace, And Admin

### `task-scheduler`

**Current sources:** `src/server/cmd/task/`, `src/server/internal/task/`, `src/server/internal/schedule/`.

- [ ] Replace simulated step execution with generated Agent and Workflow clients.
- [ ] Implement Cron/timezone, misfire policy, idempotent claim, retry, cancellation, distributed scheduling, backlog SLO, and history.
- [ ] Add tests for DST transition, missed schedule, duplicate claim, worker loss, retry budget, cancellation, and target completion propagation.

### `marketplace`

- [ ] Implement listing/version, publish/review/appeal, install, entitlement, order intent, refund revocation, settlement, payout obligation, reserve, reconciliation, and abuse governance.
- [ ] Add tests for review reassignment, appeal deadline, duplicate order, refund after payout, seller negative balance, chargeback, payout retry, and entitlement removal.

### `admin-console`

- [ ] Separate customer-console and platform-admin permissions while retaining one admin-console service boundary.
- [ ] Implement tenant/provider/payment/channel operations, approval queues, policy overrides, evidence access, break-glass, dual authorization, reconciliation operations, and audit.
- [ ] Add tests for self-approval rejection, break-glass expiry, cross-tenant admin denial, policy rollback, evidence access scope, and immutable override audit.

**Post-runtime verification:**

```bash
pnpm --dir src/web exec playwright test \
  src/web/e2e/target/j07-task-scheduler.spec.ts \
  src/web/e2e/target/j08-marketplace-commerce.spec.ts \
  src/web/e2e/target/j01-admin-configuration.spec.ts
```

Expected: Scheduler dispatches real targets, Marketplace closes purchase through payout/reconciliation, and Admin actions require correct approval/audit.

## Task 5: Implement Ten Real Golden Journeys

**Files:**

```text
src/web/e2e/target/j01-admin-configuration.spec.ts
src/web/e2e/target/j02-chat-relay-billing.spec.ts
src/server/test/integration/golden/j03_realtime_batch_file_test.go
src/web/e2e/target/j04-rag-ingestion-retrieval.spec.ts
src/web/e2e/target/j05-agent-sandbox-resume.spec.ts
src/web/e2e/target/j06-workflow-recovery.spec.ts
src/web/e2e/target/j07-task-scheduler.spec.ts
src/web/e2e/target/j08-marketplace-commerce.spec.ts
test/recovery/j09_upgrade_restore.sh
test/install/j10_airgap_install.sh
src/web/playwright.target.config.ts
```

- [ ] **Step 1: Create target Playwright configuration**

The config requires a real target base URL, forbids fixture routes, captures trace/video on failure, and uses generated credentials from the external evidence workspace.

- [ ] **Step 2: Implement J01 through J08**

Every test records request ID, trace ID, service run IDs, usage ID, reservation ID, ledger transaction ID, audit ID, and target evidence reference when applicable.

- [ ] **Step 3: Implement J09 and J10**

J09 upgrades and restores then reruns J02, J04, and J08. J10 installs in a network-denied environment, calls a local model endpoint, performs backup/restore, upgrades, and uninstalls.

- [ ] **Step 4: Remove commercial fixture routes from target execution**

Existing `src/web/e2e/fixtures/commercialJourney.ts` remains only for frontend unit-level coverage and is not loaded by `playwright.target.config.ts`.

- [ ] **Step 5: Run each journey three times**

```bash
pnpm --dir src/web exec playwright test \
  -c playwright.target.config.ts --repeat-each=3
(cd src/server && go test -count=3 ./test/integration/golden/...)
```

Expected: zero flakes, zero fixture routes, zero `.only`, and complete correlation IDs.

## Task 6: Add Race, Load, Soak, Chaos, Security, And Recovery Gates

**Files:**

```text
test/load/commercial.js
test/load/soak-8h.js
test/chaos/run.sh
test/security/tenant-isolation.sh
test/security/ssrf.sh
test/security/dast.sh
scripts/verify-no-skipped-tests.sh
```

- [ ] **Step 1: Run race tests**

Run: `(cd src/server && go test -race -count=1 ./...)`

Expected: zero data race across duplicate webhook, duplicate event, scheduler claim, worker lease, WebSocket reconnect, usage settlement, marketplace claim, and RAG indexing tests.

- [ ] **Step 2: Run commercial load**

Run: `k6 run test/load/commercial.js`

Expected: error rate below `0.1%`, CRUD p95 below `300ms`, Gateway/Relay platform overhead p95 below `100ms`, and no ledger mismatch.

- [ ] **Step 3: Run eight-hour soak**

Run: `k6 run test/load/soak-8h.js`

Expected: RSS growth below `10%`; no sustained goroutine, connection, queue, lease, or file-descriptor growth.

- [ ] **Step 4: Run chaos**

Run: `bash test/chaos/run.sh`

Expected: controlled degradation and recovery for service pod, PostgreSQL, Kafka, Redis/Valkey, Qdrant, object storage, provider timeout/429, and network loss; committed data RPO is zero and service recovery is under five minutes where the approved service SLO is stricter.

- [ ] **Step 5: Run security gates**

```bash
bash scripts/check.sh security
bash test/security/tenant-isolation.sh
bash test/security/ssrf.sh
bash test/security/dast.sh
```

Expected: no unwaived Critical or High finding; every waiver has owner, compensating control, and expiry.

- [ ] **Step 6: Run migration and recovery gates**

```bash
bash scripts/verify-migration-replay.sh
bash test/recovery/j09_upgrade_restore.sh
```

Expected: empty to current, previous RC to current, repeated replay, backup restore, rollback/forward-fix, and restored golden journeys pass.

## Task 7: Integrate Daily And Enforce Merge Queue

- [ ] **Step 1: Limit active writers**

At most six write agents run concurrently; keep one contract reviewer and one test/release reviewer available.

- [ ] **Step 2: Reject shared-path drift**

Any service diff touching a leased shared path is rejected and recreated as a contract-change request.

- [ ] **Step 3: Run daily integration**

```bash
bash scripts/check.sh docs
bash scripts/check.sh contracts
bash scripts/check.sh server
bash scripts/test.sh server
bash scripts/check.sh web
bash scripts/test.sh web
bash scripts/test.sh e2e
```

Expected: integration branch remains green; a failure blocks new merges until fixed or reverted.

- [ ] **Step 4: Require independent review**

Implementer agents cannot approve their own work. Review includes contract compatibility, tenant isolation, recovery behavior, tests, telemetry, and write-set compliance.

## Task 8: Execute The Final RC Gate

- [ ] **Step 1: Freeze the RC commit and digest set**

No feature changes after F2 freeze. Blocker fixes require Release Commander approval and a full evidence rerun.

- [ ] **Step 2: Verify no skipped coverage**

Run: `bash scripts/verify-no-skipped-tests.sh`

Expected: no `.only`, required skip, dynamic ignore, or expired quarantine.

- [ ] **Step 3: Run the final commercial verifier**

```bash
COMMERCIAL_COMPLETION_RUN_DEPLOY=true \
COMMERCIAL_COMPLETION_RUN_K8S=true \
COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true \
COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=false \
OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=false \
TEST_DATABASE_URL="$RC_TEST_DATABASE_URL" \
OBLIVIOUS_TARGET_EVIDENCE_FILE="$EXTERNAL_EVIDENCE/target-release-evidence.json" \
OBLIVIOUS_TARGET_ARTIFACT_DIR="$EXTERNAL_EVIDENCE/artifacts" \
bash scripts/verify-commercial-completion.sh
```

Expected: exit `0`; commit, image, Helm, SBOM, provenance, schema, migrations, and evidence manifest match exactly.

- [ ] **Step 4: Obtain human go/no-go approvals**

Product, Engineering, Security, Operations, Finance, Tax/Legal, and Release owners approve the same evidence manifest. Release approval cannot be delegated to an AI agent.
