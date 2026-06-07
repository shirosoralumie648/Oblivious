# Historical Material Notice

> 本文件属于历史阶段材料，不再作为当前现状、优先级判断或里程碑规划依据。
> 当前唯一执行基线请改看：`CURRENT_STATUS.md`、`ROADMAP.md`、`docs/architecture/current-system-contracts.md`

# Oblivious TODO 事项跟踪表

日期：2026-04-04

## 1. 说明

本表只把“当前根主线 `src/server` + `src/web` 需要推进的事项”整理成执行型 backlog。  
`new-api` / `lobehub` 的显式 TODO/FIXME 不直接纳入本表，只在附录中做外部仓说明。

优先级定义：

- `关键`：阻断主链路、编译/运行、契约一致性或设计基线
- `重要`：不阻断主链路，但会显著影响产品能力或工程质量
- `一般`：增强、清理、文档化和长期治理

工作量为粗估：

- `S`：0.5-1 天
- `M`：1-3 天
- `L`：3-7 天
- `XL`：1-3 周

## 2. 主线 backlog

| ID | 优先级 | 事项 | 范围 | 预估工作量 | 依赖 | 证据 / 说明 |
| --- | --- | --- | --- | --- | --- | --- |
| K-01 | 关键 | 修复前端基础编译断裂：补齐 `useAppContext`、`AuthStore` 缺失方法、`types/api.ts`、`HttpClient.put/delete`、Chat API 缺失方法 | `src/web` | L | 无 | 当前页面和测试依赖的接口在实现中不存在，推断前端无法稳定 typecheck/build |
| K-02 | 关键 | 对齐前后端 envelope 契约，在前端统一做 `Envelope` 解包和错误映射 | `src/web` + `src/server` | M | K-01 | 后端统一返回 `{ ok, data, error }`，前端直接把 JSON 当业务对象 |
| K-03 | 关键 | 补齐工作区路由与权限守卫：挂载 `/knowledge`、`/knowledge/:id`、`/solo`、`/solo/new`，并把 `ProtectedRoute` 接入 | `src/web` | M | K-01 | 页面代码已存在但路由未挂载；`ProtectedRoute` 已定义但未接入 router |
| K-04 | 关键 | 让 Chat / Knowledge / SOLO 至少形成一条可跑通的主路径，并清理当前占位页与测试预期之间的断裂 | `src/web` + `src/server` | L | K-01, K-02, K-03 | `ChatPage`、`OnboardingPage`、`SettingsPage` 仍是占位；测试预期明显超前 |
| K-05 | 关键 | 重写/补齐设计文档：更新已过期的 Task 5，补 Task 6/7 当前真实实现说明 | `docs` | M | 无 | 现有 Task 5 文档仍写着“无 DB、无真实 auth、无业务模块” |
| K-06 | 关键 | 统一环境变量与部署契约：`.env.example` 改成 `DATABASE_URL` 主模式，明确 Cookie/LLM/模型配置项实际用法 | `config` + `docs` | M | K-05 | `.env.example` 和 `config.Load()` 当前不一致 |
| K-07 | 关键 | 把后端测试改造成 CI 友好模式：支持 testcontainers、临时库或自动迁移，不再依赖固定本地 PostgreSQL | `src/server` | L | K-06 | `server_test.go` 直接连 `postgres://postgres:postgres@localhost:5432/oblivious` |
| I-01 | 重要 | 把 Knowledge 从 CRUD 升级为真实 RAG 基础能力：文档 ingestion、切片、索引、检索接口 | `src/server` + `src/web` | XL | K-02, K-03, K-05 | 当前页面和文案明确承认 retrieval/indexing/ingestion 尚未落地 |
| I-02 | 重要 | 把 SOLO 从 starter 状态机升级为真实 agent runtime：执行日志、审批点、步骤推进、恢复语义、结果产物 | `src/server` + `src/web` | XL | K-04, K-05 | 当前 `resume` 直接完成任务，`result_summary` 为固定模板 |
| I-03 | 重要 | 强化 Chat 网关：支持 streaming、provider abstraction、重试/降级、tool use/runtime 边界 | `src/server` | L | K-02, K-05 | 当前仅 OpenAI-compatible 单路径，未配置时回退 demo reply |
| I-04 | 重要 | 安全加固：CSRF、登录限流、密码策略、审计日志、会话轮换、生产强制 secure cookie | `src/server` | L | K-06 | 当前已有 HttpOnly Cookie，但未看到这些增强项 |
| I-05 | 重要 | 建立质量基线：coverage、API 基准、热点 SQL 检查、前端关键页面交互回归 | `src/server` + `src/web` | L | K-07 | 目前无正式 coverage/perf harness |
| I-06 | 重要 | 清理未使用配置与潜在死代码：`CORSAllowedOrigins`、`SessionSecret`、`ModelBaseURL`、`ModelAPIKey` 等 | `src/server` | M | K-05, K-06 | 这些配置字段已加载，但未在服务逻辑里被消费 |
| G-01 | 一般 | 拆分前端大页面：把 `SoloPage`、`KnowledgePage` 拆为容器 + 视图 + hooks + actions | `src/web` | L | K-01, K-04 | 当前单页职责过多，后续维护成本高 |
| G-02 | 一般 | 标准化 API 类型生成或共享：避免前后端契约手写漂移 | `src/web` + `src/server` | M | K-02 | 当前 `types/api.ts` 明显滞后于后端接口 |
| G-03 | 一般 | 完善 README / API 示例 / 本地开发指南，明确主线与嵌入上游仓的关系 | 根仓文档 | M | K-05, K-06 | 当前仓库结构容易让新成员误判 `new-api` / `lobehub` 是主线 |
| G-04 | 一般 | 明确 `new-api` / `lobehub` 的管理策略：仅参考、镜像依赖，还是后续集成目标 | 根仓文档/决策 | S | K-05 | 目前只是嵌入目录，根项目未形成代码级集成关系 |

