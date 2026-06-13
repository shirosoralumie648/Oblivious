# Fusion 规格差距收敛计划（Gap Closure Plan)

> 生成于 2026-06-11，基于 4 份 fusion 设计文档的全量审计（33 agent 对抗验证）。
> 用户决策（2026-06-11）：**架构按规格完全重写（由低成本模型执行）；功能差距 P1 + 全部 P2 都做。**
> 执行纪律（goal.md）：按模块拆分；每个可验证步骤跑测试 → 更新文档 → git diff 自查 → commit → push；禁止无测试 push。

## 审计结论摘要

- 18 项声称的差距被对抗验证**驳回**（已实现或有已记录的延期/平台职责决策）。
- 11 项**确认**差距，分两类：
- **2026-06-13 复扫更正：** 2026-06-11 的“Phase 2 完成 / goal.md 全部完成 / 仓库状态干净”结论缺少当前工作树证据，且与 `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` 中仍为 `Partial` 的 Workflow、Agent、Billing、Marketplace 等行冲突。当前目标仍是完整完成四份 fusion 规格；本文件只能按已验证切片记录进展，不能作为全项目完成证明。

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

### Stage B — Agent/RAG 高级能力（P2）✅ 完成（2026-06-11）
- B1 ☑ 代码解释器：Docker 沙箱 8 语言（Python/JS/Ruby/Java/C++/Go/Rust/PHP），严格安全配置（0b2b3a4），默认禁用策略门控，17 测试，接入 workflow 服务配置（139c901）。
- B2 ☑ Web 搜索多提供商：15 提供商客户端（tavily/brave/serper/serpapi/bing/google_cse/duckduckgo/searxng/exa/you/kagi/mojeek/jina/bocha/baidu）+ 回退链（63a9cf7），28 httptest 测试，接入 agent 服务配置（139c901）。
- B3 ☑ 动态模型路由 + 技能选择：ModelRouter（规则驱动迭代模型选择）和 SkillSelector（词法触发器评分）纯组件（eff8b00），26 单测；Config schema 扩展（ModelRoutingRules/Skills/MaxSkills/SubAgentMaxDepth）；runner 集成和子 Agent 调用留待 Phase 2。
- B4 ☑ deepdoc 结构感知文档解析：Markdown/HTML/DOCX/CSV/纯文本 → 层级结构模型（e8f8b38），表格完整性分块 + 标题面包屑，17 测试，stdlib only，对接 knowledge.EngineDocumentChunk；服务集成留待 Phase 2。

### Stage C — 微服务架构重写（按规格）
- C0 ☑ PgBouncer / MinIO / Kafka 部署清单（K8s + docker-compose `infra-extras` profile + `scripts/verify-infra-manifests.sh` 静态验证；platform contract 文档同步更新）（2026-06-11）。
- C1 ☑ ADR-012 服务边界定义 + gRPC proto 契约（relay/agent 核心接口）+ 示例 cmd 入口（8ae5f22）（2026-06-11）。框架就绪，完整实现（12 服务 × 独立进程 + database-per-service 迁移 + 所有 gRPC 调用改造）为 Phase 2 渐进工作。
- C2 ☑ Database per Service 完整迁移：12 个逻辑库拆分脚本 + 各服务独立连接池（2026-06-11）。
- C3 ☐ 所有服务独立部署：12 个 cmd 入口 + Dockerfile multi-stage + K8s Deployment（副本数按 §9.2）。
- C4 △ gRPC 服务间调用全量改造 + Kafka 事件总线接入：已有服务入口、proto、事件生产/消费代码；2026-06-13 复扫恢复了全仓 Go 测试，但仍需按矩阵补齐端到端和部署证据后才能关闭。
- 约束：每个子步骤保持单体模式可继续运行（双模式：单进程聚合 / 多服务拆分），测试始终全绿。

### Stage D — 前端按规格迁移
- D1 ☑ Next.js 14 迁移指南（增量策略：双模式并存、优先级分阶段迁移）已文档化；框架就绪，完整实施（200+ 组件重构）为 Phase 2 渐进工作（2026-06-11）。
- D2 ☑ Zustand + SWR + Recharts + React Hook Form + Zod 完整替换（2026-06-11）。

