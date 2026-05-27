# Current System Contracts

日期：2026-05-17

本文件是当前 v03.3 Mainline Consolidation 主线系统 `src/server` + `src/web` 的执行基线。

- 主线交付范围：`src/server`、`src/web`
- 非主线参考仓：`new-api/`、`lobehub/`
- 历史设计参考：`docs/superpowers/specs/2026-04-01-task5-go-backend-infrastructure-design.md`
- 当前执行评估：`docs/reports/2026-04-06-execution-progress-review.md`

## 1. Scope

当前系统已经不再是 Task 5 中定义的 scaffold 阶段，而是 v03.3 主线整合基线：

- 后端已路由 Auth、Chat、Agent、Memory、MCP、Notification、Quota、Console、Admin、Marketplace 和 Relay `/v1/*` surface
- 前端已挂载营销页、工作区页、控制台页、Admin 管理页和 Marketplace 页面
- 发布候选以 `docs/API.md`、本文件、`docs/release/rc-checklist.md` 和脚本质量门禁作为证据链

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
- `new-api/` 与 `lobehub/` 当前不属于 root workspace、root CI 或 root 交付链路的一部分。

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

### 6.8 Agent, Memory, MCP, Notification, And Quota

| Area | Method | Path | Purpose |
| --- | --- | --- | --- |
| Agent | `GET/POST` | `/api/v1/app/agents` | 列出或创建 Agent |
| Agent | `GET/PUT/DELETE` | `/api/v1/app/agents/{agentId}` | 读取、更新或删除 Agent |
| Agent | `GET/POST` | `/api/v1/app/agents/{agentId}/conversations` | 列出或创建 Agent 会话 |
| Agent | `GET` | `/api/v1/app/agents/{agentId}/tools` | 列出 Agent 可用工具 |
| Agent | `GET/DELETE` | `/api/v1/app/agents/conversations/{conversationId}` | 读取或删除 Agent 会话 |
| Agent | `GET/POST` | `/api/v1/app/agents/conversations/{conversationId}/messages` | 列出或发送 Agent 会话消息 |
| Memory | `GET/POST` | `/api/v1/app/memory/documents` | 列出或添加 memory 文档 |
| Memory | `GET/PUT/DELETE` | `/api/v1/app/memory/documents/{documentId}` | 读取、更新或删除 memory 文档 |
| Memory | `GET` | `/api/v1/app/memory/documents/{documentId}/chunks` | 列出文档 chunks |
| Memory | `POST` | `/api/v1/app/memory/search` | 用户隔离的 memory 搜索 |
| MCP | `GET/POST` | `/api/v1/app/mcp-servers` | 列出或添加 MCP server |
| MCP | `GET/DELETE` | `/api/v1/app/mcp-servers/{serverId}` | 读取或删除 MCP server |
| MCP | `POST` | `/api/v1/app/mcp-servers/{serverId}/connect` | 连接 MCP server |
| MCP | `POST` | `/api/v1/app/mcp-servers/{serverId}/disconnect` | 断开 MCP server |
| MCP | `GET` | `/api/v1/app/mcp-servers/{serverId}/tools` | 列出 MCP tools |
| MCP | `GET` | `/api/v1/app/mcp-servers/{serverId}/status` | 读取连接状态 |
| MCP | `POST` | `/api/v1/app/mcp-servers/{serverId}/execute` | 执行 MCP tool |
| Notification | `GET/POST` | `/api/v1/app/notifications` | 列出或创建通知 |
| Notification | `GET` | `/api/v1/app/notifications/unread-count` | 未读通知计数 |
| Notification | `POST` | `/api/v1/app/notifications/mark-all-read` | 全部标为已读 |
| Notification | `PATCH/DELETE` | `/api/v1/app/notifications/{notificationId}` | 标记已读或删除通知 |
| Quota | `GET` | `/api/v1/app/quota` | 当前 quota 余额和使用量 |
| Quota | `GET` | `/api/v1/app/packages` | quota package 列表 |
| Quota | `POST` | `/api/v1/app/quota/topup` | quota 充值 |

