# ARCHAEOLOGY_REPORT

## 1. 扫描范围与证据基础 (Audit Scope)

### 1.1 审计对象与纳入规则

本次按“是否能直接证明系统边界、模块关系、数据流、部署方式、依赖约束”执行向心式扫描。

**核心证据优先级：**
代码实证 > 入口注释 > 根 README > 规划文档。

**强证据：**
- 可执行入口与装配点：`src/server/cmd/server/main.go`、`src/server/internal/http/router.go`、`src/web/src/app/router.tsx`
- 配置与运行时契约：`src/server/internal/config/config.go`、`config/.env.example`、`package.json`、`pnpm-workspace.yaml`、`.github/workflows/ci.yml`
- 核心领域实现：`src/server/internal/auth/*`、`chat/*`、`knowledge/*`、`task/*`、`console/*`
- 数据访问与数据库边界：`src/server/internal/db/db.go`、`src/server/cmd/migrate/main.go`、`src/server/internal/*/store.go`
- 前端 API 与页面闭环：`src/web/src/features/*/api.ts`、`src/web/src/routes/workspace/*.tsx`
- 主线契约文档：`docs/architecture/current-system-contracts.md`

**降级为背景材料：**
- `docs/reports/*.md`：可反映历史认知，但不能压过代码实证
- `docs/superpowers/specs/*.md`：仅在与当前代码相符时作为解释材料
- `lobehub/`、`new-api/`：仅在判断“仓库真实边界”时纳入，不作为主线系统能力结论依据

**明确跳过范围：**
- `src/web/node_modules/**`
- `.worktrees/**`
- 构建/缓存目录与第三方依赖产物

纳入 `lobehub/` 与 `new-api/` 的唯一原因是：它们直接影响仓库边界认知、工作区归属判断与“哪些内容不属于主线”的架构结论。除此之外，其内部能力不作为主线实现结论依据。

### 1.2 实际扫描范围

**根级：**
- `README.md`
- `package.json`
- `pnpm-workspace.yaml`
- `config/.env.example`
- `.github/workflows/ci.yml`
- `scripts/dev.sh`
- `scripts/check.sh`
- `scripts/test.sh`
- `scripts/verify-quality-gates.sh`

**主线后端：**
- `src/server/cmd/server/main.go`
- `src/server/cmd/migrate/main.go`
- `src/server/internal/config/config.go`
- `src/server/internal/db/db.go`
- `src/server/internal/http/server.go`
- `src/server/internal/http/router.go`
- `src/server/internal/http/chat_handler.go`
- `src/server/internal/http/knowledge_handler.go`
- `src/server/internal/http/task_handler.go`
- `src/server/internal/http/console_handler.go`
- `src/server/internal/chat/service.go`
- `src/server/internal/knowledge/service.go`
- `src/server/internal/task/service.go`
- `src/server/internal/task/runtime.go`
- `src/server/internal/console/service.go`
- `src/server/internal/*/store.go`（通过目录扫描与 grep 交叉验证）

**主线前端：**
- `src/web/package.json`
- `src/web/src/app/router.tsx`
- `src/web/src/features/chat/api.ts`
- `src/web/src/features/knowledge/api.ts`
- `src/web/src/features/tasks/api.ts`
- `src/web/src/routes/workspace/KnowledgePage.tsx`
- `src/web/src/routes/workspace/SoloPage.tsx`
- 以及 `src/web/src` 下路由、features、services、store 的目录级扫描

**文档与治理：**
- `docs/architecture/current-system-contracts.md`
- `docs/superpowers/specs/2026-04-06-m0-baseline-freeze-closeout-design.md`
- `docs/reports/2026-04-04-codebase-analysis.md`
- `docs/reports/2026-04-04-progress-plan.md`
- `docs/reports/2026-04-04-todo-tracker.md`
- `docs/reports/2026-04-05-explicit-markers.md`
- `docs/reports/2026-04-05-technical-audit.md`
- `docs/reports/2026-04-05-todo-tracker.md`

**边界参考目录：**
- `new-api/CLAUDE.md`
- `new-api/AGENTS.md`
- `lobehub/CLAUDE.md`
- `lobehub/AGENTS.md`

