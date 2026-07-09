# Parallel Agent Execution Plan

Date: 2026-07-01

Scope: build the Oblivious product skeleton first, then run parallel implementation agents against bounded vertical slices. This plan uses `reference/` as capability evidence and implementation inspiration, but Oblivious runtime code, contracts, migrations, and verification remain the source of truth.

This is an execution plan, not a completion claim.

## Objective

Create a stable development skeleton that lets multiple agents fill product depth concurrently without drifting API shapes, database state, billing semantics, tenant boundaries, or verification rules.

The first execution spine is:

1. Chat request enters the app.
2. Relay selects a provider/model and streams a real upstream response.
3. Usage is captured with request id, route decision, tokens, latency, error state, and tenant/user context.
4. Billing records an immutable price snapshot and quota settlement.
5. Observability stores the request log.
6. Admin can inspect the evidence.

RAG, Agent, Workflow, Marketplace, and later OpenAI-compatible surfaces should attach to this same evidence spine instead of creating separate accounting or trace models.

## Non-Negotiable Rules

1. No production demo, fake, local-only, noop, or mock path can count as product evidence.
2. Unsupported commercial surfaces must fail explicitly rather than appearing successful.
3. Every billable or externally integrated flow must carry tenant, user, organization, request/run id, audit, and failure evidence.
4. API and database contracts must be stabilized before parallel feature agents implement against them.
5. Agents must work in bounded vertical slices: API, service, persistence, UI state if relevant, tests, docs, and verification evidence together.
6. Shared skeleton files are owned by the contract agent. Feature agents can propose changes to those files, but should not independently mutate them while other agents are active.
7. Reference projects provide design evidence only. A reference capability is not proof that Oblivious implements it.

## Current Preconditions

Before starting parallel implementation, resolve or isolate the current working tree:

- Existing modified files: `docs/api/openapi.yaml`, `docs/reports/2026-06-07-fusion-spec-completion-matrix.md`, `scripts/verify-openapi-contract.sh`, and admin HTTP settlement tests/handler files.
- Existing untracked audit docs: `docs/audit/`.
- Recommended action: commit or stash the current Admin Marketplace settlement response contract patch before spawning implementation agents.

Agents should run from separate worktrees or isolated branches. Do not let multiple agents edit the same checkout at the same time.

Suggested layout:

```text
.worktrees/agent-contract
.worktrees/agent-relay-chat-billing
.worktrees/agent-rag-ingestion
.worktrees/agent-agent-runtime
.worktrees/agent-workflow-durability
.worktrees/agent-observability
.worktrees/agent-admin-rbac
```

## Skeleton First

### S0. Contract Owner

Purpose: lock the shared contracts that all implementation agents depend on.

Owns:

- Supported and unsupported route matrix.
- OpenAPI response envelope and required fields.
- Request/run/trace id propagation rules.
- Usage ledger, price snapshot, quota settlement, and request-log field names.
- Migration naming and sequencing.
- Tenant/RBAC invariants.
- Quality-gate command list.

Reference inputs:

- `docs/architecture/current-system-contracts.md`
- `docs/audit/product-roadmap-v2-from-reference.md`
- `docs/audit/current-implementation-depth.md`
- `docs/audit/vertical-slice-gap-report.md`
- `docs/audit/reference-capability-evidence-v3.json`

Deliverables:

- A small contract update in `docs/architecture/current-system-contracts.md`.
- Any required OpenAPI schema updates in `docs/api/openapi.yaml`.
- Contract verifier updates in `scripts/verify-openapi-contract.sh`.
- A short evidence template under `docs/audit/agent-evidence-template.md`.

Acceptance:

- `bash scripts/check.sh docs`
- `bash scripts/verify-openapi-contract.sh`
- `git diff --check`

### S1. Evidence Spine Contract

Purpose: define one shared event and accounting model before features attach to it.

Minimum fields:

