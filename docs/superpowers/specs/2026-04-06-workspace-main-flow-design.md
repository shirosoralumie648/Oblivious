# Workspace Main Flow Design

日期：2026-04-06

## Context

主线 `src/web` 和 `src/server` 已经具备以下基础能力：

- 认证会话与偏好接口已存在
- `/chat`、`/knowledge`、`/solo`、`/settings` 已接入工作区路由
- Knowledge retrieval MVP 与 SOLO runtime MVP 已可运行

当前缺口不在后端能力，而在“工作区主路径”的产品化收敛：

- `ProtectedRoute` 仍强制未完成 onboarding 的用户跳转到 `/onboarding`
- `OnboardingPage`、`ChatPage`、`SettingsPage` 仍偏向功能占位，不足以支撑端到端体验
- `KnowledgePage` 与 `SoloPage` 各自可用，但与 Chat 的入口和回跳链路不够清晰
- 登录/注册页仍是占位实现，因此本轮不扩展认证 UI，而是聚焦认证后的工作区主流程

本设计定义下一里程碑的目标：在不新增后端 API 的前提下，把工作区主路径收敛为一条可演示、可测试、可回归的用户流。

## Goals

- 打通 `onboarding -> chat -> knowledge -> solo -> settings` 的工作区主链路
- 允许用户跳过 onboarding，但继续保留首次引导入口
- 让 `/chat` 成为工作区默认主入口和跨页面中枢
- 让 Knowledge 和 SOLO 与 Chat 形成明确的来回跳转关系
- 为主链路补齐最小可用的空状态、错误态、加载态和关键 CTA
- 用前端测试覆盖这条链路，并保持 `src/web build` 与 `src/web test` 继续通过

## Non-Goals

- 不新增或修改后端 API 契约
- 不实现 chat streaming、provider abstraction 或 tool runtime 增强
- 不扩展 Knowledge retrieval 能力
- 不扩展 SOLO runtime 能力
- 不调整 Console 范围
- 不重做营销页、登录页、注册页的完整产品体验
- 不做大规模视觉改版，只做主链路所需的产品化补完

## Primary User Stories

### Story 1: First-Time User, Guided But Not Blocked

1. 用户在已认证状态进入工作区
2. 若 `onboardingCompleted=false`，系统默认落到 `/onboarding`
3. 用户可以：
   - 选择 Chat 作为默认模式并继续
   - 选择 SOLO 作为默认模式并继续
   - 点击 `Skip for now` 直接进入 `/chat`
4. 只有显式完成 onboarding 时，才把 `onboardingCompleted` 写为 `true`

### Story 2: Chat-Centered Work Session

1. 用户进入 `/chat`
2. 若还没有会话，页面展示空状态与创建首个会话 CTA
3. 创建成功后跳转到 `/chat/:conversationId`
4. 用户可发送消息、绑定知识库、将当前对话 handoff 到 SOLO

### Story 3: Knowledge As Chat Extension

1. 用户从 Chat 发现当前无知识库可用
2. 页面提供明确入口跳到 `/knowledge`
3. 从 Chat 跳转到 Knowledge 时保留 `returnTo`
4. 用户在 Knowledge 完成创建或编辑后，可以明显返回原 Chat 上下文

### Story 4: SOLO As Chat Continuation

1. 用户从 Chat 触发 `Hand off to SOLO`
2. Chat 生成 task draft，并跳转到 `/solo?taskId=...`
3. SOLO 页面优先展示该任务详情
4. 用户可从 SOLO 返回 Chat

### Story 5: Settings As Long-Term Preference Page

1. 用户进入 `/settings`
2. 用户修改默认模式、模型策略、联网提示
3. 保存后停留在当前页
4. 页面提供返回 Chat 的明确入口

## Current Contradictions To Resolve

### 1. Onboarding Enforcement

当前 [`src/web/src/features/auth/ProtectedRoute.tsx`](../../../src/web/src/features/auth/ProtectedRoute.tsx) 会在 `onboardingCompleted=false` 时强制跳转到 `/onboarding`。

