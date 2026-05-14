# Phase 2 Discussion Log

**Phase:** 02-agent-memory-enhancement
**Date:** 2026-04-27

## Discussion Summary

Phase 2 context captured decisions on three key areas discussed:

### Area 1: 工具调用循环实现
- **Question:** 工具调用循环应该如何处理 LLM 返回的 tool_calls？
- **Selected:** 自动循环执行
- **Rationale:** LLM 返回 tool_calls 时自动执行工具，将结果加入消息历史，继续调用 LLM 直到无更多 tool_calls

### Area 2: Quota-Relay 集成
- **Question:** 配额系统集成点应该在哪里？
- **Selected:** Relay 集成 Quota
- **Rationale:** Relay BillingHook 调用 Quota.PreConsume 预扣，请求完成后 Settle 结算。失败时 Refund 退款

### Area 3: 向量索引优化
- **Question:** 向量索引策略如何选择？
- **Selected:** 切换到 HNSW
- **Rationale:** HNSW 适合大规模高召回场景，可在迁移中从 IVFFlat 切换

## Deferred Ideas
- Memory 批量导入 API (Phase 3)
- 工具调用结果缓存 (Phase 3)
- Admin 配额管理 UI (Phase 3)

---

*Discussion completed: 2026-04-27*