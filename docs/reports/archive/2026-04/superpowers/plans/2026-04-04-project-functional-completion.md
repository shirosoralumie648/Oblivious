# Oblivious Project Functional Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Oblivious 从“后端先行、前端契约断裂、文档滞后”的状态推进到可运行工作区 Beta，再交付 Knowledge retrieval MVP、SOLO runtime MVP，并建立可发布的工程化基线。

**Architecture:** 以后端 Go API 和 PostgreSQL 模型为事实来源，先完成前端状态层、HTTP 契约和路由收敛，再让 Chat、Knowledge、SOLO、Console 形成用户可运行闭环。后续在不推翻现有业务壳的前提下，为 Knowledge 和 SOLO 增加真实能力，并在最后补齐 CI、测试、安全和发布流程。

**Tech Stack:** Go 1.22, PostgreSQL, `net/http`, `database/sql`, `bcrypt`, React 18, React Router 6, Vite 5, Vitest, Testing Library

---

## Scope Check

这个计划覆盖多个独立子系统：

- 前端基础设施与 HTTP 契约
- 工作区页面与路由
- 后端配置/测试环境收敛
- Knowledge retrieval MVP
- SOLO runtime MVP
- 工程化与发布加固

执行时应按任务顺序推进，并在 Task 5 完成后决定是否把 Task 9 和 Task 10 再拆成单独执行计划。

## File Structure

### 文档与契约

- Create: `docs/architecture/current-system-contracts.md`
  - 当前前后端 API、envelope、环境变量、路由、状态模型的统一事实来源
- Modify: `docs/superpowers/specs/2026-04-01-task5-go-backend-infrastructure-design.md`
  - 加历史说明或追加 handoff，避免继续把过期文档当执行基线
- Modify: `config/.env.example`
  - 统一为当前 Go 服务真实要求的环境变量契约

### 前端核心

- Create: `src/web/src/app/appContext.tsx`
  - 承载 `useAppContext`、`authState`、`updatePreferences`、`bootstrap` 等应用级状态
- Modify: `src/web/src/app/providers.tsx`
  - 接入真实 context provider
- Modify: `src/web/src/app/router.tsx`
  - 挂载 `/knowledge`、`/knowledge/:knowledgeBaseId`、`/solo`、`/solo/new`，接入 `ProtectedRoute`
- Modify: `src/web/src/features/auth/store.ts`
  - 修复 auth store 契约
- Modify: `src/web/src/features/auth/useAuthBootstrap.ts`
  - 与 store 契约对齐
- Modify: `src/web/src/features/auth/ProtectedRoute.tsx`
  - 作为工作区路由保护门禁
- Create: `src/web/src/services/http/envelope.ts`
  - 统一前端的 envelope 解包逻辑
- Modify: `src/web/src/services/http/client.ts`
  - 补齐 `put` / `delete`，接入 envelope 解包和错误映射
- Modify: `src/web/src/types/api.ts`
  - 定义完整 API 类型

### 前端业务页

- Modify: `src/web/src/features/chat/api.ts`
- Modify: `src/web/src/features/knowledge/api.ts`
- Modify: `src/web/src/features/tasks/api.ts`
- Modify: `src/web/src/features/console/api.ts`
- Modify: `src/web/src/features/layouts/WorkspaceLayout.tsx`
- Modify: `src/web/src/features/layouts/ConsoleLayout.tsx`
- Modify: `src/web/src/routes/workspace/ChatPage.tsx`
- Modify: `src/web/src/routes/workspace/KnowledgePage.tsx`
- Modify: `src/web/src/routes/workspace/SoloPage.tsx`
- Modify: `src/web/src/routes/workspace/OnboardingPage.tsx`
- Modify: `src/web/src/routes/workspace/SettingsPage.tsx`
- Modify: `src/web/src/routes/console/ConsoleHomePage.tsx`
- Modify: `src/web/src/routes/console/UsagePage.tsx`
- Modify: `src/web/src/routes/console/ModelsPage.tsx`
- Modify: `src/web/src/routes/console/BillingPage.tsx`
- Modify: `src/web/src/routes/console/AccessPage.tsx`

### 后端收敛与增强

- Modify: `src/server/internal/config/config.go`
  - 清理或落实配置项消费路径
