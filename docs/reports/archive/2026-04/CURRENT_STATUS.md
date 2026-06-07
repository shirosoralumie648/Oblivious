# CURRENT_STATUS.md

## 1. 审计头信息

- Base Commit ID: `b3ebed5be66528d3cab3262e10b0303e8008aa1a`
- Audit Date: `2026-04-07`
- Audit Basis: `main 分支存在；当前审计基于当前工作区文件系统 + 当前可获取 HEAD Commit + main 作为优先锚点的代码现实`
- Project Health Score: `71/100`
- Project Health Score 理由: `项目主线边界已经冻结，核心代码与当前契约文档基本一致，具备工程化整顿基础；但 SOLO 仍停留在受限 runtime MVP、Knowledge 仍非真正 RAG、路由装配集中膨胀、历史报告大量过时，工程推进若继续混用旧文档将再次失控。`
- Environment Preconditions: `git 仓库存在，main 分支存在，HEAD Commit 可获取，README/架构契约文档存在，主核心源码目录存在且可扫描；无阻断级环境缺失。`
- Scan Scope Summary: `主核心目录为 src/server 与 src/web/src；次核心目录为 config、scripts、.github/workflows、docs/architecture；纳入入口、路由、服务、存储、配置、CI、测试主文件与主线契约文档；排除 node_modules、.git、缓存/构建产物、参考仓内部实现细节；核心模块判定聚焦 auth、preferences、chat、knowledge、task、console、http 装配、配置与数据库边界。`
- 审计降级项: `无强制降级项；仅对 new-api / lobehub 采取边界级扫描，不深挖其内部实现。`
- 审计可信度总评: `高`
- 最高优先级风险: `历史报告与当前代码现实存在严重版本错层，若继续拿旧报告当现状依据，会直接污染后续 ROADMAP 和工程排序。`
- Blockers 摘要: `当前不存在“核心闭环完全不可推进”的系统级 Blocker，但存在会阻碍严格工程化推进的高优先级治理问题：旧文档失真、SOLO 能力被高估、Knowledge 能力被高估、router.go 单点膨胀。`
- 下一步建议: `以 CURRENT_STATUS.md 和 ROADMAP.md 取代旧报告作为唯一执行基线，先清理认知债，再推进 Phase 1 加固。`

## 2. 环境前提检查

### 环境前提检查表

| 前提项 | 检查结果 | 证据/命令 | 降级处理 | 影响说明 |
| --- | --- | --- | --- | --- |
| 当前目录是否为 git 仓库 | 满足 | `git rev-parse --is-inside-work-tree` => `true` | 无 | 可基于版本库状态审计 |
| 是否存在 main 分支 | 满足 | `git branch --list main` => `main` | 无 | 可使用 main 作为优先锚点 |
| 是否存在 master 分支 | 未发现 | `git branch --list master` => 空 | 无需降级 | 不影响，因为 main 已存在 |
| 是否可获取 Commit ID | 满足 | `git rev-parse HEAD` => `b3ebed5be66528d3cab3262e10b0303e8008aa1a` | 无 | 可精确版本锚定 |
| 当前检出分支是否有效 | 满足 | `git branch --show-current` => `m2a-knowledge-beta-closeout` | 无 | 可记录当前工作上下文 |
| 是否存在 README / Specs / 设计文档 | 满足 | `README.md`、`docs/architecture/current-system-contracts.md`、`docs/superpowers/specs/*.md` | 无 | 可交叉重建设计意图 |
| 是否存在核心源码目录与可扫描文件 | 满足 | `src/server/**`、`src/web/src/**`、`config/**`、`scripts/**` | 无 | 具备完整主线扫描条件 |
| 是否存在可用于验证主线边界的工程文件 | 满足 | `package.json`、`pnpm-workspace.yaml`、`.github/workflows/ci.yml`、`scripts/*.sh` | 无 | 可验证构建/运行/CI 边界 |

### 环境前提结论

当前项目满足工程化审计的最低前提，而且不只是“能扫”，而是已经具备足够强的代码、配置、脚本、文档四类证据链。问题不在环境缺失，问题在认知版本混乱：旧报告还在说昨天的真相，当前代码已经是明天的真相。

