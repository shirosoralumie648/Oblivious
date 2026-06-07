# Historical Material Notice

> 本文件属于历史阶段材料，不再作为当前现状、优先级判断或里程碑规划依据。
> 当前唯一执行基线请改看：`CURRENT_STATUS.md`、`ROADMAP.md`、`docs/architecture/current-system-contracts.md`

# Oblivious 技术审计报告

日期：2026-04-05  
审计范围：`/home/shirosora/code_storage/Oblivious`

## 0. 摘要

当前仓库不是单一应用，而是一个“主线产品 + 两个嵌入式上游仓”的混合工作区：

- 主线交付物是 `src/server` 和 `src/web`。
- `new-api` 与 `lobehub` 是独立的大型上游仓，当前被纳入 root `pnpm-workspace.yaml`，但未接入根脚本主链路。
- 后端 `src/server` 已经超出 `Task 5` 设计文档定义的 scaffold 阶段，真实实现了认证、偏好、聊天、知识库、SOLO 任务和控制台统计。
- 前端 `src/web` 处于“页面与测试目标先行、基础状态层和 API 契约未收敛”的状态，当前可以确认 `vitest` 失败、`tsc` 失败，无法交付。
- 工作区工程层还存在一个额外风险：root `pnpm-lock.yaml` 只覆盖 `src/web`，但 workspace 已包含 `lobehub` 和 `new-api`，导致标准 `pnpm install --frozen-lockfile` 被 `lobehub/package.json` 漂移直接阻断。

结论上，项目不是“后端没做”，而是“后端已有 MVP 业务壳，前端主路径和工程基线尚未闭环，设计文档明显滞后”。

## 1. 审计方法与验证结果

### 1.1 取证方法

- 阅读根级配置、脚本、设计文档和现有分析文档。
- 逐个核对 `src/server` 和 `src/web` 的入口、路由、服务层、存储层和测试。
- 识别 `new-api`、`lobehub` 的真实角色与耦合程度。
- 实际运行当前可行的构建/测试命令，区分“代码失败”和“环境失败”。

### 1.2 本次实际执行过的验证

| 命令 | 结果 | 结论 |
| --- | --- | --- |
| `pnpm --dir src/web test` | 失败，46 个测试中 29 通过、17 失败 | 前端测试目标与实际实现显著脱节 |
| `pnpm --dir src/web build` | 失败，`tsc` 报 44 个类型错误 | 前端类型契约未闭合 |
| `go test ./...` in `src/server` | 大部分包通过，`internal/http/server_test.go` 因本地 PostgreSQL 凭证失败 | 后端单元层可测，集成测试强依赖本地 DB |
| `go test -cover ./internal/chat` | 通过，覆盖率 `40.8%` | 聊天域有可运行单元测试 |
| `go test -cover ./internal/config` | 通过，覆盖率 `87.8%` | 配置加载测试最完整 |
| `go test -cover ./internal/console` | 通过，覆盖率 `18.5%` | 控制台逻辑较薄，覆盖有限 |
| `go test -cover ./internal/knowledge` | 通过，覆盖率 `11.8%` | 知识库测试以服务层 mock 为主 |
| `go test -cover ./internal/task` | 通过，覆盖率 `30.9%` | SOLO 任务状态机有基础覆盖 |
| `go test -cover ./internal/http -run 'Test(ChatHandler|KnowledgeHandler|TaskHandler)'` | 通过，覆盖率 `22.0%` | handler 层可测，但整包仍被 DB 集成测试拖住 |

说明：

- `internal/chat/gateway_test.go` 依赖 `httptest.NewServer` 本地监听；在沙箱下会触发端口权限问题，需要放宽本地 loopback。
- `src/web` 的依赖需要以 `--ignore-workspace --no-lockfile` 方式隔离安装，否则会先被 root workspace 锁文件漂移阻断。

## 2. 仓库结构与子系统角色

### 2.1 根工作区的真实边界

