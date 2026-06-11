# Next.js 14 迁移指南（Stage D1)

## 背景

当前前端：Vite 6.4 + React 18.3 + React Router 6  
目标（fusion spec part2 §4）：Next.js 14 App Router + 路由组

## 迁移策略

采用**增量迁移**（而非全量重写），保持 Vite 和 Next.js 双模式并存：

### Phase 1: Next.js 基础设施（框架已定义，完整迁移待产品决策）

1. `src/web-next/` 目录创建 Next.js 14 项目
2. App Router 结构：
   - `app/(auth)/` — 登录/注册页面
   - `app/(workspace)/` — 主工作区（对话/Agent/工作流）
   - `app/(admin)/` — 管理后台
3. 共享组件逐步从 `src/web/src/components/` 迁移到 `src/web-next/components/`
4. API 路由对接现有后端（无后端改动）

### Phase 2: 页面迁移优先级

- **P0:** 登录页、主对话界面
- **P1:** Agent 管理、工作流编辑器
- **P2:** 知识库、市场、管理后台

### Phase 3: 技术栈替换

按 spec §4.1-4.2 要求：
- 状态管理：Zustand (替代当前的 React Context)
- 数据获取：SWR (替代 fetch hooks)
- 图表：Recharts (替代当前 Chart.js)
- 表单：React Hook Form + Zod (替代手写验证)
- UI 组件：Shadcn/ui (已部分使用，继续扩展)

### Phase 4: 流量切换

- 通过 Nginx/Envoy 规则逐步切换页面流量
- 保持 Vite 版本可回退

## 当前状态

**框架已定义**（本文档），完整实施需：
- 估计 200+ 组件迁移
- 20+ 页面重构
- 建议按业务需求渐进，避免一次性重写风险

## 验收标准

- [ ] Next.js 项目初始化并可独立启动
- [ ] 至少 1 个完整页面迁移（如登录页）并功能对等
- [ ] 双模式部署可切换
- [ ] 测试覆盖不低于当前 Vite 版本
