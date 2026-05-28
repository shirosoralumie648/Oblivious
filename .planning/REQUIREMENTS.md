# Requirements: Oblivious v08 Product Completeness

**Defined:** 2026-05-28
**Current milestone:** v08 Product Completeness — complete
**Source spec:** `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`
**Core Value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

## Current Milestone Requirements

### Product Completeness

- [x] **PROD-01**: Built-in MCP tools such as `web_search`, `calculator`, `datetime`, and `http_request` either use real providers/parsers with tenant-safe configuration and tests, or are disabled from default commercial use with product copy that reflects the disabled state. Completed in Phase 25 with default commercial builtin policy, real calculator parser, real datetime output, default-disabled `web_search`/`http_request`, Agent executor/listing enforcement, focused tests, API docs, and quality gates.
- [x] **PROD-02**: Agent workflows are durable and observable, with persisted tool runs, human approval points where needed, memory injection, execution state, and failure/retry evidence rather than placeholder tool output. Completed in Phase 26 with `agent_runs`, `agent_tool_runs`, approval/reject/retry APIs, memory evidence, failure evidence, tenant-scoped status APIs, DB-backed Agent/HTTP tests, API docs, and quality gates.
- [x] **PROD-03**: Knowledge behavior matches customer-facing product copy. If marketed as RAG, ingestion, embedding-backed retrieval, indexing, and source citation must be implemented and verified; otherwise copy must explicitly describe text retrieval. Completed in Phase 27 with Relay embedding-backed Knowledge indexing/retrieval, pgvector chunk search, `embedding_rag` metadata, source citations, UI citation rendering, focused backend/frontend tests, API docs, commercial gate docs, and quality gates.
- [x] **PROD-04**: Chat, Agent, Knowledge, Admin, and Marketplace customer journeys are production-ready with quota enforcement, clear error states, and no enabled placeholder pages or fake commercial behavior. Completed in Phase 28 with focused Chat, SOLO/Agent, Knowledge, Marketplace, and Admin frontend tests, commercial action/empty/error states, review and settlement boundary copy, docs gates, and `28-VERIFICATION.md`.
- [x] **PROD-05**: Public docs, onboarding, pricing, and operator guides align with implemented tenant, Relay, billing, Marketplace, operations, and product behavior. Completed in Phase 29 with README commercial framing, `docs/product/` public overview/onboarding/pricing/operator guide, updated API and architecture contracts, commercial gate docs, quality gates, stale-doc scans, and `29-VERIFICATION.md`.
- [x] **PROD-06**: End-to-end commercial journeys pass: signup, create organization, configure provider/channel, subscribe/top up, chat, create agent, use knowledge, publish agent, install agent, bill usage, inspect admin dashboards, deploy, backup, and restore. Completed in Phase 30 with DB-backed backend journey, browser journey, strict commercial verifier, deploy validation, backup/restore smoke, and no skipped checks.
- [x] **AUDIT-01**: Final commercial completion audit maps every commercial gate to files, tests, runtime evidence, skipped checks, and accepted residual risk before final readiness can be claimed. Completed in Phase 30 with `docs/release/commercial-completion-audit.md`, strict `30-VERIFICATION.md` evidence, and `30-01-SUMMARY.md`.

## Historical v07 Requirements

### Production Orchestration

- [x] **OPS-01**: Kubernetes or equivalent production orchestration validation starts the actual stack, applies migrations, proves `/healthz`, and proves app and Relay paths without live provider secrets. Completed in Phase 21 with migration-aware Docker compose proof using a local pgvector fallback image, configurable host ports, and `/healthz`/`/metrics`/app/Relay smoke.
- [x] **OPS-02**: Runtime smoke covers both normal network and restricted-network deployment paths, including documented proxy/registry overrides and explicit evidence when cluster tooling is unavailable. Completed in Phase 24 with restricted-network/fallback smoke plus bare default `scripts/deploy-validate.sh` runtime smoke after default image tags were locally available and `Dockerfile.server` reused the `/go/pkg/mod` cache during `go build`. Fresh Docker Hub daemon pulls remain environment-specific on this host and are not hidden as proof. Missing `kubectl` evidence is recorded as non-success Kubernetes proof.

### Backup And Recovery

- [x] **OPS-03**: Backup and restore runbooks plus automated smoke prove PostgreSQL tenant data can be backed up and restored into a fresh database with migration ledger integrity. Completed in Phase 22 with `backup-postgres.sh`, `restore-postgres.sh`, `backup-restore-smoke.sh`, `backup-restore-runbook.md`, disposable pgvector PostgreSQL smoke, and 30-row migration ledger checksum verification.

### Observability