| 路径 | 角色 | 当前状态 | 是否主线 |
| --- | --- | --- | --- |
| `src/server` | Go 后端 | 已实现真实业务壳 | 是 |
| `src/web` | React/Vite 前端 | 路由与页面草稿存在，但主路径未闭环 | 是 |
| `new-api` | 独立 LLM 网关 | 独立 `.git` 仓，未接根脚本 | 否 |
| `lobehub` | 独立大型 Agent Workspace | 独立 `.git` 仓，未接根脚本 | 否 |
| `docs/superpowers/specs` | 设计基线 | 仅 `Task 5` 有正式文档，且已过期 | 是 |

### 2.2 根脚本与工作区编排

- 根 `package.json` 只编排 `src/web` 和 `src/server`。
- `src/server` 是独立 Go module，不在 `pnpm` workspace 内。
- `pnpm-workspace.yaml` 却把 `src/web`、`lobehub`、`new-api` 都纳入了 workspace。
- root `pnpm-lock.yaml` 只存在 `src/web` importer，没有 `lobehub` / `new-api` importer。

这意味着：

1. 工程主链路只依赖 `src/web` 和 `src/server`。
2. workspace 级安装行为却会被 `lobehub` / `new-api` 的 package 元数据影响。
3. “主线产品”与“参考仓”在包管理层没有被明确隔离。

## 3. 当前已实现模块与能力边界

### 3.1 后端 `src/server`

| 模块 | 已实现能力 | 能力边界 |
| --- | --- | --- |
| 启动与配置 | `cmd/server` 启动、优雅退出、环境变量加载、PostgreSQL 打开 | 无连接池调优、无配置版本化 |
| 数据迁移 | `cmd/migrate` 顺序执行 SQL migration | 无 migration table、无回滚、依赖执行目录 |
| HTTP 基础设施 | `ServeMux`、panic recover、request id、访问日志、统一 envelope | 路由集中在单文件，未使用子路由/路由声明式注册 |
| 认证与会话 | 注册、登录、登出、`me`、服务端 session + HttpOnly Cookie | 无 CSRF、无限流、无密码策略、`SESSION_SECRET` 未实际参与签名 |
| 用户偏好 | `defaultMode`、`modelStrategy`、`networkEnabledHint`、`onboardingCompleted` 读写 | 仅用户级偏好，无更细粒度工作区策略 |
| 聊天 | 会话 CRUD 子集、消息发送、会话配置、模型列表、转 SOLO 草稿 | 无流式输出、无工具执行、无 provider abstraction |
| 模型网关 | OpenAI-compatible `/chat/completions` 调用 | 未配置时直接回退 demo reply |
| 知识库 | knowledge base / document CRUD、会话绑定知识库 | 没有 ingestion、切片、索引、检索、召回 |
| SOLO 任务 | task CRUD 子集、启动/审批/暂停/恢复/取消/预算修改 | 仅 starter 状态机，不是真实 agent runtime |
| 控制台 | usage / access / models / billing 汇总 | 统计维度简单，未形成运营控制台 |
| 数据持久化 | 手写 SQL + PostgreSQL schema | 无 repository 统一抽象，无分页策略 |

### 3.2 前端 `src/web`

| 模块 | 已实现能力 | 能力边界 |
| --- | --- | --- |
| 基础框架 | React 18、Vite、React Router、Vitest | 只完成基础搭建 |
| 营销页 | `/`、`/login`、`/register` 路由存在 | 页面极简 |
| 工作区布局 | `WorkspaceLayout` 与测试存在 | 缺导航链接、未接 `ProtectedRoute` |
| Console 布局 | `/console` 子路由存在 | 布局和页面均为占位 |
| Chat 页面 | 路由已挂载 | 页面仍只有 `<h1>Chat</h1>` |
| Knowledge 页面 | 页面逻辑较完整，测试通过 | 路由未挂载，依赖缺失的 `useAppContext` / API 类型 / `put/delete` |
| SOLO 页面 | 页面逻辑最完整，测试通过 | 路由未挂载，桥接依赖的 Chat API 实际不存在 |
| Settings / Onboarding | 只有标题占位 | 测试全部失败 |
| 全局状态 | `createAppStore`、`createAuthStore` 雏形存在 | `AppProviders` 为空，`useAppContext` 未实现，auth store 形状与测试不一致 |
| API 层 | `chat/knowledge/tasks/console/auth` API 文件存在 | 类型、方法和后端真实接口不一致 |

