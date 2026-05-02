# Phase 4: 质量与发布 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-02
**Phase:** 04-quality-release
**Areas discussed:** Test Matrix, Browser E2E Scope, API Documentation And RC Checklist, Deployment Validation

---

## Test Matrix

| Option | Description | Selected |
|--------|-------------|----------|
| Broad release gate | Use `go test ./... -count=1` for backend release-risk changes and make current focused scripts catch up where needed. | ✓ |
| Focused-only gate | Keep `scripts/check.sh server` and `scripts/test.sh server` as the only server gates. | |
| Database-only integration push | Prioritize DB-backed integration tests before improving the broader unit/package matrix. | |

**User's choice:** Auto-selected recommended default during `$gsd-next`.
**Notes:** Current scripts run focused packages and gate `./internal/http` behind `TEST_DATABASE_URL`; Phase 4 should make skip conditions explicit and avoid silent release coverage gaps.

---

## Browser E2E Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Critical workflows only | Cover Admin and Marketplace primary paths that prove route wiring, auth/admin protection, and successful actions. | ✓ |
| Exhaustive page states | Cover every page loading/error/empty permutation in browser E2E. | |
| Visual regression first | Add visual baselines as the primary E2E gate. | |

**User's choice:** Auto-selected recommended default during `$gsd-next`.
**Notes:** Vitest already covers many route and component states. Initial E2E should not depend on live provider keys, Stripe, or external LLM calls.

---

## API Documentation And RC Checklist

| Option | Description | Selected |
|--------|-------------|----------|
| Reconcile against live routes | Update `docs/API.md`, `current-system-contracts.md`, and `rc-checklist.md` against current router/script behavior. | ✓ |
| Rewrite docs from product intent | Treat documentation as a fresh product spec regardless of current route wiring. | |
| Keep docs advisory | Leave current docs mostly as-is and rely on tests for release evidence. | |

**User's choice:** Auto-selected recommended default during `$gsd-next`.
**Notes:** Documentation should describe verified current behavior only. Future behavior belongs in roadmap/backlog, not current-system contracts.

---

## Deployment Validation

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal runnable mainline stack | Add Docker/Kubernetes config for active `src/server` + `src/web`, PostgreSQL, Redis as needed, migrations, health, and smoke commands. | ✓ |
| Import upstream deploy stacks | Adapt `lobehub/` or `new-api/` Docker setups as the primary deployment path. | |
| Docs-only deploy story | Document deployment expectations without runnable config. | |

**User's choice:** Auto-selected recommended default during `$gsd-next`.
**Notes:** Imported source trees are references, not the root release path. Secrets must remain placeholders/env var names.

---

## Agent's Discretion

- Exact E2E framework choice if it fits the current Vite/React toolchain and headless CI.
- Exact disposable database approach for integration tests.
- Exact plan split, as long as TEST-01, TEST-02, DOC-01, and DEPLOY-01 each map clearly.

## Deferred Ideas

- Production observability/alerting beyond current `/metrics`.
- Stripe production hardening and commercial revenue-share flows.
- Mobile-specific RC testing.