### 1.3 子项目边界与归属

| 子项目/目录 | 归属判定 | 证据 | 结论 |
| --- | --- | --- | --- |
| `src/server` | 主线后端 | `README.md`、`scripts/dev.sh`、`router.go` | 属于主线 |
| `src/web` | 主线前端 | `pnpm-workspace.yaml:1-2`、`package.json`、`README.md` | 属于主线 |
| `config` | 主线配置 | `README.md`、`scripts/check.sh`、`docs/architecture/current-system-contracts.md:33-37` | 属于主线 |
| `scripts` | 主线工程入口 | `package.json`、`scripts/*.sh` | 属于主线 |
| `.github/workflows` | 主线 CI 边界 | `.github/workflows/ci.yml` | 属于主线 |
| `docs/architecture` / `docs/governance` / `docs/release` | 主线治理与契约文档 | `scripts/verify-quality-gates.sh:31-98` | 属于主线 |
| `new-api` | 仓内参考仓 | `README.md:15,66-67`、`pnpm-workspace.yaml` 未纳入、目录内独立规则文件 | 不属于主线 |
| `lobehub` | 仓内参考仓 | `README.md:15,66-67`、`pnpm-workspace.yaml` 未纳入、目录内独立规则文件 | 不属于主线 |

### 1.4 关键实证结论

1. **项目真实形态不是多仓联动平台，而是单仓内的“主线产品 + 两个参考仓”。**
   - 证据：`README.md:3,15,66-67` 明确声明主线范围为 Go backend + React frontend，`lobehub` 与 `new-api` 仅为 reference directories。
   - 证据：`pnpm-workspace.yaml:1-2` 只纳入 `src/web`。
   - 证据：`scripts/dev.sh:19-29,36-58` 只编排 `src/web` 与 `src/server`。

2. **主线后端是可运行的业务壳，不是 scaffold。**
   - 证据：`src/server/cmd/server/main.go` 打开数据库并装配 HTTP server。
   - 证据：`src/server/internal/http/router.go:18-319+` 已注册 auth/chat/knowledge/task/console 的完整 HTTP 矩阵。
   - 证据：`src/server/internal/http/server.go`（已读）负责实际 server 启动与关闭。

3. **主线数据库边界是 PostgreSQL 单库，不存在多数据库抽象。**
   - 证据：`src/server/internal/db/db.go:8-19` 直接使用 `lib/pq` 与 `postgres` driver。
   - 证据：`src/server/internal/config/config.go` 强依赖 `DATABASE_URL`。
   - 证据：`src/server/cmd/migrate/main.go:20-49` 直接对同一 DB 执行 SQL migration。

4. **前端路由已经围绕 Chat / Knowledge / Solo / Console 形成工作区闭环，而不是早期占位状态。**
   - 证据：`src/web/src/app/router.tsx` 已注册 `/knowledge`、`/knowledge/:knowledgeBaseId`、`/solo`、`/console/*` 等主线路由。
   - 证据：`src/web/src/routes/workspace/KnowledgePage.tsx` 与 `SoloPage.tsx` 为真实交互页面，而非纯标题占位。

5. **Knowledge 已进入 Beta 检索闭环，但仍是文本检索，不是向量 RAG。**
   - 证据：`docs/architecture/current-system-contracts.md:211-215` 明确写出 retrieval 已进入 Knowledge Beta，但仍基于文本匹配。
   - 证据：`src/server/internal/http/knowledge_handler.go` 存在 `/retrieve` 接口；`src/web/src/features/knowledge/api.ts:23-24,44-45` 已消费。

6. **SOLO 当前仍是受限 runtime MVP，不是真实 agent orchestration。**
   - 证据：`docs/architecture/current-system-contracts.md:230-235` 直接声明“不是完整多 agent orchestration”。
   - 证据：`src/server/internal/task/runtime.go:11-55,128-225` 显示其本质是固定状态推进器与模板化 result summary。
   - 证据：`src/web/src/routes/workspace/SoloPage.tsx:64-97` 下载结果时导出的是结构化任务摘要，不是执行产物流水线。