## 3. 术语判定与扫描边界

### 3.1 核心源码目录判定规则

判定遵循以下优先级：
1. **优先级 1**：直接承载运行时入口、主请求链路、核心业务服务、路由装配、持久化访问的目录。
2. **优先级 2**：与主流程强相关的配置装配、数据库连接、CI、脚本、契约文档目录。
3. **优先级 3**：补充性目录，如 tests、reports、参考仓、历史 specs，仅作为支撑证据，不能替代核心目录判断。

判定证据来源：`README.md`、`package.json`、`pnpm-workspace.yaml`、`scripts/dev.sh`、`.github/workflows/ci.yml`、运行入口与路由装配文件、当前契约文档。

### 3.2 核心源码目录判定表

| 候选目录 | 优先级 | 判定依据 | 是否纳入核心范围 | 备注 |
| --- | --- | --- | --- | --- |
| `src/server` | 1 | `scripts/dev.sh` 直接启动；`cmd/server/main.go` 为后端入口；`internal/http/router.go` 承载主 API 装配 | 是 | 主核心目录 |
| `src/web/src` | 1 | `pnpm-workspace.yaml` 仅纳入 `src/web`；`src/web/package.json` 为前端构建入口；`app/router.tsx` 为前端主路由入口 | 是 | 主核心目录 |
| `src/web` | 2 | 前端包级入口、依赖、测试脚本所在 | 是 | 次核心，包级容器 |
| `config` | 2 | `.env.example` 定义运行配置样例；与 `src/server/internal/config/config.go` 对应 | 是 | 次核心，配置边界 |
| `scripts` | 2 | root `dev/check/test` 的真实执行入口 | 是 | 次核心，工程装配边界 |
| `.github/workflows` | 2 | CI 边界与本地脚本一致性验证来源 | 是 | 次核心，交付边界 |
| `docs/architecture` | 2 | `current-system-contracts.md` 为当前可用架构基线 | 是 | 次核心，文档基线 |
| `docs/reports` | 3 | 历史阶段性分析报告 | 有条件纳入 | 仅作为历史对照，不能覆盖代码 |
| `docs/superpowers/specs` | 3 | 阶段性设计文档 | 有条件纳入 | 仅在与当前代码一致时使用 |
| `new-api` | 3 | README 明确其为 reference directory | 否（仅边界审计） | 参考仓，不属于主线 |
| `lobehub` | 3 | README 明确其为 reference directory | 否（仅边界审计） | 参考仓，不属于主线 |
| `src/web/node_modules` | 低 | 第三方依赖安装产物 | 否 | 默认排除 |
| `.git` / `.worktrees` / 缓存目录 | 低 | 非业务源码 | 否 | 默认排除 |

### 3.3 可扫描文件规则表

| 文件类型/目录 | 优先级 | 纳入/排除 | 规则依据 | 备注 |
| --- | --- | --- | --- | --- |
| 入口文件（`cmd/server/main.go`、`app/router.tsx`） | 高 | 纳入 | 直接决定主运行链路 | 核心证据 |
| 路由/控制器/handler | 高 | 纳入 | 直接决定系统边界与主请求链路 | 核心证据 |
| service / store / db / config | 高 | 纳入 | 决定业务流转、持久化与依赖约束 | 核心证据 |
| `package.json` / `pnpm-workspace.yaml` / CI / 脚本 | 高 | 纳入 | 决定构建、运行与交付边界 | 核心证据 |
| `docs/architecture/current-system-contracts.md` | 高 | 纳入 | 当前主线契约基线 | 文档级强证据 |
| 页面主文件（Knowledge/Solo/Chat 等） | 中高 | 纳入 | 证明前端闭环实际状态 | 关键补充证据 |
| 测试主文件 | 中 | 补充纳入 | 用于评估可用性与工程质量 | 不替代实现判断 |
| 历史报告与旧 TODO 文档 | 中低 | 条件纳入 | 只用于识别设计漂移和过时认知 | 不能当现状依据 |
| 静态资源、截图、缓存、构建产物 | 低 | 排除 | 不直接证明运行时边界 | 默认排除 |
| `node_modules` / `dist` / `build` / `coverage` / `.git` | 低 | 排除 | 标准排除项 | 除非唯一线索，否则不纳入 |
| 参考仓内部实现文件 | 低 | 排除（边界识别除外） | 不属于主线交付链 | 仅用于说明“不是主线” |