- Modify: `src/server/internal/http/middleware.go`
  - 增补 CORS 或环境相关中间件时的落点
- Modify: `src/server/internal/http/router.go`
  - 如需拆分路由注册，按业务域拆出子注册函数
- Modify: `src/server/internal/http/server_test.go`
  - 将固定本地数据库依赖改造成 CI 友好模式
- Modify: `src/server/internal/chat/gateway.go`
  - 增加 streaming/provider 抽象阶段的入口
- Modify: `src/server/internal/knowledge/*`
  - 增加 ingestion / retrieval MVP
- Modify: `src/server/internal/task/*`
  - 将 starter 状态流升级为 runtime MVP

### 测试与工程化

- Create: `src/web/src/app/appContext.test.tsx`
- Create: `src/web/src/features/auth/useAuthBootstrap.test.ts`
- Modify: `src/web/src/services/http/client.test.ts`
- Modify: `src/web/src/app/router.test.tsx`
- Modify: `src/web/src/routes/workspace/*.test.tsx`
- Modify: `src/web/src/routes/console/*.test.tsx`
- Modify: `src/server/internal/http/*.go`
- Modify: `scripts/test.sh`
- Modify: `scripts/check.sh`
- Create: `.github/workflows/ci.yml`

## Milestones

| Milestone | Target Date | Exit Criteria |
| --- | --- | --- |
| M0 基线冻结 | 2026-04-10 | 文档、接口、环境变量和 owner 全部冻结 |
| M1 主线可运行 | 2026-04-24 | 前端可构建、工作区核心路由可访问、前后端契约一致 |
| M2 工作区 Beta | 2026-05-15 | Chat / Knowledge CRUD / SOLO starter / Settings / Console 跑通 |
| M3 能力 Beta | 2026-06-05 | Knowledge retrieval MVP、SOLO runtime MVP、Chat 网关增强完成 |
| M4 RC 候选版 | 2026-06-19 | CI、测试、安全、文档、发布流程齐备 |

## Task 1: Freeze Contracts And Documentation

**Files:**
- Create: `docs/architecture/current-system-contracts.md`
- Modify: `docs/superpowers/specs/2026-04-01-task5-go-backend-infrastructure-design.md`
- Modify: `config/.env.example`
- Reference: `docs/reports/2026-04-04-codebase-analysis.md`
- Reference: `docs/reports/2026-04-04-progress-plan.md`

- [ ] **Step 1: Create the current-system contract matrix document**

核心结构：

```md
# Current System Contracts

## HTTP Envelope
- success: { ok: true, data, error: null }
- failure: { ok: false, data: null, error: { code, message } }

## Auth State
- idle
- authenticated
- unauthenticated

## Environment Variables
- DATABASE_URL
- SESSION_SECRET
- SESSION_COOKIE_NAME
- SESSION_COOKIE_SECURE
- LLM_BASE_URL
- LLM_API_KEY
```

- [ ] **Step 2: Mark Task 5 spec as historical and add current-handoff notes**

追加说明应明确：

```md
## Historical Note
This document reflects the original Task 5 scaffold target.
Current code has already implemented PostgreSQL, auth, chat, console, knowledge, and task modules.
Execution should follow `docs/architecture/current-system-contracts.md`.
```

- [ ] **Step 3: Rewrite `.env.example` to match actual runtime config**

目标示例：

```env
WEB_PORT=5173
WEB_API_BASE_URL=http://localhost:8080
SERVER_PORT=8080
DATABASE_URL=postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable
SESSION_SECRET=change-me
SESSION_COOKIE_NAME=oblivious_session
SESSION_COOKIE_SECURE=false
LLM_BASE_URL=
LLM_API_KEY=
LLM_TIMEOUT_MS=30000
MODEL_DEFAULT_NAME=demo-reply
```

- [ ] **Step 4: Verify documentation and env contract consistency**

Run:

```bash
rg -n "DATABASE_URL|SESSION_SECRET|LLM_BASE_URL|LLM_API_KEY|MODEL_DEFAULT_NAME" \
  config/.env.example \
  docs/architecture/current-system-contracts.md \
  src/server/internal/config/config.go
```

Expected: all required runtime variables appear in all three places without contradictory names.