- [x] **OPS-04**: Structured logs, Prometheus metrics, OpenTelemetry tracing hooks, and error-tracking integration points cover HTTP, Relay, billing, jobs, and provider failures. Completed in Phase 23 with shared observability primitives, HTTP/Relay/provider structured events, Prometheus metrics, span hooks, and focused package tests.
- [x] **OPS-05**: Alert rules, dashboards, and SLO definitions exist for Relay outage, quota settlement failure, webhook failure, migration failure, high provider error rate, and tenant isolation incidents. Completed in Phase 23 with `deploy/observability/prometheus-alerts.yaml`, `deploy/observability/grafana-dashboard.json`, `docs/release/observability-slos.md`, and docs quality-gate coverage.

### Runbooks And Evidence

- [x] **OPS-06**: Release, rollback, incident response, and disaster recovery runbooks are documented and verified against the deployment, restore, alert, and evidence commands. Completed in Phase 24 with the three runbooks, docs gate coverage, release-path evidence, and the no-final-readiness boundary.
- [x] **DOC-06**: v07 evidence maps production-operations requirements to scripts, manifests, runbooks, smoke output, skipped checks, residual v08 work, and a no-final-readiness boundary. Completed in Phase 24 with `docs/release/v07-operations-evidence.md`, `24-VERIFICATION.md`, `24-01-SUMMARY.md`, and `.planning/milestones/v07-*`.

## Out of Scope For v07

| Feature | Reason |
|---------|--------|
| RAG upgrade and Agent workflow expansion | Product completeness belongs to v08 |
| Final public pricing/onboarding/operator guides | Final product/documentation completion belongs to v08 |
| Customer-facing placeholder removal | Product completeness belongs to v08 |
| External managed observability account provisioning | v07 must define integration points, config, and verifiable local artifacts; live vendor onboarding can be deployment-specific |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| OPS-01 | Phase 21 — Production Orchestration Runtime Proof | Complete |
| OPS-02 | Phase 21 — Production Orchestration Runtime Proof; Phase 24 — Release Rollback Incident DR and v07 Closeout | Complete — restricted/fallback smoke and bare default-command smoke passed; fresh Docker Hub daemon pull remains environment-specific |
| OPS-03 | Phase 22 — Backup Restore and Migration Recovery | Complete |
| OPS-04 | Phase 23 — Observability Alerts Dashboards and SLOs | Complete |
| OPS-05 | Phase 23 — Observability Alerts Dashboards and SLOs | Complete |
| OPS-06 | Phase 24 — Release Rollback Incident DR and v07 Closeout | Complete |
| DOC-06 | Phase 24 — Release Rollback Incident DR and v07 Closeout | Complete |
| PROD-01 | Phase 25 — MCP Tool Commercial Behavior | Complete |
| PROD-02 | Phase 26 — Durable Agent Workflows | Complete |
| PROD-03 | Phase 27 — Knowledge Product Promise Alignment | Complete |
| PROD-04 | Phase 28 — Commercial UX and Journey Hardening | Complete |
| PROD-05 | Phase 29 — Public Docs Onboarding Pricing and Operator Guides | Complete |
| PROD-06 | Phase 30 — End-to-End Commercial Journey and Final Audit | Complete |
| AUDIT-01 | Phase 30 — End-to-End Commercial Journey and Final Audit | Complete |

## Historical Validated Requirements

### v06 Billing And Marketplace Operations

- [x] **PAY-01**: Stripe checkout routes are mounted in the running server, require authenticated tenant context, persist payment intent metadata, and can be tested without live Stripe keys. Completed in Phase 17 with `POST /api/v1/billing/checkout`, fake checkout route tests, `payment_intents`, and Stripe metadata propagation.
- [x] **PAY-02**: Stripe webhook route verifies signatures from the raw request body, records provider events idempotently, rejects invalid signatures, and preserves processing status/errors for admin inspection. Completed in Phase 17 with `POST /api/v1/billing/stripe/webhook`, `stripe_webhook_events`, and signed fixture/idempotency tests.
- [x] **PAY-03**: Subscription lifecycle, invoices, refunds, failed-payment states, plan changes, and top-ups are implemented as auditable state transitions. Completed in Phase 18 with `billing_lifecycle_events`, `billing_invoices`, `billing_refunds`, `LifecycleService`, payment-backed top-up fulfillment, refund quota reversal, duplicate webhook lifecycle retry, and DB-backed lifecycle/route tests.
- [x] **MARKET-03**: Marketplace publisher revenue, platform fee, payout state, and refund impact are modeled before paid Marketplace operation is enabled. Completed in Phase 19 with `marketplace_orders`, `marketplace_settlements`, `marketplace_payouts`, paid install checkout/webhook application, refund settlement impact, and settlement-backed publisher stats.
- [x] **MARKET-04**: Marketplace moderation and abuse workflows cover publish, approve, reject, takedown, appeal, and audit paths. Completed in Phase 19 with takedown, appeal, reinstate, abuse report, resolve/dismiss routes, and append-only governance events.
- [x] **ADMIN-BILL-01**: Admin can inspect billing sessions, webhook events, subscriptions, top-ups, invoices, refunds, settlements, and payout state. Completed in Phase 20 with read-only `/api/v1/admin/billing/*` routes, `BillingInspectionStore`, Admin Billing UI, DB-backed route tests, and focused Vitest coverage.
- [x] **DOC-05**: v06 evidence maps money-movement and Marketplace governance requirements to files, tests, runtime/database proof, and residual v07/v08 work. Completed in Phase 20 with `20-VERIFICATION.md`, `20-01-SUMMARY.md`, API docs, commercial-gate docs, and v06 milestone snapshots.

