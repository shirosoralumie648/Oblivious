# Product Roadmap v2 From Reference

Date: 2026-06-29

Scope: product roadmap reorder based on Reference Deep Rescan v3. This is not an implementation plan and does not claim any Oblivious module is Proven. Reference capability is not proof of Oblivious implementation.

## Roadmap Principle

Do not promote every reference capability into P0. P0 is the smallest Private Beta loop that proves Oblivious can run real, metered, observable AI product flows without demo, fake, local-only, or scaffold paths.

The core product loop order is:

1. Chat + Relay + usage/billing.
2. RAG upload -> parse -> chunk -> embed -> retrieve -> citation.
3. Agent run -> tool call -> approval -> trace -> budget.
4. Workflow trigger -> execution -> logs -> retry.
5. Billing checkout -> webhook -> quota -> ledger -> refund/reconcile.
6. Marketplace publish -> review -> install -> paid install -> settlement/payout.
7. Observability request log -> cost -> latency -> error -> alert/SLO.
8. Admin control plane.

## P0: Private Beta Must

P0 should prove a narrow but real product spine. The strongest references are `new-api`, `sub2api`, `litellm`, `llmgateway`, `gateway`, `one-api`, `bifrost`, `ragflow`, `MaxKB`, `coze-studio`, `LibreChat`, `open-webui`, `CPA-Manager`, and selected `helicone` request-log concepts.

### 1. Chat + Relay + Usage/Billing

Requirements:

- Explicit route map for supported chat/completions/responses-style flows and explicit unsupported errors for everything else.
- Fail-closed chat runtime with no production demo reply fallback.
- Provider/model registry with credentials, model aliases, allow/deny lists, status, priority, and route decision audit.
- True upstream streaming, cancellation, client abort handling, provider error normalization, retry/failover, and usage capture.
- Request log and usage ledger for every billable call, including provider, requested model, resolved model, latency, tokens, cost, status, error, route decision, tenant/user/key, and trace id.
- Versioned price catalog and immutable price snapshot per usage record.

Reference evidence:

- `reference/new-api/router/relay-router.go:69`
- `reference/new-api/service/billing_session.go:20`
- `reference/gateway/src/handlers/handlerUtils.ts:288`
- `reference/gateway/src/handlers/streamHandler.ts:392`
- `reference/llmgateway/packages/db/src/schema.ts:471`
- `reference/litellm/schema.prisma:12`
- `reference/bifrost/core/bifrost.go:6474`

### 2. RAG Upload -> Retrieve -> Citation

Requirements:

- Upload creates durable document/job state, not request-path ingestion.
- Parser/chunker/embedder workers with retry, cancel, dead-letter, and status history.
- Vector upsert/delete/reindex lifecycle tied to SQL state through outbox or equivalent durable mechanism.
- Retrieval with tenant filters, top-k/threshold/debug fields, and selected chunk evidence.
- Citation payload stored and returned with source title, document version, page/offset, URL or file id, chunk id, and highlight anchor where available.

Reference evidence:

- `reference/ragflow/api/apps/restful_apis/document_api.py:59`
- `reference/ragflow/rag/svr/task_executor.py:270`
- `reference/ragflow/rag/nlp/search.py:562`
- `reference/ragflow/rag/nlp/search.py:242`
- `reference/MaxKB/apps/knowledge/task/embedding.py:61`
- `reference/MaxKB/apps/knowledge/serializers/document.py:1432`

### 3. Agent Run -> Tool Call -> Approval -> Trace -> Budget

Requirements:

- Agent run must fail closed when tool capability is unavailable.
- Tool calls must be extracted, recorded, approved or auto-approved by policy, executed with timeout/resource limits, and linked to budget.
- Tool execution must produce trace entries, input/output redaction metadata, and retry/failure states.
- Custom code execution must not run on the host without sandbox policy.

Reference evidence:

- `reference/LibreChat/api/server/controllers/agents/openai.js:421`
- `reference/open-webui/backend/open_webui/utils/tools.py:430`
- `reference/bifrost/core/mcp/agent.go:35`
- `reference/coze-studio/backend/domain/plugin/service/exec_tool.go:45`
- `reference/coze-studio/backend/domain/plugin/service/tool/invocation_http.go:59`

