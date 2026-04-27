# ROADMAP.md

## 1. 路线图头信息

- Plan Basis: `docs/superpowers/plans/2026-04-22-full-delivery-plan.md + .planning/codebase/`
- 当前项目状态摘要: `项目已具备核心骨架，Relay 独立模块已实现但未集成，无 Agent Runtime，Knowledge 非向量 RAG`
- 目标状态摘要: `Phase 1 完成：Relay 集成到主应用，Chat 走 Relay 调用 LLM，Agent Runtime 核心可用，MCP Client 骨架完成`
- 关键风险: `Relay 集成复杂度、流式响应兼容性、Agent-Relay 计费集成`
- 总体推进策略: `按里程碑顺序执行，每个里程碑完成后验证`
- Phase 列表: `M1.1 Relay 挂载; M1.2 Chat 走 Relay; M1.3 Agent Runtime; M1.4 MCP Client`
- 最高优先级里程碑: `M1.1 Relay 挂载到主应用`
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

---
*Roadmap created: 2026-04-27*
