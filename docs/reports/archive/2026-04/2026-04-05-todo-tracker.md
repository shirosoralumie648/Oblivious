# Historical Material Notice

> 本文件属于历史阶段材料，不再作为当前现状、优先级判断或里程碑规划依据。
> 当前唯一执行基线请改看：`CURRENT_STATUS.md`、`ROADMAP.md`、`docs/architecture/current-system-contracts.md`

# Oblivious TODO 跟踪表

日期：2026-04-05

说明：

- 本表只纳入当前主线 `src/server` + `src/web` + 根工程治理事项。
- `new-api` 与 `lobehub` 的显式 marker 只作为外部参考，不直接并入主线执行清单。
- 工作量为粗估：
  - `S`：0.5-1 天
  - `M`：1-3 天
  - `L`：3-7 天
  - `XL`：1-3 周

## 1. 关键

| ID | 事项 | 范围 | 证据 | 预估 | 依赖 |
| --- | --- | --- | --- | --- | --- |
| K-01 | 明确 root workspace 策略，隔离或正式纳管 `lobehub` / `new-api`，修复 lockfile 漂移 | 根工程 | `pnpm install --frozen-lockfile` 被 `lobehub/package.json` 漂移阻断 | M | 无 |
| K-02 | 实现 `AppProviders`、`useAppContext`、auth bootstrap 和会话状态 | `src/web` | `ProtectedRoute`、`KnowledgePage`、`SoloPage`、测试都依赖该能力，但实现不存在 | L | K-01 |
| K-03 | 补齐 `AuthStore` 能力：`preferences`、`startLoading`、`setAuthenticatedSession`、`subscribe` | `src/web` | `auth/store.test.ts` 3 个测试全部失败，`tsc` 同步报错 | M | K-02 |
| K-04 | 补齐 `types/api.ts` 的主线业务类型 | `src/web` | `tsc` 报 44 个类型错误，核心原因之一是 API 类型缺失 | M | K-02 |
| K-05 | 统一前后端 envelope 契约，在 `HttpClient` 实现解包和错误映射，并补 `put/delete` | `src/web` | 后端统一返回 `{ ok, data, error }`，当前前端直接把 JSON 当业务对象使用 | M | K-04 |
| K-06 | 补齐 `ChatApi` / `ConsoleApi` / `AuthApi` 的真实接口实现 | `src/web` | `SoloPage` 和 console 测试依赖的方法在 API 层不存在 | M | K-05 |
| K-07 | 将 `/knowledge`、`/knowledge/:knowledgeBaseId`、`/solo`、`/solo/new` 正式接入路由，并挂上 `ProtectedRoute` | `src/web` | 页面存在但路由树没有对应入口，`ProtectedRoute` 只定义未使用 | M | K-02, K-05 |
| K-08 | 实现 Chat 页面主路径，接通知识库配置和转 SOLO 流程 | `src/web` | `ChatPage.behavior.test.tsx` 2 个行为测试全部失败，页面仍是标题占位 | L | K-06, K-07 |
| K-09 | 实现 Settings / Onboarding / Console 子页，不再使用纯占位页 | `src/web` | 相关测试共 12 个失败，页面仅有 `<h1>` | L | K-02, K-06, K-07 |
| K-10 | 修复根 `check.sh` 与 `src/web/package.json` 脚本契约不一致的问题 | 根工程 | root `check.sh` 调用 `pnpm --dir src/web check`，但 `src/web` 没有 `check` script | S | K-01 |
| K-11 | 将后端集成测试改造成 CI 友好的临时库 / testcontainers / 自动迁移模式 | `src/server` | `server_test.go` 12 个测试依赖固定本地 PostgreSQL 且当前认证失败 | L | 无 |
| K-12 | 重写或补档 Task 6/7 设计文档，替换已过期的 Task 5 基线 | `docs` | 当前正式设计文档与真实实现阶段严重脱节 | M | 无 |
| K-13 | 统一环境变量契约，改写 `.env.example` 为 `DATABASE_URL` 主模式 | `config` + `docs` | 当前 `.env.example` 提供的是 `POSTGRES_*`，而 `config.Load()` 强依赖 `DATABASE_URL` | M | K-12 |