### 4. Workflow Trigger -> Execution -> Logs -> Retry

Requirements:

- Minimal versioned node registry and validated workflow graph.
- Durable run row, transition events, node execution logs, retry/cancel/fail state, and error diagnostics.
- No requirement for full Flowise/Dify-style visual builder in Private Beta.

Reference evidence:

- `reference/coze-studio/backend/domain/workflow/internal/compose/workflow_run.go:46`
- `reference/dify/api/core/workflow/workflow_entry.py:185`
- `reference/FastGPT/packages/service/core/workflow/dispatch/index.ts:159`
- `reference/Flowise/packages/server/src/utils/buildChatflow.ts:299`

### 5. Billing Checkout -> Quota -> Ledger

Requirements:

- Checkout/webhook/quota changes must be idempotent.
- Usage ledger must be append-only or auditable.
- Refund/reconcile must exist for core paid plan and usage credits.
- Non-core payment providers can be hidden until configured.

Reference evidence:

- `reference/new-api/service/billing_session.go:20`
- `reference/one-api/relay/billing/billing.go:23`
- `reference/CPA-Manager/usage-service/internal/store/store.go:119`
- `reference/sub2api/backend/ent/schema/user.go:36`

### 6. Observability Request Log

Requirements:

- Production request-log sink cannot silently noop for enabled billable flows.
- Log records must join to usage ledger, traces, provider route decision, and admin dashboards.
- P0 includes request log/cost/latency/error/token visibility. Alert/SLO policy can be P1.

Reference evidence:

- `reference/helicone/worker/src/lib/dbLogger/DBLoggable.ts:590`
- `reference/helicone/clickhouse/migrations/schema_41_request_response_replacing_merge_tree.sql:1`
- `reference/bifrost/plugins/telemetry/main.go:230`
- `reference/bifrost/plugins/otel/main.go:570`

### 7. Admin Control Plane

Requirements:

- Backend-enforced RBAC for users, API keys, providers, pricing, quota, usage logs, request logs, and risk settings.
- Frontend route guards are not enough.
- Every admin mutation must be auditable.

Reference evidence:

- `reference/sub2api/backend/internal/server/middleware/admin_auth.go:14`
- `reference/new-api/router/api-router.go:227`
- `reference/litellm/litellm/proxy/auth/route_checks.py:67`
- `reference/LibreChat/api/server/routes/admin/roles.js:34`

## P1: Commercial Beta Must

P1 expands product completeness without destabilizing the Private Beta spine.

- Provider/account pool: OAuth refresh/revoke, account health/cooldown, sticky sessions, and account-level quota. References: `CLIProxyAPI`, `CliRelay`, `openai-oauth`, `sub2api`.
- Workflow debugger: persisted per-node snapshots, replay/retention APIs, richer node registry, and canvas validation. References: `dify`, `FastGPT`, `Flowise`, `coze-studio`.
- Agent/MCP lifecycle: install/test/uninstall MCP servers, user-scoped auth, tool timeout/approval policies, tool-call history. References: `LibreChat`, `open-webui`, `bifrost`, `lobehub`.
- RAG quality: retrieval debug UI, rerank tuning, document preview, page/bbox/highlight anchors, reparse controls. References: `ragflow`, `MaxKB`, `coze-studio`.
- Observability: SLO/error-rate/cost/latency alert policies, Slack/email/webhook sinks, alert history and dedupe. References: `helicone`, `bifrost`.
- Admin dashboards: ops dashboard, QPS/usage views, provider health, model pricing sync/import/export. References: `sub2api`, `llmgateway`, `CPA-Manager`, `litellm`.
- Marketplace basic lifecycle: publish/review/install for free or internal assets, without paid settlement. References: `coze-studio`, `dify`, `lobehub`.

## P2: GA Before Public Launch

P2 is required for a full commercial platform, but should not block Private Beta.

