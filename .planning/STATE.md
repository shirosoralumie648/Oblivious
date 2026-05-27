---
gsd_state_version: 1.0
milestone: v05
milestone_name: Relay Billing Completeness
status: planning
stopped_at: Phase 14 ready to plan
last_updated: "2026-05-28T06:20:00+08:00"
last_activity: 2026-05-28 -- Phase 13 Relay endpoint authority completed
progress:
  total_phases: 4
  completed_phases: 1
  total_plans: 1
  completed_plans: 1
  percent: 25
---

# STATE.md

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-28)

**Core value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

**Current focus:** Phase 14 planning

## Current Status

**Milestone v05: Relay Billing Completeness — ACTIVE**

v04 Commercial Foundation is complete and archived through `.planning/milestones/v04-*`. v05 now starts the Relay Authority Gate from `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`.

The overall commercial-complete objective remains open. v05 must prove the Relay invariant across every commercial `/v1/*` surface before v06 Billing And Marketplace Operations, v07 Production Operations, and v08 Product Completeness can finish the final SaaS target.

## Current Position

Phase: 14 — Relay Provider Bypass and Cost-Abuse Guardrails
Plan: not created
Status: Ready to plan
Last activity: 2026-05-28 -- Phase 13 completed with route policy registry and production fail-closed behavior

## Current Scope

| Requirement | Status | Target |
|-------------|--------|--------|
| RELAY-08 | Complete | Every registered `/v1/*` route is classified as commercial-supported and billed, internal/admin-only, or disabled in production |
| RELAY-09 | Complete | Unsupported or partial `/v1/*` endpoints fail closed in production before provider calls |
| RELAY-10 | Planned | CI proves app services do not import provider SDKs or call direct provider URLs outside Relay/channel adapters |
| RELAY-11 | Planned | Supported Relay endpoints enforce tenant identity, auth policy, rate-limit policy, and audit semantics |
| BILL-01 | Planned | Supported Relay calls pre-authorize quota, settle exactly once per idempotency key, and refund failed calls |
| BILL-02 | Planned | Streaming/realtime, file, batch, and async flows have explicit settlement models or are production-disabled |
| DOC-04 | Planned | Relay route table, endpoint policy, and v05 verification evidence document the commercial Relay Authority Gate |

## Next Suggested Step

Plan Phase 14 Relay Provider Bypass and Cost-Abuse Guardrails.

Phase 14 should start from the Phase 13 route policy registry and add provider-bypass checks, endpoint auth/tenant identity policy, rate-limit guardrails, and Relay audit semantics.

## Worktree Context

Continue in `.worktrees/phase-10-membership-auth-security` on branch `gsd/phase-10-membership-auth-security`. The root `main` worktree is behind this branch and has unrelated dirty/untracked files; do not use it for v05 implementation unless the branch is merged or the user directs a switch.

`gsd-sdk query init.new-milestone` still reports stale phase archive metadata under v03.2, so v05 planning is being maintained from local `.planning` truth rather than unsafe helper-driven movement.

## Completed Work

| Milestone | Completed | Requirements |
|-----------|-----------|--------------|
| Phase 1 Relay/Chat/Agent/MCP foundation | 2026-04-27 | RELAY-01~07, CHAT-01~05, AGENT-01~10, MCP-01~07 |
| Phase 2 Agent 与 Memory 增强 | 2026-04-28 | EXEC-01~03, MEM-01~03, QUOTA-01 |
| Phase 3a Admin 与 Marketplace 后端 | 2026-04-29 | ADMIN-01~03, MARKET-01 |
| v03.1 Admin 与 Marketplace UI | 2026-05-02 | ADMIN-04, MARKET-02 |
| v03.2 Quality and Release | 2026-05-14 | TEST-01, TEST-02, DOC-01, DEPLOY-01 |
| v03.3 Mainline Consolidation | 2026-05-27 | CONS-01, ROUTE-01, CHAT-06, AUTH-01, DEPLOY-02, DOC-02, VERIFY-01 |
| v04 Commercial Foundation | 2026-05-28 | TENANT-01~05, SEC-01~03, MIGR-01, CI-01, DOC-03 |

## Deferred Commercial Program Items

These remain required for the final user goal:

| Milestone | Item |
|-----------|------|
| v06 Billing And Marketplace Operations | Stripe production routes/webhooks, subscription lifecycle, top-ups, refunds, invoices, Marketplace settlement, payouts, and moderation |
| v07 Production Operations | Kubernetes/equivalent orchestration proof, backup/restore smoke, logs, metrics, tracing, alerts, dashboards, runbooks, release/rollback |
| v08 Product Completeness | Real or disabled built-in MCP tools, durable Agent workflows, Knowledge behavior matching product copy, commercial UX, docs, onboarding, pricing |

## Context Files

- Project: `.planning/PROJECT.md`
- Requirements: `.planning/REQUIREMENTS.md`
- Roadmap: `.planning/ROADMAP.md`
- Phase 13 context: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-CONTEXT.md`
- Phase 13 plan: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-01-PLAN.md`
- Phase 13 summary: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-01-SUMMARY.md`
- Commercial gates: `docs/release/commercial-gates.md`
- Commercial complete spec: `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`
- v04 roadmap archive: `.planning/milestones/v04-ROADMAP.md`
- v04 requirements archive: `.planning/milestones/v04-REQUIREMENTS.md`
- v04 state archive: `.planning/milestones/v04-STATE.md`
- Codebase Map: `.planning/codebase/`

## Key Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-05-27 | v04 Commercial Foundation initialized manually | `gsd-sdk init.new-milestone` still pointed phase archive metadata at v03.2, so manual planning updates avoided unsafe phase directory movement |
| 2026-05-28 | v04 Commercial Foundation completed | All 11 v04 requirements complete; v05-v08 remain required for final commercial completeness |
| 2026-05-28 | v05 Relay Billing Completeness initialized manually | Helper metadata is still stale; current `.planning` artifacts and branch state are authoritative |
| 2026-05-28 | Phase 13 planned as the v05 first phase | `/v1/*` route classification and production fail-closed behavior are the safest first enforcement boundary before billing/audit expansion |
| 2026-05-28 | Phase 13 completed | Route policy registry, production fail-closed behavior, route table docs, focused Relay handler tests, broader relay/http tests, docs gate, and diff check passed |

---
*State updated: 2026-05-28 after completing Phase 13 Relay endpoint authority*