### 1.5 证据冲突与裁决

| 冲突项 | 高优先级证据 | 低优先级证据 | 裁决 |
| --- | --- | --- | --- |
| 前端是否仍处于“路由未挂载、页面占位”阶段 | `src/web/src/app/router.tsx`、`KnowledgePage.tsx`、`SoloPage.tsx`、`docs/architecture/current-system-contracts.md:247-279` | `docs/reports/2026-04-04-progress-plan.md:167-175`、`docs/reports/2026-04-05-technical-audit.md:96-102` | 以代码与当前契约文档为准。旧报告已过时。 |
| `lobehub`/`new-api` 是否属于主线 | `README.md:15,66-67`、`pnpm-workspace.yaml:1-2`、`scripts/verify-quality-gates.sh:60-61,85-86` | 历史报告中对 workspace 风险的旧描述 | 以当前根配置为准。它们已被排除在主线之外。 |
| Knowledge 是否完全未做检索 | `src/server/internal/http/knowledge_handler.go`、`src/web/src/features/knowledge/api.ts`、`docs/architecture/current-system-contracts.md:211-215` | `docs/reports/2026-04-05-technical-audit.md:205-209` | 旧结论已被后续实现覆盖。当前为文本检索 Beta，不是“完全没有检索”。 |
| SOLO 是否只是“resume 直接完成任务” | `src/server/internal/task/runtime.go` 与 `task/service.go` 当前逻辑 | `docs/reports/2026-04-04-progress-plan.md:145-147` 的旧风险描述 | 风险方向仍成立，但应更新表述：当前是固定 runtime 模板，不等于完全空转。 |

### 1.6 审计置信度

**总体置信度：高**

原因：
- 关键边界结论同时有代码、配置、脚本、README 与当前契约文档交叉支撑。
- 核心主线入口、路由装配、数据库边界、工作区边界均已直接读取实证文件。
- 低置信度区域仅限未逐个深读的参考仓内部实现，以及未展开的 migration SQL 细节；这些不影响主线架构主轴结论。

---

## 2. 文档权威等级映射 (Authority Hierarchy)

分类规则：
- **[L] 原始底座**：当前仍直接约束主线运行、边界、契约的底座文件。
- **[V] 过程产物**：收敛过程中的中间报告/设计，反映阶段性判断。
- **[F] 未来占位**：描述未来目标、后续增强、尚未完全落地能力。
- **[G] 脱节幻觉**：与当前代码/依赖/边界明显脱节，或已被新实证覆盖。

