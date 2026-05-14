# Oblivious

## What This Is

Oblivious 是一个多租户 AI 平台，整合 LobeHub（C端体验）和 New-API（B端功能）。提供 Chat、Agent 编排、知识库、多渠道 LLM Relay、Admin 管理和 Agent Marketplace 等核心能力。

**技术栈**: Go 后端 (Gin) + React 前端 (Vite) + PostgreSQL + Redis

## Core Value

**统一的多渠道 LLM 调用层** — 所有 AI 调用必须经过 Relay，确保计费、限流、监控统一。

## Current State: v03.2 Shipped

Milestone v03.2 Quality and Release is archived. The project has a release-candidate baseline with backend release gates, browser E2E, API/release documentation, and a proven Docker compose startup smoke path.

**Shipped in v03.2:**
- 集成测试覆盖关键后端协作路径，尤其是 Admin/Marketplace API 与 Relay/Quota/Agent/Memory 交互。
- E2E 测试覆盖 Admin 与 Marketplace 的核心浏览器工作流。
- API 文档、系统契约和发布检查清单支持交接、验收和候选版本发布。
- Docker compose 可以构建、启动并健康检查当前服务栈；受限网络环境下使用文档化的 registry / Go proxy overrides。

## Next Milestone Goals

Next milestone is not defined yet. Use `$gsd:new-milestone` to choose the next product direction and refresh active requirements.

Likely candidate areas from current accepted debt:
- Phase 01 summary reconstruction and planning artifact cleanup.
- Legacy workspace Marketplace route cleanup.
- Production operations hardening beyond the current release-candidate Docker validation.
- Decide whether future milestone closes should reset `.planning/REQUIREMENTS.md` or preserve it as living cross-phase context.

## Requirements

### Validated

- ✓ 用户认证与会话管理 — Auth/Session 模块
- ✓ Chat 会话与消息管理 — Chat 模块
- ✓ 知识库基础存储 — Knowledge 模块
- ✓ 用户偏好设置 — Preferences 模块
- ✓ 控制台使用量展示 — Console 模块
- ✓ Relay 独立模块 — Handler + Router + Billing + Metrics
- ✓ 前端骨架 — 营销页、工作区、控制台页面
- ✓ Relay 挂载、Chat RelayGateway、Agent Runtime、MCP Client — Phase 1
- ✓ Agent 工具循环、Memory HNSW、Quota-Billing 串联 — Phase 2
- ✓ Admin 与 Marketplace 后端 API — Phase 3a
- ✓ Admin 管理面板与 Agent Marketplace UI — v03.1
- ✓ Phase 4 release gates: backend integration tests, Admin/Marketplace browser E2E, API/RC docs, and deployment config
- ✓ Docker compose runtime validation — v03.2 DEPLOY-01 passed with real build/start/smoke evidence

### Active

- [ ] Define the next milestone with `$gsd:new-milestone`.

### Out of Scope

- 多租户计费商业化细节 — 需要真实支付/运营策略确认
- 大规模生产观测与告警 — 发布候选之后单独规划
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
    ├── Service Layer (Auth/Chat/Agent/Knowledge/Task/Memory/Admin/Marketplace)
    ├── Relay Layer (已挂载为统一 LLM 调用入口)
    └── Data Layer (PostgreSQL + MongoDB)
```

**当前状态**:
- v03.1 已交付可用 Admin UI 与 Marketplace UI。
- v03.2 已完成质量、E2E、文档和 Docker 部署 smoke 收口。
- 直接 Docker Hub / 默认 Go module 路径在本机网络仍不稳定；受限网络验证命令已记录在 Phase 4 summary、completion audit 和 release docs 中。
- `kubectl` 未安装，因此 Kubernetes 仍是未执行的替代路径，不影响已通过的 Docker runtime path。
- `src/web/src/routes/workspace/MarketplacePage.tsx` 是接受的 v03.1 清理债务：不再由 `/marketplace` 使用。

## Constraints

- **技术栈**: Go 1.22+, Node.js 20+, PostgreSQL 14+, Redis
- **架构**: Relay 必须作为所有 LLM 调用的统一入口
- **计费**: Agent 消耗的 Token 计入用户配额
- **隔离**: 向量检索按 user_id 隔离

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go 统一后端 | Agent Runtime、MCP Tools、Relay 全部 Go 重写 | ✓ Good — Phase 1/2/3 APIs built on Go |
| Relay 作为统一入口 | 计费/限流/监控统一 | ✓ Good — Chat/Agent/Quota paths route through Relay |
| pgvector 向量检索 | PostgreSQL 原生支持，运维简单 | ✓ Good — HNSW migration shipped in Phase 2 |
| Admin/Marketplace UI on shared primitives | Keep page work consistent and testable | ✓ Good — v03.1 closed with 12 focused Vitest files |
| v03.2 skips new domain research | Quality/release work is scoped by shipped code and existing Phase 4 requirements | ✓ Good — TEST-01/TEST-02/DOC-01/DEPLOY-01 closed |
| Docker compose runtime path satisfies DEPLOY-01 | Requirement accepted one real Docker or Kubernetes runtime path | ✓ Good — compose build/start/smoke passed; Kubernetes remains alternate path |
| Preserve living REQUIREMENTS.md for now | This repo still uses it for cross-phase context and has an explicit backlog item to decide reset policy | — Pending — v03.2 archive was created, but living file remains |

---
*Last updated: 2026-05-14 after v03.2 milestone archive*