### 3.3 嵌入仓简表

| 子仓 | 体量 | 角色 | 当前对根项目的影响 |
| --- | ---: | --- | --- |
| `new-api` | 约 192k 代码 LOC | 独立 Go LLM Gateway | 作为 workspace 成员存在，但未被主线调用 |
| `lobehub` | 约 642k 代码 LOC，约 380k 测试 LOC | 独立大型 Agent Workspace | 作为 workspace 成员存在，并造成 lockfile 漂移 |

## 4. 当前系统架构

### 4.1 主线架构图

```mermaid
flowchart LR
  Browser[Browser]
  Router[React Router]
  Marketing[Marketing Routes]
  Workspace[Workspace Routes]
  Console[Console Routes]
  HttpClient[createHttpClient]
  GoAPI[Go HTTP Server]
  Middleware[recover/request-id/logging/auth]

  AuthH[Auth Handler]
  PrefH[Preferences Handler]
  ChatH[Chat Handler]
  KnowledgeH[Knowledge Handler]
  TaskH[Task Handler]
  ConsoleH[Console Handler]

  AuthS[Auth Service]
  PrefS[Preferences Service]
  ChatS[Chat Service]
  KnowledgeS[Knowledge Service]
  TaskS[Task Service]
  ConsoleS[Console Service]
  UsageS[Usage Recorder]

  SQL[(PostgreSQL)]
  LLM[OpenAI-compatible Gateway\nor demo fallback]

  Browser --> Router
  Router --> Marketing
  Router --> Workspace
  Router --> Console
  Workspace --> HttpClient
  Console --> HttpClient
  HttpClient --> GoAPI
  GoAPI --> Middleware

  Middleware --> AuthH --> AuthS --> SQL
  Middleware --> PrefH --> PrefS --> SQL
  Middleware --> ChatH --> ChatS --> SQL
  ChatS --> UsageS --> SQL
  ChatS --> LLM
  Middleware --> KnowledgeH --> KnowledgeS --> SQL
  Middleware --> TaskH --> TaskS --> SQL
  Middleware --> ConsoleH --> ConsoleS --> SQL
```

### 4.2 工作区整体图

```mermaid
flowchart TD
  Root[Root workspace]
  Mainline[Mainline delivery]
  References[Embedded upstream repos]

  Root --> Mainline
  Root --> References

  Mainline --> Web[src/web]
  Mainline --> Server[src/server]
  Mainline --> Docs[docs/superpowers/specs]

  References --> NewAPI[new-api]
  References --> LobeHub[lobehub]

  Root -. package scripts .-> Web
  Root -. package scripts .-> Server
  Root -. pnpm workspace .-> Web
  Root -. pnpm workspace .-> NewAPI
  Root -. pnpm workspace .-> LobeHub
```

### 4.3 关键交互流

#### 认证

1. 前端调用 `/api/v1/auth/register` 或 `/api/v1/auth/login`。
2. `authHandler` 解码凭据并调用 `auth.Service`。
3. `auth.SQLStore` 写入 `users/workspaces/sessions`。
4. 中间件写 HttpOnly Cookie。
5. 后续受保护路由通过 Cookie 解析会话并注入 request context。

#### 聊天

1. 前端调用 `conversations` / `messages` / `config`。
2. `chat.Service` 读取消息历史与会话配置。
3. `HTTPReplyGenerator` 走 OpenAI-compatible `/chat/completions`。
4. 若未配置模型网关，则回退为 `Assistant reply: <latest user message>`。
5. assistant reply 被落库，同时记录 usage。

#### 知识库

1. 前端维护知识库和文档的 CRUD。
2. 后端只做结构化持久化和会话/任务绑定。
3. 检索增强、索引和召回当前都不存在。