### 6.9 Admin

Admin API 均要求已认证 admin session。

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/stats` | 系统统计 |
| `GET/POST` | `/api/v1/admin/channels` | 列出或创建渠道 |
| `POST` | `/api/v1/admin/channels/batch` | 批量更新渠道 |
| `GET/PUT/DELETE` | `/api/v1/admin/channels/{channelId}` | 读取、更新或删除渠道 |
| `POST` | `/api/v1/admin/channels/{channelId}/test` | 测试渠道 |
| `GET` | `/api/v1/admin/channels/{channelId}/health` | 读取渠道健康状态 |
| `GET/POST` | `/api/v1/admin/routes` | 列出或创建模型路由 |
| `GET/PUT/DELETE` | `/api/v1/admin/routes/{routeId}` | 读取、更新或删除模型路由 |
| `GET/POST` | `/api/v1/admin/plans` | 列出或创建套餐 |
| `GET/PUT/DELETE` | `/api/v1/admin/plans/{planId}` | 读取、更新或停用套餐 |
| `GET` | `/api/v1/admin/users` | 列出用户 |
| `GET/PUT/PATCH/DELETE` | `/api/v1/admin/users/{userId}` | 读取、更新、调整 quota 或删除用户 |
| `POST` | `/api/v1/admin/users/{userId}/disable` | 禁用用户 |
| `POST` | `/api/v1/admin/users/{userId}/enable` | 启用用户 |
| `GET` | `/api/v1/admin/audit-logs` | 审计日志 |
| `GET` | `/api/v1/admin/reviews` | 待审核 Marketplace agents |
| `POST` | `/api/v1/admin/reviews/{agentId}/approve` | 审核通过 agent |
| `POST` | `/api/v1/admin/reviews/{agentId}/reject` | 审核拒绝 agent |

### 6.10 Marketplace

Discovery endpoints 可公开访问；发布、安装、my-agents、review 提交和 publisher stats 要求 authenticated session。

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/marketplace/featured` | 推荐 agents |
| `GET` | `/api/v1/marketplace/curated` | 精选 section |
| `GET` | `/api/v1/marketplace/categories` | 分类列表 |
| `GET` | `/api/v1/marketplace/search` | 搜索 agents |
| `GET/POST` | `/api/v1/marketplace/agents` | 列表或发布 agent |
| `GET` | `/api/v1/marketplace/my-agents` | 当前用户发布的 agents |
| `GET` | `/api/v1/marketplace/installs` | 当前用户已安装 agents |
| `DELETE` | `/api/v1/marketplace/installs/{agentId}` | 卸载 agent |
| `GET` | `/api/v1/marketplace/publisher/stats` | 发布者统计 |
| `GET/PUT/DELETE` | `/api/v1/marketplace/agents/{agentId}` | 读取、更新或删除 agent |
| `POST/DELETE` | `/api/v1/marketplace/agents/{agentId}/install` | 安装或卸载 agent |
| `GET/POST` | `/api/v1/marketplace/agents/{agentId}/reviews` | 列出或提交 review |
| `GET` | `/api/v1/marketplace/agents/{agentId}/versions` | agent 版本 |
| `GET` | `/api/v1/marketplace/agents/{agentId}/stats` | agent 统计 |

