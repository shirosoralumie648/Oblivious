# Phase 1: Relay 集成与基础能力 - Context

**Gathered:** 2026-04-27
**Status:** Ready for verification

<domain>
## Phase Boundary

将 Relay 模块集成到主应用，使 Chat 和 Agent 通过 Relay 调用 LLM，实现统一的计费和监控。包括 Agent Runtime 核心功能和 MCP Client 骨架。

</domain>

<decisions>
## Implementation Decisions

### Relay 集成 (M1.1)
- **D-01:** Relay 模块已挂载到主 HTTP server (`internal/http/server.go`)
- **D-02:** `/v1/*` 路由走 Relay Engine
- **D-03:** 渠道配置从数据库读取 (`internal/relay/store.go`)
- **D-04:** 开发环境自动创建默认 OpenAI 渠道
- **D-05:** 数据库迁移 `0013_channels.sql` 已完成

### Chat 走 Relay (M1.2)
- **D-06:** Chat Gateway 重构为接口化设计 (`ReplyGenerator` interface)
- **D-07:** `RelayGateway` 实现通过 Relay 调用 LLM
- **D-08:** `CompositeGateway` 支持 Relay + Local fallback
- **D-09:** 流式响应支持 (`RelayGateway.CompleteStream`)
- **D-10:** 配置切换：`RelayEnabled` 控制是否使用 Relay

### Agent Runtime (M1.3)
- **D-11:** Agent Service 实现完整 CRUD (`internal/agent/service.go`)
- **D-12:** Agent 对话通过 Relay 调用 LLM
- **D-13:** 数据库迁移 `0014_agents.sql` 已完成
- **D-14:** Agent 与 Memory 服务集成
- **D-15:** Agent 与 MCP Client 集成

### MCP Client (M1.4)
- **D-16:** MCP Client 实现连接管理 (`internal/mcp/client.go`)
- **D-17:** 工具发现和调用支持
- **D-18:** 内置工具实现 (`internal/mcp/builtin.go`)
- **D-19:** 数据库迁移 `0015_mcp_servers.sql` 已完成

### Claude's Discretion
- 测试覆盖率优化
- 错误处理完善
- 文档同步更新

</decisions>

<specifics>
## Specific Ideas

- 所有 LLM 调用必须经过 Relay，确保计费统一
- Agent 和 Chat 共享 RelayGateway 实例
- Memory 服务使用 Relay Embedder 进行向量嵌入

</specifics>

<canonical_refs>
## Canonical References

### 设计文档
- `docs/superpowers/plans/2026-04-22-full-delivery-plan.md` — 完整功能交付计划
- `docs/superpowers/specs/2026-04-09-oblivious-integration-design.md` — 整合设计文档
- `docs/superpowers/specs/2026-04-09-relay-redesign-design.md` — Relay 重设计文档

### 核心实现
- `src/server/internal/http/server.go` — Relay 挂载点
- `src/server/internal/http/router.go` — 服务组装
- `src/server/internal/relay/relay.go` — Relay 核心
- `src/server/internal/chat/relay_gateway.go` — Chat Relay 网关
- `src/server/internal/agent/service.go` — Agent 服务
- `src/server/internal/mcp/client.go` — MCP 客户端

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `RelayGateway`: 可复用于任何需要调用 LLM 的服务
- `CompositeGateway`: 支持 fallback 的复合网关
- `Memory.NewRelayEmbedder`: 通过 Relay 进行向量嵌入

### Established Patterns
- `ReplyGenerator` interface: Chat 和 Agent 共享的 LLM 调用接口
- Service-Store 分离: 所有服务遵循 Service + Store 模式
- 依赖注入: 服务通过构造函数注入依赖

### Integration Points
- `router.go`: 所有服务组装入口
- `server.go`: Relay 挂载到 HTTP server
- `agent/service.go`: Agent 与 Memory、MCP 集成

</code_context>

<verification_needed>
## Verification Needed

### 构建验证
```bash
cd src/server && go build ./...
cd src/web && pnpm build
```

### 测试验证
```bash
cd src/server && go test ./... -count=1
cd src/web && pnpm test
```

### 功能验证
- [ ] `GET /v1/models` 返回可用模型
- [ ] `POST /v1/chat/completions` 通过 Relay 调用 LLM
- [ ] Chat 消息走 Relay
- [ ] Agent 对话走 Relay
- [ ] MCP 工具调用正常

</verification_needed>

<deferred>
## Deferred Ideas

- Memory/RAG 向量检索优化 (Phase 2)
- Agent 工具执行循环增强 (Phase 2)
- Admin API 与 UI (Phase 3)
- Marketplace (Phase 3)

</deferred>

---

*Phase: 01-relay-integration*
*Context gathered: 2026-04-27*
