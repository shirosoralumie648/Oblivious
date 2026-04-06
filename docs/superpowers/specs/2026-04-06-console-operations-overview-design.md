# Console Operations Overview Design

日期：2026-04-06

## 1. Goal

在不修改现有 `/api/v1/console/*` 后端契约的前提下，把当前已挂载但仍偏“分散数据页”的 Console，收敛成一条可演示、可测试、可回归的运营总览链路。

本里程碑的目标不是做真正的多租户管理控制台，而是：

- 用当前 workspace / 当前 session 的真实数据，提供一个面向个人用户与管理员阅读习惯都成立的 Console
- 让 `/console` 成为稳定的运营总览入口
- 让成本 / 用量成为最优先的 drill-down 主路径
- 让 models / access 成为辅助 drill-down 页面
- 用“管理员壳层”表达结构和导航，而不是伪造不存在的后台治理能力

## 2. Non-Goals

本里程碑明确不做以下内容：

- 不新增、删除或重定义 `/api/v1/console/*` 后端接口
- 不引入真实的多工作区、多用户、多租户管理能力
- 不在 Console 内直接执行偏好修改或治理写操作
- 不伪造趋势图、时间序列、分组统计或 API 没有提供的筛选能力
- 不改动 Chat / Knowledge / SOLO 主路径逻辑

## 3. User And Scope Model

### 3.1 Primary Audience

这一版 Console 同时服务两类阅读姿态：

- 个人工作区用户：希望快速理解当前 workspace 的成本、请求量、模型使用和访问上下文
- 管理员/运营视角用户：希望以更“控制台化”的方式阅读当前 workspace 状态，并从中跳转到对应页面处理问题

### 3.2 Data Scope

虽然阅读姿态兼顾个人和管理员，但数据范围必须始终明确为：

- 当前 workspace
- 当前 session
- 当前用户上下文

这意味着本里程碑只能构造“管理员壳层”，不能构造“管理员实权数据”。页面中需要通过文案、标签或上下文区域清晰说明当前 scope，避免把 Console 误读为团队级控制台。

## 4. Product Direction

### 4.1 Hybrid Structure

本里程碑采用 A/B 混合方案，而不是做一个统一风格的单层控制台：

- `/console` 使用 executive overview 风格
- `/console/*` drill-down 子页使用 ops workbench 风格

这样做的原因：

- 总览页的职责是“快速判断状态”和“明确下一步点击”
- drill-down 页的职责是“围绕单个主题继续阅读细节”
- 现有 API 只支持摘要型读取，不适合把首页做成重型分析工作台
- 但 drill-down 页面仍然需要更高的信息密度，来支撑“不是只看一眼”的使用场景

### 4.2 Overview First Narrative

Console 首页必须优先讲清楚这 4 件事：

1. 当前成本是否异常
2. 当前请求量是否值得关注
3. 当前最主要模型是什么
4. 当前访问与 workspace 上下文是否正确

其中成本和用量是最主要的故事线，因此对应的卡片应该成为最醒目的入口，并优先链接到对应 drill-down 页面。

## 5. Route-Level Behavior

### 5.1 `/console`

`/console` 是 Console 运营总览页。

职责：

- 聚合 `usage`、`billing`、`models`、`access` 四个现有接口
- 用总览卡片和摘要区表达当前 workspace 状态
- 提供清晰的 drill-down 入口
- 提供“管理员壳层”导航和只读快捷跳转

信息布局：

- 顶部：Console 标题、scope 提示、workspace/session 状态、跳转动作
- 第一层：4 个 KPI 卡片
  - Estimated cost
  - Requests
  - Top model
  - Access posture
- 第二层：主叙事区
  - 左侧主区：成本/用量摘要与 drill-down 提示
  - 右侧辅助区：model snapshot / billing snapshot / access snapshot

### 5.2 `/console/billing`

`/console/billing` 是成本优先的 drill-down 页面，也是从首页进入的主目标页之一。

职责：

- 展示 billing 接口返回的成本相关信息
- 用 workbench 风格提升可读性
- 提供 context rail，显示当前 workspace / session / shortcut actions
- 支持跳回 overview 或切换到 sibling drill-down 页面

### 5.3 `/console/usage`

`/console/usage` 是请求量优先的 drill-down 页面，也是首页另一条主目标页。

职责：

- 展示 usage 接口返回的请求量摘要
- 使用与 billing drill-down 一致的 workbench 信息结构
- 在上下文区补充当前 workspace / session / default mode / model strategy 等说明

### 5.4 `/console/models`

`/console/models` 是辅助 drill-down 页面，不承担首页主叙事。

职责：

- 展示模型使用摘要列表
- 从“支持性视角”补充总览页中的 top model 信息
- 保持 workbench 风格，但信息密度应服务于“理解模型使用情况”，而不是伪造更细的分析维度

### 5.5 `/console/access`

`/console/access` 是辅助 drill-down 页面，用于解释当前 session / workspace / user scope。

职责：

- 明确当前 Console 的数据边界
- 展示用户、工作区、session、默认模式等访问上下文
- 为管理员壳层提供“阅读解释面”，而不是管理写操作面