#### SOLO

1. 前端创建任务时提交目标、预算、执行模式、授权范围、工具白/黑名单和知识库绑定。
2. 后端按执行模式切换到 `running` 或 `awaiting_confirmation`。
3. `approve/pause/resume/cancel` 更新 starter 状态机。
4. `resume` 当前会直接把任务置为 `completed` 并生成固定模板 `result_summary`。

## 5. 技术栈选型与实现细节

| 层 | 选型 | 选型理由 | 当前实现特点 |
| --- | --- | --- | --- |
| 后端框架 | Go 1.22 + `net/http` | 依赖少、可控、适合快速搭骨架 | 全部手写 `ServeMux` 和 middleware |
| 数据库 | PostgreSQL + `database/sql` + `lib/pq` | SQL 能力完整、事务和数组类型适合任务/工具边界 | 完全手写 SQL，未抽象 ORM |
| 鉴权 | 服务端 session + Cookie | 最小可用闭环，便于前端直接复用会话 | Cookie 是 HttpOnly，但 `SESSION_SECRET` 未使用 |
| 前端框架 | React 18 + React Router 6 + Vite | 起步轻量，测试与路由配置直接 | 当前更像原型仓，状态层未落地 |
| 测试 | Go `testing`、Vitest、Testing Library | 原生工具链，快速覆盖单元与组件 | 前端测试目标超过实际实现，后端集成测试绑死本地 Postgres |
| 包管理 | root `pnpm` workspace | 统一管理 TS 子项目 | 工作区边界不清，嵌入仓干扰主线 install |

补充观察：

- `CORS_ALLOWED_ORIGINS`、`SESSION_SECRET`、`MODEL_BASE_URL`、`MODEL_API_KEY` 在配置层已声明，但未被服务真正消费。
- `scripts/check.sh` 假定 `src/web` 存在 `check` script，但 `src/web/package.json` 只有 `dev/build/test`，因此根校验脚本本身不可用。

## 6. 代码质量、复杂度与覆盖率

### 6.1 体量指标

| 范围 | 代码 LOC | 测试 LOC | 测试/代码比 | 近似分支点 | 近似分支点 / KLOC |
| --- | ---: | ---: | ---: | ---: | ---: |
| `src/server` | 4467 | 2894 | 0.65 | 794 | 177.7 |
| `src/web` | 1581 | 1917 | 1.21 | 224 | 141.7 |
| `new-api` | 192451 | 6670 | 0.03 | 24307 | 126.3 |
| `lobehub` | 642411 | 380256 | 0.59 | 90128 | 140.3 |

说明：

- 分支点是基于关键字的近似指标，不等同于严格 cyclomatic complexity。
- 主线仓里最需要关注的不是总体规模，而是“少量核心文件承担过多责任”。

### 6.2 主线热点文件

| 文件 | 行数 | 观察 |
| --- | ---: | --- |
| `src/server/internal/task/store.go` | 588 | SOLO 状态机和 SQL 全堆在一个文件 |
| `src/server/internal/http/router.go` | 321 | 路由注册和路径解析集中，已接近维护上限 |
| `src/server/internal/chat/service.go` | 314 | 配置合并、usage 记录、生成回复都在一处 |
| `src/web/src/routes/workspace/SoloPage.tsx` | 661 | 页面、状态、动作、导出逻辑全部混在单组件 |
| `src/web/src/routes/workspace/KnowledgePage.tsx` | 353 | CRUD、编辑状态和导航全在页面层 |

### 6.3 覆盖率现状

| 包/层 | 当前结果 | 解读 |
| --- | --- | --- |
| `internal/config` | `87.8%` | 默认值和错误分支覆盖最好 |
| `internal/chat` | `40.8%` | 服务层和网关基础逻辑有覆盖 |
| `internal/task` | `30.9%` | 状态流转有单测，但仍偏原型 |
| `internal/http`（定向 handler） | `22.0%` | handler 层可测，但整包被 DB 集成测试拖累 |
| `internal/console` | `18.5%` | 统计逻辑简单，覆盖有限 |
| `internal/knowledge` | `11.8%` | 以服务层 mock 为主，store 细节覆盖不足 |