## 2. 重要

| ID | 事项 | 范围 | 证据 | 预估 | 依赖 |
| --- | --- | --- | --- | --- | --- |
| I-01 | 优化 handler 错误映射：`sql.ErrNoRows`、鉴权失败、校验错误分别返回正确状态码 | `src/server` | 现在多数错误都被映射成 `500 internal_error` | M | K-11 |
| I-02 | 添加真实 CORS 中间件，并清理未消费的配置字段 | `src/server` | `CORSAllowedOrigins` 已加载但未使用；`SessionSecret` / `ModelBaseURL` / `ModelAPIKey` 也未消费 | M | K-13 |
| I-03 | 增加安全加固：CSRF、登录限速、密码策略、会话轮换、审计日志 | `src/server` | 当前仅有 HttpOnly Cookie，生产安全不足 | L | K-13 |
| I-04 | 把 Knowledge 从 CRUD 升级为 ingestion / chunking / indexing / retrieval 基础链路 | `src/server` + `src/web` | 当前知识库只是文档容器，没有检索价值 | XL | K-05, K-07, K-12 |
| I-05 | 把 SOLO 从 starter 状态机升级为真实 agent runtime | `src/server` + `src/web` | `resume` 直接完成任务，仅适合演示 | XL | K-08, K-12 |
| I-06 | 强化 Chat 网关：streaming、provider abstraction、tool runtime、重试/降级 | `src/server` | 当前只有 OpenAI-compatible 单路径和 demo fallback | L | K-08, K-12 |
| I-07 | 拆分 `router.go`，引入模块级路由注册 | `src/server` | 路由文件已 320+ 行，继续扩展维护成本过高 | M | K-11 |
| I-08 | 拆分 `SoloPage.tsx` 和 `KnowledgePage.tsx`，沉淀 hooks / actions / container | `src/web` | 两个页面分别约 661 行和 353 行，已超过页面层合理复杂度 | L | K-08, K-09 |
| I-09 | 为列表和统计接口补分页、过滤和性能基线 | `src/server` | 当前 list/summary 接口无分页，也无 benchmark | L | K-11 |
| I-10 | 为 `usage_records` 等热点表补复合索引与查询评估 | `src/server` | 当前统计查询按 `workspace_id` + 时间窗口过滤，但 migration 只有单列索引 | M | I-09 |

## 3. 一般

| ID | 事项 | 范围 | 证据 | 预估 | 依赖 |
| --- | --- | --- | --- | --- | --- |
| G-01 | 为主线补完整 README、开发指南和架构说明 | 根文档 | 当前根 README 没有提供任何真实上下文 | M | K-12, K-13 |
| G-02 | 为 API 提供共享 schema 或自动生成类型 | `src/server` + `src/web` | 当前手写类型已明显漂移 | M | K-05 |
| G-03 | 补充性能与稳定性观测：日志字段、trace、metrics、slow query | `src/server` | 当前只有基础 access log | L | I-09 |
| G-04 | 将 `cmd/migrate` 升级为可追踪 migration 机制 | `src/server` | 当前只是顺序执行 SQL 文件，没有版本跟踪表 | M | K-11 |
| G-05 | 评估 `new-api` / `lobehub` 的长期关系：参考、镜像还是集成目标 | 根工程/文档 | 当前目录存在但角色不清晰，已影响包管理 | S | K-01, K-12 |

## 4. 推荐执行顺序

1. `K-01` → `K-10`：先恢复 workspace 和脚本的基本可用性  
2. `K-02` → `K-07`：收敛前端状态、API、路由契约  
3. `K-08` → `K-09`：完成工作区主路径 UI  
4. `K-11` → `K-13`：修复后端测试与文档/配置基线  
5. `I-04` → `I-06`：升级产品核心能力  
6. `I-07` 之后处理 `I-08` / `I-09` / `I-10` / `G-*`：工程治理和长期演进