这与本设计确认的“onboarding 可跳过，不再全局强制拦截”直接冲突。

本轮必须把 onboarding gating 从“强制阻断”改为“推荐引导”。

### 2. Chat Is Not Yet The Real Hub

当前 `ChatPage` 已具备 knowledge binding 和 SOLO handoff 的局部逻辑，但仍缺：

- `/chat` 空状态和首会话创建入口
- 明确的消息收发体验
- 无知识库时的引导
- 对未完成 onboarding 用户的 setup 提示

### 3. Knowledge / SOLO Lack Return Links

当前 `KnowledgePage` 和 `SoloPage` 可以单独使用，但没有对“从 Chat 来、处理完再回 Chat”的路径做统一约定。

本轮必须把 `returnTo` 和 `taskId` 作为前端层的主路径参数正式使用起来。

## Route Behavior

### ProtectedRoute

- 负责鉴权
- 未登录时重定向到 `/login`
- 登录中显示 loading 状态
- 不再负责强制 onboarding

### Onboarding Landing

- 已认证且 `onboardingCompleted=false` 的用户，默认落点为 `/onboarding`
- 已认证且 `onboardingCompleted=true` 的用户，按 `defaultMode` 落到：
  - `chat` -> `/chat`
  - `solo` -> `/solo/new`

说明：

- 本轮不新增新的工作区根入口路径，例如 `/app` 或 `/workspace`
- 由于登录/注册页当前仍为占位，本轮不要求完整实现 auth form UX
- 上述落点行为由现有路由重定向逻辑与会话态测试保证，而不是通过完整登录表单交互验证

### Query Parameters

本轮允许新增并规范使用以下 query 参数：

- `returnTo`
  - 用于从 Chat 跳到 Knowledge 后返回原页面
- `taskId`
  - 用于 SOLO 页面优先展示指定任务

## Page Responsibilities

### `/onboarding`

- 负责首次偏好设置
- 提供 `Start with Chat`、`Start with SOLO`、`Skip for now`
- `Continue` 行为：
  - 保存 `defaultMode`
  - 保存 `modelStrategy`
  - 保存 `networkEnabledHint`
  - 显式把 `onboardingCompleted=true`
- `Skip for now` 行为：
  - 直接进入 `/chat`
  - 不修改 `onboardingCompleted`

### `/chat`

- 成为工作区默认主入口
- 负责：
  - 无会话空状态
  - 创建首个会话 CTA
  - 最近会话入口
  - 对未完成 onboarding 的 setup 提示卡

### `/chat/:conversationId`

- 负责：
  - 消息收发
  - Conversation settings
  - Knowledge base binding
  - SOLO handoff
- 如果当前没有知识库：
  - 显示跳转到 `/knowledge` 的 CTA
  - CTA 需携带 `returnTo`

### `/knowledge`

- 负责知识库列表和新建
- 如果通过 `returnTo` 进入，应展示返回原路径的入口

### `/knowledge/:knowledgeBaseId`

- 负责：
  - 文档 CRUD
  - retrieval
  - 保存后的回跳入口

### `/solo`

- 若存在 `taskId`，优先载入对应任务详情
- 若来自 Chat handoff，显示 `Back to chat`

### `/solo/new`

- 独立创建任务
- 不强依赖 Chat 上下文

### `/settings`

- 负责长期偏好修改
- 保存后停留本页
- 提供返回 Chat 的 CTA
- 如果用户此前跳过 onboarding，可以在这里修改偏好，但不替代 onboarding 页面职责

## Navigation Rules

- 工作区导航顺序调整为：
  - `Chat`
  - `Knowledge`
  - `SOLO`
  - `Settings`
  - `Console`
- Chat 为默认主入口
- Knowledge 和 SOLO 被视为 Chat 的扩展流，而不是并列的首次入口

## UX Baseline For This Milestone

本轮每个页面至少具备：