## 3. 推荐执行顺序

### 第一阶段：先恢复主线闭环

1. `K-01` 前端基础编译修复
2. `K-02` envelope 契约统一
3. `K-03` 路由和权限守卫接入
4. `K-04` Chat / Knowledge / SOLO 主路径打通

### 第二阶段：补设计和部署基线

1. `K-05` 设计文档补档
2. `K-06` 环境变量与部署契约统一
3. `K-07` CI 友好测试改造

### 第三阶段：升级产品能力

1. `I-01` Knowledge 增强
2. `I-02` SOLO 真执行
3. `I-03` Chat 网关增强

### 第四阶段：工程治理

1. `I-04` 安全加固
2. `I-05` 质量基线
3. `I-06` 未使用配置清理
4. `G-01` / `G-02` / `G-03` / `G-04`

## 4. 风险依赖关系图

```mermaid
flowchart TD
  K01[K-01 前端基础修复]
  K02[K-02 Envelope 契约统一]
  K03[K-03 路由与权限守卫]
  K04[K-04 主路径打通]
  K05[K-05 设计文档补档]
  K06[K-06 环境变量与部署契约统一]
  K07[K-07 CI 友好测试]
  I01[I-01 Knowledge 增强]
  I02[I-02 SOLO 真执行]
  I03[I-03 Chat 网关增强]
  I04[I-04 安全加固]
  I05[I-05 质量基线]
  I06[I-06 未使用配置清理]

  K01 --> K02 --> K04
  K01 --> K03 --> K04
  K05 --> K06 --> K07
  K04 --> I02
  K02 --> I01
  K02 --> I03
  K05 --> I01
  K05 --> I02
  K05 --> I03
  K06 --> I04
  K07 --> I05
  K05 --> I06
  K06 --> I06
```

## 5. 建议的里程碑定义

| 里程碑 | 完成标准 |
| --- | --- |
| M1 主线可运行 | 前端可构建，登录后能进入 chat/knowledge/solo，基础接口契约一致 |
| M2 文档可依赖 | Task 5/6/7 文档与实际代码一致，部署环境变量文档可直接使用 |
| M3 原型能力升级 | Knowledge 具备检索闭环，SOLO 具备真实执行闭环，Chat 支持流式/增强网关 |
| M4 工程化达标 | CI 可跑、覆盖率可见、性能与安全基线建立 |