### 6.4 可运行性判断

#### 后端

- `go test ./...` 能证明大部分业务包可以运行。
- `internal/http/server_test.go` 中 12 个测试失败，原因是测试把 DSN 固定写死为 `postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable`，当前环境下出现 `password authentication failed for user "postgres"`。
- 这意味着后端最大质量问题不是代码完全失效，而是集成测试环境不可复现。

#### 前端

- `vitest` 共有 46 个测试，其中 29 个通过、17 个失败。
- 失败集中在 `auth store`、`Console*Page`、`WorkspaceLayout`、`ConsoleLayout`、`ChatPage.behavior`、`OnboardingPage`、`SettingsPage`。
- `tsc --noEmit` 报 44 个类型错误，错误集中在：
  - `useAppContext` 未实现
  - `AuthStore` 缺少 `startLoading` / `setAuthenticatedSession` / `subscribe`
  - `types/api.ts` 缺少大量业务类型
  - `HttpClient` 缺少 `put/delete`
  - `ChatApi` 缺少 `createConversation` / `getConversationConfig` / `updateConversationConfig` / `sendMessage`

## 7. 实现方式深度解析

### 7.1 Auth / Session

实现路径：

1. `auth.Service` 对密码做 `bcrypt` 哈希。
2. `auth.SQLStore` 在注册时事务性创建 `user + workspace + session`。
3. 登录时按 email 查询用户，`bcrypt.CompareHashAndPassword` 校验后新建 session。
4. `authMiddleware` 直接从 Cookie 取 session id，查库命中后放入 context。

优点：

- 结构简单，MVP 闭环完整。
- 使用服务端 session，便于后续扩展审计和失效控制。

边界与问题：

- `SESSION_SECRET` 被要求配置，但没有任何签名用途。
- 未做会话轮换、限速、CSRF、防爆破、邮箱/密码格式校验。
- `sql.ErrNoRows` 等领域错误多数被 handler 统一映射为 `500`，语义偏粗。

### 7.2 Chat / Gateway

实现路径：

1. `chat.Service.SendMessage` 先写 user message。
2. 再读取消息列表与 conversation config。
3. `mergeConversationConfig` 合并覆盖项。
4. `HTTPReplyGenerator` 将消息转成 OpenAI-compatible payload。
5. 成功后写 assistant message，并通过 `usage.SQLRecorder` 记录 token 估算。

设计特点：

- `ConversationConfig` 把模型、system prompt、temperature、token 上限、tools 开关和 knowledge binding 统一到一起。
- token 统计是近似估算，不是模型真实 usage。

当前问题：

- 未配置网关时直接回退 demo reply，适合演示，不适合生产。
- `toolsEnabled` 只是写一条 system message，不是真正的工具调度。
- 没有 streaming，也没有 provider fallback。

### 7.3 Knowledge

实现路径：

1. `knowledge.Service` 只包一层 workspace 作用域。
2. `knowledge.SQLStore` 直接操作 `knowledge_bases` / `knowledge_documents`。
3. 会话和任务通过绑定表引用知识库。

设计原理：

- 先落“结构化知识容器”，后续再叠加 ingestion / retrieval。

当前问题：

- 没有检索、没有索引、没有 chunking，知识库目前只是文档盒子。
- 列表接口没有分页。
- `CreateKnowledgeDocument` 没有检查 `INSERT ... SELECT` 是否真正插入行，错误语义偏松。

### 7.4 SOLO Task

实现路径：

1. `task.Service.Create` 规范化目标、预算、执行模式、知识库和工具规则。
2. `task.SQLStore.StartTask` 生成 starter plan step。
3. `safe` 模式进入 `awaiting_confirmation`，其余进入 `running`。
4. `resume` 会把暂停任务直接标记为 `completed`，并生成固定文案结果。

设计原理：

- 当前不是执行器，而是“有界流程原型”。
- 模式和授权范围先以数据模型落库，为真实 agent runtime 预留接口。

