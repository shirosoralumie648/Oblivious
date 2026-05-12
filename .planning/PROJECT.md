# Oblivious

## What This Is

Oblivious 是一个多租户 AI 平台，整合 LobeHub（C端体验）和 New-API（B端功能）。提供 Chat、Agent 编排、知识库、多渠道 LLM Relay 等核心能力。

**技术栈**: Go 后端 (Gin) + React 前端 (Vite) + PostgreSQL + Redis

## Core Value

**统一的多渠道 LLM 调用层** — 所有 AI 调用必须经过 Relay，确保计费、限流、监控统一。

## Current Milestone: v03.2 Quality and Release

**Goal:** 把已交付的 Relay、Agent、Admin、Marketplace 能力收口到可验证、可文档化、可部署的发布候选状态。

**Target features:**
- 集成测试覆盖关键后端协作路径，尤其是 Admin/Marketplace API 与 Relay/Quota/Agent 交互。
- E2E 测试覆盖前端主要工作流，证明用户可以完成管理、浏览、安装和发布路径。
- API 文档与发布检查清单可以支持交接、验收和候选版本发布。
- Docker/Kubernetes 配置可以启动并验证当前服务栈。

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
- ✓ Phase 4 release gates: backend integration tests, Admin/Marketplace browser E2E, API/RC docs, and deployment config
- ⚠ Phase 4 deployment runtime validation remains blocked by Docker registry/proxy access and kubectl availability

### Active

<!-- v03.2 / Phase 4 需求：质量与发布 -->

- [x] TEST-01: 集成测试覆盖关键后端协作路径
- [x] TEST-02: E2E 测试覆盖核心前端工作流
- [x] DOC-01: API 文档与发布检查清单可用于验收
- [ ] DEPLOY-01: Docker/Kubernetes 配置可启动并验证当前服务栈（配置完成；真实运行时验证阻塞）

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
- v03.2 已完成质量、E2E、文档和部署配置收口
- 当前本机 Docker daemon 权限已经恢复，但 Docker image build 仍无法拉取 Docker Hub metadata；同时缺少 `kubectl`，因此 DEPLOY-01 不能视为完全完成
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
| v03.2 skips new domain research | Quality/release work is scoped by shipped code and existing Phase 4 requirements | Partial — TEST-01/TEST-02/DOC-01 closed; DEPLOY-01 runtime validation blocked |

---

*Last updated: 2026-05-12 after DEPLOY-01 recheck*
