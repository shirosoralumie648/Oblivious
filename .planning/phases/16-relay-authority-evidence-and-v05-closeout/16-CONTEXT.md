# Phase 16: Relay Authority Evidence and v05 Closeout - Context

**Gathered:** 2026-05-28
**Status:** Ready for execution planning
**Source:** Current goal context, `.planning/STATE.md`, `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md`, Phase 13-15 summaries, Relay route table, commercial gate docs

<domain>
## Phase Boundary

Phase 16 owns `DOC-04` for v05 Relay Billing Completeness.

It must close the v05 Relay Authority Gate evidence chain:
- every registered `/v1/*` route has a documented commercial class, production status, auth policy, tenant identity requirement, rate-limit policy, audit policy, billing policy, disabled reason, and future owner;
- `docs/release/commercial-gates.md` records that v05 Relay Authority evidence is complete only after Phase 13, Phase 14, Phase 15, and Phase 16 verification pass;
- `.planning` state closes v05 and routes the commercial program to v06 without claiming final commercial readiness;
- milestone snapshots preserve the v05 closeout state under `.planning/milestones/`.

This phase does not implement Stripe checkout/webhooks, subscription lifecycle, Marketplace settlement, Kubernetes proof, backup/restore smoke, observability, RAG upgrade, Agent workflow expansion, public onboarding, or final commercial readiness. Those remain v06-v08 work.
</domain>

<current_state>
## Live Evidence Facts

Current v05 evidence chain:
- Phase 13 completed route classification and production fail-closed behavior for every registered `/v1/*` route.
- Phase 14 completed provider-bypass checks, supported-route trusted identity enforcement, route rate-limit policy, and route-decision audit events.
- Phase 15 completed quota preauthorization, exactly-once idempotency, settlement/refund behavior, provider usage parsing, explicit route billing policy, and streaming/async/file production-disablement evidence.

Current open item:
- `DOC-04` remains planned in `.planning/REQUIREMENTS.md`.
- `.planning/STATE.md` says Phase 16 plan is not created and v05 is 75% complete.
- `docs/release/commercial-gates.md` still says Phase 16 closeout evidence is required before the Relay Authority Gate is complete.
- `docs/release/relay-route-table.md` documents the policy ledger, but supported route `Future owner` values still point at Phase 16 evidence instead of a closed v05 state.

Current worktree facts:
- Continue in `.worktrees/phase-10-membership-auth-security`.
- Root `main` worktree has unrelated dirty/untracked files and must not be used for v05 closeout.
- `gsd-sdk` state is stale for this repo, so local `.planning` artifacts are authoritative.
</current_state>

<decisions>
## Phase 16 Decisions

### Evidence-only closeout

Phase 16 should not add new commercial Relay behavior unless verification exposes a real blocker. The code boundary was implemented in Phases 13-15; Phase 16 turns that into reproducible evidence and milestone state.

### Gate wording

The Relay Authority Gate may be marked complete for v05 only, not for the full commercial SaaS objective. Gate wording must keep v06 Billing And Marketplace Operations, v07 Production Operations, and v08 Product Completeness as required future work.

### Verification scope

Phase 16 verification should include:
- route-table and commercial-gate docs checks;
- Relay security boundary check;
- focused Relay/http package tests covering route policy, billing, provider response usage, and streaming production rejection;
- DB-backed `scripts/test.sh all` when a disposable PostgreSQL test database is available;
- broad `scripts/check.sh all` if web/server dependencies are available in the current worktree.

Skipped checks are allowed only if explicitly recorded with environment reason. They do not prove final commercial readiness.

### Milestone archive

v05 closeout should create:
- `.planning/milestones/v05-REQUIREMENTS.md`
- `.planning/milestones/v05-ROADMAP.md`
- `.planning/milestones/v05-STATE.md`

The living `.planning/REQUIREMENTS.md` remains the cross-phase requirements file and must not be deleted or reset.
</decisions>

<canonical_refs>
## Canonical References

Downstream agents MUST read these before editing:

### Commercial Objective
- `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`
- `.planning/PROJECT.md`
- `.planning/REQUIREMENTS.md`
- `.planning/ROADMAP.md`
- `.planning/STATE.md`

### v05 Evidence Chain
- `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-01-SUMMARY.md`
- `.planning/phases/14-relay-provider-bypass-and-cost-abuse-guardrails/14-01-SUMMARY.md`
- `.planning/phases/15-relay-billing-settlement-and-refund-semantics/15-01-SUMMARY.md`
- `docs/release/relay-route-table.md`
- `docs/release/commercial-gates.md`

### Verification Entrypoints
- `scripts/check.sh`
- `scripts/test.sh`
- `scripts/verify-quality-gates.sh`
- `scripts/verify-relay-security.sh`
- `src/server/internal/relay/handler/policy.go`
- `src/server/internal/relay/handler/router.go`
- `src/server/internal/relay/router.go`
- `src/server/internal/relay/billing.go`
</canonical_refs>

<verification>
## Expected Verification

- A docs-gate RED check should fail before the Phase 16 verification artifact exists.
- GREEN docs checks must pass after `16-VERIFICATION.md` and gate docs are updated.
- Relay/http package tests must pass after closeout edits; Phase 16 should not change runtime behavior.
- DB-backed script verification should use the local disposable PostgreSQL test database if available.
- Phase 16 summary must close only `DOC-04`, close v05, and keep v06-v08 visible as required future work.
</verification>

---
*Phase: 16-relay-authority-evidence-and-v05-closeout*
*Context gathered: 2026-05-28*