当前问题：

- `resume` 直接完成任务，无法代表真实恢复执行。
- 没有执行日志、步骤产物、审批点上下文、工具调用记录。
- `toolAllowList` / `toolDenyList` 是良好的数据边界，但没有真正执行层消费它们。

### 7.5 前端数据流与状态管理

当前状态：

- `AppProviders` 为空。
- 页面通过 `createHttpClient()` 即席创建 API client。
- `KnowledgePage` / `SoloPage` 各自维护本地状态机。
- `ProtectedRoute` 定义了理想的会话门禁和 onboarding 跳转，但没有进入路由树。

问题本质：

1. 全局状态没有真正落地。
2. API 类型没有和后端 contract 对齐。
3. 页面层承担了过多副作用、导航和转换逻辑。

### 7.6 可扩展性与可维护性评估

#### 后端

优点：

- handler / service / store 的基础分层是成立的。
- 模块边界已经按业务域拆开，继续扩展比从零重构容易。

限制：

- 路由集中在一个文件。
- store 层完全手写 SQL，长期需要统一 query 约定和错误语义。
- 缺少分页、观测性、安全中间件和 DB 集成测试基线。

结论：

- 后端具备“继续演进”的基础，但已到需要补工程纪律的阶段。

#### 前端

优点：

- 测试已经把目标行为写出来，需求轮廓清晰。
- SOLO / Knowledge 页面原型比当前路由树成熟得多。

限制：

- 目标态和实现态明显错位。
- 页面文件过大，缺少容器/状态/hooks 拆分。
- API/类型/状态三层没有统一 contract。

结论：

- 前端当前可维护性偏低，扩功能之前必须先做契约收敛。

## 8. 显式标记、未实现项与隐性需求

### 8.1 显式 TODO / FIXME / TBD

当前重扫结果（过滤 `context.TODO()` 和历史报告自引用后）：

| 范围 | TODO | FIXME | TBD | 说明 |
| --- | ---: | ---: | ---: | --- |
| `src/server` | 0 | 0 | 0 | 主线后端无显式 marker |
| `src/web` | 0 | 0 | 0 | 主线前端无显式 marker |
| `docs/superpowers/specs` | 0 | 0 | 0 | 设计文档的问题主要是过期，不是 marker |
| `new-api` | 123 | 1 | 0 | 主要集中在 provider adaptor 的 `implement me` |
| `lobehub` | 112 | 5 | 1 | 分散在 agent runtime、memory、discover、UI 和 types |

高密度文件簇：

- `new-api/relay/channel/cohere/adaptor.go`：7 个 marker
- `new-api/relay/channel/dify|mistral|mokaai|palm|tencent|xunfei|zhipu/adaptor.go`：各 6 个
- `lobehub/src/server/services/memory/userMemory/extract.ts`：5 个
- `lobehub/src/server/services/discover/index.ts`：4 个

结论：

- 主线项目当前最大问题不在注释 marker，而在“已写出来的目标代码未接上”。
- 外部嵌入仓保留了大量显式待办，但它们不应直接等同于根主线 backlog。

### 8.2 设计中声明但未实现或未收敛的能力

| 能力 | 现状 | 证据 |
| --- | --- | --- |
| AppContext / 会话 bootstrap | 未实现 | `AppProviders` 为空，但页面和测试大量依赖 `useAppContext` |
| 前端 API 类型层 | 未收敛 | `types/api.ts` 只定义了极少类型 |
| Chat 前端能力 | 未实现 | `ChatPage` 仍是标题，占位明显 |
| Console 页面 | 未实现 | 页面只有 `<h1>`，但测试要求完整 dashboard |
| Settings / Onboarding | 未实现 | 页面只有 `<h1>`，测试要求可交互表单 |
| Knowledge / SOLO 路由 | 未接入 | 页面存在，但 `router.tsx` 未挂载 `/knowledge` 和 `/solo` |
| 真实 RAG | 未实现 | 只有 CRUD 和绑定，没有检索链路 |
| 真实 agent runtime | 未实现 | 只有 starter 状态机，没有执行器 |

