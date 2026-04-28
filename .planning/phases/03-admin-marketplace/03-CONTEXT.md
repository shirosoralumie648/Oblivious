# Phase 3: Admin 与 Marketplace - Context

**Gathered:** 2026-04-28
**Status:** Ready for planning

<domain>
## Phase Boundary

本 Phase 交付两大部分：(1) Admin 管理面板 — 渠道/套餐/用户管理，含 RBAC 权限控制和操作审计；(2) Marketplace — Agent 发布审核流水线、发现/搜索/安装、评分评价系统、定价与分成。

Requirements: ADMIN-01~04, MARKET-01~02
</domain>

<decisions>
## Implementation Decisions

### Admin 架构与渠道管理
- **D-01:** Admin 是独立子系统，独立导航栏和 `/admin/*` 路由组，仅管理员角色可访问
- **D-02:** 渠道管理采用表格列表 + 行内操作（启用/禁用/编辑/删除），抽屉或模态框创建/编辑表单
- **D-03:** 渠道状态通过轮询 HealthChecker 实时更新，绿色/黄色/红色状态指示器
- **D-04:** 模型路由配置独立页面管理（与渠道列表分开），支持路由映射的增删改查
- **D-05:** 权限控制采用 RBAC（预定义角色集: admin / moderator / user），Middleware 检查权限
- **D-06:** 渠道提供手动测试连接按钮（轻量 API 调用验证可用性 + 延迟显示）
- **D-07:** Admin 首页为指标仪表板：总渠道数/在线率、总用户数、API 调用量、活跃 Agent 数
- **D-08:** 支持渠道批量操作（批量启用/禁用）+ 操作审计日志（操作人/时间/IP/变更内容）
- **D-09:** Admin 导航为动态可搜索侧边栏（模块自动分组 + 搜索跳转）

### 套餐定价与用户管理
- **D-10:** 混合定价模式：基础月费 + 超额按量；Free 套餐纯按量，Pro 以上月费含配额 + 超额付费
- **D-11:** 套餐通过 Admin 表单配置（名称/价格/token配额/模型访问列表/Agent数量上限）
- **D-12:** 完整用户生命周期管理：列表（搜索/筛选/排序）+ 查看详情 + 编辑角色/套餐 + 禁用/启用
- **D-13:** 用量统计和账单在 Admin 端和用户端都展示（token 消耗/API 调用次数/费用）
- **D-14:** 套餐公开可见，用户可自助浏览/订阅/升降级（变更下个计费周期生效）
- **D-15:** 集成 Stripe 支付网关（前端支付 + Webhook 回调更新套餐状态）
- **D-16:** RBAC 角色为预定义角色集，Admin 端可管理角色分配

### Agent 发布流程
- **D-17:** 管理员审核发布流程：用户提交 → Admin 审核 → 批准/拒绝（附原因）→ 上架
- **D-18:** 发布元数据包含：名称、描述、图标、分类标签、工具列表、示例对话、系统提示词
- **D-19:** 版本管理：每次更新需重新审核，旧版本保持可用，用户可选择版本
- **D-20:** 用户安装后立即可用，无需额外配置（Agent 和工具开箱即用）
- **D-21:** 三态可见性控制：公开（Marketplace 可见）、私有（仅自己）、未上架（待审核）
- **D-22:** 支持定价与分成：发布者可设置免费/一次性付费/订阅价格，平台抽成
- **D-23:** 为发布者提供基础使用统计（安装量、活跃用户数、API 调用量）
- **D-24:** 完整审核流水线：提交 → 审核中 → 批准/拒绝（附原因）→ 上架，用户可查看审核状态

### Marketplace 发现与安装
- **D-25:** 首页为精选推荐布局（横幅 + 精选推荐 + 分类展示热门 + 搜索栏）
- **D-26:** Agent 详情页包含：名称/描述/图标、截图/演示区、工具列表、示例对话、安装量/评分、用户评价
- **D-27:** 5 星评分 + 文字评价系统，安装后可用
- **D-28:** 预定义分类（对话/编程/写作/图像/数据分析等）+ 发布者自选标签
- **D-29:** 精选推荐由算法自动选出（基于评分/安装量/活跃度）
- **D-30:** 全文搜索（名称+描述）+ 多维度筛选（分类/标签/评分/价格）+ 多排序方式

### Claude's Discretion
- 加载骨架屏/错误状态/空态设计细节
- 具体 UI 间距和排版
- 算法推荐的具体权重公式
- 套餐配额具体数值
- Stripe Webhook 的重试和幂等处理细节
</decisions>

<specifics>
## Specific Ideas

- Admin 仪表板风格应简洁专业，参考 Vercel/Linear 的管理面板设计
- 渠道管理操作应快速高效，减少页面跳转（行内编辑/抽屉式表单）
- Marketplace 应参考 App Store 的用户体验（精选 + 分类浏览）

</specifics>

<canonical_refs>
## Canonical References

No external specs — requirements are fully captured in:

### Requirements
- `.planning/REQUIREMENTS.md` §v2 — ADMIN-01~04, MARKET-01~02

### Existing code
- `src/server/internal/http/router.go` — Route registration pattern, existing admin routes
- `src/server/internal/relay/` — Channel management backend, HealthChecker infrastructure
- `src/server/internal/quota/` — Quota service (reference for billing integration)
- `src/server/internal/auth/` — Auth middleware pattern (reference for RBAC)
- `src/web/src/` — Frontend patterns: factory functions, named exports, PascalCase components

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Admin 路由已存在 (`/admin`, `/admin/users`) 和 Admin Layout
- HealthChecker 基础设施可用于渠道状态轮询
- Prometheus metrics 可复用为仪表板数据源
- 现有的 service.go + store.go + handler 三层模式
- 前端 factory 模式 (createAuthStore, createChatApi 等) 可复用

### Established Patterns
- Go backend: service.go (接口) + store.go (持久化) + http handler
- Frontend: PascalCase 组件、camelCase 模块、createXxx factory 函数、named exports
- API 响应使用统一 envelope 格式
- Relay 子系统独立使用 Gin 框架

### Integration Points
- `/api/v1/admin/*` — 新增 admin API 路由
- `/api/v1/marketplace/*` — 新增 marketplace API 路由
- Stripe Webhook 端点
- 现有 auth middleware 扩展 RBAC 检查
</code_context>

<deferred>
## Deferred Ideas

- 自定义角色编辑器（RBAC 可视化角色创建）—— 后续 Phase
- Agent 端付费 + 支付流程的完整实现 —— Phase 3 做基础定价框架，完整 Stripe 集成视情况推进
- 发布者收入仪表板和提现 —— 后续 Phase

</deferred>

---

*Phase: 03-admin-marketplace*
*Context gathered: 2026-04-28*
