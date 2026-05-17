# Oblivious

## What This Is

Oblivious 是一个多租户 AI 平台，整合 LobeHub（C端体验）和 New-API（B端功能）。提供 Chat、Agent 编排、知识库、多渠道 LLM Relay、Admin 管理和 Agent Marketplace 等核心能力。

**技术栈**: Go 后端 (Gin) + React 前端 (Vite) + PostgreSQL + Redis

## Core Value

**统一的多渠道 LLM 调用层** — 所有 AI 调用必须经过 Relay，确保计费、限流、监控统一。

## Current Milestone: v03.3 Mainline Consolidation

**Goal:** 将当前工作树里已经存在的主线改动整理成一致、可验证、可提交的版本，而不是扩张新的产品范围。

**Target features:**
- Route and service integration for Agent, Memory, MCP, Notification, Quota, WebSocket, and Relay-backed Chat/Agent paths.
- Auth/session/user preference contract cleanup for user name, role, admin boundaries, and app preferences.
- Deployment, CI, E2E, Playwright, and restricted-network Docker path stabilization.
- Documentation, API/system-contract reconciliation, and commit-boundary cleanup for the current mainline work.

## Current State: v03.3 Phase 7 Execution

Milestone v03.2 Quality and Release is archived, Phase 6 backend hardening is complete, and Phase 7 has a UI contract plus three execution plans ready. The repository still contains substantial uncommitted frontend, deployment, documentation, and historical/reference changes. v03.3 exists to triage that already-present work, verify the cross-cutting contracts, and split it into coherent commits without mixing unrelated cleanup into product integration.

**Shipped in v03.2:**
- 集成测试覆盖关键后端协作路径，尤其是 Admin/Marketplace API 与 Relay/Quota/Agent/Memory 交互。
- E2E 测试覆盖 Admin 与 Marketplace 的核心浏览器工作流。
- API 文档、系统契约和发布检查清单支持交接、验收和候选版本发布。
- Docker compose 可以构建、启动并健康检查当前服务栈；受限网络环境下使用文档化的 registry / Go proxy overrides。

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
- ✓ Phase 6 backend mainline integration hardening — route/auth boundaries, notification ownership, Relay metadata/tool calls, auth/session payloads, and preference defaults

### Active

- [x] CONS-01: Maintainer can classify the current uncommitted source/docs into coherent work slices and avoid mixing unrelated changes. Validated in Phase 5.
- [x] ROUTE-01: Backend route/service additions for Agent, Memory, MCP, Notification, Quota, and WebSocket are registered with explicit auth boundaries and targeted tests. Validated in Phase 6.
- [x] CHAT-06: Chat and Agent calls preserve the Relay-first contract, including structured tool calls, streaming behavior, and usage metadata. Validated in Phase 6.
- [x] AUTH-01: User/session contracts expose name and role consistently while keeping admin boundaries enforceable. Validated in Phase 6.
- [ ] DEPLOY-02: Docker, compose, CI, and Playwright changes remain aligned with the proven v03.2 restricted-network deployment path.
- [ ] DOC-02: API, architecture, release, and README docs match the live routes, commands, and verification scope.
- [ ] VERIFY-01: Maintainer can run a documented targeted verification suite before committing the mainline consolidation.

### Out of Scope

- 多租户计费商业化细节 — 需要真实支付/运营策略确认
- 大规模生产观测与告警 — 发布候选之后单独规划
- 移动端专项体验 — Web 优先
- New product discovery beyond the already-present mainline changes — v03.3 is consolidation, not feature ideation
- Full Kubernetes runtime validation — Docker compose remains the proven runtime path unless Kubernetes tooling is installed later

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
- v03.3 当前工作不是从空白开始：工作树已包含路由拆分、Agent/Memory/MCP/Notification/Quota/WebSocket、Relay Chat/Agent、Auth/UserPrefs、CI/E2E/部署和文档变更。
- Phase 5 已完成当前脏工作树分类、commit-boundary 盘点和 CONS-01 verification。
- Phase 6 已完成后端主线硬化；backend commit `ef81374` 和 `06-VERIFICATION.md` 证明 ROUTE-01、CHAT-06、AUTH-01 通过 targeted 与 DB-backed Go verification。
- 下一步是执行 Phase 7 Frontend, E2E, and Deployment Gate Alignment 的三个计划。
- 直接 Docker Hub / 默认 Go module 路径在本机网络仍不稳定；受限网络验证命令已记录在 Phase 4 summary、completion audit 和 release docs 中。
- `kubectl` 未安装，因此 Kubernetes 仍是未执行的替代路径，不影响已通过的 Docker runtime path。
- `src/web/src/routes/workspace/MarketplacePage.tsx` 是接受的 v03.1 清理债务：不再由 `/marketplace` 使用。

## Constraints

- **技术栈**: Go 1.22+, Node.js 20+, PostgreSQL 14+, Redis
- **架构**: Relay 必须作为所有 LLM 调用的统一入口
- **计费**: Agent 消耗的 Token 计入用户配额
- **隔离**: 向量检索按 user_id 隔离
- **工作树安全**: 当前存在大量未提交源码/文档改动；v03.3 规划提交不得隐式带入这些源码改动

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go 统一后端 | Agent Runtime、MCP Tools、Relay 全部 Go 重写 | ✓ Good — Phase 1/2/3 APIs built on Go |
| Relay 作为统一入口 | 计费/限流/监控统一 | ✓ Good — Chat/Agent/Quota paths route through Relay |
| pgvector 向量检索 | PostgreSQL 原生支持，运维简单 | ✓ Good — HNSW migration shipped in Phase 2 |
| Admin/Marketplace UI on shared primitives | Keep page work consistent and testable | ✓ Good — v03.1 closed with 12 focused Vitest files |
| v03.2 skips new domain research | Quality/release work is scoped by shipped code and existing Phase 4 requirements | ✓ Good — TEST-01/TEST-02/DOC-01/DEPLOY-01 closed |
| Docker compose runtime path satisfies DEPLOY-01 | Requirement accepted one real Docker or Kubernetes runtime path | ✓ Good — compose build/start/smoke passed; Kubernetes remains alternate path |
| Preserve living REQUIREMENTS.md for now | This repo still uses it for cross-phase context and has an explicit backlog item to decide reset policy | — Pending — living file remains, but v03.2 archive exists |
| v03.3 consolidates current mainline changes | The worktree already contains broad integration changes; planning should make them coherent before more feature expansion | Active — Phase 5 completed triage and commit-boundary inventory; Phase 6 completed backend hardening; Phase 7 is planned and ready to execute frontend/deployment alignment |
| v03.3 skips new research | Scope is bounded by local code/docs already present, not by a new domain or market question | Active — requirements derive from repo state |
| Phase 6 backend contracts are now the frontend/deployment baseline | Route/auth, Relay metadata/tool calls, notification ownership, auth/session, and preference defaults passed backend verification | Active — Phase 7 plans align frontend, E2E, CI, Docker, and deployment gates against this surface |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition**:
1. Requirements invalidated? Move to Out of Scope with reason.
2. Requirements validated? Move to Validated with phase reference.
3. New requirements emerged? Add to Active.
4. Decisions to log? Add to Key Decisions.
5. "What This Is" still accurate? Update if drifted.

**After each milestone**:
1. Full review of all sections.
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state.

---
*Last updated: 2026-05-17 after completing Phase 7 planning*
