---
gsd_state_version: 1.0
milestone: v06
milestone_name: Billing And Marketplace Operations
status: active
stopped_at: Phase 20 ready to plan
last_updated: "2026-05-28T09:24:55+08:00"
last_activity: 2026-05-28 -- Phase 19 Marketplace settlement and governance completed
progress:
  total_phases: 4
  completed_phases: 3
  total_plans: 4
  completed_plans: 3
  percent: 75
---

# STATE.md

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-28)

**Core value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

**Current focus:** Phase 20 planning

## Current Status

**Milestone v06: Billing And Marketplace Operations — ACTIVE**

v04 Commercial Foundation is complete and archived through `.planning/milestones/v04-*`. v05 Relay Billing Completeness is complete and archived through `.planning/milestones/v05-*`.

The overall commercial-complete objective remains open. v06 now owns commercial money movement and Marketplace governance: Stripe checkout/webhooks, subscription lifecycle, top-ups, refunds, invoices, publisher settlement, platform fees, payout state, billing admin evidence, and moderation workflows.

Phase 17 completed the first v06 slice by mounting Stripe payment routes in the running server, verifying webhook signatures from the raw body, writing a dedicated idempotent webhook ledger, and preserving tenant/user/plan metadata for later subscription and Marketplace settlement phases. Phase 18 completed `PAY-03` by applying verified provider events to subscription, invoice, top-up, failed-payment, plan-change, and refund state through append-only lifecycle transitions. Phase 19 completed `MARKET-03` and `MARKET-04` with Marketplace paid install orders, settlement/platform-fee/payout-state modeling, refund impact, takedown, appeal, reinstate, abuse reports, governance events, and publisher financial stats.

## Current Position

Phase: Phase 20 — Billing Admin Evidence and v06 Closeout
Plan: not planned yet
Status: ready to plan
Last activity: 2026-05-28 -- Phase 19 completed

## Current Scope

| Requirement | Status | Target |
|-------------|--------|--------|
| PAY-01 | Complete | Stripe checkout routes are mounted, authenticated, tenant-aware, and testable without live Stripe keys |
| PAY-02 | Complete | Stripe webhook route verifies raw-body signatures, records events idempotently, and rejects invalid signatures |
| PAY-03 | Complete | Subscription, invoice, failed-payment, plan-change, top-up, and refund transitions are auditable |
| MARKET-03 | Complete | Marketplace publisher revenue, platform fee, payout state, and refund impact are modeled before paid operation |
| MARKET-04 | Complete | Marketplace moderation and abuse workflows govern publish, approve, reject, takedown, and appeal paths |
| ADMIN-BILL-01 | Planned | Admin can inspect billing sessions, webhook events, subscriptions, top-ups, invoices, refunds, settlements, and payout state |
| DOC-05 | Planned | v06 evidence maps money-movement requirements to code, tests, docs, and runtime/database verification |

## Next Suggested Step

Run `$gsd:plan-phase 20` to plan Phase 20 Billing Admin Evidence and v06 Closeout.

Phase 20 should add admin billing inspection for sessions, webhook events, subscriptions, top-ups, invoices, refunds, settlements, and payout state, then produce v06 closeout evidence. It must not claim v07 operations, v08 product completeness, or final commercial readiness.

## Worktree Context

Continue in `.worktrees/phase-10-membership-auth-security` on branch `gsd/phase-10-membership-auth-security`. The root `main` worktree is behind this branch and has unrelated dirty/untracked files; do not use it for v06 implementation unless the branch is merged or the user directs a switch.

`gsd-sdk query init.new-milestone` still reports stale phase archive metadata under v03.2, so v06 planning is being maintained from local `.planning` truth rather than unsafe helper-driven movement.

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
| v05 Relay Billing Completeness | 2026-05-28 | RELAY-08~11, BILL-01~02, DOC-04 |
| Phase 18 Subscription Invoice Top-up Refund State Machine | 2026-05-28 | PAY-03 |
| Phase 19 Marketplace Settlement and Governance | 2026-05-28 | MARKET-03, MARKET-04 |

## Deferred Commercial Program Items

These remain required for the final user goal:

| Milestone | Item |
|-----------|------|
| v06 Billing And Marketplace Operations | Admin billing inspection and v06 closeout evidence |
| v07 Production Operations | Kubernetes/equivalent orchestration proof, backup/restore smoke, logs, metrics, tracing, alerts, dashboards, runbooks, release/rollback |
| v08 Product Completeness | Real or disabled built-in MCP tools, durable Agent workflows, Knowledge behavior matching product copy, commercial UX, docs, onboarding, pricing |

## Context Files

