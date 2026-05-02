# Requirements: Oblivious v03.2 Quality and Release

**Defined:** 2026-04-27
**Current milestone:** v03.2 Quality and Release (started 2026-05-02)
**Core Value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

## Current Milestone Requirements

### Testing

- [ ] **TEST-01**: Maintainer can run integration tests that prove Admin, Marketplace, Relay, Agent, Memory, and Quota service boundaries work together without bypassing Relay.
- [ ] **TEST-02**: Release owner can run E2E tests that cover the primary Admin and Marketplace user workflows from the browser surface.

### Documentation

- [ ] **DOC-01**: Developer or operator can use the API documentation and release checklist to validate the shipped HTTP surface and release candidate readiness.

### Deployment

- [ ] **DEPLOY-01**: Operator can start and validate the current service stack with Docker/Kubernetes configuration.

## v1 Requirements (Phase 1)

### M1.1 Relay 集成

- [x] **RELAY-01**: Relay 模块挂载到主 HTTP server
- [x] **RELAY-02**: `/v1/*` 路由走 Relay Engine
- [x] **RELAY-03**: 渠道配置从数据库读取 (channels 表)
- [x] **RELAY-04**: 模型路由配置 (model_routes 表)
- [x] **RELAY-05**: 开发环境默认渠道自动创建
- [x] **RELAY-06**: `GET /v1/models` 返回可用模型列表
- [x] **RELAY-07**: `POST /v1/chat/completions` 通过 Relay 调用 LLM

### M1.2 Chat 走 Relay

- [x] **CHAT-01**: Chat Gateway 重构为接口化设计
- [x] **CHAT-02**: RelayGateway 实现 OpenAI 格式请求
- [x] **CHAT-03**: 流式响应 (SSE) 支持
- [x] **CHAT-04**: Token 使用量正确记录到 usage_records
- [x] **CHAT-05**: 配置切换：Relay 模式 vs 本地模式

### M1.3 Agent Runtime 核心

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

### M1.4 MCP Client 骨架

- [x] **MCP-01**: 数据库迁移 - mcp_servers 表
- [x] **MCP-02**: MCP Client - 连接管理
- [x] **MCP-03**: MCP Client - 工具发现 (ListTools)
- [x] **MCP-04**: MCP Client - 工具调用 (CallTool)
- [x] **MCP-05**: MCP 协议消息结构
- [x] **MCP-06**: 内置工具 - web_search, calculator, datetime, http_request
- [x] **MCP-07**: MCP HTTP Handler - REST API

## v2 Requirements (Phase 2-4 Historical Context)

### Phase 2: Agent 与 Memory 增强

- [x] **MEM-01**: pgvector 扩展与向量索引
- [x] **MEM-02**: Memory Service - 文档分块与嵌入
- [x] **MEM-03**: Memory Service - 向量相似度搜索
- [x] **EXEC-01**: Agent 工具执行器
- [x] **EXEC-02**: Agent 执行循环 (多轮工具调用)
- [x] **EXEC-03**: 记忆注入到 Agent 上下文
- [x] **QUOTA-01**: 配额系统 - 预扣/结算/退款

### Phase 3: Admin 与 Marketplace

- [x] **ADMIN-01**: 渠道管理 API
- [x] **ADMIN-02**: 套餐管理 API
- [x] **ADMIN-03**: 用户管理 API
- [x] **ADMIN-04**: Admin UI
- [x] **MARKET-01**: Agent 发布/发现/安装
- [x] **MARKET-02**: Marketplace UI

### Phase 4: 质量与发布 (Current v03.2)

- [ ] **TEST-01**: 集成测试覆盖 Admin、Marketplace、Relay、Agent、Memory、Quota 的关键协作边界
- [ ] **TEST-02**: E2E 测试覆盖 Admin 与 Marketplace 的核心浏览器工作流
- [ ] **DOC-01**: API 文档和发布检查清单支持候选版本验收
- [ ] **DEPLOY-01**: Docker/Kubernetes 配置可启动并验证当前服务栈

## Out of Scope

| Feature | Reason |
|---------|--------|
| 生产支付/分成结算 | 需要真实商业策略与 Stripe 生产配置 |
| 移动端专项应用 | 当前 Web 优先 |
| 大规模生产观测与告警 | Phase 4 之后单独规划 |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| RELAY-01 | M1.1 | Complete |
| RELAY-02 | M1.1 | Complete |
| RELAY-03 | M1.1 | Complete |
| RELAY-04 | M1.1 | Complete |
| RELAY-05 | M1.1 | Complete |
| RELAY-06 | M1.1 | Complete |
| RELAY-07 | M1.1 | Complete |
| CHAT-01 | M1.2 | Complete |
| CHAT-02 | M1.2 | Complete |
| CHAT-03 | M1.2 | Complete |
| CHAT-04 | M1.2 | Complete |
| CHAT-05 | M1.2 | Complete |
| AGENT-01 | M1.3 | Complete |
| AGENT-02 | M1.3 | Complete |
| AGENT-03 | M1.3 | Complete |
| AGENT-04 | M1.3 | Complete |
| AGENT-05 | M1.3 | Complete |
| AGENT-06 | M1.3 | Complete |
| AGENT-07 | M1.3 | Complete |
| AGENT-08 | M1.3 | Complete |
| AGENT-09 | M1.3 | Complete |
| AGENT-10 | M1.3 | Complete |
| MCP-01 | M1.4 | Complete |
| MCP-02 | M1.4 | Complete |
| MCP-03 | M1.4 | Complete |
| MCP-04 | M1.4 | Complete |
| MCP-05 | M1.4 | Complete |
| MCP-06 | M1.4 | Complete |
| MCP-07 | M1.4 | Complete |
| ADMIN-01 | 03-admin-marketplace | Complete |
| ADMIN-02 | 03-admin-marketplace | Complete |
| ADMIN-03 | 03-admin-marketplace | Complete |
| MARKET-01 | 03-admin-marketplace | Complete |
| ADMIN-04 | 03.1-admin-marketplace-ui | Complete |
| MARKET-02 | 03.1-admin-marketplace-ui | Complete |
| TEST-01 | Phase 4 / v03.2 | Planned |
| TEST-02 | Phase 4 / v03.2 | Planned |
| DOC-01 | Phase 4 / v03.2 | Planned |
| DEPLOY-01 | Phase 4 / v03.2 | Planned |

**Coverage:**
- Completed requirements: 42
- Planned requirements: 4
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-27*
*Last updated: 2026-05-02 starting v03.2 milestone*
