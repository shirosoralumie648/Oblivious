# Oblivious

## What This Is

Oblivious 是一个多租户 AI 平台，整合 LobeHub（C端体验）和 New-API（B端功能）。提供 Chat、Agent 编排、知识库、多渠道 LLM Relay 等核心能力。

**技术栈**: Go 后端 (Gin) + React 前端 (Vite) + PostgreSQL + Redis

## Core Value

**统一的多渠道 LLM 调用层** — 所有 AI 调用必须经过 Relay，确保计费、限流、监控统一。

## Requirements

### Validated

<!-- 从现有代码推断的已实现能力 -->

- ✓ 用户认证与会话管理 — Auth/Session 模块 (86%)
- ✓ Chat 会话与消息管理 — Chat 模块 (78%)
- ✓ 知识库基础存储 — Knowledge 模块 (76%，非向量)
- ✓ 用户偏好设置 — Preferences 模块 (82%)
- ✓ 控制台使用量展示 — Console 模块 (72%)
- ✓ Relay 独立模块 — Handler + Router + Billing + Metrics (70%)
- ✓ 前端骨架 — 营销页、工作区、控制台页面 (80%)
- ✓ Relay 挂载、Chat RelayGateway、Agent Runtime、MCP Client — Phase 1
- ✓ Agent 工具循环、Memory HNSW、Quota-Billing 串联 — Phase 2
- ✓ Admin 与 Marketplace 后端 API — Phase 3a
- ✓ Admin 管理面板与 Agent Marketplace UI — v03.1

### Active

<!-- Phase 4 需求：质量与发布 -->

- [ ] 集成测试与端到端测试
- [ ] API 文档与发布检查清单
- [ ] Docker/Kubernetes 部署配置
- [ ] 清理 v03.1 接受的非阻塞债务

### Out of Scope

<!-- 后续版本内容 -->

- 多租户计费商业化细节 — 需要真实支付/运营策略确认
- 大规模生产观测与告警 — 发布前单独规划
- 移动端专项体验 — Web 优先

## Context

**整合背景**: 项目是 LobeHub (B2C) + New-API (B2B) 整合的中间产物。

**当前架构**:
```
React Frontend
    │ HTTP/WebSocket
    ▼
Go Backend (Gin)
    ├── API Gateway (Auth/CORS/Recovery)
    ├── Service Layer (Auth/Chat/Agent/Knowledge/Task/Memory)
    ├── Relay Layer (已挂载为统一 LLM 调用入口)
    └── Data Layer (PostgreSQL + MongoDB)
```

**关键问题**:
- Phase 03.1 已交付可用 Admin UI 与 Marketplace UI
- Phase 4 之前仍缺少完整 E2E、发布文档和部署验证
- `src/web/src/routes/workspace/MarketplacePage.tsx` 是接受的 v03.1 清理债务：不再由 `/marketplace` 使用

## Constraints

- **技术栈**: Go 1.22+, Node.js 20+, PostgreSQL 14+, Redis
- **架构**: Relay 必须作为所有 LLM 调用的统一入口
- **计费**: Agent 消耗的 Token 计入用户配额
- **隔离**: 向量检索按 user_id 隔离 (Phase 2)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go 统一后端 | Agent Runtime、MCP Tools、Relay 全部 Go 重写 | ✓ Good — Phase 1/2/3 APIs built on Go |
| Relay 作为统一入口 | 计费/限流/监控统一 | ✓ Good — Chat/Agent/Quota paths route through Relay |
| pgvector 向量检索 | PostgreSQL 原生支持，运维简单 | ✓ Good — HNSW migration shipped in Phase 2 |
| Admin/Marketplace UI on shared primitives | Keep page work consistent and testable | ✓ Good — v03.1 closed with 12 focused Vitest files |

---

*Last updated: 2026-05-02 after v03.1 milestone*