| 路径 | 分类 | 权威权重(1-5) | 关键依据 | 真实影响 |
| --- | --- | ---: | --- | --- |
| `README.md` | L | 5 | 明确主线范围与 reference directories | 决定根层边界判断 |
| `package.json` | L | 5 | root 命令真实编排入口 | 决定本地执行面 |
| `pnpm-workspace.yaml` | L | 5 | 仅纳入 `src/web` | 决定 workspace 主线边界 |
| `config/.env.example` | L | 4 | 运行配置样例，受 `config.Load()` 约束 | 决定部署契约 |
| `.github/workflows/ci.yml` | L | 5 | 主线 CI 边界实证 | 决定交付门禁 |
| `scripts/dev.sh` | L | 5 | 主线运行装配脚本 | 决定运行时真实启动方式 |
| `scripts/check.sh` | L | 5 | 主线检查入口 | 决定静态验证边界 |
| `scripts/test.sh` | L | 5 | 主线测试入口 | 决定测试边界 |
| `src/server/cmd/server/main.go` | L | 5 | 后端真实入口 | 决定后端装配真相 |
| `src/server/internal/http/router.go` | L | 5 | 全部 HTTP 边界集中注册 | 决定能力边界与路由矩阵 |
| `src/server/internal/config/config.go` | L | 5 | 环境变量真实消费点 | 决定部署与依赖约束 |
| `src/server/internal/db/db.go` | L | 5 | 明确 PostgreSQL 单驱动 | 决定数据库边界 |
| `src/web/src/app/router.tsx` | L | 5 | 前端真实路由矩阵 | 决定前端主路径 |
| `src/web/src/routes/workspace/KnowledgePage.tsx` | L | 4 | Knowledge 页面真实交互 | 证明功能非占位 |
| `src/web/src/routes/workspace/SoloPage.tsx` | L | 4 | SOLO 页面真实交互 | 证明任务闭环已接通 |
| `src/web/src/features/chat/api.ts` | L | 4 | Chat API 实际消费接口 | 证明前后端契约对接 |
| `src/web/src/features/knowledge/api.ts` | L | 4 | Knowledge API 实际消费接口 | 证明 retrieval/CRUD 对接 |
| `src/web/src/features/tasks/api.ts` | L | 4 | Task API 实际消费接口 | 证明 SOLO runtime 前后端闭环 |
| `docs/architecture/current-system-contracts.md` | L | 5 | 与当前代码高度一致，且被质量脚本强引用 | 当前唯一可信架构基线文档 |
| `docs/superpowers/specs/2026-04-06-m0-baseline-freeze-closeout-design.md` | V | 4 | 解释为何将参考仓排除出主线 | 解释边界冻结意图 |
| `docs/reports/2026-04-05-explicit-markers.md` | V | 3 | 对 reference repos 的 marker 分布有辅助价值 | 用于噪音剔除 |
| `docs/reports/2026-04-04-codebase-analysis.md` | V | 2 | 部分判断已被后续代码覆盖 | 仅可做历史对照 |
| `docs/reports/2026-04-05-technical-audit.md` | G | 1 | 多项结论已与当前代码冲突 | 若继续引用会误导架构判断 |
| `docs/reports/2026-04-04-progress-plan.md` | G | 1 | 对前端状态、Knowledge、SOLO 的完成度判断已过时 | 若作为现状依据会产生错误 backlog |
| `docs/reports/2026-04-04-todo-tracker.md` | G | 1 | 依据旧断层生成的 backlog | 会把已实现内容当成未启动 |
| `docs/reports/2026-04-05-todo-tracker.md` | G | 1 | 同上，且多项已被主线修正 | 继续沿用会制造伪问题 |
| `new-api/CLAUDE.md` | F | 2 | 描述独立子仓规则，不作用于主线 | 仅用于确认其独立性 |
| `new-api/AGENTS.md` | F | 2 | 同上 | 仅用于确认其独立性 |
| `lobehub/CLAUDE.md` | F | 2 | 描述独立子仓规则，不作用于主线 | 仅用于确认其独立性 |
| `lobehub/AGENTS.md` | F | 2 | 同上 | 仅用于确认其独立性 |

**关键冲突说明：**
- `docs/architecture/current-system-contracts.md` 与当前代码一致，应压过更早的 `docs/reports/*` 阶段性诊断。
- `docs/reports/2026-04-05-technical-audit.md` 曾有价值，但其关于 Knowledge/路由/workspace 的若干判断已被当前配置与实现推翻，因此降为 **G**。

---

## 3. 核心意图冲突与实现漂移 (Implementation Drift)

### 3.1 意图断层

#### 冲突 A：仓库是否仍是“主线 + 嵌入仓共同干扰的 workspace”
- 旧文档说法：`docs/reports/2026-04-05-technical-audit.md:7-15,59-68` 认为 `lobehub` 与 `new-api` 会干扰 root workspace/lockfile。
- 当前实证：`README.md:15,66-67`、`pnpm-workspace.yaml:1-2`、`scripts/verify-quality-gates.sh:60-61,85-86` 已明确并强制它们不属于主线 workspace。
- 结论：该冲突已被 M0 冻结收敛。继续沿用旧说法属于文档滞后，不是代码现实。

#### 冲突 B：前端是否仍未接入 Knowledge / Solo 路由
- 旧文档说法：`docs/reports/2026-04-04-progress-plan.md:167-175`、`docs/reports/2026-04-05-technical-audit.md:96-99` 指出 `/knowledge`、`/solo` 未挂载。
- 当前实证：`src/web/src/app/router.tsx:44-45` 已注册 knowledge 路由，且当前契约文档 `docs/architecture/current-system-contracts.md:255-260` 已明确接入。
- 结论：旧分析已失效。