- Paid marketplace install, settlement hold, external payout, refund/chargeback, reconciliation, and audit.
- Full visual workflow builder parity: rich canvas, large node catalog, node migration, debugger replay UX.
- Broader OpenAI-compatible route lifecycle: batch, files, fine-tuning, Assistants, Threads/Runs, if offered.
- ClickHouse-scale observability warehouse or equivalent for large tenants, raw body object storage with redaction policy.
- Hardened deployment profiles, Kubernetes/Helm options, HA stores, secret rotation, production-deny plaintext policy.
- Advanced RAG parsing: OCR/layout/table/figure handling, citation previews, document diffs, and parser selection.
- Guardrail registry and policy packs if sold as an admin/governance feature.

Reference anchors:

- `reference/helicone/clickhouse/migrations/schema_77_stats_page_mv.sql:5`
- `reference/ai-gateway/internal/extensionserver/post_translate_modify.go:55`
- `reference/Flowise/packages/ui/src/views/canvas/index.jsx:586`
- `reference/coze-studio/idl/marketplace/product_common.thrift:72`
- `reference/litellm/litellm/proxy/guardrails/init_guardrails.py:19`

## Later

These are reference-rich but should not enter the current Oblivious core roadmap unless the business target changes.

- Kubernetes/Envoy-native AI gateway CRDs and ext_proc productization from `ai-gateway`.
- Desktop/PWA/native bridge and local IDE editing workflows from `NextChat`, `lobehub`, `copilot-api`, and `claude-code-api`.
- Local-only OAuth helpers from `codex-oauth`, `openai-oauth`, `copilot-api`, and `claude-code-api` as production features.
- Full multi-agent/team runtime and remote agent gateway.
- Exhaustive provider catalog parity with LiteLLM.
- Full MCP marketplace, community ratings, trust/review systems, and paid extension economy.
- Demo/mocker/test-only plugin behavior from projects like `bifrost` mocker or dashboard mock data.

## Roadmap Changes From Old P0

| Old P0 area | New placement | Action |
| --- | --- | --- |
| Real gateway proxy and chat streaming | P0 | Keep, but split into route contract, streaming, provider registry, request log, usage ledger, and fail-closed chat. |
| Relay price snapshots and provider-source audit | P0 | SQL catalog and immutable usage price snapshots are in place; complete provider import/sync, approval audit, and reconciliation. |
| Durable RAG ingestion and vector delete | P0 | Keep, split into upload enqueue/status plus raw parser replay, retrieval/citation evidence, and target Postgres/Qdrant recovery proof. Upload now enqueues durable raw-payload ingestion jobs, vector repair retry/dead-letter and transactional vector intent enqueue cover document create/update, chunk edit/split/merge, and delete cleanup in repository-local code; target Qdrant/Postgres proof and retrieval debug remain required. |
| Agent sandbox | P0 narrow | Keep safe tool execution/approval/trace/budget; defer full code-mode/MCP ecosystem. |
| Workflow durable state/debugger | P0/P1 split | Durable run/event rows and SQL node-execution debug snapshot storage are in place; replay/retention and standalone tracer/state-machine consolidation stay P0/P1, rich debugger/canvas is P1/P2. |
| Observability request-log/SLO | P0/P1 split | Request log and cost/latency/error are P0; alerts/SLO are P1. |
| Realtime API | P2/later | Keep disabled until there is auth/origin/prebill/abort settlement/usage capture. |
| Batch API | P2 | Not needed for Private Beta. |
| Relay files API | P1/P2 split | Attachment/RAG file lifecycle is earlier; OpenAI Files API parity is later. |
| Fine-tuning/Assistants/Threads/Runs | later | Explicit unsupported state is acceptable. |
| Marketplace external payout | P2/GA | Required only when paid marketplace is launched. |
| Kubernetes/Envoy gateway | later | Useful enterprise deployment pattern, not core loop. |
| Desktop/PWA/IDE/local bridge | later | Useful for developer workflow product, not SaaS core. |

## Single Next Implementation Ticket

`P0-Execution-Evidence-Spine`

Goal: one end-to-end Chat + Relay request path that is fail-closed, provider-routed, truly streamed, request-logged, usage-metered, price-snapshotted, quota-settled, admin-visible, and trace-linked.

Why this ticket first: every later core loop depends on the same evidence spine. RAG, agent, workflow, billing, observability, and admin all need shared request/run ids, usage ledger semantics, pricing authority, trace linkage, and fail-closed runtime behavior. Starting elsewhere risks building more surfaces that still cannot prove runtime product depth.
