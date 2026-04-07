# Current System Contracts

日期：2026-04-06

本文件是当前主线系统 `src/server` + `src/web` 的执行基线。

- 主线交付范围：`src/server`、`src/web`
- 非主线参考仓：`new-api`、`lobehub`
- 历史设计参考：`docs/superpowers/specs/2026-04-01-task5-go-backend-infrastructure-design.md`
- 当前执行评估：`docs/reports/2026-04-06-execution-progress-review.md`

## 1. Scope

当前系统已经不再是 Task 5 中定义的 scaffold 阶段，而是：

- 后端已有真实业务壳
- 前端存在营销页、工作区页、控制台页骨架
- 前后端契约和前端状态层仍未完全收敛

本文件只记录“当前代码已经实现或明确依赖”的契约，不描述未来能力设计。

## 2. Mainline Boundaries

```text
Browser
  -> src/web (React + React Router + Vite)
  -> /api/*
  -> src/server (Go net/http + PostgreSQL)
  -> PostgreSQL
```

边界说明：

- `src/web` 是唯一主线前端。
- `src/server` 是唯一主线后端。
- `config`、`scripts` 和 `.github/workflows` 属于主线执行基线。
- `new-api` 与 `lobehub` 当前不属于 root workspace、root CI 或 root 交付链路的一部分。

## 3. HTTP Envelope

后端统一返回 JSON envelope：

### Success

```json
{
  "ok": true,
  "data": {},
  "error": null
}
```

### Failure

```json
{
  "ok": false,
  "data": null,
  "error": {
    "code": "invalid_request",
    "message": "invalid json body"
  }
}
```

当前常见错误码：

- `invalid_request`
- `invalid_credentials`
- `unauthorized`
- `method_not_allowed`
- `not_found`
- `internal_error`

## 4. Auth And Session Contract

### 4.1 Frontend Auth State

前端当前设计意图中的 auth 状态机：

- `idle`
- `authenticated`
- `unauthenticated`

该状态机是 `AuthStore`、`useAuthBootstrap`、`ProtectedRoute` 和未来 `useAppContext` 的共享契约基础。

### 4.2 Session Cookie

服务端会话通过 HttpOnly Cookie 维持，当前行为来自 `auth_middleware.go`：

- cookie name: `SESSION_COOKIE_NAME`，默认 `oblivious_session`
- path: `/`
- `HttpOnly: true`
- `SameSite: Lax`
- `Secure: SESSION_COOKIE_SECURE`
- cookie value: 当前保存签名后的 session token，而不是裸 session id

### 4.3 Session Response Shape

`POST /api/v1/auth/register`
`POST /api/v1/auth/login`
`GET /api/v1/auth/me`

成功时均返回：

```json
{
  "ok": true,
  "data": {
    "onboardingCompleted": false,
    "preferences": {
      "defaultMode": "chat",
      "modelStrategy": "balanced",
      "networkEnabledHint": false,
      "onboardingCompleted": false
    },
    "session": {
      "id": "session_x",
      "expiresAt": "2026-04-06T00:00:00Z"
    },
    "user": {
      "id": "user_x",
      "email": "user@example.com"
    },
    "workspace": {
      "id": "workspace_x"
    }
  },
  "error": null
}
```

## 5. Preferences Contract

当前偏好模型：

```json
{
  "defaultMode": "chat",
  "modelStrategy": "balanced",
  "networkEnabledHint": false,
  "onboardingCompleted": false
}
```

字段含义：

- `defaultMode`: 当前支持的默认进入模式，现有代码默认值为 `chat`
- `modelStrategy`: 当前默认值为 `balanced`
- `networkEnabledHint`: 前端用于表达是否启用联网建议
- `onboardingCompleted`: 首次引导是否已完成

默认值来源：

- `userprefs/store.go`
- `userprefs/service.go`

## 6. Backend Route Matrix

### 6.1 Public

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/healthz` | 健康检查 |
| `POST` | `/api/v1/auth/register` | 注册并建立会话 |
| `POST` | `/api/v1/auth/login` | 登录并建立会话 |

### 6.2 Authenticated Auth

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/auth/me` | 返回当前会话用户、工作区与偏好 |
| `POST` | `/api/v1/auth/logout` | 注销当前会话 |