## 6. Interaction Rules

### 6.1 Allowed Actions

本里程碑只允许跳转型动作：

- 从 overview 跳到 billing / usage / models / access
- 从 drill-down 返回 overview
- 在 drill-down 子页之间切换
- 跳转到 workspace settings
- 跳转回 workspace 主路径

### 6.2 Disallowed Actions

本里程碑禁止在 Console 中直接执行以下动作：

- 修改偏好
- 修改模型策略
- 修改访问边界
- 执行管理员写操作
- 触发任何需要新增后端能力的筛选或查询

## 7. Data Flow

### 7.1 Overview Data Flow

`/console` 首页在进入时并行读取：

- `GET /api/v1/console/usage`
- `GET /api/v1/console/billing`
- `GET /api/v1/console/models`
- `GET /api/v1/console/access`

前端只做“摘要重排”，不做“新统计创造”。

允许的前端组合行为包括：

- 从 `models[0]` 提取 top model
- 把 `billing`、`usage`、`access` 映射成统一 KPI 卡片
- 把 access 数据注入 admin shell / scope 区域

不允许的组合行为包括：

- 伪造趋势图
- 伪造时间维度切换
- 伪造 workspace 对比
- 伪造团队级聚合

### 7.2 Drill-Down Data Flow

每个 drill-down 页以其主接口为核心：

- usage page -> `usage`
- billing page -> `billing`
- models page -> `models`
- access page -> `access`

若需要统一的上下文侧栏，可额外读取一次 `access`。这属于“上下文补充”，不是接口扩展。

## 8. Error And Empty-State Strategy

### 8.1 Overview

首页需要支持两类失败：

- 整页加载失败：显示 dashboard unavailable 级别的页面提示
- 局部摘要失败：对应卡片降级为 unavailable，但整体页面仍保持可进入

原因是 overview 的职责是“入口页”，不能因为某一块摘要失败就让整个 Console 失效。

### 8.2 Drill-Down

drill-down 页允许更直接的主题级失败提示，例如：

- Unable to load usage summary
- Unable to load billing summary

但页面框架和上下文区应尽量稳定，避免用户丢失导航能力。

### 8.3 Empty State

现有 console API 没有复杂明细列表，因此空态更多体现为“数值为 0”或“摘要为空”，而不是长列表无数据。页面文案应解释“当前 workspace 暂无可观测活动”，而不是暗示系统异常。

## 9. Component And File Strategy

本里程碑不改变路由树，但要把 Console 从“页面里直接堆内容”收敛成可维护的展示层。

建议新增一层 Console 展示组件，分为三类：

- overview cards
  - 负责首页 KPI 和摘要卡片
- context rail
  - 负责 workbench 页的上下文说明、scope 提示、快捷跳转
- detail sections
  - 负责 billing / usage / models / access 各自的主题内容区域

设计原则：

- executive overview 与 ops workbench 使用同一视觉语言，但信息密度不同
- 不把所有逻辑继续堆到 page 文件中
- 保持展示组件只做映射和组合，不引入新的业务状态机

## 10. Testing Strategy

### 10.1 Overview Tests

需要新增或扩展测试以锁定：

- overview 是否聚合 4 个 console 接口
- KPI 卡片是否正确渲染核心摘要
- 成本 / 用量 / top model / access posture 是否各自跳转到正确 drill-down 页
- overview 是否展示 workspace scope 和跳转型操作

### 10.2 Drill-Down Tests

需要新增或扩展测试以锁定：

- billing / usage drill-down 是否切换为 workbench 风格布局
- context rail 是否出现且包含 scope / session / shortcuts
- models / access 页是否作为辅助 drill-down 保持一致的结构语言

### 10.3 Failure And Degrade Tests

需要覆盖：

- overview 局部摘要失败时，单块降级而非整页白屏
- drill-down 接口失败时，保留基础框架和导航能力

### 10.4 Router And Contract Updates

需要同步更新：

- router smoke tests
- Console 相关页面测试
- `docs/architecture/current-system-contracts.md` 中 Console 路由状态描述

## 11. Milestone Completion Criteria

当以下条件同时满足时，本里程碑完成：

1. `/console` 被明确实现为运营总览页，而不再是占位或纯摘要拼接页
2. `/console/billing` 与 `/console/usage` 成为首页的两条主 drill-down 路径
3. `/console/models` 与 `/console/access` 成为辅助 drill-down 页面
4. Console 整体兼顾个人用户与管理员阅读姿态，但页面中明确声明当前 scope 仅为当前 workspace / session
5. 页面只包含跳转型操作，不引入写操作
6. Console 相关测试、路由烟测与契约文档全部同步更新

## 12. Recommendation

本设计推荐把下一里程碑命名为：

`Console Operations Overview`

实现上优先级如下：

1. 收敛 `/console` 首页
2. 收敛 `/console/billing` 和 `/console/usage`
3. 收敛 `/console/models` 和 `/console/access`
4. 更新文档与回归测试

这样可以先完成你最看重的“成本 / 用量”主路径，再补齐 supporting pages，而不会在一开始就把范围拖进伪管理后台。
