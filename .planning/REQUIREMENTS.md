# Requirements: Oblivious Phase 1

**Defined:** 2026-04-27
**Core Value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

## v1 Requirements (Phase 1)

### M1.1 Relay 集成

- [ ] **RELAY-01**: Relay 模块挂载到主 HTTP server
- [ ] **RELAY-02**: `/v1/*` 路由走 Relay Engine
- [ ] **RELAY-03**: 渠道配置从数据库读取 (channels 表)
- [ ] **RELAY-04**: 模型路由配置 (model_routes 表)
- [ ] **RELAY-05**: 开发环境默认渠道自动创建
- [ ] **RELAY-06**: `GET /v1/models` 返回可用模型列表
- [ ] **RELAY-07**: `POST /v1/chat/completions` 通过 Relay 调用 LLM

### M1.2 Chat 走 Relay

- [ ] **CHAT-01**: Chat Gateway 重构为接口化设计
- [ ] **CHAT-02**: RelayGateway 实现 OpenAI 格式请求
- [ ] **CHAT-03**: 流式响应 (SSE) 支持
- [ ] **CHAT-04**: Token 使用量正确记录到 usage_records
- [ ] **CHAT-05**: 配置切换：Relay 模式 vs 本地模式

### M1.3 Agent Runtime 核心

- [ ] **AGENT-01**: 数据库迁移 - agents 表
- [ ] **AGENT-02**: 数据库迁移 - agent_conversations 表
- [ ] **AGENT-03**: 数据库迁移 - agent_messages 表
- [ ] **AGENT-04**: Agent Service - CRUD 操作
- [ ] **AGENT-05**: Agent Service - 创建对话
- [ ] **AGENT-06**: Agent Service - 发送消息 (通过 Relay)
- [ ] **AGENT-07**: Agent Service - 流式消息响应
- [ ] **AGENT-08**: Agent HTTP Handler - REST API
- [ ] **AGENT-09**: 前端 Agent 页面骨架
- [ ] **AGENT-10**: 对话历史正确保存

### M1.4 MCP Client 骨架

- [ ] **MCP-01**: 数据库迁移 - mcp_servers 表
- [ ] **MCP-02**: MCP Client - 连接管理
- [ ] **MCP-03**: MCP Client - 工具发现 (ListTools)
- [ ] **MCP-04**: MCP Client - 工具调用 (CallTool)
- [ ] **MCP-05**: MCP 协议消息结构
- [ ] **MCP-06**: 内置工具 - web_search, calculator, datetime, http_request
- [ ] **MCP-07**: MCP HTTP Handler - REST API

## v2 Requirements (Phase 2-4)

### Phase 2: Agent 与 Memory 增强

- **MEM-01**: pgvector 扩展与向量索引
- **MEM-02**: Memory Service - 文档分块与嵌入
- **MEM-03**: Memory Service - 向量相似度搜索
- **EXEC-01**: Agent 工具执行器
- **EXEC-02**: Agent 执行循环 (多轮工具调用)
- **EXEC-03**: 记忆注入到 Agent 上下文
- **QUOTA-01**: 配额系统 - 预扣/结算/退款

### Phase 3: Admin 与 Marketplace

- **ADMIN-01**: 渠道管理 API
- **ADMIN-02**: 套餐管理 API
- **ADMIN-03**: 用户管理 API
- **ADMIN-04**: Admin UI
- **MARKET-01**: Agent 发布/发现/安装
- **MARKET-02**: Marketplace UI

### Phase 4: 质量与发布

- **TEST-01**: 集成测试
- **TEST-02**: E2E 测试
- **DOC-01**: API 文档
- **DEPLOY-01**: Docker/Kubernetes 配置

## Out of Scope

| Feature | Reason |
|---------|--------|
| Memory/RAG 向量检索 | Phase 2 范围 |
| Agent 工具执行循环 | Phase 2 范围 |
| Admin API/UI | Phase 3 范围 |
| Marketplace | Phase 3 范围 |
| E2E 测试 | Phase 4 范围 |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| RELAY-01 | M1.1 | Pending |
| RELAY-02 | M1.1 | Pending |
| RELAY-03 | M1.1 | Pending |
| RELAY-04 | M1.1 | Pending |
| RELAY-05 | M1.1 | Pending |
| RELAY-06 | M1.1 | Pending |
| RELAY-07 | M1.1 | Pending |
| CHAT-01 | M1.2 | Pending |
| CHAT-02 | M1.2 | Pending |
| CHAT-03 | M1.2 | Pending |
| CHAT-04 | M1.2 | Pending |
| CHAT-05 | M1.2 | Pending |
| AGENT-01 | M1.3 | Pending |
| AGENT-02 | M1.3 | Pending |
| AGENT-03 | M1.3 | Pending |
| AGENT-04 | M1.3 | Pending |
| AGENT-05 | M1.3 | Pending |
| AGENT-06 | M1.3 | Pending |
| AGENT-07 | M1.3 | Pending |
| AGENT-08 | M1.3 | Pending |
| AGENT-09 | M1.3 | Pending |
| AGENT-10 | M1.3 | Pending |
| MCP-01 | M1.4 | Pending |
| MCP-02 | M1.4 | Pending |
| MCP-03 | M1.4 | Pending |
| MCP-04 | M1.4 | Pending |
| MCP-05 | M1.4 | Pending |
| MCP-06 | M1.4 | Pending |
| MCP-07 | M1.4 | Pending |

**Coverage:**
- v1 requirements: 29 total
- Mapped to phases: 29
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-27*
*Last updated: 2026-04-27 after initialization*
