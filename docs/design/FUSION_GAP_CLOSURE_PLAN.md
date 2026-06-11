# Fusion 规格差距收敛计划（Gap Closure Plan)

> 生成于 2026-06-11，基于 4 份 fusion 设计文档的全量审计（33 agent 对抗验证）。
> 用户决策（2026-06-11）：**架构按规格完全重写（由低成本模型执行）；功能差距 P1 + 全部 P2 都做。**
> 执行纪律（goal.md）：按模块拆分；每个可验证步骤跑测试 → 更新文档 → git diff 自查 → commit → push；禁止无测试 push。

## 审计结论摘要

- 18 项声称的差距被对抗验证**驳回**（已实现或有已记录的延期/平台职责决策）。
- 11 项**确认**差距，分两类：

### 功能补齐类（先做）

| ID | 优先级 | 内容 | 状态 |
|---|---|---|---|
| part2-gap-1 | P1 | 内置工具 150+（现仅 6 个） | ☐ Stage A |
| agent-001/part2-gap-2 | P2 | 代码解释器 8 语言沙箱 | ☐ Stage B1 |
| agent-002 | P2 | Web 搜索 15+ 提供商 | ☐ Stage B2 |
| part2-gap-2 | P2 | 子 Agent 调用、动态模型路由、技能选择 | ☐ Stage B3 |
| rag-002 | P2 | deepdoc 深度文档理解 | ☐ Stage B4 |
| gap-part3-03/04 | P2 | MinIO / PgBouncer 部署清单 | ☐ Stage C0 |

### 架构重写类（后做，按规格执行）

| ID | 优先级 | 内容 | 状态 |
|---|---|---|---|
| arch-002 | P0 | Database per Service（12 服务独立库） | ☐ Stage C2 |
| gap-part3-01 | P1 | 12 微服务独立 K8s 部署（含副本数规格） | ☐ Stage C3 |
| frontend-001 | P1 | 前端迁移 Next.js 14 App Router | ☐ Stage D1 |
| part2-gap-4 | P2 | Zustand + SWR + Recharts + RHF/Zod | ☐ Stage D2 |

## 阶段拆分

### Stage A — 内置工具扩充（P1）✅ 完成（2026-06-11）
- A1 ☑ 工具目录设计：164 个工具 / 15 类（`.tmp/audit/tool-catalog.json`）。
- A2 ☑ 注册机制：`registerBuiltins()`（31c26ca）。
- A3 ☑ 实现：15 类全部落地（12 类并行 31edfc9 + 3 类串行收尾），**注册工具总数 170 ≥ 150**；全部真实 stdlib 实现，网络类默认禁用。
- A4 ☑ 全套件验证（go test ./... exit 0、vet/gofmt 干净）。
- 教训：同包并行实现引发 agent 互相禁用文件的事故；已改为串行/隔离策略（见进度日志）。

### Stage B — Agent/RAG 高级能力（P2）
- B1 ☐ 代码解释器：沙箱 Runner 接口已存在；实现容器化执行器（默认禁用，策略门控），8 语言。
- B2 ☐ Web 搜索多提供商：Provider 接口 + 15+ 实现（API key 配置驱动，无 key 则禁用）。
- B3 ☐ 子 Agent 调用 + 动态模型路由 + 技能选择。
- B4 ☐ deepdoc：版面分析/表格抽取管线（接口 + 可用实现 + 测试）。

### Stage C — 微服务架构重写（按规格）
- C0 ☑ PgBouncer / MinIO / Kafka 部署清单（K8s + docker-compose `infra-extras` profile + `scripts/verify-infra-manifests.sh` 静态验证；platform contract 文档同步更新）（2026-06-11）。
- C1 ☐ 重写 ADR + 服务边界与接口契约冻结（12 服务，gRPC proto 定义）。
- C2 ☐ Database per Service：12 个逻辑库 + 各服务独立连接 + 迁移拆分。
- C3 ☐ 服务可独立部署：每服务 cmd 入口 + Dockerfile target + K8s Deployment（副本数按 §9.2）。
- C4 ☐ gRPC 服务间调用 + Kafka 事件总线接入。
- 约束：每个子步骤保持单体模式可继续运行（双模式：单进程聚合 / 多服务拆分），测试始终全绿。

### Stage D — 前端按规格迁移
- D1 ☐ Next.js 14 App Router 迁移（(auth)/(workspace)/(admin) 路由组）。
- D2 ☐ Zustand + SWR + Recharts + React Hook Form + Zod。
- 约束：分页面增量迁移，迁移期间测试套件持续通过。

## 执行模型策略（用户要求省钱）
- 实现类 agent：sonnet；机械验证/窄范围检查：haiku；主循环只做编排与关键决策。

## 进度日志
- 2026-06-11：审计完成；测试基线全绿（web 602/602、Go 全包、quality gates exit 0）；修复 React Flow 真实画布测试（9c5e351）。
- 2026-06-11：Stage A 完成 — 内置工具 6→170（31c26ca、31edfc9 + 收尾提交），全部真实实现+测试；Stage C0 完成 — PgBouncer/MinIO/Kafka 参考清单与验证脚本。事故记录：12 类并行实现时 agent 互相禁用文件，损失 3 类后改串行收尾；后续同包工作必须串行或 worktree 隔离。下一步：Stage B（代码解释器沙箱、Web 搜索多提供商、子 Agent/动态路由、deepdoc）。