### v05 Relay Billing Completeness

- [x] **RELAY-08**: Every registered `/v1/*` route is classified as commercial-supported and billed, internal/admin-only, or disabled in production. Completed in Phase 13 with `policy.go`, coverage tests, and `docs/release/relay-route-table.md`.
- [x] **RELAY-09**: Unsupported or partially implemented `/v1/*` endpoints fail closed in production before any upstream provider call. Completed in Phase 13 with `RegisterRoutesWithOptions`, `RejectIfProductionDisabled`, and production-disabled route tests.
- [x] **RELAY-10**: CI proves app services do not import provider SDKs or call direct provider URLs outside Relay/channel adapters. Completed in Phase 14 with `scripts/verify-relay-security.sh`, `bash scripts/check.sh relay-security`, and CI release-gate coverage.
- [x] **RELAY-11**: Supported Relay endpoints enforce tenant identity, auth policy, rate-limit policy, and audit semantics. Completed in Phase 14 with supported-route policy fields, production trusted internal identity guard, route-decision audit sink, and Chat/Agent/Memory Relay metadata tests.
- [x] **BILL-01**: Supported Relay calls pre-authorize quota, settle exactly once per idempotency key, and refund failed or partial calls. Completed in Phase 15 with channel-scoped preauthorization, billing idempotency snapshots, provider usage parsing, settlement/refund error propagation, and router lifecycle tests.
- [x] **BILL-02**: Streaming/realtime, file, batch, and async flows have explicit settlement models or are production-disabled. Completed in Phase 15 with route `BillingPolicy` coverage, Chat/Responses streaming rejection, production-disabled async/file route evidence, and route table/docs quality gates.
- [x] **DOC-04**: Relay route table, endpoint policy, and v05 verification evidence document the commercial Relay Authority Gate. Completed in Phase 16 with `16-VERIFICATION.md`, route table/gate docs, quality-gate assertions, DB-backed script verification, and v05 milestone snapshots.

### v04 Commercial Foundation

- [x] **TENANT-01**: Admin can create and manage organizations as first-class tenants.
- [x] **TENANT-02**: User can belong to multiple organizations with member, admin, and owner roles.
- [x] **TENANT-03**: User can invite, accept, revoke, remove, and transfer organization ownership with audit events.
- [x] **TENANT-04**: Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, and Marketplace publisher data are scoped by tenant.
- [x] **TENANT-05**: Tests prove cross-tenant access is denied for representative read and write paths.
- [x] **SEC-01**: Cookie-authenticated mutating routes require CSRF protection.
- [x] **SEC-02**: Login, registration, password reset, and sensitive admin/organization actions are rate limited.
- [x] **SEC-03**: Password policy and session rotation are enforced.
- [x] **MIGR-01**: Migration runner records applied migrations in `schema_migrations`.
- [x] **CI-01**: CI server job runs DB-backed HTTP integration tests instead of silently skipping persistence coverage.
- [x] **DOC-03**: Commercial gate documentation defines what must be true before any future milestone can claim commercial readiness.

### v03.3 Mainline Consolidation

- [x] **CONS-01**: Maintainer can classify the current uncommitted source, documentation, deployment, and frontend test changes into coherent work slices before committing.
- [x] **ROUTE-01**: Backend maintainer can register Agent, Memory, MCP, Notification, Quota, and WebSocket routes/services with explicit auth/admin boundaries and targeted tests.
- [x] **CHAT-06**: Chat and Agent calls preserve the Relay-first contract, including structured tool calls, streaming behavior, request metadata, and usage accounting hooks.
- [x] **AUTH-01**: Admin and app clients can rely on user/session contracts that expose user name and role consistently while keeping admin-only operations enforceable.
- [x] **DEPLOY-02**: Operator can use Docker, compose, CI, and Playwright changes without regressing the v03.2 restricted-network runtime validation path.
- [x] **DOC-02**: Developer can compare API, architecture, release, and README docs against live routes and commands for the consolidated mainline.
- [x] **VERIFY-01**: Maintainer can run a documented targeted verification suite before committing the consolidated work slices.