### 3.4 核心模块判定规则

核心模块至少满足以下一项：
- 直接承载主业务能力
- 位于主请求/任务链路
- 控制关键状态流转
- 负责核心数据读写
- 承担认证授权、调度、持久化、外部接口等关键职责

判定优先依据：
1. 文档声明（`README.md`、`current-system-contracts.md`）
2. 运行入口与路由映射（`router.go`、`app/router.tsx`）
3. 依赖装配关系（`NewService`、`NewSQLStore`、`create*Api`）
4. 配置与脚本引用关系
5. 页面与 API 的对接闭环

### 3.5 核心模块判定表

| 模块名称 | 模块类型(核心/次核心/支撑) | 判定依据 | 关键职责 | 不确定性 |
| --- | --- | --- | --- | --- |
| Auth / Session | 核心 | README + `/api/v1/auth/*` 路由 + middleware 装配 | 注册、登录、登出、会话恢复、权限门禁 | 低 |
| Preferences | 核心 | `/api/v1/app/me/preferences` + userprefs service/store | 保存 onboarding/defaultMode/modelStrategy 等偏好 | 低 |
| Chat | 核心 | `/api/v1/app/conversations*` + `ChatPage` 路由 + `chat/service.go` | 会话、消息、配置、模型调用、handoff 到 SOLO | 低 |
| Knowledge | 核心 | `/api/v1/app/knowledge-bases*` + KnowledgePage + retrieval API | 知识库/文档 CRUD 与文本检索 | 低 |
| SOLO Task Runtime | 核心 | `/api/v1/app/tasks*` + `task/runtime.go` + SoloPage | 任务状态推进、预算、审批、知识源绑定 | 低 |
| Console | 核心 | `/api/v1/console/*` + console pages + console service | usage/models/billing/access 摘要 | 低 |
| HTTP 装配层 | 核心 | `internal/http/router.go` + `server.go` | 把所有模块装成系统边界 | 低 |
| Config | 次核心 | `config.go` + `.env.example` | 运行时环境变量装配 | 低 |
| DB / Migration | 次核心 | `db.Open` + `cmd/migrate` | PostgreSQL 连接与 schema 应用 | 低 |
| Scripts / CI | 支撑 | root scripts + workflow | 保证本地与 CI 一致 | 低 |
| 历史报告系统 | 支撑（负向） | `docs/reports/*.md` | 影响认知，不承载运行逻辑 | 中；其问题在于会误导 |

## 4. 项目形态判断