- [ ] **Step 5: Commit**

```bash
git add docs/architecture/current-system-contracts.md docs/superpowers/specs/2026-04-01-task5-go-backend-infrastructure-design.md config/.env.example
git commit -m "docs: freeze current contracts and runtime configuration"
```

## Task 2: Rebuild Frontend App Context And Auth Bootstrap

**Files:**
- Create: `src/web/src/app/appContext.tsx`
- Modify: `src/web/src/app/providers.tsx`
- Modify: `src/web/src/features/auth/store.ts`
- Modify: `src/web/src/features/auth/useAuthBootstrap.ts`
- Modify: `src/web/src/features/auth/ProtectedRoute.tsx`
- Modify: `src/web/src/types/api.ts`
- Test: `src/web/src/features/auth/store.test.ts`
- Test: `src/web/src/app/appContext.test.tsx`
- Test: `src/web/src/features/auth/useAuthBootstrap.test.ts`

- [ ] **Step 1: Write failing tests for app context and bootstrap flow**

测试目标：

```tsx
it('provides authState and updatePreferences via app context', () => {
  // render AppProviders + consumer
});

it('bootstraps authenticated session into store', async () => {
  // mock authApi.me -> session envelope data
});
```

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/features/auth/store.test.ts \
  src/app/appContext.test.tsx \
  src/features/auth/useAuthBootstrap.test.ts
```

Expected: FAIL because context and bootstrap contracts do not exist yet.

- [ ] **Step 2: Expand auth store to match bootstrap needs**

目标接口：

```ts
type AuthStore = {
  getState: () => AuthState;
  startLoading: () => void;
  setAuthenticatedSession: (user: ApiUser, preferences: UserPreferences) => void;
  setAuthenticatedUser: (user: ApiUser) => void;
  clearUser: () => void;
};
```

- [ ] **Step 3: Implement `AppContext` and wire it through `AppProviders`**

目标能力：

```tsx
export function AppProviders({ children }: { children: ReactNode }) {
  return <AppContextProvider>{children}</AppContextProvider>;
}
```

Context 至少暴露：

- `authState`
- `bootstrapAuth`
- `updatePreferences`

- [ ] **Step 4: Run focused auth/context tests**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/features/auth/store.test.ts \
  src/app/appContext.test.tsx \
  src/features/auth/useAuthBootstrap.test.ts
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/web/src/app/appContext.tsx src/web/src/app/providers.tsx src/web/src/features/auth/store.ts src/web/src/features/auth/useAuthBootstrap.ts src/web/src/features/auth/ProtectedRoute.tsx src/web/src/types/api.ts src/web/src/features/auth/store.test.ts src/web/src/app/appContext.test.tsx src/web/src/features/auth/useAuthBootstrap.test.ts
git commit -m "feat(web): restore app context and auth bootstrap"
```

## Task 3: Normalize HTTP Client And Frontend API Contracts

**Files:**
- Create: `src/web/src/services/http/envelope.ts`
- Modify: `src/web/src/services/http/client.ts`
- Modify: `src/web/src/services/http/client.test.ts`
- Modify: `src/web/src/types/api.ts`
- Modify: `src/web/src/features/chat/api.ts`
- Modify: `src/web/src/features/knowledge/api.ts`
- Modify: `src/web/src/features/tasks/api.ts`
- Modify: `src/web/src/features/console/api.ts`

- [ ] **Step 1: Write failing tests for envelope unwrapping and new HTTP verbs**

测试目标：

```ts
it('unwraps successful envelope payloads', async () => {
  // { ok: true, data: { requests: 3 }, error: null }
});

it('supports PUT and DELETE requests', async () => {
  // verify request method and parsed payload
});
```

Run:

```bash
pnpm --dir src/web exec vitest run src/services/http/client.test.ts
```

Expected: FAIL because `put`/`delete` and envelope parsing do not exist.

- [ ] **Step 2: Implement envelope helper and extend client verbs**

目标接口：

```ts
export type ApiEnvelope<T> = {
  ok: boolean;
  data: T | null;
  error: { code: string; message: string } | null;
};
```

`HttpClient` 目标接口：