- `request_id`
- `trace_id`
- `organization_id`
- `workspace_id`
- `user_id`
- `api_key_id` when applicable
- `source_surface`
- `provider`
- `requested_model`
- `resolved_model`
- `route_id`
- `route_decision`
- `status`
- `error_code`
- `latency_ms`
- `input_tokens`
- `output_tokens`
- `total_tokens`
- `price_catalog_version`
- `price_snapshot`
- `cost_amount`
- `quota_delta`
- `started_at`
- `completed_at`

Agents may add fields only through the contract owner.

## Wave 1 Agents

Run these after S0/S1 lands.

### Agent A: Chat + Relay + Usage/Billing

Goal: implement the first real end-to-end metered AI request path.

Reference focus:

- `reference/new-api/service/channel_select.go`
- `reference/new-api/service/billing_session.go`
- `reference/new-api/relay/helper/stream_scanner.go`
- `reference/gateway/src/handlers/streamHandler.ts`
- `reference/litellm/litellm/proxy/proxy_server.py`
- `reference/bifrost/core/bifrost.go`

Scope:

- Fail-closed chat runtime for production profiles.
- Provider/model route decision.
- True upstream streaming and client-abort handling.
- Usage capture and immutable price snapshot.
- Quota preconsume, settle, and refund on failure.
- Admin-visible request/usage evidence.

Must not own:

- Full realtime, batch, files, fine-tuning, Assistants, Threads/Runs parity.
- Marketplace payout.
- RAG ingestion.

Deliverables:

- Runtime code under `src/server/internal/relay`, `src/server/internal/chat`, billing/quota modules as needed.
- Focused tests for successful stream, provider failure, client abort, quota failure, and unsupported route.
- Docs update for supported and unsupported Relay surfaces.

Acceptance:

- Targeted Go tests for relay/chat/billing packages.
- `bash scripts/check.sh docs`
- `bash scripts/test.sh server`
- Evidence note showing a request id joined across Relay, usage, billing, and request log.

### Agent B: Durable RAG Ingestion

Goal: move Knowledge upload from request-path processing to durable ingestion and vector lifecycle.

Reference focus:

- `reference/ragflow/api/apps/restful_apis/document_api.py`
- `reference/ragflow/rag/svr/task_executor.py`
- `reference/ragflow/rag/nlp/search.py`
- `reference/MaxKB/apps/knowledge/task/embedding.py`
- `reference/MaxKB/apps/knowledge/serializers/document.py`
- `reference/dify/api/services/file_service.py`

Scope:

- Document/job state model.
- Parser/chunker/embedder worker contract.
- Retry, cancel, dead-letter, and status history.
- Durable vector upsert/delete/reindex.
- Citation payload completeness.

Must not own:

- Full advanced OCR/layout/table parsing.
- Marketplace document sale or plugin marketplace.

Deliverables:

- Migration for ingestion job/outbox state.
- Worker/service implementation.
- API response changes for document status.
- Tests for restart-safe job recovery, embedding retry, vector delete, and tenant-scoped retrieval.

Acceptance:

- Targeted Go tests for Knowledge service and HTTP routes.
- Retrieval cannot return deleted vector chunks.
- Evidence note with document id, job id, vector operation id, and retrieved citation fields.

### Agent C: Agent Runtime, Tool Execution, And Sandbox Boundary

Goal: make tool-capable agents fail closed, record tool decisions, and execute tools under a safe policy boundary.

Reference focus:

- `reference/LibreChat/api/server/controllers/agents/openai.js`
- `reference/open-webui/backend/open_webui/utils/tools.py`
- `reference/coze-studio/backend/domain/plugin/service/exec_tool.go`
- `reference/coze-studio/backend/domain/plugin/service/tool/invocation_http.go`
- `reference/bifrost/core/mcp/agent.go`

Scope:

- Structured tool capability enforcement.
- Tool call record, approval/reject/retry, timeout, and budget linkage.
- Trace entries with redaction metadata.
- Sandbox policy boundary for custom code tools.