### v03.2 Quality and Release

- [x] **TEST-01**: Maintainer can run integration tests that prove Admin, Marketplace, Relay, Agent, Memory, and Quota service boundaries work together without bypassing Relay.
- [x] **TEST-02**: Release owner can run E2E tests that cover the primary Admin and Marketplace user workflows from the browser surface.
- [x] **DOC-01**: Developer or operator can use the API documentation and release checklist to validate the shipped HTTP surface and release candidate readiness.
- [x] **DEPLOY-01**: Operator can start and validate the current service stack with Docker/Kubernetes configuration. Docker compose build/start/smoke passed on 2026-05-12 with documented restricted-network overrides.

### Foundation through v03.1

- [x] **RELAY-01**: Relay 模块挂载到主 HTTP server
- [x] **RELAY-02**: `/v1/*` 路由走 Relay Engine
- [x] **RELAY-03**: 渠道配置从数据库读取 (channels 表)
- [x] **RELAY-04**: 模型路由配置 (model_routes 表)
- [x] **RELAY-05**: 开发环境默认渠道自动创建
- [x] **RELAY-06**: `GET /v1/models` 返回可用模型列表
- [x] **RELAY-07**: `POST /v1/chat/completions` 通过 Relay 调用 LLM
- [x] **CHAT-01**: Chat Gateway 重构为接口化设计
- [x] **CHAT-02**: RelayGateway 实现 OpenAI 格式请求
- [x] **CHAT-03**: 流式响应 (SSE) 支持
- [x] **CHAT-04**: Token 使用量正确记录到 usage_records
- [x] **CHAT-05**: 配置切换：Relay 模式 vs 本地模式
- [x] **AGENT-01**: 数据库迁移 - agents 表
- [x] **AGENT-02**: 数据库迁移 - agent_conversations 表
- [x] **AGENT-03**: 数据库迁移 - agent_messages 表
- [x] **AGENT-04**: Agent Service - CRUD 操作
- [x] **AGENT-05**: Agent Service - 创建对话
- [x] **AGENT-06**: Agent Service - 发送消息 (通过 Relay)
- [x] **AGENT-07**: Agent Service - 流式消息响应
- [x] **AGENT-08**: Agent HTTP Handler - REST API
- [x] **AGENT-09**: 前端 Agent 页面骨架
- [x] **AGENT-10**: 对话历史正确保存
- [x] **MCP-01**: 数据库迁移 - mcp_servers 表
- [x] **MCP-02**: MCP Client - 连接管理
- [x] **MCP-03**: MCP Client - 工具发现 (ListTools)
- [x] **MCP-04**: MCP Client - 工具调用 (CallTool)
- [x] **MCP-05**: MCP 协议消息结构
- [x] **MCP-06**: 内置工具 - web_search, calculator, datetime, http_request
- [x] **MCP-07**: MCP HTTP Handler - REST API
- [x] **MEM-01**: pgvector 扩展与向量索引
- [x] **MEM-02**: Memory Service - 文档分块与嵌入
- [x] **MEM-03**: Memory Service - 向量相似度搜索
- [x] **EXEC-01**: Agent 工具执行器
- [x] **EXEC-02**: Agent 执行循环 (多轮工具调用)
- [x] **EXEC-03**: 记忆注入到 Agent 上下文
- [x] **QUOTA-01**: 配额系统 - 预扣/结算/退款
- [x] **ADMIN-01**: 渠道管理 API
- [x] **ADMIN-02**: 套餐管理 API
- [x] **ADMIN-03**: 用户管理 API
- [x] **ADMIN-04**: Admin UI
- [x] **MARKET-01**: Agent 发布/发现/安装
- [x] **MARKET-02**: Marketplace UI

## Living Close Policy

- `.planning/REQUIREMENTS.md` remains the living cross-phase context file for this repository.
- Milestone completion archives requirements snapshots under `.planning/milestones/`.
- Future milestone completion must not reset or delete this living file unless a dedicated GSD policy migration explicitly owns that change.
- Historical traceability cleanup should use additive rows or notes, not broad rewrites of completed requirements.

**Coverage:**
- Active v08 requirements: 0
- Completed v08 requirements: 7
- Completed v07 requirements: 7
- Planned v08 phase mappings: 7
- Completed historical requirements: 78
- Blocked requirements: 0
- Unmapped v08 requirements: 0

---
*Requirements updated: 2026-05-29 after completing Phase 30 End-to-End Commercial Journey and Final Audit*
