---
gsd_state_version: 1.0
milestone: v04
milestone_name: Commercial Foundation
status: planning
stopped_at: Phase 9 planned and ready to execute
last_updated: "2026-05-27T22:30:00+08:00"
last_activity: 2026-05-27 -- v04 Commercial Foundation initialized
progress:
  total_phases: 4
  completed_phases: 0
  total_plans: 1
  completed_plans: 0
  percent: 0
---

# STATE.md

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-27)

**Core value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

**Current focus:** Phase 9 execution

## Current Status

**Milestone v04: Commercial Foundation — ACTIVE**

v03.3 Mainline Consolidation is complete and archived through snapshots under `.planning/milestones/`. The commercial complete program is defined in `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`; v04 starts that program by establishing tenant identity, production auth hardening, migration ledger, DB-backed CI, and commercial gate documentation.

## Current Position

Phase: 9 — Tenant Model and Migration Ledger
Plan: 09-01 planned
Status: Ready to execute
Last activity: 2026-05-27 -- v04 Commercial Foundation initialized

## Current Scope

| Requirement | Status | Target |
|-------------|--------|--------|
| TENANT-01 | Planned | Admin can create and manage organizations as first-class tenants |
| TENANT-02 | Planned | User can belong to multiple organizations with member, admin, and owner roles |
| TENANT-03 | Planned | User can invite, accept, remove, and transfer organization ownership with audit events |
| TENANT-04 | Planned | Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, and Marketplace publisher data are scoped by tenant |
| TENANT-05 | Planned | Tests prove cross-tenant access is denied for representative read and write paths |
| SEC-01 | Planned | Cookie-authenticated mutating routes require CSRF protection |
| SEC-02 | Planned | Login, registration, password reset, and sensitive admin actions are rate limited |
| SEC-03 | Planned | Password policy and session rotation are enforced |
| MIGR-01 | Planned | Migration runner records applied migrations in `schema_migrations` |
| CI-01 | Planned | CI server job runs DB-backed HTTP integration tests instead of silently skipping persistence coverage |
| DOC-03 | Planned | Commercial gate documentation defines what must be true before future milestones claim commercial readiness |

## Next Suggested Step

Execute `.planning/phases/09-tenant-model-and-migration-ledger/09-01-PLAN.md`.

Recommended Phase 9 baseline:
- Use `organizations` as the first-class tenant table.
- Add `schema_migrations` as the append-only migration ledger.
- Keep user-owned records backward-compatible until Phase 11 migrates core domains.
- Expose tenant identity through authenticated request context, not client-controlled request bodies.
- Prove idempotent migrations and organization CRUD through DB-backed tests.

## Worktree Context

Current dirty/untracked files outside `.planning` appear unrelated to v04 initialization and must not be staged unless a later phase explicitly owns them. Root historical/reference docs such as `CURRENT_STATUS.md`, `ROADMAP.md`, `ARCHAEOLOGY_REPORT.md`, and `docs/superpowers/*` should not override `.planning` state unless deliberately incorporated.

## Completed Work

| Milestone | Completed | Requirements |
|-----------|-----------|--------------|
| Phase 1 Relay/Chat/Agent/MCP foundation | 2026-04-27 | RELAY-01~07, CHAT-01~05, AGENT-01~10, MCP-01~07 |
| Phase 2 Agent 与 Memory 增强 | 2026-04-28 | EXEC-01~03, MEM-01~03, QUOTA-01 |
| Phase 3a Admin 与 Marketplace 后端 | 2026-04-29 | ADMIN-01~03, MARKET-01 |
| v03.1 Admin 与 Marketplace UI | 2026-05-02 | ADMIN-04, MARKET-02 |
| v03.2 Quality and Release | 2026-05-14 | TEST-01, TEST-02, DOC-01, DEPLOY-01 |
| v03.3 Mainline Consolidation | 2026-05-27 | CONS-01, ROUTE-01, CHAT-06, AUTH-01, DEPLOY-02, DOC-02, VERIFY-01 |

## Deferred Commercial Program Items

These are deliberately outside v04 and remain required for the final user goal:

| Milestone | Item |
|-----------|------|
| v05 Relay Billing Completeness | Classify every `/v1/*` endpoint, prove Relay-only provider access, and implement endpoint auth/rate-limit/billing/audit behavior |
| v06 Billing And Marketplace Operations | Stripe production routes/webhooks, subscription lifecycle, top-ups, refunds, invoices, Marketplace settlement, payouts, and moderation |
| v07 Production Operations | Kubernetes/equivalent orchestration proof, backup/restore smoke, logs, metrics, tracing, alerts, dashboards, runbooks, release/rollback |
| v08 Product Completeness | Real or disabled built-in MCP tools, durable Agent workflows, Knowledge behavior matching product copy, commercial UX, docs, onboarding, pricing |

## Context Files

- Project: `.planning/PROJECT.md`
- Requirements: `.planning/REQUIREMENTS.md`
- Roadmap: `.planning/ROADMAP.md`
- Phase 9 context: `.planning/phases/09-tenant-model-and-migration-ledger/09-CONTEXT.md`
- Phase 9 plan: `.planning/phases/09-tenant-model-and-migration-ledger/09-01-PLAN.md`
- Commercial complete spec: `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`
- v03.3 roadmap archive: `.planning/milestones/v03.3-ROADMAP.md`
- v03.3 requirements archive: `.planning/milestones/v03.3-REQUIREMENTS.md`
- v03.3 state archive: `.planning/milestones/v03.3-STATE.md`
- Codebase Map: `.planning/codebase/`

## Key Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-27 | 从 Phase 1 开始执行 | Relay 集成是所有后续工作的基础 |
| 2026-05-02 | 完成 v03.1 时保留 living REQUIREMENTS.md | 当前仓库仍依赖该文件承载跨阶段上下文，直接删除风险高 |
| 2026-05-12 | DEPLOY-01 通过 Docker compose 真实运行验证 | 使用 documented registry / Go proxy overrides 后，`scripts/deploy-validate.sh` 完成镜像构建、compose 启动和 `/healthz` smoke |
| 2026-05-14 | v03.3 聚焦 Mainline Consolidation | 工作树已有大批主线改动，先分类、验证、对齐文档和提交边界 |
| 2026-05-27 | Phase 999.2 verify-work passed | UAT verified MarketplacePage cleanup, living requirements policy, and debt traceability; milestone v03.3 is complete |
| 2026-05-27 | v04 Commercial Foundation initialized manually | `gsd-sdk init.new-milestone` still points phase archive metadata at v03.2, so manual planning updates avoid unsafe phase directory movement |
| 2026-05-27 | Phase 9 planned | Locked organization tenant model, migration ledger, admin organization routes, and DB-backed verification requirements |

---
*State updated: 2026-05-27 after initializing v04 Commercial Foundation*