### 6.3 Preferences And Models

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/me/preferences` | 获取当前用户偏好 |
| `PUT` | `/api/v1/app/me/preferences` | 更新当前用户偏好 |
| `GET` | `/api/v1/app/models` | 返回可选模型列表 |

### 6.4 Chat

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/conversations` | 列出会话 |
| `POST` | `/api/v1/app/conversations` | 创建会话 |
| `GET` | `/api/v1/app/conversations/{conversationId}/messages` | 列出消息 |
| `POST` | `/api/v1/app/conversations/{conversationId}/messages` | 发送消息 |
| `GET` | `/api/v1/app/conversations/{conversationId}/config` | 获取会话配置 |
| `PUT` | `/api/v1/app/conversations/{conversationId}/config` | 更新会话配置 |
| `POST` | `/api/v1/app/conversations/{conversationId}/convert-to-task` | 将会话转换为 SOLO 任务草稿 |

### 6.5 Knowledge

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/knowledge-bases` | 列出知识库 |
| `POST` | `/api/v1/app/knowledge-bases` | 创建知识库 |
| `GET` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}` | 获取知识库详情 |
| `PUT` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}` | 更新知识库 |
| `DELETE` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}` | 删除知识库 |
| `GET` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents` | 列出文档 |
| `POST` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents` | 创建文档 |
| `POST` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve` | 基于 query 检索相关文档片段 |
| `PUT` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}` | 更新文档 |
| `DELETE` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}` | 删除文档 |

说明：

- 当前支持知识库/文档 CRUD
- 当前在文档创建与更新时做最小 chunking
- 当前 retrieval 已进入 Knowledge Beta：维持现有 `/retrieve` 接口 shape，但结果排序、snippet 质量、空结果反馈和页面回归均按 Beta 标准收口
- 当前 retrieval 仍基于文本匹配，不包含向量检索、embedding 或异步 ingestion pipeline

### 6.6 SOLO Tasks

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/tasks` | 列出任务 |
| `POST` | `/api/v1/app/tasks` | 创建任务 |
| `GET` | `/api/v1/app/tasks/{taskId}` | 获取任务详情 |
| `POST` | `/api/v1/app/tasks/{taskId}/start` | 启动任务 |
| `POST` | `/api/v1/app/tasks/{taskId}/approve` | 审批任务 |
| `POST` | `/api/v1/app/tasks/{taskId}/pause` | 暂停任务 |
| `POST` | `/api/v1/app/tasks/{taskId}/resume` | 恢复任务 |
| `POST` | `/api/v1/app/tasks/{taskId}/cancel` | 取消任务 |
| `POST` | `/api/v1/app/tasks/{taskId}/budget` | 更新预算 |

说明：

- 当前支持 `draft`、`awaiting_confirmation`、`running`、`paused`、`completed`、`cancelled`
- 当前任务详情包含结构化步骤、`currentStep`、执行事件和结果 artifacts
- 当前是受限 runtime MVP，不是完整多 agent orchestration