- Project: `.planning/PROJECT.md`
- Requirements: `.planning/REQUIREMENTS.md`
- Roadmap: `.planning/ROADMAP.md`
- Phase 13 context: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-CONTEXT.md`
- Phase 13 plan: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-01-PLAN.md`
- Phase 13 summary: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-01-SUMMARY.md`
- Phase 14 context: `.planning/phases/14-relay-provider-bypass-and-cost-abuse-guardrails/14-CONTEXT.md`
- Phase 14 plan: `.planning/phases/14-relay-provider-bypass-and-cost-abuse-guardrails/14-01-PLAN.md`
- Phase 14 summary: `.planning/phases/14-relay-provider-bypass-and-cost-abuse-guardrails/14-01-SUMMARY.md`
- Phase 15 context: `.planning/phases/15-relay-billing-settlement-and-refund-semantics/15-CONTEXT.md`
- Phase 15 plan: `.planning/phases/15-relay-billing-settlement-and-refund-semantics/15-01-PLAN.md`
- Phase 15 summary: `.planning/phases/15-relay-billing-settlement-and-refund-semantics/15-01-SUMMARY.md`
- Phase 16 context: `.planning/phases/16-relay-authority-evidence-and-v05-closeout/16-CONTEXT.md`
- Phase 16 plan: `.planning/phases/16-relay-authority-evidence-and-v05-closeout/16-01-PLAN.md`
- Phase 16 verification: `.planning/phases/16-relay-authority-evidence-and-v05-closeout/16-VERIFICATION.md`
- Phase 16 summary: `.planning/phases/16-relay-authority-evidence-and-v05-closeout/16-01-SUMMARY.md`
- Phase 17 context: `.planning/phases/17-stripe-payment-authority-and-webhook-ledger/17-CONTEXT.md`
- Phase 17 plan: `.planning/phases/17-stripe-payment-authority-and-webhook-ledger/17-01-PLAN.md`
- Phase 17 summary: `.planning/phases/17-stripe-payment-authority-and-webhook-ledger/17-01-SUMMARY.md`
- Phase 18 context: `.planning/phases/18-subscription-invoice-topup-refund-state-machine/18-CONTEXT.md`
- Phase 18 plan: `.planning/phases/18-subscription-invoice-topup-refund-state-machine/18-01-PLAN.md`
- Phase 18 summary: `.planning/phases/18-subscription-invoice-topup-refund-state-machine/18-01-SUMMARY.md`
- Phase 19 context: `.planning/phases/19-marketplace-settlement-and-governance/19-CONTEXT.md`
- Phase 19 plan: `.planning/phases/19-marketplace-settlement-and-governance/19-01-PLAN.md`
- Phase 19 summary: `.planning/phases/19-marketplace-settlement-and-governance/19-01-SUMMARY.md`
- Commercial gates: `docs/release/commercial-gates.md`
- Commercial complete spec: `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`
- v04 roadmap archive: `.planning/milestones/v04-ROADMAP.md`
- v04 requirements archive: `.planning/milestones/v04-REQUIREMENTS.md`
- v04 state archive: `.planning/milestones/v04-STATE.md`
- v05 roadmap archive: `.planning/milestones/v05-ROADMAP.md`
- v05 requirements archive: `.planning/milestones/v05-REQUIREMENTS.md`
- v05 state archive: `.planning/milestones/v05-STATE.md`
- Codebase Map: `.planning/codebase/`

## Key Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-05-27 | v04 Commercial Foundation initialized manually | `gsd-sdk init.new-milestone` still pointed phase archive metadata at v03.2, so manual planning updates avoided unsafe phase directory movement |
| 2026-05-28 | v04 Commercial Foundation completed | All 11 v04 requirements complete; v05-v08 remain required for final commercial completeness |
| 2026-05-28 | v05 Relay Billing Completeness initialized manually | Helper metadata is still stale; current `.planning` artifacts and branch state are authoritative |
| 2026-05-28 | Phase 13 planned as the v05 first phase | `/v1/*` route classification and production fail-closed behavior are the safest first enforcement boundary before billing/audit expansion |
| 2026-05-28 | Phase 13 completed | Route policy registry, production fail-closed behavior, route table docs, focused Relay handler tests, broader relay/http tests, docs gate, and diff check passed |
| 2026-05-28 | Phase 14 completed | Provider-bypass CI checks, supported-route production identity guard, route-decision audit sink, route rate-limit policy, and Chat/Agent/Memory trusted Relay metadata are in place |
| 2026-05-28 | Phase 15 completed | Supported Relay billing now preauthorizes selected-channel quota, settles exactly once per idempotency key, refunds failed or partial calls, parses provider usage, declares route billing policies, and keeps streaming/async/file flows disabled or explicitly rejected until tested |
| 2026-05-28 | Phase 16 completed | Relay route table, commercial gate docs, `16-VERIFICATION.md`, docs gate assertions, DB-backed `scripts/test.sh all`, broad `scripts/check.sh all`, and v05 milestone snapshots close `DOC-04` |
| 2026-05-28 | v05 Relay Billing Completeness completed | Relay Authority Gate evidence is complete for v05; v06-v08 remain required for final commercial completeness |
| 2026-05-28 | v06 Billing And Marketplace Operations initialized | The next commercial gate is money movement and Marketplace governance; Phase 17 starts with Stripe route authority and webhook ledger before subscription lifecycle and settlement work |
| 2026-05-28 | Phase 17 completed | Stripe checkout and webhook routes are mounted; checkout persists tenant payment intents through a fake-testable creator; webhook signatures use raw body verification and record idempotent `stripe_webhook_events`; PAY-01 and PAY-02 are complete |
| 2026-05-28 | Phase 18 planned | PAY-03 is decomposed into lifecycle schema, Stripe event application service, payment-backed top-up checkout, invoice/refund/subscription transitions, DB-backed route tests, and evidence updates |
| 2026-05-28 | Phase 18 completed | Verified Stripe events now apply subscription, invoice, failed-payment, plan-change, top-up, and refund state through `billing_lifecycle_events`; duplicate webhook deliveries can retry lifecycle application safely through transition-key idempotency |
| 2026-05-28 | Phase 19 planned | MARKET-03 and MARKET-04 are decomposed into settlement schema, paid install checkout, Marketplace refund impact, local payout-state modeling, takedown/appeal/reinstate/abuse workflows, publisher financial stats, DB-backed route tests, and evidence updates |
| 2026-05-28 | Phase 19 completed | Paid Marketplace orders, settlements, platform fees, local payout state, refund impact, governance routes/events, abuse reports, publisher financial stats, DB-backed package tests, docs gate, broad checks, and diff hygiene close MARKET-03/MARKET-04 |

---
*State updated: 2026-05-28 after completing Phase 19 Marketplace Settlement and Governance*