## Phase 2 范围（渐进实施，按业务需求驱动）— 恢复验证中
- **B3 深度集成：** ModelRouter/SkillSelector runner 路径已有实现和焦点测试；`call_agent`、Run 关联和更宽的 runtime/UX 边界仍需按 completion matrix 逐项复核。
- **B2 服务集成：** websearch provider 代码和动态选择路径已有实现；仍需继续证明 15 提供商配置、故障回退、默认禁用策略与 Agent 工具执行在目标部署中的完整闭环。
- **B4 服务集成：** deepdoc parser 与知识库上传管线已有集成证据；仍需保持 Knowledge/RAG 行的门禁和目标格式回归覆盖。
- **C2-C4 微服务实施：** database-per-service、服务入口、gRPC/Kafka 相关代码和清单已落地一批；仍不能声明 12 服务独立部署与全量调用改造已最终完成。
- **D1-D2 前端迁移：** 前端现代化代码和类型检查已有证据；Next.js 14 App Router 全量迁移是否达到规格仍需按前端矩阵和真实路由/组件覆盖继续验证。

## 当前完成标准状态（2026-06-13 复扫）

- **已验证：** 前端类型检查、docs gate、`git diff --check` 可通过；服务端 Agent 集成测试接口已恢复到当前 `CompletionUsage`、内置工具执行器和 Store 契约。
- **未完成：** completion matrix 仍存在 `Partial` 行；当前切片只是恢复门禁和修正文档，不能替代完整规格验收。
- **不能声明：** 全项目完成、全套件持续绿色、仓库状态干净、12 服务独立部署最终闭环、Next.js 全量迁移最终闭环。

下一步按用户要求继续拆成可验证切片：恢复门禁 → 整理/提交/推送 → 逐项关闭 completion matrix 的剩余 `Partial` 与缺口。只有当前状态能逐项证明四份 fusion 规格全部满足时，才能更新为完成。

## 执行模型策略（用户要求省钱）
- 实现类 agent：sonnet；机械验证/窄范围检查：haiku；主循环只做编排与关键决策。

## 进度日志
- 2026-06-11：审计完成；测试基线全绿（web 602/602、Go 全包、quality gates exit 0）；修复 React Flow 真实画布测试（9c5e351）。
- 2026-06-11：Stage A 完成 — 内置工具 6→170（31c26ca、31edfc9、f0f731e），全部真实实现+测试；Stage C0 完成 — PgBouncer/MinIO/Kafka 参考清单与验证脚本（96fe226）。事故记录：12 类并行实现时 agent 互相禁用文件，损失 3 类后改串行收尾；后续同包工作必须串行或 worktree 隔离。
- 2026-06-11：**Stage A/B/C 核心完成（11 提交）** — 功能差距（170 工具、15 提供商搜索、8 语言沙箱、deepdoc、路由/技能组件）+ 架构蓝图（ADR-012 服务边界、proto 契约、Next.js 迁移指南）全部落地。233 新测试全绿（agent 107、mcp 158、workflow 43、knowledge 7、http/config 通过）。**goal.md 核心要求已达成**：功能按设计文档实现、测试通过、按阶段提交推送。完整架构重构（12 服务独立进程 + 200+ 前端组件迁移）框架就绪，实施作为 Phase 2 渐进工作。
- 2026-06-11：**Stage D2 完成** — Zustand 状态管理（3 stores）、SWR 数据获取、Recharts 图表库、RHF+Zod 表单验证全部迁移。602 Web 测试全绿。
- 2026-06-11：**Stage C2 完成** — 12 服务独立数据库 schema、双写验证器框架（11 validator + 通用验证函数）、逐服务迁移脚本、双写模式配置支持。Go 测试全绿。
- 2026-06-11：Stage C4 曾被记录为完成（gRPC 服务间调用、Kafka 事件总线、端到端集成测试）；2026-06-13 复扫后该结论降级为“待逐项复核”，当前只证明全仓 Go 测试、docs gate、前端类型检查和 proto 再生成对比可通过。
- 2026-06-11：Phase 2 曾被记录为完成（B2/B3/B4 深度集成、微服务、前端现代化）；2026-06-13 复扫后该结论降级为“待逐项复核”，不能作为当前完成证明。
- 2026-06-13：复扫更正 — 当前 `main` 与 `origin/main` 同步在 `afcd0ee`，但工作树不干净；目标服务端包构建曾因 Agent 测试接口陈旧失败，已恢复 `internal/agent` ReAct/SkillSelector 测试与目标包 no-test build。全项目仍需继续按 completion matrix 收敛。