```ts
type HttpClient = {
  get: <T>(path: string, init?: RequestInit) => Promise<T>;
  post: <T>(path: string, body?: unknown, init?: RequestInit) => Promise<T>;
  put: <T>(path: string, body?: unknown, init?: RequestInit) => Promise<T>;
  delete: <T>(path: string, init?: RequestInit) => Promise<T>;
};
```

- [ ] **Step 3: Fill out missing API types and feature API methods**

至少补齐：

- `UserPreferences`
- `SessionResponse`
- `ConversationConfig`
- `CreateConversationRequest`
- `KnowledgeBaseSummary`
- `KnowledgeDocumentSummary`
- `CreateTaskRequest`
- `TaskSummary`
- `TaskDetail`
- `AccessSummary`
- `BillingSummary`
- `ModelSummary`

Chat API 至少补齐：

```ts
createConversation
getConversationConfig
updateConversationConfig
sendMessage
convertConversationToTask
```

Console API 至少补齐：

```ts
getAccess
getBilling
getModels
getUsage
```

- [ ] **Step 4: Run API/client tests**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/services/http/client.test.ts \
  src/routes/console/UsagePage.test.tsx \
  src/routes/console/ConsoleHomePage.test.tsx
```

Expected: PASS for client tests; console tests may still fail until UI tasks are implemented, but imports and API shape should resolve.

- [ ] **Step 5: Commit**

```bash
git add src/web/src/services/http/envelope.ts src/web/src/services/http/client.ts src/web/src/services/http/client.test.ts src/web/src/types/api.ts src/web/src/features/chat/api.ts src/web/src/features/knowledge/api.ts src/web/src/features/tasks/api.ts src/web/src/features/console/api.ts
git commit -m "feat(web): align http client and api contracts"
```

## Task 4: Wire Protected Workspace Routes And Layouts

**Files:**
- Modify: `src/web/src/app/router.tsx`
- Modify: `src/web/src/features/auth/ProtectedRoute.tsx`
- Modify: `src/web/src/features/layouts/WorkspaceLayout.tsx`
- Modify: `src/web/src/features/layouts/ConsoleLayout.tsx`
- Test: `src/web/src/app/router.test.tsx`
- Test: `src/web/src/features/layouts/WorkspaceLayout.test.tsx`
- Test: `src/web/src/features/layouts/ConsoleLayout.test.tsx`

- [ ] **Step 1: Add failing router tests for knowledge and solo routes**

测试目标：

```tsx
it('renders knowledge route inside protected workspace layout', () => {});
it('renders solo route inside protected workspace layout', () => {});
```

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/app/router.test.tsx \
  src/features/layouts/WorkspaceLayout.test.tsx \
  src/features/layouts/ConsoleLayout.test.tsx
```

Expected: FAIL because routes are not mounted and layout content is still placeholder.

- [ ] **Step 2: Mount protected workspace routes**

需要挂载：

```tsx
/chat
/chat/:conversationId
/knowledge
/knowledge/:knowledgeBaseId
/solo
/solo/new
/settings
/onboarding
```

工作区路由应统一包在：

```tsx
<ProtectedRoute />
```

之内。

- [ ] **Step 3: Upgrade layout shells to match page expectations**

`WorkspaceLayout` 至少要提供：

- 导航入口：Chat / Knowledge / SOLO / Settings / Console
- 当前用户/工作区上下文插槽
- `Outlet`

`ConsoleLayout` 至少要提供：

- Console 导航
- 返回 workspace 入口
- 子路由容器

- [ ] **Step 4: Run routing/layout tests**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/app/router.test.tsx \
  src/features/layouts/WorkspaceLayout.test.tsx \
  src/features/layouts/ConsoleLayout.test.tsx \
  src/routes/workspace/ChatPage.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/web/src/app/router.tsx src/web/src/features/auth/ProtectedRoute.tsx src/web/src/features/layouts/WorkspaceLayout.tsx src/web/src/features/layouts/ConsoleLayout.tsx src/web/src/app/router.test.tsx src/web/src/features/layouts/WorkspaceLayout.test.tsx src/web/src/features/layouts/ConsoleLayout.test.tsx
