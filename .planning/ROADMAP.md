# ROADMAP.md

## 1. 路线图头信息

- Plan Basis: `docs/superpowers/plans/2026-04-22-full-delivery-plan.md + .planning/codebase/`
- 当前项目状态摘要: `Phase 1, 2 已完成；Agent 工具循环、Memory HNSW、Quota-Billing 全部实现`
- 目标状态摘要: `Phase 3: Admin 管理面板与 Marketplace`
- 关键风险: `前端 Admin UI 复杂度、Marketplace 发布/安装流程设计`
- 总体推进策略: `按里程碑顺序执行，每个里程碑完成后验证`
- Phase 列表: `M1.1 Relay 挂载; M1.2 Chat 走 Relay; M1.3 Agent Runtime; M1.4 MCP Client; Phase 2 Agent 与 Memory 增强`
- 最高优先级里程碑: `Phase 3 Admin 与 Marketplace`
- 验收策略: `go test ./... + pnpm test + bash scripts/check.sh all`

## 2. Phase 1: Relay 集成与基础能力 (4周)

### M1.1 Relay 挂载到主应用 (Week 1)

**Goal**: 将 Relay 模块挂载到主 HTTP server，使 `/v1/*` 路由走 Relay

**Requirements**: RELAY-01 ~ RELAY-07

**Tasks**:
1. 数据库迁移 - channels 和 model_routes 表
2. 配置扩展 - Relay 配置项
3. Relay Store - 数据库持久化
4. 主应用集成 - server.go 修改
5. 开发环境默认渠道

**Success Criteria**:
- [ ] `GET /v1/models` 返回可用模型列表
- [ ] `POST /v1/chat/completions` 通过 Relay 调用 OpenAI
- [ ] 渠道配置可从数据库读取
- [ ] 无渠道时返回友好错误

---

### M1.2 Chat 走 Relay 调用 LLM (Week 1-2)

**Goal**: 修改 Chat 模块，使其通过 Relay 调用 LLM

**Requirements**: CHAT-01 ~ CHAT-05

**Tasks**:
1. Chat Gateway 重构 - 接口化设计
2. RelayGateway 实现 - OpenAI 格式请求
3. 流式响应支持 - SSE 解析
4. Token 使用量记录
5. 配置切换机制

**Success Criteria**:
- [ ] Chat 消息通过 Relay 发送到 OpenAI
- [ ] 流式响应正常工作
- [ ] Token 使用量正确记录
- [ ] 原有测试不回归

---

### M1.3 Agent Runtime 核心 (Week 2-3)

**Goal**: 建立 Agent 服务基础，支持创建、管理 Agent 并进行对话

**Requirements**: AGENT-01 ~ AGENT-10

**Tasks**:
1. 数据库迁移 - agents, agent_conversations, agent_messages
2. Agent Service - CRUD + 对话
3. Agent HTTP Handler - REST API
4. 前端 Agent 页面骨架

**Success Criteria**:
- [ ] Agent CRUD API 正常工作
- [ ] Agent 对话通过 Relay 调用 LLM
- [ ] 前端可创建和管理 Agent
- [ ] 对话历史正确保存

---

### M1.4 MCP Client 骨架 (Week 3-4)

**Goal**: 实现 MCP 客户端，支持工具发现和调用

**Requirements**: MCP-01 ~ MCP-07

**Tasks**:
1. 数据库迁移 - mcp_servers
2. MCP Client - 连接管理
3. MCP 协议消息
4. 内置工具实现
5. MCP HTTP Handler

**Success Criteria**:
- [ ] 可连接外部 MCP Server
- [ ] 可发现 MCP Server 提供的工具
- [ ] 可调用 MCP 工具
- [ ] 内置工具正常工作

---

## 3. 里程碑依赖

```
M1.1 (Relay 挂载)
    ↓
M1.2 (Chat 走 Relay)
    ↓
M1.3 (Agent Runtime) ──→ M1.4 (MCP Client)
```

## 4. 验收命令

每个里程碑完成后执行：

```bash
# 后端测试
cd src/server && go test ./... -count=1

# 前端测试
cd src/web && pnpm test

# 完整检查
bash scripts/check.sh all
bash scripts/test.sh all
```

## 5. Phase 2: Agent 与 Memory 增强 (Week 5-6)

**Goal**: 完成 Agent 自动工具调用循环、Memory 向量检索加固，以及 Quota 与 Relay Billing 的真实集成

**Requirements**: MEM-01 ~ MEM-03, EXEC-01 ~ EXEC-03, QUOTA-01

**Tasks**:
1. Chat/Relay 契约扩展 - 保留 tools/tool_calls/usage 等结构化字段，支持 Agent 工具调用
2. Agent Runner 串联 - 让 `agent.Service` 走 `Runner` 自动循环并持久化工具结果
3. Memory 加固 - HNSW 索引迁移、检索与嵌入路径验证、补充测试
4. Quota-Billing 串联 - Relay Billing Hook 接入 `quota.Service`，补全预扣/结算/退款链路

**Success Criteria**:
- [x] Agent 可自动执行内置工具与 MCP 工具并继续对话
- [x] 工具结果正确写入 `agent_messages`，最终回复可返回给前端
- [x] Memory 向量搜索保持按用户隔离并通过测试验证
- [x] Relay 请求可完成 Quota 预扣/结算/退款而不绕过 Relay

**Completed**: 2026-04-28 | **Tests**: 61 passing | **Commits**: 8 tasks + SUMMARY

## 6. Backlog

### Phase 999.1: Follow-up — Phase 01 incomplete plan artifacts (BACKLOG)

**Goal**: 补齐 Phase 01 缺失的执行总结工件，避免后续阶段缺少完成记录
**Source phase:** `01-relay-integration`
**Deferred at:** `2026-04-27` during `$gsd-next` advancement to `02-agent-memory-enhancement`
**Plans:**
- [ ] `01-relay-integration/PLAN.md` ran to verification completion but has no matching `SUMMARY.md`
**Notes:**
- [ ] Reconstruct a concise `SUMMARY.md` from `PLAN.md` + `VERIFICATION.md`
- [ ] Confirm whether missing P0/P1 tests should be promoted into an active follow-up phase

---
*Roadmap created: 2026-04-27*
