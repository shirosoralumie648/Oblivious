# Requirements: Oblivious v04 Commercial Foundation

**Defined:** 2026-05-27
**Current milestone:** v04 Commercial Foundation
**Source spec:** `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`
**Core Value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

## Current Milestone Requirements

### Tenant And Identity

- [x] **TENANT-01**: Admin can create and manage organizations as first-class tenants. Completed in Phase 9 with `organizations` schema, tenant service/store, admin routes, and DB-backed lifecycle tests.
- [x] **TENANT-02**: User can belong to multiple organizations with member, admin, and owner roles. Completed in Phase 10 with `organization_memberships`, owner/admin/member roles, creator owner membership, list-membership APIs, and DB-backed lifecycle tests.
- [x] **TENANT-03**: User can invite, accept, revoke, remove, and transfer organization ownership with audit events. Completed in Phase 10 with invitation token hashing, accept/revoke routes, role/removal/ownership flows, and audit-backed mutation tests.
- [x] **TENANT-04**: Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, and Marketplace publisher data are scoped by tenant. Completed in Phase 11 with `organization_id` migration/backfill, session-derived active organization scope, tenant filters across core services, and tenant-aware Marketplace/Admin audit data.
- [x] **TENANT-05**: Tests prove cross-tenant access is denied for representative read and write paths. Completed in Phase 11 with DB-backed HTTP tests across Chat, Knowledge, Console, Agent, Memory, MCP, Quota, Marketplace publisher data, and Admin audit organization visibility.

### Production Auth Security

- [x] **SEC-01**: Cookie-authenticated mutating routes require CSRF protection. Completed in Phase 10 with `security_middleware.go`, session CSRF tokens, and route-surface tests for missing-token rejection.
- [x] **SEC-02**: Login, registration, password reset, and sensitive admin/organization actions are rate limited. Completed in Phase 10 with SQL-backed `auth_rate_limits` and 429 tests for auth and organization writes.
- [x] **SEC-03**: Password policy and session rotation are enforced. Completed in Phase 10 with strong-password validation, password reset session revocation, invitation-accept session rotation, and membership-change session revocation tests.

### Migration And CI Evidence

- [x] **MIGR-01**: Migration runner records applied migrations in `schema_migrations`. Completed in Phase 9 with checksum-aware ledgered migration execution and idempotency tests.
- [ ] **CI-01**: CI server job runs DB-backed HTTP integration tests instead of silently skipping persistence coverage.
- [ ] **DOC-03**: Commercial gate documentation defines what must be true before any future milestone can claim commercial readiness.

## Future Requirements

- [ ] v05 Relay Billing Completeness: classify all `/v1/*` routes, fail closed in production for unsupported endpoints, and prove auth/rate-limit/billing/audit behavior per endpoint class.
- [ ] v06 Billing And Marketplace Operations: Stripe checkout/webhooks, subscription lifecycle, invoices, refunds, top-ups, Marketplace publisher settlement, platform fees, payout state, and moderation flows.
- [ ] v07 Production Operations: Kubernetes or equivalent production orchestration proof, backup/restore smoke, structured logs, tracing, metrics, alerts, dashboards, and runbooks.
- [ ] v08 Product Completeness: real or disabled built-in MCP tools, durable Agent workflows, Knowledge behavior matching product copy, commercial Admin/Marketplace UX, public docs, onboarding, pricing, and operator guides.

## Out of Scope For v04

| Feature | Reason |
|---------|--------|
| Full Stripe production rollout | Requires tenant foundation and billing policy; owned by v06 |
| Marketplace payout accounting | Requires billing state and settlement model; owned by v06 |
| All Relay endpoint billing completion | Requires endpoint classification and settlement model; owned by v05 |
| Kubernetes runtime proof | Production orchestration belongs to v07 after SaaS foundation is stable |
| RAG upgrade and Agent workflow expansion | Product completeness belongs to v08 |
| Mobile-specific experience | Web control plane remains the commercial priority |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| TENANT-01 | Phase 9 — Tenant Model and Migration Ledger | Complete |
| MIGR-01 | Phase 9 — Tenant Model and Migration Ledger | Complete |
| TENANT-02 | Phase 10 — Membership, Roles, and Auth Security | Complete |
| TENANT-03 | Phase 10 — Membership, Roles, and Auth Security | Complete |
| SEC-01 | Phase 10 — Membership, Roles, and Auth Security | Complete |
| SEC-02 | Phase 10 — Membership, Roles, and Auth Security | Complete |
| SEC-03 | Phase 10 — Membership, Roles, and Auth Security | Complete |
| TENANT-04 | Phase 11 — Tenant Scope Across Core Domains | Complete |
| TENANT-05 | Phase 11 — Tenant Scope Across Core Domains | Complete |
| CI-01 | Phase 12 — Commercial Gate CI and Evidence | Planned |
| DOC-03 | Phase 12 — Commercial Gate CI and Evidence | Planned |

## Historical Validated Requirements

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
- Active v04 requirements: 2
- Completed v04 requirements: 9
- Planned v04 phase mappings: 11
- Completed historical requirements: 57
- Blocked requirements: 0
- Unmapped v04 requirements: 0

---
*Requirements updated: 2026-05-28 after completing Phase 11 tenant scope across core domains*