git commit -m "feat(web): wire protected workspace and console routes"
```

## Task 5: Implement Workspace Chat, Onboarding, And Settings

**Files:**
- Modify: `src/web/src/routes/workspace/ChatPage.tsx`
- Modify: `src/web/src/routes/workspace/OnboardingPage.tsx`
- Modify: `src/web/src/routes/workspace/SettingsPage.tsx`
- Test: `src/web/src/routes/workspace/ChatPage.behavior.test.tsx`
- Test: `src/web/src/routes/workspace/OnboardingPage.test.tsx`
- Test: `src/web/src/routes/workspace/SettingsPage.test.tsx`

- [ ] **Step 1: Run failing behavior tests**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/workspace/ChatPage.behavior.test.tsx \
  src/routes/workspace/OnboardingPage.test.tsx \
  src/routes/workspace/SettingsPage.test.tsx
```

Expected: FAIL because pages are still placeholder implementations.

- [ ] **Step 2: Implement Onboarding and Settings to match test-defined target state**

Onboarding 最低目标：

- `Start with Chat`
- `Start with SOLO`
- `Skip for now`
- 选择后展示 model strategy 和继续按钮

Settings 最低目标：

- `Default mode`
- `Model strategy`
- `Enable web suggestions`
- 保存后显示成功消息

- [ ] **Step 3: Implement Chat page using existing chat API and conversation config**

最低目标：

- 会话加载
- knowledge base 绑定设置
- 将当前会话转换为 SOLO 任务

核心交互示例：

```tsx
await chatApi.updateConversationConfig(conversationId, {
  knowledgeBaseIds: selectedKnowledgeBaseIds
});
```

- [ ] **Step 4: Re-run workspace page tests**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/workspace/ChatPage.behavior.test.tsx \
  src/routes/workspace/OnboardingPage.test.tsx \
  src/routes/workspace/SettingsPage.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/web/src/routes/workspace/ChatPage.tsx src/web/src/routes/workspace/OnboardingPage.tsx src/web/src/routes/workspace/SettingsPage.tsx src/web/src/routes/workspace/ChatPage.behavior.test.tsx src/web/src/routes/workspace/OnboardingPage.test.tsx src/web/src/routes/workspace/SettingsPage.test.tsx
