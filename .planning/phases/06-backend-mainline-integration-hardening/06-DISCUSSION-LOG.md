# Phase 6: Backend Mainline Integration Hardening - Discussion Log

> Audit trail only. Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md - this log preserves the alternatives considered.

**Date:** 2026-05-14
**Phase:** 06-backend-mainline-integration-hardening
**Areas discussed:** Route registry and auth boundaries, Relay-first Chat and Agent contract, Auth/session/user preferences contract, Verification and commit boundary

---

## Route Registry And Auth Boundaries

| Option | Description | Selected |
|--------|-------------|----------|
| Keep router.go active and test current routes | Preserve the current composition root and harden the live route surface. | Yes |
| Complete route-split refactor now | Wire all `routes_*.go` helpers and remove matching inline registrations in the same backend slice. | Agent discretion |
| Leave partial route files unverified | Keep new route files as passive additions without proving live registration. | No |

**User's choice:** Auto-selected from Phase 5 handoff and current repository state because no interactive AskUserQuestion tool is available in this session.
**Notes:** The selected policy keeps `router.go` authoritative unless the plan intentionally completes the split refactor with route parity tests.

---

## Relay-First Chat And Agent Contract

| Option | Description | Selected |
|--------|-------------|----------|
| Enforce Relay-first when Relay is enabled | Chat and Agent calls use local Relay, internal metadata headers, and no silent production provider bypass. | Yes |
| Keep direct fallback as production behavior | Allow direct model provider fallback whenever Relay errors. | No |
| Disable tool-call hardening | Defer structured tool-call and streaming checks. | No |

**User's choice:** Auto-selected from project core value and CHAT-06.
**Notes:** Direct fallback can remain a development/demo path only if the plan makes the production behavior explicit. Structured tool calls and final answer streaming remain part of Phase 6 verification.

---

## Auth, Session, And User Preferences Contract

| Option | Description | Selected |
|--------|-------------|----------|
| Normalize user name and role across auth flows | Register, login, session lookup, and `/me` return consistent user fields while preserving `requireAdmin`. | Yes |
| Treat register/login/session drift as acceptable | Leave inconsistent `name` and `role` fields for clients to work around. | No |
| Expand auth scope into OAuth/MFA | Add new identity provider or MFA capabilities. | No |

**User's choice:** Auto-selected from AUTH-01.
**Notes:** OAuth/MFA would be new product scope. Phase 6 stays on existing email/password sessions and role boundaries.

---

## Verification And Commit Boundary

| Option | Description | Selected |
|--------|-------------|----------|
| Add focused backend tests first | Cover route/auth/session/Relay contracts before backend slice commit. | Yes |
| Commit backend slice before tests | Preserve speed but defer regression protection. | No |
| Mix backend with frontend/deploy/docs commit groups | Collapse Phase 5 boundaries into one broad commit. | No |

**User's choice:** Auto-selected from Phase 5 commit-boundary policy and Phase 6 success criteria.
**Notes:** Backend staging must follow `05-COMMIT-BOUNDARIES.md`; frontend/deploy/docs stay for Phase 7 and Phase 8.

---

## Agent's Discretion

- Decide whether the route-split refactor is worth completing during Phase 6 or whether inline route registration should stay until a later cleanup.
- Choose the exact test shape, favoring package-local Go tests that prove the route and service contract with minimal fixture fragility.
- Decide whether DB-backed tests are required for each contract; if `TEST_DATABASE_URL` is unavailable, record skips explicitly.

## Deferred Ideas

- Frontend, Playwright, Docker, CI, and deployment validation remain Phase 7.
- API/release/architecture documentation reconciliation remains Phase 8.
- OAuth/MFA, Stripe route promotion, and broad metrics security policy are not part of Phase 6 unless promoted as explicit future work.
