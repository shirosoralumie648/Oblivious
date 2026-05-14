# Phase 2: Agent 与 Memory 增强 - Context

**Gathered:** 2026-04-27
**Status:** Ready for planning

<domain>
## Phase Boundary

增强 Agent 执行能力（多轮工具调用循环）和 Memory 向量检索（pgvector 优化），实现配额预扣/结算机制与 Relay Billing 集成。

</domain>

<decisions>
## Implementation Decisions

### Memory/RAG 向量检索 (MEM-01~03)
- **D-01:** pgvector 扩展已启用，使用 `vector(1536)` 存储 OpenAI embeddings
- **D-02:** IVFFlat 索引已创建 (`idx_memory_chunks_embedding`)
- **D-03:** `RelayEmbedder` 通过 Relay 调用 `/v1/embeddings` API
- **D-04:** `TextChunker` 默认 512 字符分块，64 字符重叠
- **D-05:** 向量搜索使用余弦距离 (`<=>` 操作符)

### Agent 工具执行循环 (EXEC-01~03)
- **D-06:** `Runner` 已有 `MaxIterations` 配置（默认 10 次）
- **D-07:** `ToolExecutor` 支持内置工具和 MCP 工具执行
- **D-08:** 工具调用检测需要 LLM 返回 `tool_calls` 字段
- **D-09:** 当前 `RunWithTools` 是简化实现，未完整实现循环
- **D-09b:** 工具调用循环采用**自动循环执行**模式：LLM 返回 tool_calls → 执行工具 → 结果加入上下文 → 继续调用 LLM

### 向量索引优化
- **D-13:** 从 IVFFlat 切换到 HNSW 索引，适合大规模高召回场景

### 配额系统集成 (QUOTA-01)
- **D-10:** `Quota.Service` 已有 `PreConsume`/`Settle`/`Refund` 方法
- **D-11:** `BillingSession` 支持幂等性检查
- **D-12:** Relay Billing Hook 调用 Quota 服务进行预扣/结算/退款

### Claude's Discretion
- 向量索引参数调优（lists 数量）
- 工具调用超时处理
- 错误重试策略

</decisions>

<specifics>
## Specific Ideas

- 工具调用循环应支持流式响应
- Memory 搜索结果应注入到系统提示后
- 配额不足时应返回友好错误而非直接拒绝

</specifics>

<canonical_refs>
## Canonical References

### 设计文档
- `docs/superpowers/plans/2026-04-22-full-delivery-plan.md` — 完整功能交付计划
- `docs/superpowers/specs/2026-04-09-oblivious-integration-design.md` — 整合设计文档

### 核心实现
- `src/server/internal/memory/service.go` — Memory 服务（分块、嵌入、搜索）
- `src/server/internal/memory/embedder.go` — RelayEmbedder 实现
- `src/server/internal/memory/chunker.go` — 文本分块器
- `src/server/internal/agent/runner.go` — Agent 执行循环
- `src/server/internal/agent/executor.go` — 工具执行器
- `src/server/internal/quota/service.go` — 配额服务

### 数据库迁移
- `src/server/migrations/0016_pgvector.sql` — pgvector 扩展和 memory 表
- `src/server/migrations/0017_quotas.sql` — 配额表

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `RelayEmbedder`: 可复用于任何需要向量嵌入的服务
- `TextChunker`: 支持段落边界分割和句子边界检测
- `ToolExecutor`: 已支持内置工具和 MCP 工具
- `Quota.Service`: 完整的预扣/结算/退款逻辑

### Established Patterns
- `MemorySearcher` interface: Agent 注入 Memory 的接口
- `RunnerConfig`: 执行循环配置模式
- `BillingSession`: 幂等性计费会话模式

### Integration Points
- `agent/service.go`: `SetMemory()` 注入 Memory 服务
- `relay/billing.go`: 需要集成 Quota 预扣/结算
- `agent/runner.go`: `RunWithTools()` 需要完整实现

### Gaps Identified
1. **Runner.RunWithTools()** - 简化实现，未检测 LLM 返回的 tool_calls
2. **向量索引优化** - IVFFlat 参数固定，未根据数据量调整
3. **Quota-Relay 集成** - Billing Hook 未调用 Quota 服务

</code_context>

<verification_needed>
## Verification Needed

### 构建验证
```bash
source ~/.g/env && cd src/server && go build ./...
```

### 测试验证
```bash
source ~/.g/env && cd src/server && go test ./internal/memory/... ./internal/agent/... ./internal/quota/...
```

### 功能验证
- [ ] Memory 向量搜索返回正确结果
- [ ] Agent 工具调用循环正常工作
- [ ] 配额预扣/结算正确记录

</verification_needed>

<deferred>
## Deferred Ideas

- Memory 批量导入 API (Phase 3)
- 向量索引自动调优 (Phase 3)
- 工具调用结果缓存 (Phase 3)
- Admin 配额管理 UI (Phase 3)

</deferred>

---

*Phase: 02-agent-memory-enhancement*
*Context gathered: 2026-04-27*