git commit -m "feat(web): implement chat onboarding and settings flows"
```

## Task 6: Finish Knowledge Workspace CRUD Integration

**Files:**
- Modify: `src/web/src/routes/workspace/KnowledgePage.tsx`
- Modify: `src/web/src/features/knowledge/api.ts`
- Modify: `src/web/src/types/api.ts`
- Test: `src/web/src/routes/workspace/KnowledgePage.test.tsx`

- [ ] **Step 1: Run failing knowledge page tests**

Run:

```bash
pnpm --dir src/web exec vitest run src/routes/workspace/KnowledgePage.test.tsx
```

Expected: FAIL or partially fail until route, types, and API contracts are aligned.

- [ ] **Step 2: Keep Knowledge page as CRUD-complete MVP**

必须完成：

- 知识库列表
- 知识库详情
- 创建/更新/删除知识库
- 创建/更新/删除文档
- 从知识页跳转到设置页

注意：不要在这个任务里开始做检索增强；只收敛现有 CRUD 目标。

- [ ] **Step 3: Extract the most complex local handlers if file size keeps growing**

建议先拆出：

- `loadKnowledgeState`
- `knowledgeDocumentFormState`
- `knowledgeBaseActions`

避免继续把所有逻辑留在单一页面组件。

- [ ] **Step 4: Re-run knowledge tests**

Run:

```bash
pnpm --dir src/web exec vitest run src/routes/workspace/KnowledgePage.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/web/src/routes/workspace/KnowledgePage.tsx src/web/src/features/knowledge/api.ts src/web/src/types/api.ts src/web/src/routes/workspace/KnowledgePage.test.tsx
git commit -m "feat(web): complete knowledge workspace crud"
```

## Task 7: Finish SOLO Starter Workspace Integration

**Files:**
- Modify: `src/web/src/routes/workspace/SoloPage.tsx`
- Modify: `src/web/src/features/tasks/api.ts`
- Modify: `src/web/src/features/chat/api.ts`
- Modify: `src/web/src/types/api.ts`
- Test: `src/web/src/routes/workspace/SoloPage.test.tsx`

- [ ] **Step 1: Run failing SOLO tests**

Run:

```bash
pnpm --dir src/web exec vitest run src/routes/workspace/SoloPage.test.tsx
```

Expected: FAIL until task and chat APIs are aligned.

- [ ] **Step 2: Complete the starter flow without expanding into runtime MVP**

必须完成：

- 任务列表分组
- `/solo/new` 创建视图
- 创建并启动任务
- safe 模式等待审批
- 暂停 / 恢复 / 取消 / 更新预算
- 重试
- 继续到 Chat
- 导出结果

- [ ] **Step 3: Stabilize tool boundary and knowledge source rendering**

确保页面显示：

- `toolAllowList`
- `toolDenyList`
- `authorizationScope`
- `knowledgeBaseIds`

这些字段必须与后端 detail payload 一致。

- [ ] **Step 4: Re-run SOLO tests**

Run:

```bash
pnpm --dir src/web exec vitest run src/routes/workspace/SoloPage.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/web/src/routes/workspace/SoloPage.tsx src/web/src/features/tasks/api.ts src/web/src/features/chat/api.ts src/web/src/types/api.ts src/web/src/routes/workspace/SoloPage.test.tsx
git commit -m "feat(web): complete solo starter workspace flow"
```

## Task 8: Implement Console Dashboard And Child Pages

**Files:**
- Modify: `src/web/src/features/console/api.ts`
- Modify: `src/web/src/routes/console/ConsoleHomePage.tsx`
- Modify: `src/web/src/routes/console/UsagePage.tsx`
- Modify: `src/web/src/routes/console/ModelsPage.tsx`
- Modify: `src/web/src/routes/console/BillingPage.tsx`
- Modify: `src/web/src/routes/console/AccessPage.tsx`
- Test: `src/web/src/routes/console/ConsoleHomePage.test.tsx`
- Test: `src/web/src/routes/console/UsagePage.test.tsx`
- Test: `src/web/src/routes/console/ModelsPage.test.tsx`
- Test: `src/web/src/routes/console/BillingPage.test.tsx`
- Test: `src/web/src/routes/console/AccessPage.test.tsx`

- [ ] **Step 1: Run console page tests**

Run:

```bash
pnpm --dir src/web exec vitest run src/routes/console/*.test.tsx
```

Expected: FAIL because console pages are placeholders and API only exposes usage.

- [ ] **Step 2: Expand console API client to match backend**

目标接口：

```ts
getUsage(): Promise<UsageSummary>
getModels(): Promise<ModelSummary[]>
getBilling(): Promise<BillingSummary>
getAccess(): Promise<AccessSummary>
```

- [ ] **Step 3: Implement console UI pages using loading/success/error states**

Console Home 至少展示：

- 7d 请求数
- 30d 估算成本
- top model
- 当前 workspace / user

其他页面按测试定义显示基础摘要。

- [ ] **Step 4: Re-run console tests**

Run:

```bash
pnpm --dir src/web exec vitest run src/routes/console/*.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/web/src/features/console/api.ts src/web/src/routes/console/ConsoleHomePage.tsx src/web/src/routes/console/UsagePage.tsx src/web/src/routes/console/ModelsPage.tsx src/web/src/routes/console/BillingPage.tsx src/web/src/routes/console/AccessPage.tsx src/web/src/routes/console/*.test.tsx
git commit -m "feat(web): implement console dashboard and summary pages"
```

## Task 9: Harden Backend Runtime Configuration And Testability

**Files:**
- Modify: `src/server/internal/config/config.go`
- Modify: `src/server/internal/http/middleware.go`
- Modify: `src/server/internal/http/router.go`
- Modify: `src/server/internal/http/server_test.go`
- Modify: `scripts/test.sh`
- Modify: `scripts/check.sh`
- Test: `src/server/internal/config/config_test.go`
- Test: `src/server/internal/http/server_test.go`

- [ ] **Step 1: Write failing tests for config/runtime cleanup**

至少覆盖：

- `.env.example` 中的变量与 `config.Load()` 一致
- `CORS_ALLOWED_ORIGINS` 真正有消费路径或被正式移除
- 测试数据库初始化不再写死本地固定实例

Run:

```bash
cd src/server && GOCACHE=../../.tmp/go-build GOMODCACHE=../../.tmp/go-mod go test ./internal/config ./internal/http
```

Expected: current config tests may pass, but runtime/testability assertions should fail until new setup exists.

- [ ] **Step 2: Decide each config field's fate**

对以下配置做出明确决策：

- `CORSAllowedOrigins`
- `SessionSecret`
- `ModelBaseURL`
- `ModelAPIKey`

规则：

- 要么接入运行路径
- 要么移除并同步文档

- [ ] **Step 3: Replace fixed localhost Postgres assumptions in integration tests**

首选方案：

- 读取测试专用 `DATABASE_URL`
- 在缺失时跳过 integration 组，或使用容器化测试策略

最低要求：

```go
databaseURL := os.Getenv("TEST_DATABASE_URL")
if databaseURL == "" {
    t.Skip("TEST_DATABASE_URL is required for integration tests")
}
```

- [ ] **Step 4: Run backend focused tests**

Run:

```bash
cd src/server && GOCACHE=../../.tmp/go-build GOMODCACHE=../../.tmp/go-mod go test \
  ./internal/config \
  ./internal/chat \
  ./internal/knowledge \
  ./internal/task \
  ./internal/console \
  ./internal/http
```

Expected: PASS in environments with dependencies available; if integration tests are skipped, skip conditions must be explicit and documented.

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/config/config.go src/server/internal/http/middleware.go src/server/internal/http/router.go src/server/internal/http/server_test.go scripts/test.sh scripts/check.sh src/server/internal/config/config_test.go
git commit -m "chore(server): harden runtime config and testability"
```

## Task 10: Deliver Knowledge Retrieval MVP

**Files:**
- Modify: `src/server/internal/knowledge/service.go`
- Modify: `src/server/internal/knowledge/store.go`
- Modify: `src/server/internal/http/knowledge_handler.go`
- Modify: `src/server/migrations/*.sql`
- Modify: `src/web/src/routes/workspace/KnowledgePage.tsx`
- Modify: `src/web/src/types/api.ts`
- Test: `src/server/internal/knowledge/service_test.go`
- Test: `src/server/internal/http/knowledge_handler_test.go`

- [ ] **Step 1: Write failing tests for retrieval flow**

最小闭环：

- 创建文档
- 切片/索引
- 基于 query 返回相关片段或文档摘要

示例测试目标：

```go
func TestRetrieveReturnsRelevantDocumentSnippets(t *testing.T) {}
```

- [ ] **Step 2: Add the minimal storage model for retrieval**

先做最小可行，不引入复杂 provider 依赖。推荐数据模型：

- `knowledge_document_chunks`
  - `id`
  - `document_id`
  - `content`
  - `chunk_index`

第一版检索可以先做：

- 标题匹配
- `ILIKE` 文本匹配
- 最小排序策略

不要在此任务里直接引入重向量基础设施，除非已有明确依赖方案。

- [ ] **Step 3: Add retrieval endpoint and surface it in Knowledge page**

建议接口：

```http
POST /api/v1/app/knowledge-bases/{id}/retrieve
```

请求体：

```json
{ "query": "..." }
```

- [ ] **Step 4: Run retrieval tests**

Run:

```bash
cd src/server && GOCACHE=../../.tmp/go-build GOMODCACHE=../../.tmp/go-mod go test ./internal/knowledge ./internal/http
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/knowledge src/server/internal/http/knowledge_handler.go src/server/migrations src/web/src/routes/workspace/KnowledgePage.tsx src/web/src/types/api.ts
git commit -m "feat(knowledge): add retrieval mvp"
```

## Task 11: Replace SOLO Starter With Runtime MVP

**Files:**
- Modify: `src/server/internal/task/service.go`
- Modify: `src/server/internal/task/store.go`
- Modify: `src/server/internal/http/task_handler.go`
- Modify: `src/web/src/routes/workspace/SoloPage.tsx`
- Modify: `src/web/src/types/api.ts`
- Test: `src/server/internal/task/service_test.go`
- Test: `src/server/internal/http/task_handler_test.go`
- Test: `src/web/src/routes/workspace/SoloPage.test.tsx`

- [ ] **Step 1: Write failing tests that reject the starter-only behavior**

必须覆盖：

- `resume` 不再直接完成任务
- `awaiting_confirmation` 之后存在继续执行状态
- 任务详情包含执行日志或结构化步骤推进

示例测试目标：

```go
func TestResumeTransitionsPausedTaskBackToRunning(t *testing.T) {}
```

- [ ] **Step 2: Define runtime MVP state model**

建议状态：

- `draft`
- `awaiting_confirmation`
- `running`
- `paused`
- `completed`
- `cancelled`
- `failed`

新增 detail 字段建议：

- `events`
- `currentStep`
- `approvalRequests`
- `resultArtifacts`

- [ ] **Step 3: Implement runtime progression without overbuilding orchestration**

最低要求：

- start -> running / awaiting_confirmation
- approve -> running
- pause -> paused
- resume -> running
- running 完成后写结构化结果，而不是固定模板字符串

不要在此任务里尝试实现完整多 agent 框架；只交付受限 runtime MVP。

- [ ] **Step 4: Run task tests**

Run:

```bash
cd src/server && GOCACHE=../../.tmp/go-build GOMODCACHE=../../.tmp/go-mod go test ./internal/task ./internal/http
pnpm --dir src/web exec vitest run src/routes/workspace/SoloPage.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/task src/server/internal/http/task_handler.go src/web/src/routes/workspace/SoloPage.tsx src/web/src/types/api.ts src/server/internal/task/service_test.go src/server/internal/http/task_handler_test.go src/web/src/routes/workspace/SoloPage.test.tsx
git commit -m "feat(task): replace solo starter with runtime mvp"
```

## Task 12: Add Quality Gates, CI, And Release Checks

**Files:**
- Create: `.github/workflows/ci.yml`
- Modify: `scripts/test.sh`
- Modify: `scripts/check.sh`
- Modify: `README.md`
- Create: `docs/release/rc-checklist.md`

- [ ] **Step 1: Write the release checklist and CI acceptance criteria**

最低检查项：

- web build
- web vitest suite
- server unit/integration split
- docs/env consistency
- no P0/P1 defects open

示例：

```md
- [ ] Frontend build passes
- [ ] Core route smoke tests pass
- [ ] Server contract tests pass
- [ ] Runtime configuration matches docs
```

- [ ] **Step 2: Add CI workflow for web and server checks**

最低工作流：

```yaml
jobs:
  web:
    steps:
      - run: pnpm --dir src/web install --frozen-lockfile
      - run: pnpm --dir src/web build
      - run: pnpm --dir src/web test
  server:
    steps:
      - run: cd src/server && go test ./internal/config ./internal/chat ./internal/knowledge ./internal/task ./internal/console
```

- [ ] **Step 3: Update root scripts and README to match CI**

必须保证：

- 本地执行路径与 CI 执行路径一致
- README 提供最小启动方式

- [ ] **Step 4: Run local verification**

Run:

```bash
bash scripts/check.sh
bash scripts/test.sh
```

Expected: PASS in a fully provisioned environment; if environment-dependent steps are skipped, skip behavior must be explicit and documented.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/ci.yml scripts/test.sh scripts/check.sh README.md docs/release/rc-checklist.md
git commit -m "chore: add quality gates and release checks"
```

## Execution Order

严格按以下顺序执行：

1. Task 1
2. Task 2
3. Task 3
4. Task 4
5. Task 5
6. Task 6
7. Task 7
8. Task 8
9. Task 9
10. Milestone checkpoint for M1/M2
11. Task 10
12. Task 11
13. Task 12

## Risk Controls

- 不在 Task 5 之前做 Knowledge retrieval
- 不在 Task 7 之前做 SOLO runtime
- 不在 Task 9 完成前宣称 backend testability 已经稳定
- 不在 Task 12 完成前宣称项目具备 RC 级质量门禁

## Success Criteria

- M1 时：主线工作区和控制台都可进入运行态，前后端契约统一
- M2 时：Chat / Knowledge CRUD / SOLO starter / Settings / Console 全部形成用户可见闭环
- M3 时：Knowledge retrieval MVP 和 SOLO runtime MVP 均可演示，且不破坏前面里程碑
- M4 时：测试、文档、CI、安全、发布清单齐备