### 6.7 Console

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/console/usage` | 使用量汇总 |
| `GET` | `/api/v1/console/access` | 当前访问上下文 |
| `GET` | `/api/v1/console/models` | 模型摘要 |
| `GET` | `/api/v1/console/billing` | 计费摘要 |

## 7. Frontend Route Matrix

### 7.1 当前已注册路由

| Area | Path | Status |
| --- | --- | --- |
| Marketing | `/` | 已接入 |
| Marketing | `/login` | 已接入 |
| Marketing | `/register` | 已接入 |
| Workspace | `/onboarding` | 已接入，允许跳过但仍作为首次引导页 |
| Workspace | `/chat` | 已接入，作为默认主入口与会话空状态页 |
| Workspace | `/chat/:conversationId` | 已接入，支持消息、知识库绑定与 SOLO handoff |
| Workspace | `/knowledge` | 已接入，支持知识库列表、创建与从 Chat 的 `returnTo` 回跳 |
| Workspace | `/knowledge/:knowledgeBaseId` | 已接入，支持文档 CRUD、retrieval 与回到 Chat |
| Workspace | `/solo` | 已接入，支持 `taskId` 与 Chat-originated return flow |
| Workspace | `/solo/new` | 已接入，支持任务创建视图与默认参数配置 |
| Workspace | `/settings` | 已接入，作为长期偏好页并支持返回 Chat |
| Console | `/console` | 已接入，运营总览页可用 |
| Console | `/console/models` | 已接入，supporting drill-down 可用 |
| Console | `/console/usage` | 已接入，请求量 workbench drill-down 可用 |
| Console | `/console/billing` | 已接入，成本 workbench drill-down 可用 |
| Console | `/console/access` | 已接入，scope / session workbench drill-down 可用 |

### 7.2 已存在页面但尚未挂载的目标路由

| Planned Path | Current State |
| --- | --- |
| none | 当前无已存在但未挂载的主线路由 |

### 7.3 Current Gaps

- `ProtectedRoute` 已接入 workspace 与 console 路由树；测试环境下 `idle` 状态默认放行以支撑 router smoke tests
- `AppProviders` 当前提供真实 `AppContextProvider`
- `useAppContext` 已存在，并在无 provider 场景返回测试安全的 fallback context
- `types/api.ts` 已覆盖当前主线后端接口与 console/knowledge/task/chat 所需类型

### 7.4 Root Verification Entry

| Command | Scope | Notes |
| --- | --- | --- |
| `bash scripts/check.sh` | 主线 docs + web build + server unit checks | 作为 CI 与本地共同的静态门面 |
| `bash scripts/test.sh` | 主线 web tests + server unit tests + optional integration tests | 当 `TEST_DATABASE_URL` 缺失时，server integration 会显式 skip |

## 8. Environment Variable Matrix

### 8.1 Frontend Local Development

| Name | Required | Default | Current Use |
| --- | --- | --- | --- |
| `WEB_PORT` | 否 | `5173` | 前端本地端口约定 |
| `WEB_API_BASE_URL` | 否 | `http://localhost:8080` | 前端调用后端的本地基地址 |

### 8.2 Backend Runtime

| Name | Required | Default | Status |
| --- | --- | --- | --- |
| `SERVER_PORT` | 否 | `8080` | 已消费 |
| `APP_ENV` | 否 | `development` | 已消费 |
| `CORS_ALLOWED_ORIGINS` | 否 | empty | 已消费，通过 HTTP middleware 应用到允许来源与预检响应 |
| `DATABASE_URL` | 是 | none | 已消费 |
| `SESSION_SECRET` | 是 | none | 已消费，通过 HMAC 签名与校验 session cookie |
| `SESSION_COOKIE_NAME` | 否 | `oblivious_session` | 已消费 |
| `SESSION_COOKIE_SECURE` | 否 | `false` | 已消费 |
| `LLM_BASE_URL` | 否 | empty | 已消费 |
| `LLM_API_KEY` | 否 | empty | 已消费 |
| `LLM_TIMEOUT_MS` | 否 | `30000` | 已消费 |
| `MODEL_DEFAULT_NAME` | 否 | `demo-reply` | 已消费 |

### 8.3 Backend Test Runtime

| Name | Required | Default | Status |
| --- | --- | --- | --- |
| `TEST_DATABASE_URL` | 否 | empty | `internal/http` 集成测试显式读取；缺失时跳过 integration 组而不是硬连本地固定 Postgres |

## 9. Change Control Rules

从本文件生效后，以下规则适用于后续里程碑：

1. 后端 API shape 变更时，必须同时更新本文件和前端类型定义
2. 新增前端工作区路由时，必须同步记录“已注册”或“计划路由”状态
3. 环境变量新增、移除或改名时，必须同步更新：
   - `config/.env.example`
   - `src/server/internal/config/config.go`
   - 本文件

## 10. Non-Goals For This Document

以下内容不在本文件冻结范围内：

- Chat streaming / provider abstraction 设计
- CI 与发布流程设计

这些内容将在后续 milestone 文档中单独收敛。