- 项目形态判断结论: `前后端混合单主线仓库 + 仓内参考子仓`
- 设计意图来源: `README.md`、`docs/architecture/current-system-contracts.md`、运行入口、路由装配、工程脚本`

### 项目形态说明

这不是典型 monorepo，也不是多服务平台。它的真实形态更像：
- **主线产品**：`src/server` + `src/web`
- **工程边界**：`config` + `scripts` + `.github/workflows`
- **参考输入**：`new-api` + `lobehub`

`pnpm-workspace.yaml` 只纳入 `src/web`，而 Go 后端通过 shell 脚本独立启动。这说明该仓库虽然目录里挂了多个大块内容，但交付主链路并不复杂：一个 Go API，一个 React 前端，一套 root 级脚本与 CI。最大的特殊性不在架构，而在“历史上曾经存在多真相并存”，现在需要强制只保留一个真相。

## 5. 设计意图与架构重建

### 5.1 核心架构描述

本项目的底层逻辑是：
- **后端**：轻量分层架构（Router/Handler → Service → Store/DB），运行在 `net/http + PostgreSQL` 之上
- **前端**：React + React Router 的工作区型 SPA，围绕 Chat / Knowledge / Solo / Console 组织页面
- **系统主流向**：Browser → SPA 路由 → HTTP Client → Go API Router → Service → SQL Store → PostgreSQL

这不是 MVC，也不是插件平台，也不是事件驱动系统。它是一个很直接的工作区产品架构：**页面驱动、接口驱动、数据库驱动**。优点是简单直接；缺点是所有装配点都容易集中膨胀，尤其 `router.go` 和大页面组件已经开始有明显草台化堆积。

### 5.2 数据流与模型

#### 后端主链路
1. `src/server/cmd/server/main.go` 加载配置、打开数据库、创建 HTTP server。
2. `src/server/internal/http/router.go` 集中装配 auth、preferences、chat、knowledge、task、console 模块。
3. 每个模块使用 `Service + SQLStore` 组合，直接访问 PostgreSQL。
4. 所有业务接口统一返回 JSON envelope；`/healthz` 为例外。

#### 前端主链路
1. `src/web/src/app/router.tsx` 注册营销页、工作区页、控制台页。
2. 页面通过 `createHttpClient()` 与 `create*Api()` 调用后端。
3. `client.ts` 统一做 envelope 解包。
4. `KnowledgePage.tsx`、`SoloPage.tsx` 等页面直接驱动主业务闭环。

#### 核心数据结构
- Auth / Session：用户、会话、工作区、偏好
- Chat：Conversation、ConversationMessage、ConversationConfig、ModelOption
- Knowledge：KnowledgeBase、KnowledgeDocument、KnowledgeRetrievalResult
- Task：TaskSummary、TaskDetail、TaskStep、TaskEvent、TaskResultArtifact
- Console：UsageSummary、ModelSummary、BillingSummary、AccessSummary

### 5.3 边界说明

- 主核心目录: `src/server`、`src/web/src`
- 次核心目录: `src/web`、`config`、`scripts`、`.github/workflows`、`docs/architecture`
- 排除目录: `src/web/node_modules`、`.git`、`.worktrees`、缓存目录、构建产物、参考仓内部实现
- 核心模块清单: `Auth / Session`、`Preferences`、`Chat`、`Knowledge`、`SOLO Task Runtime`、`Console`、`HTTP 装配层`

### 5.4 当前设计真相

当前设计真相不是“系统还在半成品草稿期”，而是：
- 主线边界已经冻结
- 工作区主功能已经连成闭环
- 但智能能力的深度被明显高估

说白了，**产品骨架是真的，智能内核还很浅；工程边界是真的，历史报告还很脏。**

## 6. 评分口径说明

### 6.1 完成度评分分档标准（0-100%）
- 0-10%：仅占位或概念代码
- 11-30%：局部结构已开始实现，但主流程不可运行
- 31-50%：有部分实现，但关键能力缺失
- 51-70%：主流程基本存在，可在受限条件下工作
- 71-85%：主体完整，常规路径可运行，但工程质量仍有明显缺口
- 86-95%：功能高度完整，仅剩少量非关键缺陷
- 96-100%：设计、实现、测试、文档高度一致

### 6.2 可用性评分分档标准（1-5）
- 1 分：基本不可用
- 2 分：勉强存在部分能力，但无法支撑真实使用
- 3 分：有限场景可用，鲁棒性与维护性不足
- 4 分：可用于生产前阶段或低风险场景
- 5 分：工程质量成熟，可支撑持续演进

### 6.3 Project Health Score 评分明细表

| 评分维度 | 当前评分 | 扣分项 | 评分依据 |
| --- | ---: | --- | --- |
| 架构一致性 | 14/20 | SOLO 与 Knowledge 的对外叙事高于真实实现；router.go 装配集中 | 代码与当前契约文档大体一致，但实现深度被高估 |
| 功能完成度 | 15/20 | 真正的 RAG、真实 agent runtime、深度 console 能力缺失 | 主闭环存在，属于 Beta 收口态 |
| 可用性与稳定性 | 14/20 | 大页面过重、运行语义简化、边界处理能力有限 | 常规路径可跑，但不是强鲁棒系统 |
| 测试与质量保障 | 11/15 | 本次未见完整自动化回归链闭环证据；路由与状态机热点仍集中 | 有测试痕迹与质量脚本，但工程化成熟度不是顶级 |
| 文档与设计可追溯性 | 8/10 | 历史报告大量失真 | 当前契约文档可靠，但旧报告制造噪音 |
| 工程前提完整性 | 4/5 | 参考仓仍占目录注意力 | git、main、HEAD、README、核心源码均齐全 |
| 扫描边界清晰度与核心范围判定可信度 | 5/10 | 参考仓与旧文档仍容易误导新参与者 | 当前可判定，但需要在治理上强制收口 |
| **总分** | **71/100** | **主要扣分来自历史认知污染与能力高估** | **项目已可治理，但还远没到“健康且无需强治理”的程度** |

## 7. 全维度功能审计表

### 7.1 核心模块

| 功能模块 | 模块划分依据 | 原始设计意图 | 当前代码实现现状 | 完成度 (0-100%) | 可用性评分 (1-5) | 审计置信度 | 差异与潜在风险 | 证据链 |
| --- | --- | --- | --- | ---: | ---: | --- | --- | --- |
| Auth / Session | 文档声明 + `/api/v1/auth/*` 路由 + middleware 装配 | 提供注册、登录、登出、会话恢复与工作区进入前提 | 注册/登录/me/logout 已完整挂载，前后端 API 已对接，作为工作区入口成立 | 86%（常规认证闭环已完成，但高级安全能力未见） | 4（主流程可用，但未见更强安全治理） | 高 | 不是草稿，但也远不是成熟 auth 平台；若继续扩 scope，安全债会马上爆 | `src/server/internal/http/router.go`、`src/web/src/features/auth/api.ts`、`src/web/src/features/auth/store.ts`、`docs/architecture/current-system-contracts.md` |
| Preferences | 核心 | `/api/v1/app/me/preferences` + userprefs service/store | 保存 onboarding/defaultMode/modelStrategy 等偏好 | GET/PUT 已接入，session 返回也包含 preferences，前端类型已对齐 | 82%（主体完整，扩展性一般） | 4（低风险场景可用） | 高 | 结构清楚，但仍是轻量偏好层，不是复杂设置中心 | `router.go:68-77`、`types/api.ts:16-38`、`current-system-contracts.md:132-156` |
| Chat | 核心 | 文档声明 + conversations/message/config 路由 + Chat API | 作为主工作区入口，提供会话、消息、配置、知识绑定、转 SOLO 草稿 | 后端接口完整挂载，前端 API 已对齐；但能力重心仍是标准消息链路，不是高级 agent workspace | 78%（主流程完整，但高级能力明显缺失） | 4（可用于 Beta 阶段） | 高 | 若对外宣称 tool runtime / provider orchestration，会构成夸大 | `router.go:85-133`、`src/web/src/features/chat/api.ts`、`src/server/internal/chat/service.go` |
| Knowledge | 核心 | 路由映射 + KnowledgePage + retrieval API + 文档契约 | 提供知识库/文档管理，并形成基础检索闭环 | CRUD 与 retrieval 均已接通，页面非占位；但 retrieval 仍是文本匹配，不是 embedding RAG | 76%（Beta 文本检索成立，但不是完整知识增强系统） | 3（有限场景可用，扩展性和能力上限不足） | 高 | 最大风险是“把文本匹配说成 RAG”，属于典型偷懒式产品叙事膨胀 | `router.go:134-203`、`KnowledgePage.tsx`、`features/knowledge/api.ts`、`current-system-contracts.md:194-215` |
| SOLO Task Runtime | 核心 | task 路由 + runtime.go + SoloPage | 作为 SOLO 工作流入口，支持任务创建、审批、预算、知识源绑定与结果导出 | 整体闭环已经存在，但 runtime 本质是固定状态推进器，不是真实 agent execution engine | 68%（主流程存在，但核心执行深度明显不足） | 3（演示与受限 Beta 可用，真实执行场景不可靠） | 高 | 这里最容易出现“代码看起来很多，实际执行很浅”的草台错觉 | `router.go:204-294`、`task/runtime.go`、`task/service.go`、`SoloPage.tsx`、`features/tasks/api.ts` |
| Console | 核心 | 文档声明 + `/api/v1/console/*` + console service | 为工作区提供使用量、模型、计费、访问上下文的总览 | 路由与 service 都存在，能形成 starter console | 72%（基础摘要完整，但离运营后台很远） | 3（有限场景可用） | 高 | 如果继续堆需求，当前轻量数据模型会很快不够用 | `router.go:295-329`、`console_handler.go`、`console/service.go` |
| HTTP 装配层 | 核心 | `NewRouter` 装配关系 + middleware | 作为统一系统边界，把所有模块接成单一 API 面 | 单文件装配全部主线模块，边界清晰但已明显集中膨胀 | 74%（能工作，但结构已开始失控） | 3（当前可维护，继续增长会恶化） | 高 | `router.go` 是最典型的“先能跑再说”堆叠点，继续放任就是技术债放大器 | `src/server/internal/http/router.go` |
| Config | 次核心 | config.Load + `.env.example` + server 启动依赖 | 提供统一运行时参数装配 | 关键变量已消费，当前契约文档与实现对齐 | 84%（关键配置完整） | 4（运行前提清晰） | 高 | 配置层整体还算规整，是少数不像草台班子的部分 | `src/server/internal/config/config.go`、`config/.env.example` |
| DB / Migration | 次核心 | db.Open + cmd/migrate + SQL store | 使用 PostgreSQL 作为唯一系统记录源 | PostgreSQL 边界清晰，migration 入口存在，各模块 SQL store 直连 DB | 79%（边界稳定，但 migration 机制仍偏简陋） | 4（当前阶段可用） | 高 | 不是灾难，但还没到成熟 schema governance 阶段 | `src/server/internal/db/db.go`、`src/server/cmd/migrate/main.go` |

### 7.2 次核心与支撑模块

| 功能模块 | 模块划分依据 | 原始设计意图 | 当前代码实现现状 | 完成度 (0-100%) | 可用性评分 (1-5) | 审计置信度 | 差异与潜在风险 | 证据链 |
| --- | --- | --- | --- | ---: | ---: | --- | --- | --- |
| Root Scripts / CI | root package.json + scripts + workflow | 保证本地、CI、主线边界一致 | 已形成统一入口，且质量脚本会校验主线边界 | 88%（工程门面成熟度较高） | 4（适合工程化推进） | 高 | 这是项目里最像“被认真治理过”的区域 | `package.json`、`scripts/*.sh`、`.github/workflows/ci.yml` |
| 当前架构契约文档 | `docs/architecture/current-system-contracts.md` | 成为主线唯一执行基线 | 与当前代码高度一致，可直接作为审计依据 | 90%（当前最可信文档） | 4（可用于治理） | 高 | 最大风险不是它不准，而是别人不用它 | `docs/architecture/current-system-contracts.md` |
| 历史报告系统 | `docs/reports/*.md` | 提供阶段性审计与计划材料 | 多份报告已被当前代码覆盖，继续引用会误导 | 35%（历史价值有，但现状价值低） | 2（不能作为当前执行依据） | 高 | 这是认知债，不是功能债；不清理就会持续误导路线图 | `docs/reports/2026-04-04-*.md`、`2026-04-05-*.md` |

## 8. 缺口分析

### 缺口分析汇总表

| 问题类别 | 问题描述 | 影响范围 | 紧急程度 | 建议动作 |
| --- | --- | --- | --- | --- |
| Blocker | 历史报告与当前代码现实冲突，继续引用会污染执行优先级 | 全项目治理、Roadmap、跨人协作 | 高 | 立即废止旧报告的“现状依据”资格 |
| Blocker | SOLO 被包装成 runtime，但真实执行语义明显偏浅 | task、solo、产品承诺 | 高 | 在路线图中明确先做“能力定位”而不是继续模糊叙事 |
| Blocker | Knowledge 当前仅到文本检索 Beta，却容易被误读为 RAG | knowledge、产品定位、后续架构 | 高 | 在文档和路线图中明确能力边界 |
| 设计偏移 | `router.go` 集中装配所有模块，已经出现单点膨胀 | 后端可维护性 | 中高 | 拆分为模块级注册 |
| 设计偏移 | 大页面（尤其 SoloPage / KnowledgePage）承载过多状态与动作 | 前端可维护性 | 中高 | 逐步拆 container / actions / hooks |
| 环境阻塞项 | 无硬性阻塞，但参考仓仍在目录中吸走注意力 | 认知与审计效率 | 中 | 在文档中持续强调非主线角色 |
| 边界风险 | 新成员若忽略 README 和 current-system-contracts，仍可能把参考仓当主线 | 扫描边界与治理一致性 | 中 | 强制把 CURRENT_STATUS.md / ROADMAP.md 设为唯一基线 |

### 8.1 致命缺陷 (Blockers)

严格按“可用性低于 3 分必须立即修复”这一条，当前**没有核心模块低于 3 分**。这说明项目已经脱离“完全失控的草台原型”阶段。

但工程化推进视角下，仍有三个**准 Blocker**：
1. **历史认知污染**：旧报告大量过时，继续引用会直接带偏执行顺序。
2. **SOLO 能力定位失真**：代码量不小，但真实 runtime 深度不足，容易被误判为可扩展执行器。
3. **Knowledge 能力定位失真**：当前只有文本检索 Beta，不应继续借 RAG 话术偷懒。

### 8.2 设计偏移

1. **从“简单装配”滑向“中心化大文件”**
   - `src/server/internal/http/router.go` 已承担全部 API 注册与路径解析。
   - 这是典型的 Vibe Coding 后遗症：先把功能堆起来，之后没有及时做边界回收。

2. **从“页面承载交互”滑向“页面吞掉应用逻辑”**
   - `SoloPage.tsx`、`KnowledgePage.tsx` 承担了大量请求、状态、导出、局部规则逻辑。
   - 这不是灾难，但继续演进就会迅速变成前端维护地狱。

3. **从“Beta 能力”滑向“过度叙事”**
   - SOLO 和 Knowledge 都存在“实现比宣称浅”的问题。
   - 这不是代码 bug，这是架构管理上的偷懒：用更大的词包装更浅的能力。

### 8.3 环境阻塞项

- 无 git / 分支 / commit / 文档 / 核心源码缺失。
- 当前阻塞不是基础设施，而是**执行基线必须统一**。

### 8.4 边界风险

- `new-api` / `lobehub` 仍在仓库中，可导致错误聚焦。
- `docs/reports/*` 大量历史文件仍然存在，极易被误当最新真相。
- 若后续不在 ROADMAP 中明确“唯一基线文档”，则边界风险会持续存在。

## 9. 边界修订记录

本次审计未发生扫描边界变更，边界在初始判定后保持稳定。

### 边界修订记录表

| 修订时间/阶段 | 修订前 | 修订后 | 触发证据 | 影响评估 |
| --- | --- | --- | --- | --- |
| N/A | N/A | N/A | N/A | 本轮无边界修订 |

## 10. 总结与下一步建议

### 10.1 总结

这个项目已经不再是“有没有东西”的问题，而是“有没有把真相说清楚”的问题。

- 主线已经明确：`src/server` + `src/web`
- 核心闭环已经形成：Auth / Chat / Knowledge / SOLO / Console
- 当前最严重的问题不是功能缺失，而是：
  1. 历史文档仍在制造旧真相
  2. SOLO 与 Knowledge 的能力深度被高估
  3. 路由装配与大页面开始暴露 Vibe Coding 的结构后遗症

### 10.2 下一步建议

1. **立即冻结文档权威层级**：CURRENT_STATUS.md 与 ROADMAP.md 成为新的执行基线；旧报告降级为历史材料。
2. **先做工程化对齐，不急着加新功能**：拆装配点、拆大页面、收敛能力边界表述。
3. **把 SOLO 和 Knowledge 的“能力真相”写死到路线图里**：避免再用大词掩盖浅实现。
4. **将 Project Health Score 目标定义为两步走**：先从 71 拉到 78-82（可稳定治理），再评估是否进入更深能力迭代。

## 11. 基线锁定声明

- 当前唯一执行基线文档：`CURRENT_STATUS.md`、`ROADMAP.md`、`docs/architecture/current-system-contracts.md`
- `docs/reports/*` 仅作为历史材料保留，不再作为当前现状、优先级判断或里程碑规划依据
- 后续里程碑默认以 root 验证门面作为 DoD 入口：`bash scripts/check.sh`、`bash scripts/test.sh`
