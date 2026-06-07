# SOLO Runtime Evolution Decision

## 1. Decision

SOLO 在后续阶段选择 **A 路线：明确维持为受限 runtime MVP / 结构化任务编排 UI**，不在当前路线图周期内升级为真实 agent runtime 或多 agent orchestration。

## 2. Why

- 当前 `task/runtime.go` 的真实能力是固定状态推进、结构化步骤、审批/暂停/恢复、预算与结果摘要导出。
- 当前主线已经接通 `/api/v1/app/tasks*` 与 `SoloPage` 闭环，但执行语义仍明显受限。
- 若直接升级为真实执行器，需要重定义任务生命周期、事件流、执行持久化、工具调用、恢复语义与失败模型；这不是当前路线图里的“小演进”，而是架构换挡。
- 当前最危险的问题不是能力太少，而是继续用更大的词描述更浅的实现。

## 3. Scope In This Roadmap

本路线图周期内，SOLO 允许推进的范围：

- 任务创建、审批、暂停、恢复、预算、结果导出的体验与稳定性优化
- 页面复杂度治理与结构拆分
- 文档与路线图中对真实能力边界的统一表述
- 与 root DoD 一致的验证补齐

本路线图周期内，SOLO 明确不做：

- 真实 agent executor
- 多 agent orchestration
- 通用工具调用运行时
- 持久化事件流执行引擎
- 对外使用“自治执行器已完成”类表述

## 4. Architecture Consequence

- `src/server/internal/task/runtime.go` 继续按受限 runtime MVP 定位维护，不为假想未来执行器引入过度抽象。
- `src/web/src/routes/workspace/SoloPage.tsx` 的后续拆分以降低页面复杂度和澄清交互边界为目标，不偷渡真实执行器能力。
- `docs/architecture/current-system-contracts.md` 中对 SOLO 的定义继续保持：**受限 runtime MVP，而非完整多 agent orchestration**。

## 5. Exit Criteria For Reconsideration

只有在以下条件满足后，才重新评估是否进入真实执行器路线：

1. 主线结构债已收敛；
2. 文档权威层级与 DoD 门面已稳定；
3. 当前 SOLO MVP 的使用边界已被明确验证；
4. 路线图显式批准进入新的执行器架构阶段。