- loading state
- error state
- empty state
- 1 个主 CTA
- 1 个返回或继续下一步的 CTA

本轮不追求重视觉设计，但必须消除“页面只有标题或只有功能碎片”的状态。

## Acceptance Criteria

### 1. Landing And Onboarding

- `onboardingCompleted=false` 时默认进入 `/onboarding`
- `onboardingCompleted=true` 时按 `defaultMode` 进入 `/chat` 或 `/solo/new`
- `Start with Chat` 后保存偏好并进入 `/chat`
- `Start with SOLO` 后保存偏好并进入 `/solo/new`
- `Skip for now` 后进入 `/chat`
- `Skip for now` 不写入 `onboardingCompleted=true`

### 2. Chat Main Flow

- `/chat` 无会话时展示空状态与“创建首个会话”
- 创建成功后跳到 `/chat/:conversationId`
- `/chat/:conversationId` 可发送消息
- 存在知识库时可完成绑定
- 不存在知识库时显示前往 Knowledge 的入口
- 存在会话时可 handoff 到 SOLO，并跳到 `/solo?taskId=...`

### 3. Knowledge Return Flow

- 从 Chat 进入 Knowledge 时保留 `returnTo`
- 在 Knowledge detail 完成操作后，用户可明确返回原 Chat

### 4. SOLO Return Flow

- 从 Chat handoff 进入的任务，SOLO 页面有 `Back to chat`
- 直接访问 `/solo/new` 不需要 Chat 上下文

### 5. Settings Flow

- 保存偏好后停留当前页
- 页面存在 `Return to chat`
- 页面继续显示 onboarding 完成状态，但不承担首次引导流程

### 6. Quality Gates

- `pnpm --dir src/web build` 通过
- `pnpm --dir src/web test` 通过
- 与主链路相关的 router / page tests 覆盖上述行为

## Testing Strategy

### Route-Level

- router tests 覆盖 authenticated session 下的默认落点
- route tests 覆盖 onboarding 可跳过、不再被 `ProtectedRoute` 强制阻断

### Page-Level

- `OnboardingPage.test.tsx`
  - start with chat
  - start with solo
  - skip for now
- `ChatPage.behavior.test.tsx`
  - 创建首个会话
  - 发送消息
  - 无知识库 CTA
  - handoff 到 solo
- `KnowledgePage.test.tsx`
  - 接收 `returnTo`
  - 显示返回 Chat 入口
- `SoloPage.test.tsx`
  - `taskId` 加载
  - 来自 Chat 时显示返回入口
- `SettingsPage.test.tsx`
  - 保存后停留当前页
  - 返回 Chat 入口

## Risks

- `ChatPage.tsx` 已较长，本轮补主路径时可能需要拆分局部子组件或 hooks
- 当前登录/注册页仍为占位，因此“登录后默认落点”的测试需要通过 router/session 模拟完成
- `ProtectedRoute` 的行为变更会影响现有 route smoke tests，需要同步更新

## Implementation Boundary Summary

- 只改前端主线 `src/web`
- 不新增后端接口
- 允许重构：
  - `src/web/src/routes/workspace/OnboardingPage.tsx`
  - `src/web/src/routes/workspace/ChatPage.tsx`
  - `src/web/src/routes/workspace/KnowledgePage.tsx`
  - `src/web/src/routes/workspace/SoloPage.tsx`
  - `src/web/src/routes/workspace/SettingsPage.tsx`
  - `src/web/src/app/router.tsx`
  - `src/web/src/features/auth/ProtectedRoute.tsx`
  - 相关测试文件

## Done Definition

当以下条件同时满足时，本里程碑完成：

- 用户可以从首次进入工作区一路走到 Chat、接入 Knowledge、再 handoff 到 SOLO
- onboarding 可跳过，但首次引导入口仍存在
- Settings 作为长期偏好页独立成立
- 主链路前端测试通过
- `src/web build` 和 `src/web test` 保持通过
