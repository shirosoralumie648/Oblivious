# Knowledge Evolution Decision

## 1. Decision

Knowledge 在后续阶段选择 **A 路线：继续沿当前文本检索 Beta 收口**，不在当前路线图周期内升级为 embedding + 向量索引 + 异步 ingestion 的完整 RAG 架构。

## 2. Why

- 当前主线已具备知识库、文档 CRUD 与 `/retrieve` 检索闭环，现状与 `docs/architecture/current-system-contracts.md` 一致。
- 当前 retrieval 明确仍是文本匹配；若直接切换到完整 RAG，将显著改变 `knowledge/*` 的存储模型、索引流程、失败恢复与运维边界。
- 当前路线图的优先顺序是：先冻结真相、再修结构、后做深化。Knowledge 当前最紧迫的问题是能力表述真实与 Beta 质量收口，不是立即换架构。

## 3. Scope In This Roadmap

本路线图周期内，Knowledge 允许推进的范围：

- 检索排序质量优化
- snippet 质量优化
- 空结果反馈优化
- 页面回归与交互稳定性治理
- 与 root DoD 一致的验证口径补齐

本路线图周期内，Knowledge 明确不做：

- embedding 存储
- 向量索引 / ANN 检索
- 异步 ingestion pipeline
- 独立检索任务队列
- 对外使用“RAG 已完成”类表述

## 4. Architecture Consequence

- `src/server/internal/knowledge/*` 继续保持当前文本检索 Beta 架构，不为未来 RAG 预埋额外复杂抽象。
- `src/web/src/routes/workspace/KnowledgePage.tsx` 的后续拆分以降低页面复杂度为目标，不引入新的能力承诺。
- `docs/architecture/current-system-contracts.md` 中对 Knowledge 的定义继续保持：**文本检索 Beta，而非完整 RAG**。

## 5. Exit Criteria For Reconsideration

只有在以下条件满足后，才重新评估是否进入完整 RAG 路线：

1. `router.go` 与大页面拆分完成，主线结构债已收敛；
2. root DoD 门面已稳定执行；
3. 当前文本检索 Beta 的质量问题已收口；
4. 路线图显式批准进入新的架构换挡阶段。