#### 冲突 C：Knowledge 是否只是 CRUD 容器
- 旧文档说法：`docs/reports/2026-04-05-technical-audit.md:83-84,205-209` 认为当前没有 retrieval/召回。
- 当前实证：`src/server/internal/http/router.go:179-186` 注册 `/retrieve`；`src/web/src/features/knowledge/api.ts:23-24,44-45` 消费 retrieval；`docs/architecture/current-system-contracts.md:211-215` 定义其为文本检索 Beta。
- 结论：当前不是“无检索”，而是“有 Beta 文本检索，但没有向量 RAG”。

### 3.2 实现欺诈

以下不是“代码不存在”，而是“文档/页面容易让人高估实现成熟度”的点。

1. **SOLO 被包装成 runtime，但本质仍是模板化状态推进器。**
   - 证据：`src/server/internal/task/runtime.go:11-55` 以固定步骤推进状态；`buildRuntimeResultSummary` 直接拼装模板文本（`202-225`）。
   - 结论：它可演示，但不具备真实 agent 执行语义。

2. **Knowledge 被表述为 Beta retrieval，但仍没有 embedding、向量索引、异步 ingestion pipeline。**
   - 证据：`docs/architecture/current-system-contracts.md:213-215` 已主动承认。
   - 结论：Beta 成立，但要严格限定为“文本匹配检索 Beta”，不能对外误称 RAG 已完成。

3. **Console 已形成页面与接口矩阵，但仍偏摘要看板，不是成熟运营后台。**
   - 证据：`src/server/internal/http/console_handler.go:21-83` 只暴露 usage/access/models/billing 四类摘要接口；`src/server/internal/console/service.go:11-80` 数据结构轻量。
   - 结论：已完成 starter console，不应宣称为完整 admin dashboard。

4. **主线治理已冻结，但历史报告仍大量保留“系统尚未接路由/尚未收敛”的旧叙事。**
   - 证据：`docs/reports/2026-04-04-*`、`2026-04-05-*` 与 `docs/architecture/current-system-contracts.md` 冲突。
   - 结论：这是文档层面的实现欺诈风险：不是代码骗你，是旧报告骗你。

### 3.3 噪音剔除建议

后续审计建议**默认忽略**以下文件或将其降级到仅供背景参考：

- `docs/reports/2026-04-04-codebase-analysis.md`
- `docs/reports/2026-04-04-progress-plan.md`
- `docs/reports/2026-04-04-todo-tracker.md`
- `docs/reports/2026-04-05-technical-audit.md`
- `docs/reports/2026-04-05-todo-tracker.md`

原因：这些文件记录的是 M0/M1 前后的阶段性断层，当前已经被更高优先级证据覆盖。继续把它们当“现状依据”，只会重复制造伪 backlog。

应保留但只作**边界参考**的范围：
- `lobehub/**`
- `new-api/**`

原因：它们能证明“这两个仓是独立系统”，但不应用于反推主线能力。

---

## 4. 架构主轴重建 (Architectural Pivot)

### 4.1 一句话定义项目真实形态

**Oblivious 的真实形态是：一个以 `src/server`（Go + net/http + PostgreSQL）和 `src/web`（React + Vite）构成的单主线工作区产品，提供 Chat、Knowledge 文本检索 Beta、SOLO 受限 runtime MVP 与 Console 摘要看板，而 `lobehub`/`new-api` 仅是仓内参考仓。**

### 4.2 权威底座来源

- 根边界：`README.md`、`package.json`、`pnpm-workspace.yaml`
- 运行与交付边界：`scripts/dev.sh`、`scripts/check.sh`、`scripts/test.sh`、`.github/workflows/ci.yml`
- 后端装配：`src/server/cmd/server/main.go`、`src/server/internal/http/router.go`
- 前端装配：`src/web/src/app/router.tsx`
- 当前架构基线文档：`docs/architecture/current-system-contracts.md`

### 4.3 Vibe 修改方向