Must not own:

- Full MCP marketplace.
- Remote desktop or IDE bridge.
- Multi-agent team runtime beyond the minimal call-agent contract approved by the contract owner.

Deliverables:

- Agent runner and tool execution changes.
- Tests for unavailable structured tool support, approval path, rejection path, timeout, retry, and budget exhaustion.
- Sandbox policy documentation and production guard.

Acceptance:

- Targeted Go tests for agent runner/tool packages.
- A tool-enabled agent cannot silently degrade to plain chat in production mode.
- Evidence note linking run id, tool run id, trace id, and budget delta.

### Agent D: Workflow Durability

Goal: make workflow execution, transition history, node logs, retries, and failure evidence durable.

Reference focus:

- `reference/coze-studio/backend/domain/workflow/internal/compose/workflow_run.go`
- `reference/dify/api/core/workflow/workflow_entry.py`
- `reference/dify/api/services/workflow_run_service.py`
- `reference/FastGPT/packages/service/core/workflow/dispatch/index.ts`
- `reference/Flowise/packages/server/src/utils/buildChatflow.ts`

Scope:

- Versioned node registry minimum.
- Durable execution row and transition event log.
- Durable node execution logs.
- Retry, cancel, fail, and diagnostic states.
- Replay/read APIs for previous execution evidence.

Must not own:

- Full visual builder parity.
- Rich debugger UI beyond durable evidence retrieval.

Deliverables:

- Migration for transition/debug evidence if missing.
- Workflow executor persistence changes.
- Tests for restart-like reload, retry, cancel, and failure diagnostics.

Acceptance:

- Targeted Go tests for workflow executor/store/routes.
- Debug traces and transition history survive service recreation in tests.
- Evidence note with workflow id, execution id, transition event ids, and node log ids.

### Agent E: Observability Request Log

Goal: make request logging non-noop for enabled production billable flows and join it to usage/billing evidence.

Reference focus:

- `reference/helicone/worker/src/lib/dbLogger/DBLoggable.ts`
- `reference/helicone/clickhouse/migrations/schema_41_request_response_replacing_merge_tree.sql`
- `reference/bifrost/plugins/telemetry/main.go`
- `reference/bifrost/plugins/otel/main.go`
- `reference/CPA-Manager/usage-service/internal/store/store.go`

Scope:

- Production guard against silent request-log noop.
- Request log persistence or configured sink health.
- Join keys to usage ledger, route decision, trace id, and admin UI/API.
- Minimal cost/latency/error/token visibility.

Must not own:

- Full ClickHouse warehouse productization.
- Full SLO/alert delivery loop unless needed for request-log health.

Deliverables:

- Request-log sink enforcement and tests.
- Admin inspection path if missing.
- Documentation for production sink requirements.

Acceptance:

- Targeted Go tests for request-log configuration and admin inspection.
- Production profile fails clearly when billable flows are enabled without a request-log sink.
- Evidence note joining request log id to usage id and trace id.

### Agent F: Admin, RBAC, And OpenAPI Parity

Goal: keep the control plane aligned with runtime contracts while other agents add depth.

Reference focus:

- `reference/sub2api/backend/internal/server/middleware/admin_auth.go`
- `reference/new-api/router/api-router.go`
- `reference/litellm/litellm/proxy/auth/route_checks.py`
- `reference/LibreChat/api/server/routes/admin/roles.js`
- `reference/Cli-Proxy-API-Management-Center/src/router/MainRoutes.tsx`

Scope:

- Backend-enforced RBAC for admin-sensitive routes.
- Audit log requirements for admin mutations.
- OpenAPI parity for new or changed response shapes.
- Admin list/detail pages for evidence introduced by other agents.

Must not own:

- Business logic of Relay, RAG, Agent, Workflow, or Observability internals.

Deliverables:

- Route guard tests.
- Audit mutation tests.
- OpenAPI schema and verifier updates.
- Minimal admin UI/API updates only where they expose new evidence.

