# Oblivious

## What This Is

Oblivious 是一个多租户 AI 平台，整合 LobeHub（C 端体验）和 New-API（B 端运营能力）。目标能力包括 Chat、Agent 编排、知识库、多渠道 LLM Relay、Admin 管理和 Agent Marketplace。

**技术栈**: Go 后端 (Gin) + React 前端 (Vite) + PostgreSQL + Redis

## Core Value

**统一的多渠道 LLM 调用层** — 所有 AI 调用必须经过 Relay，确保计费、限流、监控统一。

## Current Milestone: v04 Commercial Foundation

**Goal:** 建立商业 SaaS 完全体所需的租户、安全、迁移和 CI 地基，使后续 Relay 计费、Marketplace 结算、生产运维和产品完整性工作有可信边界。

**Target features:**
- Organization/tenant model with explicit membership, roles, invitations, ownership transfer, and audit events.
- Tenant-scoped data contract for Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, and Marketplace publisher data.
- Production auth hardening: CSRF protection, rate limits, password policy, and session rotation.
- Append-only migration ledger and DB-backed CI integration guarantee.
- Commercial gate documentation that prevents future milestones from claiming commercial readiness without evidence.

## Current State: v04 Commercial Foundation Complete

Milestone v03.3 Mainline Consolidation is complete. Phase 8 reconciled contract docs and release verification, Phase 999.1 reconstructed the missing Phase 01 summary, and Phase 999.2 verified the obsolete workspace MarketplacePage cleanup plus the living requirements close policy.

The commercial complete target is defined in `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`. That spec explicitly says the prior release-candidate state is not the final product. v04 Commercial Foundation is complete: Phase 9 completed first-class organization tenants and the migration ledger, Phase 10 completed memberships/auth security, Phase 11 completed tenant scope across core domains, and Phase 12 completed reproducible DB-backed CI and commercial gate evidence. The next commercial-program milestone is v05 Relay Billing Completeness.

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
- ✓ v03.2 Quality and Release — backend integration, E2E, API/RC docs, and Docker compose runtime validation
- ✓ v03.3 Mainline Consolidation — commit-boundary triage, backend hardening, frontend/E2E/deployment alignment, contract docs, release verification, and accepted cleanup debt closeout
- ✓ TENANT-01 — Admin can create and manage organizations as first-class tenants
- ✓ MIGR-01 — Migration runner records applied migrations in `schema_migrations`
- ✓ TENANT-02 — User can belong to multiple organizations with member, admin, and owner roles
- ✓ TENANT-03 — User can invite, accept, revoke, remove, and transfer organization ownership with audit events
- ✓ TENANT-04 — Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, and Marketplace publisher data are scoped by tenant
- ✓ TENANT-05 — Tests prove cross-tenant access is denied for representative read and write paths
- ✓ SEC-01 — Cookie-authenticated mutating routes require CSRF protection
- ✓ SEC-02 — Login, registration, password reset, and sensitive admin/organization actions are rate limited
- ✓ SEC-03 — Password policy and session rotation are enforced
- ✓ CI-01 — CI server job runs DB-backed HTTP integration tests instead of silently skipping persistence coverage
- ✓ DOC-03 — Commercial gate documentation defines what must be true before any future milestone can claim commercial readiness

### Active

No active v04 requirements remain. v05-v08 future requirements remain active for the full commercial-complete program.

### Out of Scope For v04

- Full Stripe production rollout, subscription billing, top-ups, refunds, invoices, and Marketplace payout accounting.
- All Relay endpoint billing completion and production fail-closed endpoint classification.
- Kubernetes or equivalent production orchestration proof.
- Knowledge RAG upgrade and Agent workflow expansion.
- Mobile-specific experience; web remains the primary control plane.

## Context

**整合背景**: 项目是 LobeHub (B2C) + New-API (B2B) 整合到同一主线的中间产物。当前商业目标不是继续打磨 RC，而是把它推进到可直接部署商用的 SaaS 完全体。