### 6.11 WebSocket And Relay

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/ws` | 已认证用户的实时通知 WebSocket |
| `POST` | `/v1/chat/completions` | Relay Chat Completions |
| `POST` | `/v1/responses` | Relay Responses API |
| `POST` | `/v1/embeddings` | Relay Embeddings |
| `POST` | `/v1/images/generations` | Relay image generation |
| `POST` | `/v1/audio/speech` | Relay audio speech |
| `POST` | `/v1/moderations` | Relay moderation |
| `POST` | `/v1/completions` | Relay legacy completions |

Relay 还注册 files、fine-tuning、assistants、threads、runs、batch、audio transcription/translation、image edit/variation 和 realtime routes；完整 route index 以 `docs/API.md` 中的 `## Relay /v1 Endpoints` 为准。

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
| Workspace | `/marketplace` | 已接入，Marketplace browse/search 入口 |
| Workspace | `/marketplace/agents/:agentId` | 已接入，agent detail/install/review 入口 |
| Workspace | `/marketplace/publish` | 已接入，agent 发布入口 |
| Workspace | `/marketplace/my-agents` | 已接入，发布者 agents 管理入口 |
| Workspace | `/settings` | 已接入，作为长期偏好页并支持返回 Chat |
| Console | `/console` | 已接入，运营总览页可用 |
| Console | `/console/models` | 已接入，supporting drill-down 可用 |
| Console | `/console/usage` | 已接入，请求量 workbench drill-down 可用 |
| Console | `/console/billing` | 已接入，成本 workbench drill-down 可用 |
| Console | `/console/access` | 已接入，scope / session workbench drill-down 可用 |
| Admin | `/admin` | 已接入，Admin dashboard |
| Admin | `/admin/channels` | 已接入，渠道管理 |
| Admin | `/admin/routes` | 已接入，模型路由管理 |
| Admin | `/admin/plans` | 已接入，套餐管理 |
| Admin | `/admin/users` | 已接入，用户管理 |
| Admin | `/admin/audit-log` | 已接入，审计日志 |
| Admin | `/admin/reviews` | 已接入，Marketplace 审核 |

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
| `bash scripts/test.sh` | 主线 web tests + server unit tests + DB-backed integration tests | 本地缺少 `TEST_DATABASE_URL` 时 integration 会显式 skip；CI 设置 `OBLIVIOUS_REQUIRE_TEST_DATABASE=true` 后缺少 DB 会失败 |

### 7.5 Release Gate Commands

| Gate | Command | Notes |
| --- | --- | --- |
| Docs and release assets | `bash scripts/check.sh docs` | 验证 docs/API、系统契约、RC checklist、env contract 和 workspace 边界 |
| Web build | `bash scripts/check.sh web` | 执行 `pnpm --dir src/web build` |
| Server release checks | `bash scripts/check.sh server` | 执行 `go test ./... -count=1` |
| Web tests | `bash scripts/test.sh web` | Vitest suite |
| Server tests | `bash scripts/test.sh server` | Server unit tests；本地缺少 `TEST_DATABASE_URL` 时 integration 组显式 skip；CI 使用 PostgreSQL service 和 required-DB 模式 |
| Browser E2E | `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e` | Admin 与 Marketplace Playwright gate |

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
| `RELAY_ENABLED` | 否 | `true` | 已消费，控制 Relay 层是否启用 |
| `RELAY_DEFAULT_MODEL` | 否 | `gpt-4o-mini` | 已消费，Relay 默认模型 |
| `OPENAI_API_KEY` | 否 | empty | 已消费，开发环境默认渠道 API Key |
| `OPENAI_BASE_URL` | 否 | `https://api.openai.com` | 已消费，开发环境默认渠道 Base URL |

### 8.3 Backend Test Runtime

| Name | Required | Default | Status |
| --- | --- | --- | --- |
| `TEST_DATABASE_URL` | CI 是；本地否 | empty | `internal/http` 集成测试显式读取；本地缺失时跳过 integration 组而不是硬连本地固定 Postgres |
| `OBLIVIOUS_REQUIRE_TEST_DATABASE` | CI 是；本地否 | `false` | 为 `true` 时，缺少 `TEST_DATABASE_URL` 会使 `scripts/test.sh server` 失败，防止 CI 静默跳过 DB-backed coverage |

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
