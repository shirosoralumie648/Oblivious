# Requirements: Oblivious v06 Billing And Marketplace Operations

**Defined:** 2026-05-28
**Current milestone:** v06 Billing And Marketplace Operations
**Source spec:** `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`
**Core Value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

## Current Milestone Requirements

### Payment Authority

- [x] **PAY-01**: Stripe checkout routes are mounted in the running server, require authenticated tenant context, persist payment intent metadata, and can be tested without live Stripe keys. Completed in Phase 17 with `POST /api/v1/billing/checkout`, fake checkout route tests, `payment_intents`, and Stripe metadata propagation.
- [x] **PAY-02**: Stripe webhook route verifies signatures from the raw request body, records provider events idempotently, rejects invalid signatures, and preserves processing status/errors for admin inspection. Completed in Phase 17 with `POST /api/v1/billing/stripe/webhook`, `stripe_webhook_events`, and signed fixture/idempotency tests.

### Payment Lifecycle

- [ ] **PAY-03**: Subscription lifecycle, invoices, refunds, failed-payment states, plan changes, and top-ups are implemented as auditable state transitions. Target: Phase 18.

### Marketplace Operations

- [ ] **MARKET-03**: Marketplace publisher revenue, platform fee, payout state, and refund impact are modeled before paid Marketplace operation is enabled. Target: Phase 19.
- [ ] **MARKET-04**: Marketplace moderation and abuse workflows cover publish, approve, reject, takedown, appeal, and audit paths. Target: Phase 19.

### Billing Evidence

- [ ] **ADMIN-BILL-01**: Admin can inspect billing sessions, webhook events, subscriptions, top-ups, invoices, refunds, settlements, and payout state. Target: Phase 20.
- [ ] **DOC-05**: v06 evidence maps money-movement and Marketplace governance requirements to files, tests, runtime/database proof, and residual v07/v08 work. Target: Phase 20.

## Future Requirements

- [ ] v07 Production Operations: Kubernetes or equivalent production orchestration proof, backup/restore smoke, structured logs, tracing, metrics, alerts, dashboards, and runbooks.
- [ ] v08 Product Completeness: real or disabled built-in MCP tools, durable Agent workflows, Knowledge behavior matching product copy, commercial Admin/Marketplace UX, public docs, onboarding, pricing, and operator guides.

## Out of Scope For v06

| Feature | Reason |
|---------|--------|
| Kubernetes runtime proof | Production orchestration belongs to v07 |
| Backup/restore smoke and observability dashboards | Operations gate belongs to v07 |
| RAG upgrade and Agent workflow expansion | Product completeness belongs to v08 |
| Final public pricing/onboarding/operator guides | Final product/documentation completion belongs to v08 |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| PAY-01 | Phase 17 — Stripe Payment Authority and Webhook Ledger | Complete |
| PAY-02 | Phase 17 — Stripe Payment Authority and Webhook Ledger | Complete |
| PAY-03 | Phase 18 — Subscription Invoice Top-up Refund State Machine | Planned |
| MARKET-03 | Phase 19 — Marketplace Settlement and Governance | Planned |
| MARKET-04 | Phase 19 — Marketplace Settlement and Governance | Planned |
| ADMIN-BILL-01 | Phase 20 — Billing Admin Evidence and v06 Closeout | Planned |
| DOC-05 | Phase 20 — Billing Admin Evidence and v06 Closeout | Planned |

## Historical Validated Requirements

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
- Active v06 requirements: 5
- Completed v06 requirements: 2
- Planned v06 phase mappings: 7
- Completed historical requirements: 68
- Blocked requirements: 0
- Unmapped v06 requirements: 0

---
*Requirements updated: 2026-05-28 after completing Phase 17 Stripe Payment Authority and Webhook Ledger*