**当前架构**:
```
React Frontend
    │ HTTP/WebSocket
    ▼
Go Backend (Gin)
    ├── API Gateway (Auth/CORS/Recovery)
    ├── Service Layer (Auth/Chat/Agent/Knowledge/Task/Memory/Admin/Marketplace)
    ├── Relay Layer (所有 AI 调用的统一入口)
    └── Data Layer (PostgreSQL + MongoDB)
```

**当前状态**:
- v03.1 已交付可用 Admin UI 与 Marketplace UI。
- v03.2 已完成质量、E2E、文档和 Docker 部署 smoke 收口。
- v03.3 已完成主线整合、文档对齐、发布验证和两个历史 cleanup backlog。
- v04 Commercial Foundation 已完成；下一步是启动 v05 Relay Billing Completeness。
- 直接 Docker Hub / 默认 Go module 路径在本机网络仍不稳定；受限网络验证命令继续作为部署 smoke 的已验证本地路径。
- `kubectl` 未安装，因此 Kubernetes 仍属于后续 v07 Production Operations 的未验证范围。

## Constraints

- **技术栈**: Go 1.22+, Node.js 20+, PostgreSQL 14+, Redis
- **架构**: Relay 必须作为所有 LLM 调用的统一入口
- **计费**: Chat/Agent/Relay 消耗必须最终进入统一 quota/billing 账本
- **隔离**: v04 必须从 user/workspace 隔离升级到 organization/tenant 隔离
- **工作树安全**: 当前存在 unrelated dirty/untracked 文件；商业规划和后续实现提交必须保持窄范围
- **商业完成定义**: 任何 milestone 只能在对应 gate 有当前仓库证据、自动化验证和必要 runtime smoke 后才可宣称完成

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go 统一后端 | Agent Runtime、MCP Tools、Relay 全部 Go 重写 | ✓ Good — Phase 1/2/3 APIs built on Go |
| Relay 作为统一入口 | 计费/限流/监控统一 | ✓ Good — Chat/Agent/Quota paths route through Relay |
| pgvector 向量检索 | PostgreSQL 原生支持，运维简单 | ✓ Good — HNSW migration shipped in Phase 2 |
| Admin/Marketplace UI on shared primitives | Keep page work consistent and testable | ✓ Good — v03.1 closed with focused Vitest coverage |
| Docker compose runtime path satisfies DEPLOY-01 | Requirement accepted one real Docker or Kubernetes runtime path | ✓ Good — compose build/start/smoke passed; Kubernetes remains later |
| Preserve living REQUIREMENTS.md | This repo uses it for cross-phase context and archives milestone snapshots separately | ✓ Good — Phase 999.2 recorded this policy |
| Commercial target is a milestone program, not one giant phase | Tenant/security, Relay billing, money movement, operations, and product completeness have hard dependencies | Active — v04 through v08 decomposes the work |
| v04 starts with tenant/security foundation | Billing, Marketplace payouts, and production ops need tenant identity and isolation first | ✓ Good — v04 completed tenant/security/migration/CI foundation |
| v04 requirement `DOC-01` draft is recorded as `DOC-03` | Historical `DOC-01` and `DOC-02` already exist in living requirements | Active — avoids duplicate requirement IDs |
| Phase 10 is the enforceable security boundary before tenant data scoping | Tenant-scoped data migration needs membership, roles, CSRF, rate limits, password policy, and session rotation first | ✓ Good — `.planning/phases/10-membership-roles-and-auth-security/10-01-SUMMARY.md` |
| Phase 11 starts with session-derived active organization scope | Multi-organization users need a server-authoritative tenant before core data can be safely migrated | ✓ Good — `.planning/phases/11-tenant-scope-across-core-domains/11-01-SUMMARY.md` |
| Phase 12 must preserve the commercial gate | v04 cannot claim commercial readiness just because tenant isolation now passes | ✓ Good — `docs/release/commercial-gates.md` and `12-VERIFICATION.md` |

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
*Last updated: 2026-05-28 after completing v04 Commercial Foundation*
