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

### Active

<!-- Phase 1 需求：Relay 集成与基础能力 -->

- [ ] Relay 挂载到主应用 — `/v1/*` 路由走 Relay
- [ ] Chat 通过 Relay 调用 LLM — 替换本地 ReplyGenerator
- [ ] Agent Runtime 核心 — 创建、管理 Agent 并对话
- [ ] MCP Client 骨架 — 工具发现和调用

### Out of Scope

<!-- Phase 2-4 内容，本次不包含 -->

- Memory/RAG 向量检索 (Phase 2)
- Agent 工具执行与 MCP 串联 (Phase 2)
- Admin API 与 UI (Phase 3)
- Marketplace (Phase 3)
- 端到端测试与发布 (Phase 4)

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
    ├── Relay Layer (独立模块，未集成)
    └── Data Layer (PostgreSQL + MongoDB)
```

**关键问题**:
- Relay 模块已实现但未挂载到主应用
- Chat 使用本地 ReplyGenerator，未走 Relay
- 无 Agent Runtime，无法创建和管理 Agent
- Knowledge 是基础存储，非向量 RAG

## Constraints

- **技术栈**: Go 1.22+, Node.js 20+, PostgreSQL 14+, Redis
- **架构**: Relay 必须作为所有 LLM 调用的统一入口
- **计费**: Agent 消耗的 Token 计入用户配额
- **隔离**: 向量检索按 user_id 隔离 (Phase 2)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go 统一后端 | Agent Runtime、MCP Tools、Relay 全部 Go 重写 | — Pending |
| Relay 作为统一入口 | 计费/限流/监控统一 | — Pending |
| pgvector 向量检索 | PostgreSQL 原生支持，运维简单 | — Pending |

---

*Last updated: 2026-04-27 after initialization*