Acceptance:

- Targeted Go tests for admin route guards and audit entries.
- `bash scripts/verify-openapi-contract.sh`
- `bash scripts/check.sh docs`
- No admin endpoint returns planned provider/functionality as runtime-ready unless backed by implementation evidence.

## Wave 2 Agents

Start only after the Wave 1 evidence spine is merged and stable.

### Billing Reconciliation Agent

- Idempotent checkout/webhook/quota/refund reconciliation.
- Provider-specific live rail evidence for configured providers.
- Hide or fail unsupported providers explicitly.

### Marketplace Lifecycle Agent

- Publish/review/install/free/internal lifecycle first.
- Paid install, settlement hold, payout, refund, chargeback, and reconciliation only after billing reconciliation is solid.

### Provider Account Pool Agent

- OAuth/account health/cooldown/sticky session model.
- Scope carefully so it does not bypass Relay usage and billing.

### RAG Quality Agent

- Retrieval debug UI.
- Rerank tuning.
- Page/offset/highlight anchors.
- Reparse controls.

## Reference Usage Protocol

Each implementation agent must produce a short reference note before coding:

```text
Agent:
Oblivious target files:
Reference projects inspected:
Reference files and lines:
Useful patterns:
Patterns rejected and why:
Contract changes requested:
```

Do not copy architecture blindly. Translate reference patterns into Oblivious constraints:

- Go `net/http` backend under `src/server`.
- React frontend under `src/web`.
- PostgreSQL as core state.
- Relay-only provider access for AI calls.
- Multi-tenant organization/workspace boundaries.
- Existing quality gates under `scripts/`.

## File Ownership Rules

Shared owner files:

- `docs/architecture/current-system-contracts.md`
- `docs/API.md`
- `docs/api/openapi.yaml`
- `scripts/verify-openapi-contract.sh`
- `scripts/check.sh`
- `scripts/test.sh`
- `src/server/internal/http/router.go`
- migration sequence files
- global config/env contract files

Feature agents should avoid direct edits to shared owner files until the contract owner assigns a specific change.

Preferred feature ownership:

- Agent A: `src/server/internal/relay/**`, `src/server/internal/chat/**`, billing/quota integration points.
- Agent B: `src/server/internal/knowledge/**`, Knowledge HTTP routes, ingestion migrations.
- Agent C: `src/server/internal/agent/**`, MCP/tool execution integration points.
- Agent D: `src/server/internal/workflow/**`, schedule/workflow route integration points.
- Agent E: `src/server/internal/observability/**`, metrics/request-log integration points.
- Agent F: `src/server/internal/admin/**`, admin HTTP handlers/tests, OpenAPI parity.

## Merge Protocol

1. Agent opens with a reference note and contract-delta request.
2. Contract owner accepts or rejects any shared contract change.
3. Agent implements within assigned ownership.
4. Agent runs targeted tests plus docs verifier if contracts changed.
5. Agent writes an evidence note under `docs/audit/agent-evidence/`.
6. Integrator reviews for cross-agent conflicts.
7. Merge only when `git diff --check` and relevant verification pass.

## Evidence Note Template

Each merged feature should leave evidence that can be audited later:

```text
# Agent Evidence: <feature>

Date:
Agent:
Commit:

## Runtime Claim

## Reference Inputs

## Oblivious Files Changed

## Verification Commands

## Runtime Evidence IDs

## Unsupported / Deferred Surfaces

## Known Residual Risk
```

## First Recommended Execution

1. Finish or isolate the current Admin Marketplace settlement response contract patch.
2. Run S0 Contract Owner.
3. Run S1 Evidence Spine Contract.
4. Start Agent A and Agent F first. Agent A creates the execution spine; Agent F keeps Admin/OpenAPI/RBAC aligned.
5. Start Agents B, C, D, and E once Agent A's request/trace/usage id model is stable.

This order keeps the skeleton real while still allowing meaningful parallel implementation.