### 8.3 临时方案与待重构点

| 位置 | 当前做法 | 后续动作 |
| --- | --- | --- |
| `chat/gateway.go` | 无网关配置时回退 demo reply | 接入真实 provider 抽象和 streaming |
| `task/store.go` | `resume` 直接置 `completed` | 替换为真实 runtime 恢复语义 |
| `router.go` | 手动字符串切路径 | 拆为子路由注册或统一路由表 |
| `KnowledgePage.tsx` / `SoloPage.tsx` | 单文件聚合大量状态与动作 | 拆成 container + hooks + actions |
| root workspace | 参考仓与主线共用 workspace | 明确隔离策略 |

### 8.4 隐性需求

#### 性能

- 列表和统计接口缺分页、缺基准测试、缺 composite index 验证。
- `usage_records` 查询只建了单列索引，未见按 `workspace_id + created_at` 的复合索引。
- 前端没有懒加载、没有大页面拆分、没有数据缓存策略。

#### 安全

- 缺 CORS 中间件。
- 缺 CSRF、防爆破、登录限速、审计日志、密码策略。
- `SESSION_SECRET` 配置已要求但未用，说明安全边界设计未完成。

#### 文档

- 设计文档只到 Task 5，且内容已与实现脱节。
- 根 README 不能解释当前主线与嵌入仓关系。
- 环境变量文档与配置实现不一致。

## 9. 当前实现与设计文档差异

对比基线：`docs/superpowers/specs/2026-04-01-task5-go-backend-infrastructure-design.md`

| 设计文档声明 | 当前实现 | 差异结论 | 风险 |
| --- | --- | --- | --- |
| Task 5 不连接 PostgreSQL | 已有 `db.Open`、migrations、真实 schema | 文档严重过期 | 新成员会错误评估后端成熟度 |
| auth 仅 placeholder | 已有注册/登录/会话/me/logout | 文档与真实接口断层 | 前后端验收口径失真 |
| 不引入业务模块 | 已有 auth/chat/knowledge/task/console/userprefs/usage | 实现早已进入 Task 6/7 范围 | 架构基线缺失 |
| 中间件只做 recover/request-id/logging | 实际还有 auth middleware | 安全模型已变化 | 文档不能指导真实接入 |
| 最小 HTTP surface | 实际接口已扩到 app/console/tasks/knowledge | 接口文档缺位 | 前端 contract 漂移 |
| 测试不耦合 DB | `server_test.go` 硬连本地 Postgres | 与文档相反 | CI 难以复现 |

## 10. 后续开发建议路线图

### Phase 0：先恢复工程可运行性

1. 把 `lobehub` / `new-api` 从 root workspace 隔离，或补齐 lockfile 归档策略。
2. 修正 root `check.sh` 与 `src/web/package.json` 的脚本契约。
3. 把 `src/server` 测试改成 CI 友好模式，不再硬连固定本地 Postgres。

### Phase 1：收敛前端主路径

1. 实现 `AppProviders` / `useAppContext` / auth bootstrap。
2. 补齐 `types/api.ts`、`HttpClient.put/delete`、envelope 解包。
3. 补齐 `ChatApi`、`ConsoleApi`、`AuthApi` 的真实 contract。
4. 把 `/knowledge`、`/knowledge/:knowledgeBaseId`、`/solo`、`/solo/new` 接入 router。
5. 让 `ProtectedRoute` 真正进入路由树。

### Phase 2：完成工作区 MVP

1. 实现 Chat 页面。
2. 实现 Settings / Onboarding 页面。
3. 实现 Console dashboard 与各子页。
4. 完成 Chat → SOLO → Chat 的闭环。

### Phase 3：升级核心能力

1. Knowledge CRUD 升级为 ingestion / chunking / retrieval。
2. SOLO starter 升级为真实 agent runtime。
3. Chat 网关升级为 streaming + provider abstraction + tool runtime。

### Phase 4：工程化与安全加固

