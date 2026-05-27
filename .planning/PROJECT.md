# Oblivious

## What This Is

Oblivious 是一个多租户 AI 平台，整合 LobeHub（C 端体验）和 New-API（B 端运营能力）。目标能力包括 Chat、Agent 编排、知识库、多渠道 LLM Relay、Admin 管理和 Agent Marketplace。

**技术栈**: Go 后端 (Gin) + React 前端 (Vite) + PostgreSQL + Redis

## Core Value

**统一的多渠道 LLM 调用层** — 所有 AI 调用必须经过 Relay，确保计费、限流、监控统一。

## Current Milestone: v05 Relay Billing Completeness (Complete)

**Goal:** 让 Relay Authority Gate 变成可验证的生产边界：每个 `/v1/*` 路由都有商业分类，生产环境对未完成端点 fail closed，所有支持端点具备明确的认证、限流、计费、退款和审计语义。

**Target features:**
- Route policy registry covering every registered `/v1/*` endpoint.
- Production fail-closed behavior for disabled or partially implemented endpoints.
- Direct-provider bypass checks proving non-Relay services cannot call upstream LLM providers.
- Per-endpoint auth, rate-limit, billing, refund, quota settlement, and audit policy.
- v05 verification evidence that closes the Relay Authority Gate without claiming v06-v08 commercial completion.

## Current State: v06 Ready To Initialize

The commercial complete target is defined in `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`. That spec explicitly says the prior release-candidate state is not the final product.

v04 Commercial Foundation is complete: Phase 9 completed first-class organization tenants and the migration ledger, Phase 10 completed memberships/auth security, Phase 11 completed tenant scope across core domains, and Phase 12 completed reproducible DB-backed CI and commercial gate evidence.

v05 Relay Billing Completeness is complete. Phase 13 completed Relay endpoint classification and production fail-closed behavior. Phase 14 completed provider-bypass CI checks, supported endpoint auth/tenant identity policy, rate-limit policy, route-decision audit semantics, and trusted Relay metadata for Chat, Agent, and Knowledge embedding paths. Phase 15 completed quota preauthorization, exactly-once settlement, refund behavior, provider usage parsing, explicit route billing policy, and streaming/async production-disablement evidence. Phase 16 completed Relay route-table evidence, commercial gate closeout, DB-backed verification, and v05 milestone snapshots.

The next commercial-program milestone is v06 Billing And Marketplace Operations. The overall commercial-complete SaaS objective remains open until v06, v07, and v08 are also complete and verified.

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
- ✓ v04 Commercial Foundation — tenant model, membership/auth security, tenant-scoped domains, DB-backed CI, and commercial gate evidence
- ✓ RELAY-08 — Every registered `/v1/*` route is classified as commercial-supported and billed, internal/admin-only, or disabled in production
- ✓ RELAY-09 — Unsupported or partially implemented `/v1/*` endpoints fail closed in production before any upstream provider call
- ✓ RELAY-10 — CI proves app services do not import provider SDKs or call direct provider URLs outside Relay/channel adapters
- ✓ RELAY-11 — Supported Relay endpoints enforce tenant identity, auth policy, rate-limit policy, and audit semantics
- ✓ BILL-01 — Supported Relay calls pre-authorize quota, settle exactly once per idempotency key, and refund failed or partial calls
- ✓ BILL-02 — Streaming/realtime, file, batch, and async flows have explicit settlement models or are production-disabled

### Active

- No active v05 requirements remain. v06 requirements should be initialized from `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`.

### Out of Scope For v05

- Full Stripe production rollout, subscription billing, top-ups, refunds, invoices, and Marketplace payout accounting.
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
- v04 Commercial Foundation 已完成。
- v05 Relay Billing Completeness 已完成；下一步是初始化或规划 v06 Billing And Marketplace Operations。
- 直接 Docker Hub / 默认 Go module 路径在本机网络仍不稳定；受限网络验证命令继续作为部署 smoke 的已验证本地路径。
- `kubectl` 未安装，因此 Kubernetes 仍属于后续 v07 Production Operations 的未验证范围。

## Constraints

- **技术栈**: Go 1.22+, Node.js 20+, PostgreSQL 14+, Redis
- **架构**: Relay 必须作为所有 LLM 调用的统一入口
- **计费**: Chat/Agent/Relay 消耗必须最终进入统一 quota/billing 账本
- **隔离**: 组织/租户隔离必须继续作为所有商业功能的安全边界
- **工作树安全**: 当前根工作树存在 unrelated dirty/untracked 文件；商业规划和后续实现提交必须保持窄范围并继续在 active worktree 中推进
- **商业完成定义**: 任何 milestone 只能在对应 gate 有当前仓库证据、自动化验证和必要 runtime smoke 后才可宣称完成

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go 统一后端 | Agent Runtime、MCP Tools、Relay 全部 Go 重写 | ✓ Good — Phase 1/2/3 APIs built on Go |
| Relay 作为统一入口 | 计费/限流/监控统一 | Active — v05 now proves this for every commercial AI surface |
| pgvector 向量检索 | PostgreSQL 原生支持，运维简单 | ✓ Good — HNSW migration shipped in Phase 2 |
| Admin/Marketplace UI on shared primitives | Keep page work consistent and testable | ✓ Good — v03.1 closed with focused Vitest coverage |
| Docker compose runtime path satisfies DEPLOY-01 | Requirement accepted one real Docker or Kubernetes runtime path | ✓ Good — compose build/start/smoke passed; Kubernetes remains later |
| Preserve living REQUIREMENTS.md | This repo uses it for cross-phase context and archives milestone snapshots separately | ✓ Good — Phase 999.2 recorded this policy |
| Commercial target is a milestone program, not one giant phase | Tenant/security, Relay billing, money movement, operations, and product completeness have hard dependencies | Active — v04 through v08 decomposes the work |
| v04 starts with tenant/security foundation | Billing, Marketplace payouts, and production ops need tenant identity and isolation first | ✓ Good — v04 completed tenant/security/migration/CI foundation |
| v05 starts with route policy and fail-closed enforcement | Billing semantics are unsafe until every `/v1/*` route has an explicit commercial class and production behavior | Active — `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-01-PLAN.md` |
| Phase 13 disables partial Relay endpoints in production first | Passthrough/file/async endpoints must not reach providers before billing/audit/settlement semantics exist | ✓ Good — `3b9d4dd` and `docs/release/relay-route-table.md` |
| Phase 14 makes provider bypass and supported-route identity testable before settlement | Billing cannot be trusted if app services can still bypass Relay or supported routes lack tenant identity/audit policy | ✓ Good — `scripts/verify-relay-security.sh`, trusted internal identity guard, route-decision audit sink, and Memory Relay metadata tests |
| Phase 15 makes settlement/refund explicit before v05 closeout | Relay Authority Gate cannot close while supported calls can strand quota or streaming/async flows bypass settlement | ✓ Good — `RouteWithBilling` lifecycle tests, `BillingPolicy` route coverage, provider usage parsing, and route-table evidence |
| Phase 16 closes v05 with evidence rather than new runtime behavior | Relay behavior changed in Phases 13-15; v05 closeout needs reproducible proof and must not imply final commercial readiness | ✓ Good — `16-VERIFICATION.md`, route table/gate docs, docs gate assertions, DB-backed script verification, and milestone snapshots |

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
*Last updated: 2026-05-28 after completing Phase 16 Relay Authority evidence and v05 closeout*
