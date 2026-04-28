# Phase 3: Admin 与 Marketplace - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-28
**Phase:** 03-admin-marketplace
**Areas discussed:** Admin 架构与渠道管理, 套餐定价与用户管理, Agent 发布流程, Marketplace 发现与安装

---

## Admin 架构与渠道管理

### Admin 架构定位

| Option | Description | Selected |
|--------|-------------|----------|
| 独立子系统 | 独立导航栏、/admin/* 路由组、仅管理员角色可访问 | ✓ |
| 嵌入 Workspace | Admin 功能嵌入 Workspace 侧边栏 | |
| 仅 API 无 UI | 纯 REST API 管理，不做前端 | |

**User's choice:** 独立子系统 (推荐)

### 渠道管理交互模式

| Option | Description | Selected |
|--------|-------------|----------|
| 表格 + 行内操作 | 表格列表 + 行内操作（编辑/启用/禁用/删除）+ 模态框表单 | ✓ |
| 卡片式布局 | 每个渠道一张卡片，信息密度低 | |
| 向导式创建 | 分步引导创建渠道 | |

**User's choice:** 表格 + 行内操作 (推荐)

### 渠道状态展示

| Option | Description | Selected |
|--------|-------------|----------|
| 实时状态轮询 | 绿色/黄色/红色状态指示器 + HealthChecker 自动轮询 | ✓ |
| WebSocket 实时推送 | 已有 WS 基础设施，实现复杂度更高 | |

**User's choice:** 实时状态轮询 (推荐)

### 模型路由配置

| Option | Description | Selected |
|--------|-------------|----------|
| 展开式路由列表 | 渠道行内展开显示路由配置 | |
| 独立路由管理页 | 弹窗/抽屉中独立管理，减少主页面复杂度 | ✓ |

**User's choice:** 独立路由管理页 (推荐)

### 权限控制

| Option | Description | Selected |
|--------|-------------|----------|
| 单一 Admin 角色 | is_admin 字段 + Middleware | |
| RBAC 细粒度权限 | 角色 + 权限集 (channel.read, channel.write, etc.) | ✓ |

**User's choice:** RBAC 细粒度权限

### 渠道测试连接

| Option | Description | Selected |
|--------|-------------|----------|
| 测试连接功能 | 手动发送轻量 API 调用验证 | ✓ |
| 仅自动检测 | 依赖 HealthChecker 后台检测 | |

**User's choice:** 测试连接功能 (推荐)

### Admin 首页

| Option | Description | Selected |
|--------|-------------|----------|
| 指标仪表板 | 关键指标卡片 + 简单图表 | ✓ |
| 无概览页 | 直接进入渠道列表 | |

**User's choice:** 指标仪表板 (推荐)

### 批量操作与审计

| Option | Description | Selected |
|--------|-------------|----------|
| 批量操作 + 审计日志 | 批量启用/禁用 + 操作审计（操作人/时间/IP/变更） | ✓ |
| 仅单条操作 | 仅单条 CRUD | |

**User's choice:** 批量操作 + 审计日志

### Admin 导航

| Option | Description | Selected |
|--------|-------------|----------|
| 固定侧边栏分组 | 预定义分组（仪表板/渠道/用户/套餐等） | |
| 动态可搜索侧边栏 | 模块自动分组 + 搜索快速跳转 | ✓ |

**User's choice:** 动态可搜索侧边栏

---

## 套餐定价与用户管理

### 定价模型

| Option | Description | Selected |
|--------|-------------|----------|
| 分层月费订阅 | Free/Pro/Team/Enterprise | |
| 纯按量计费 | 预充值 credits | |
| 混合模式 | 基础月费 + 超额按量 | ✓ |

**User's choice:** 混合模式 (推荐)

### 套餐配置

| Option | Description | Selected |
|--------|-------------|----------|
| Admin 表单管理 | 名称/价格/token配额/模型访问/Agent上限 | ✓ |
| 预设固定套餐 | 代码中预设 3-4 个套餐 | |

**User's choice:** Admin 表单管理 (推荐)

### 用户管理深度

| Option | Description | Selected |
|--------|-------------|----------|
| 完整用户管理 | 列表（搜索/筛选/排序）+ 详情 + 编辑角色/套餐 + 禁用 | ✓ |
| 基础用户列表 | 仅列表 + 查看 + 禁用 | |

**User's choice:** 完整用户管理 (推荐)

### 用量展示

| Option | Description | Selected |
|--------|-------------|----------|
| Admin + 用户端 | 两端都展示用量统计 | ✓ |
| 仅 Admin 端 | 用户看不到详细用量 | |

**User's choice:** Admin + 用户端 (推荐)

### 套餐可见性

| Option | Description | Selected |
|--------|-------------|----------|
| 公开可见 + 自助升降级 | 套餐页面公开 + 用户可自助订阅 | ✓ |
| 仅 Admin 分配 | 管理员手动分配 | |

**User's choice:** 公开可见 + 自助升降级 (推荐)

### 支付集成

| Option | Description | Selected |
|--------|-------------|----------|
| 集成外部支付 | Stripe 支付 + Webhook 回调 | ✓ |
| 纯内部 Credit | 不接入真实支付 | |

**User's choice:** 集成外部支付 (推荐)

### RBAC 角色管理

| Option | Description | Selected |
|--------|-------------|----------|
| 预定义角色集 | admin/moderator/user 固定权限 | ✓ |
| 自定义角色编辑器 | 可视化角色创建 | |

**User's choice:** 预定义角色集 (推荐)

---

## Agent 发布流程

### 发布方式

| Option | Description | Selected |
|--------|-------------|----------|
| 管理员审核发布 | 提交 → 审核 → 批准/拒绝 → 上架 | ✓ |
| 用户自由发布 | 无需审核直接上架 | |

**User's choice:** 管理员审核发布 (推荐)

### 发布元数据

| Option | Description | Selected |
|--------|-------------|----------|
| 丰富元数据 | 名称/描述/图标/分类/工具/示例/提示词 | ✓ |
| 最小元数据 | 仅名称/描述/图标 | |

**User's choice:** 丰富元数据 (推荐)

### 版本管理

| Option | Description | Selected |
|--------|-------------|----------|
| 审核式版本管理 | 更新需重新审核，旧版可用 | ✓ |
| 自由更新 | 无需审核，直接编辑 | |

**User's choice:** 审核式版本管理 (推荐)

### 安装流程

| Option | Description | Selected |
|--------|-------------|----------|
| 一键安装即用 | 安装后立即可用，无需配置 | ✓ |
| 安装后需配置 | 需要 API key 等配置 | |

**User's choice:** 一键安装即用 (推荐)

### 可见性控制

| Option | Description | Selected |
|--------|-------------|----------|
| 三态可见性 | 公开/私有/未上架 | ✓ |
| 双态可见性 | 公开/私有 | |

**User's choice:** 三态可见性 (推荐)

### Agent 定价

| Option | Description | Selected |
|--------|-------------|----------|
| 全部免费 | 所有 Agent 免费 | |
| 支持定价与分成 | 免费/一次性/订阅 + 平台抽成 | ✓ |

**User's choice:** 支持定价与分成

### 发布者分析

| Option | Description | Selected |
|--------|-------------|----------|
| 基础发布统计 | 安装量/活跃用户/API 调用量 | ✓ |
| 无统计 | 不做发布者统计面板 | |

**User's choice:** 基础发布统计 (推荐)

### 审核流水线

| Option | Description | Selected |
|--------|-------------|----------|
| 完整审核流水线 | 提交 → 审核 → 批准/拒绝(附原因) → 上架，用户可查状态 | ✓ |
| 简化审核 | 简单列表 + 批准/拒绝按钮 | |

**User's choice:** 完整审核流水线 (推荐)

---

## Marketplace 发现与安装

### 发现机制

| Option | Description | Selected |
|--------|-------------|----------|
| 分类浏览 + 搜索 | 分类网格 + 搜索栏 + 精选/热门/最新推荐区 | ✓ |
| 列表 + 搜索 | 纯列表 + 搜索筛选 | |

**User's choice:** 分类浏览 + 搜索 (推荐)

### Agent 详情页

| Option | Description | Selected |
|--------|-------------|----------|
| 丰富详情页 | 名称/描述/图标/截图/工具/示例/评价 | ✓ |
| 简洁信息卡 | 名称/描述/图标 + 安装按钮 | |

**User's choice:** 丰富详情页 (推荐)

### 评价系统

| Option | Description | Selected |
|--------|-------------|----------|
| 5 星评分 + 评价 | 星级评分 + 文字评价 | ✓ |
| 无评价系统 | 仅按安装量排序 | |

**User's choice:** 5 星评分 + 评价 (推荐)

### 分类系统

| Option | Description | Selected |
|--------|-------------|----------|
| 预定义分类 + 标签 | Admin 预定义 + 发布者自选标签 | ✓ |
| 纯标签 | 发布者自由打标签 | |

**User's choice:** 预定义分类 + 标签 (推荐)

### 首页设计

| Option | Description | Selected |
|--------|-------------|----------|
| 精选推荐首页 | 横幅 + 精选 + 分类热门 + 搜索 | ✓ |
| 分类网格首页 | 直接分类网格 + 搜索 | |

**User's choice:** 精选推荐首页 (推荐)

### 精选管理

| Option | Description | Selected |
|--------|-------------|----------|
| 管理员手动精选 | Admin 后台选择精选 Agent | |
| 算法自动推荐 | 基于评分/安装量/活跃度自动选出 | ✓ |

**User's choice:** 算法自动推荐

### 搜索功能

| Option | Description | Selected |
|--------|-------------|----------|
| 全文搜索 + 多维度筛选 | 搜索名称描述 + 按分类/标签/评分/价格过滤 + 多排序方式 | ✓ |
| 基础搜索 | 简单名称搜索 + 分类筛选 | |

**User's choice:** 全文搜索 + 多维度筛选 (推荐)

---

## Claude's Discretion

- 加载骨架屏/错误状态/空态设计细节
- 具体 UI 间距和排版
- 算法推荐的具体权重公式
- 套餐配额具体数值
- Stripe Webhook 的重试和幂等处理细节

## Deferred Ideas

- 自定义角色编辑器（RBAC 可视化）—— 后续 Phase
- 发布者收入仪表板和提现 —— 后续 Phase