1. 安全增强：CSRF、限流、密码策略、审计。
2. 性能基线：benchmark、SQL 热点、分页、缓存。
3. 文档补档：Task 6/7 设计、API 文档、部署手册。

## 11. 主线接口文档

### 11.1 健康检查

| Method | Path | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/healthz` | 否 | 返回 `{ "status": "ok" }` |

### 11.2 Auth

| Method | Path | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/auth/register` | 否 | 注册用户、默认工作区与会话 |
| `POST` | `/api/v1/auth/login` | 否 | 登录并创建会话 |
| `GET` | `/api/v1/auth/me` | 是 | 返回用户、工作区、偏好、session |
| `POST` | `/api/v1/auth/logout` | 是 | 删除会话并清除 Cookie |

### 11.3 Preferences / Models

| Method | Path | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/app/me/preferences` | 是 | 读取用户偏好 |
| `PUT` | `/api/v1/app/me/preferences` | 是 | 更新用户偏好 |
| `GET` | `/api/v1/app/models` | 是 | 返回模型列表 |

### 11.4 Chat

| Method | Path | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/app/conversations` | 是 | 会话列表 |
| `POST` | `/api/v1/app/conversations` | 是 | 创建会话 |
| `GET` | `/api/v1/app/conversations/{id}/messages` | 是 | 消息列表 |
| `POST` | `/api/v1/app/conversations/{id}/messages` | 是 | 发送消息并生成回复 |
| `GET` | `/api/v1/app/conversations/{id}/config` | 是 | 读取会话配置 |
| `PUT` | `/api/v1/app/conversations/{id}/config` | 是 | 更新会话配置 |
| `POST` | `/api/v1/app/conversations/{id}/convert-to-task` | 是 | 将会话转换为 SOLO 草稿 |

### 11.5 Knowledge

| Method | Path | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/app/knowledge-bases` | 是 | 知识库列表 |
| `POST` | `/api/v1/app/knowledge-bases` | 是 | 创建知识库 |
| `GET` | `/api/v1/app/knowledge-bases/{id}` | 是 | 获取知识库详情 |
| `PUT` | `/api/v1/app/knowledge-bases/{id}` | 是 | 更新知识库 |
| `DELETE` | `/api/v1/app/knowledge-bases/{id}` | 是 | 删除知识库 |
| `GET` | `/api/v1/app/knowledge-bases/{id}/documents` | 是 | 文档列表 |
| `POST` | `/api/v1/app/knowledge-bases/{id}/documents` | 是 | 创建文档 |
| `PUT` | `/api/v1/app/knowledge-bases/{id}/documents/{docId}` | 是 | 更新文档 |
| `DELETE` | `/api/v1/app/knowledge-bases/{id}/documents/{docId}` | 是 | 删除文档 |

### 11.6 SOLO Tasks

| Method | Path | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/app/tasks` | 是 | 任务列表 |
| `POST` | `/api/v1/app/tasks` | 是 | 创建任务 |
| `GET` | `/api/v1/app/tasks/{id}` | 是 | 任务详情 |
| `POST` | `/api/v1/app/tasks/{id}/start` | 是 | 启动任务 |
| `POST` | `/api/v1/app/tasks/{id}/approve` | 是 | 审批继续执行 |
| `POST` | `/api/v1/app/tasks/{id}/pause` | 是 | 暂停 |
| `POST` | `/api/v1/app/tasks/{id}/resume` | 是 | 恢复 |
| `POST` | `/api/v1/app/tasks/{id}/cancel` | 是 | 取消 |
| `POST` | `/api/v1/app/tasks/{id}/budget` | 是 | 更新预算 |

### 11.7 Console

| Method | Path | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/console/usage` | 是 | 7 天请求数汇总 |
| `GET` | `/api/v1/console/access` | 是 | 当前会话与偏好摘要 |
| `GET` | `/api/v1/console/models` | 是 | 7 天模型请求汇总 |
| `GET` | `/api/v1/console/billing` | 是 | 30 天 token 与成本估算 |