从证据看，Vibe/快速迭代阶段做的不是“引入大而全架构”，而是：
- 快速把早期骨架推到可演示业务壳
- 在前端补齐工作区页面与交互闭环
- 把旧的“嵌入仓干扰主线”问题通过 M0 边界冻结清理掉
- 通过 `current-system-contracts.md` 把新的执行真相文档化

换言之，Vibe 修改方向是**把原本散乱的原型推进到有主线路由、有后端契约、有基本治理入口的 Beta 收口形态**，而不是把系统推向多仓融合。

### 4.4 当前核心业务闭环

当前最清晰的闭环是：
1. 用户通过 auth 建立 session。
2. 在 workspace 中进入 `/chat`、`/knowledge`、`/solo`。
3. Chat 可以绑定知识库并转为 SOLO 任务草稿。
4. Knowledge 支持知识库/文档 CRUD 与文本 retrieval。
5. SOLO 支持任务创建、审批、暂停、恢复、预算与结果导出，但执行语义受限。
6. Console 提供使用、模型、计费、访问上下文的摘要视图。

这条闭环已经具备产品形态，但离“智能工作区”宣传语中的强能力版本还有明显距离。

---

## 5. CTO 决策咨询 (Architectural Crossroads)

### 模糊点 1：Knowledge 的下一步到底是“产品化文本检索”还是“真正的 RAG 平台”

**选项 A：继续沿当前文本检索 Beta 收口**
- 聚焦排序、snippet、空结果反馈、规模边界与性能基线。
- 优点：与当前实现连续，能较快形成稳定交付面。
- 代价：智能含量有限，产品叙事不能拔得太高。

**选项 B：切入 embedding + indexing + async ingestion 的真正 RAG 轨道**
- 需要引入新的数据结构、异步流程、失败恢复与运维成本。
- 优点：能力上限更高。
- 代价：会显著改变后端架构主轴，当前 `knowledge/*` 几乎要重做一半。

**建议：** 如果近期目标是 Beta 收口，优先 A；若产品定位必须押注“知识增强工作流”，则尽快在 ROADMAP 里承认 B 是架构换挡，不要假装是小迭代。

### 模糊点 2：SOLO 应继续维持“受限 runtime MVP”，还是升级为真正 agent runtime

**选项 A：明确把 SOLO 定义为结构化任务编排 UI + 状态机 MVP**
- 继续强化审批、预算、知识源绑定、结果导出。
- 优点：工程风险低，与现有 `task/runtime.go` 一致。
- 代价：无法承载真正自治执行叙事。

**选项 B：引入真实执行器、事件流、工具调用、恢复语义**
- 需要重新定义任务生命周期、审计日志、执行持久化与失败模型。
- 优点：产品能力上限高。
- 代价：现有 store/service/runtime 三层都要显著重构。

**建议：** 先在路线图里二选一。现在最危险的状态不是 A 或 B，而是继续用 B 的语言描述 A 的实现。

### 模糊点 3：`lobehub` / `new-api` 的长期关系是“永久参考仓”还是“未来集成输入”

**选项 A：永久参考仓策略**
- 明确永不接入 root workspace / CI / 交付链。
- 只允许作为设计启发或手工迁移来源。
- 优点：主线边界稳定，组织认知清晰。
- 代价：可能放弃复用部分成熟能力。

**选项 B：设定受控集成计划**
- 明确哪些能力会被迁入主线、采用何种边界、何时执行。
- 优点：复用空间更大。
- 代价：一旦没有严格边界，会再次把仓库拉回“多真相并存”。

**建议：** 如果 1-2 个里程碑内没有明确迁入计划，就应先按 A 治理，避免参考仓持续污染主线判断。

---

## 最终审计结论

这不是一个“设计缺失导致无法判断”的仓库；相反，当前仓库已经有了新的设计真相，只是历史报告还在持续制造旧真相回声。真正需要冻结的不是代码边界，而是认知边界：**主线就是 `src/server` + `src/web`，能力核心就是 Chat / Knowledge 文本检索 Beta / SOLO 受限 runtime / Console starter，其他一切都应围绕这个事实说话。**
