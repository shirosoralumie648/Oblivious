# Requirements: Oblivious v03.3 Mainline Consolidation

**Defined:** 2026-05-14
**Current milestone:** v03.3 Mainline Consolidation
**Core Value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

## Current Milestone Requirements

### Mainline Consolidation

- [x] **CONS-01**: Maintainer can classify the current uncommitted source, documentation, deployment, and frontend test changes into coherent work slices before committing. Completed in Phase 5 with `05-WORKTREE-INVENTORY.md`, `05-COMMIT-BOUNDARIES.md`, and `05-VERIFICATION.md`.

### Backend Route and Service Integration

- [x] **ROUTE-01**: Backend maintainer can register Agent, Memory, MCP, Notification, Quota, and WebSocket routes/services with explicit auth/admin boundaries and targeted tests. Completed in Phase 6 with route-surface, admin-boundary, and notification ownership regression tests.
- [x] **CHAT-06**: Chat and Agent calls preserve the Relay-first contract, including structured tool calls, streaming behavior, request metadata, and usage accounting hooks. Completed in Phase 6 with Relay metadata propagation, production fail-closed, and structured tool-call tests.
- [x] **AUTH-01**: Admin and app clients can rely on user/session contracts that expose user name and role consistently while keeping admin-only operations enforceable. Completed in Phase 6 with register/login/`/me` response tests and user preference default coverage.

### Deployment, Documentation, and Verification

- [x] **DEPLOY-02**: Operator can use Docker, compose, CI, and Playwright changes without regressing the v03.2 restricted-network runtime validation path. Completed in Phase 7 with frontend/E2E gates, CI wrapper checks, Docker compose config, and restricted-network deployment smoke evidence.
- [x] **DOC-02**: Developer can compare API, architecture, release, and README docs against live routes and commands for the consolidated mainline. Completed in Phase 8 with API, architecture, README, RC checklist, and deployment-remediation reconciliation against the live route surface.
- [x] **VERIFY-01**: Maintainer can run a documented targeted verification suite before committing the consolidated work slices. Completed in Phase 8 with docs-first verification, an explicit `TEST_DATABASE_URL` integration skip, and the preserved Phase 7 deploy baseline.

## Historical Validated Requirements

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

## Future Requirements

- [ ] Production observability, alerting, and operational dashboards beyond the current release-candidate gates.
- [ ] Kubernetes runtime proof once `kubectl` and a target cluster/context are available.
- [ ] Commercial billing and revenue-share operations after product/business policy is defined.
- [ ] Mobile-specific experience after the web control plane stabilizes.

## Out of Scope

| Feature | Reason |
|---------|--------|
| New product discovery | v03.3 consolidates current mainline work already present in the repo |
| Payment provider production rollout | Requires real commercial policy and credentials |
| Full Kubernetes validation | Local tooling is unavailable; Docker compose is the accepted proven runtime path |
| Broad historical artifact rewrite | Existing phase archives remain reference material unless a phase directly needs them |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| CONS-01 | Phase 5 — Dirty Worktree Triage and Commit Boundary | Complete |
| ROUTE-01 | Phase 6 — Backend Mainline Integration Hardening | Complete |
| CHAT-06 | Phase 6 — Backend Mainline Integration Hardening | Complete |
| AUTH-01 | Phase 6 — Backend Mainline Integration Hardening | Complete |
| DEPLOY-02 | Phase 7 — Frontend, E2E, and Deployment Gate Alignment | Complete |
| DOC-02 | Phase 8 — Contract Docs and Release Verification | Complete |
| VERIFY-01 | Phase 8 — Contract Docs and Release Verification | Complete |

**Coverage:**
- Completed historical requirements: 46
- Completed v03.3 requirements: 7
- Remaining v03.3 requirements: 0
- Blocked requirements: 0
- Unmapped v03.3 requirements: 0 ✓

---
*Requirements updated: 2026-05-17 after Phase 8 verification and backlog closeout*
