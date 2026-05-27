# Phase 10: Membership, Roles, and Auth Security - Context

**Gathered:** 2026-05-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 10 makes organization membership and production auth controls enforceable before tenant-scoped domain data migration. It proves TENANT-02, TENANT-03, SEC-01, SEC-02, and SEC-03.

This phase does not migrate Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, or Marketplace publisher data to tenant ownership. It does not complete Relay endpoint billing, Stripe, Marketplace payouts, Kubernetes validation, RAG upgrades, or Agent workflow expansion.
</domain>

<decisions>
## Implementation Decisions

### Membership and roles
- **D-01:** Use `organization_memberships` as the membership table name and keep it in the existing `tenant` package boundary.
- **D-02:** Organization roles are `owner`, `admin`, and `member`; they are separate from the existing global `users.role` values `admin`, `moderator`, and `user`.
- **D-03:** Membership status is represented by active rows plus `removed_at`; removed members remain historically queryable for audit and reporting.
- **D-04:** An organization must always have exactly one active owner after role, removal, and ownership-transfer mutations.
- **D-05:** Organization admins can invite and remove members; only owners can transfer ownership or demote/remove another owner.
- **D-06:** System/global admins keep B-side authority through `/api/v1/admin/organizations`, but organization self-service routes live under `/api/v1/app/organizations`.

### Invitations and ownership transfer
- **D-07:** Use `organization_invitations` with a random raw token returned to the caller once and only a token hash stored in the database.
- **D-08:** Invitation statuses are `pending`, `accepted`, `revoked`, and `expired`.
- **D-09:** Accepting an invitation creates or reactivates membership for the invited email and records `accepted_by_user_id` and `accepted_at`.
- **D-10:** Ownership transfer is transactional: the current owner becomes admin and the target active member becomes owner in the same operation.

### Audit behavior
- **D-11:** Membership, role, invitation, removal, acceptance, and ownership-transfer mutations must be audited.
- **D-12:** Sensitive tenant mutations must fail if the audit write fails; commercial tenant authority cannot change without an audit trail.
- **D-13:** Audit entries use existing `audit_logs` where possible with actions such as `organization.member.invite`, `organization.member.accept`, `organization.member.role_update`, `organization.member.remove`, and `organization.owner.transfer`.

### CSRF protection
- **D-14:** Cookie-authenticated mutating routes must require `X-CSRF-Token`; safe methods (`GET`, `HEAD`, `OPTIONS`) remain exempt.
- **D-15:** CSRF tokens are derived from the signed session boundary and exposed through session responses so the frontend can send the header.
- **D-16:** Public login/register remain public, but logout and all session/admin/app mutating routes that rely on cookies require CSRF.

### Rate limits
- **D-17:** Implement a server-side auth/security rate limiter with explicit scopes for login, registration, password reset request/confirm, and sensitive admin/organization actions.
- **D-18:** Store rate-limit state in PostgreSQL so DB-backed tests can prove enforcement and so commercial behavior is not only process-local.
- **D-19:** Rate-limit responses use HTTP 429 with a stable `rate_limited` error code.

### Password policy and session rotation
- **D-20:** Registration, password reset, and any password-change path must enforce a policy before hashing: minimum length 12 and at least three of lowercase, uppercase, digit, and symbol classes.
- **D-21:** Existing tests that register with weak passwords must be updated intentionally; Phase 10 cannot preserve `secret` as a valid commercial password.
- **D-22:** Sensitive privilege changes revoke or rotate affected user sessions so stale cookies cannot retain old authority.
- **D-23:** Invitation acceptance rotates the accepting user's current session and returns a fresh signed session cookie.

### the agent's Discretion
- Exact helper names, DTO shapes, and file splits may follow local Go patterns as long as the above behaviors are explicit and test-covered.
- Password reset email delivery can remain adapter-backed/test-backed in Phase 10; the enforceable requirement is token storage, policy, rate limit, and session invalidation behavior.
</decisions>

<specifics>
## Specific Ideas

- Create `src/server/migrations/0026_membership_auth_security.sql`.
- Extend `src/server/internal/tenant` with membership, invitation, ownership-transfer, and audit-aware store/service APIs.
- Add `src/server/internal/http/security_middleware.go` or similarly focused file for CSRF and rate-limit HTTP composition.
- Extend `src/server/internal/auth` with password policy, password reset token persistence, session rotation/revocation, and tests.
- Add route shapes:
  - `GET /api/v1/app/organizations`
  - `GET /api/v1/app/organizations/{id}/members`
  - `POST /api/v1/app/organizations/{id}/invitations`
  - `POST /api/v1/app/organizations/{id}/invitations/{invitationID}/revoke`
  - `POST /api/v1/app/organization-invitations/{token}/accept`
  - `PUT /api/v1/app/organizations/{id}/members/{userID}`
  - `DELETE /api/v1/app/organizations/{id}/members/{userID}`
  - `POST /api/v1/app/organizations/{id}/ownership-transfer`
  - `POST /api/v1/auth/password-reset/request`
  - `POST /api/v1/auth/password-reset/confirm`
</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Commercial target and phase scope
- `.planning/PROJECT.md` - v04 scope and commercial-complete constraints.
- `.planning/REQUIREMENTS.md` - TENANT-02, TENANT-03, SEC-01, SEC-02, and SEC-03 definitions.
- `.planning/ROADMAP.md` - Phase 10 success criteria and likely verification.
- `.planning/STATE.md` - current workflow position.
- `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md` - non-negotiable commercial gates.

### Tenant foundation
- `.planning/phases/09-tenant-model-and-migration-ledger/09-CONTEXT.md` - Phase 9 tenant and migration decisions.
- `.planning/phases/09-tenant-model-and-migration-ledger/09-01-SUMMARY.md` - implemented organization and migration-ledger evidence.
- `src/server/migrations/0025_tenant_foundation.sql` - organization schema.
- `src/server/internal/tenant/types.go` - current organization DTOs.
- `src/server/internal/tenant/service.go` - current organization service validation.
- `src/server/internal/tenant/store.go` - current organization SQL store.
- `src/server/internal/http/tenant_handler.go` - current admin organization handler.

### Auth, sessions, admin audit, and route guards
- `src/server/internal/auth/service.go` - register/login/session service boundary.
- `src/server/internal/auth/store.go` - users, workspaces, and sessions persistence.
- `src/server/internal/http/auth_handler.go` - login/register/logout/session response.
- `src/server/internal/http/auth_middleware.go` - signed cookie and session/admin guard behavior.
- `src/server/internal/http/routes_auth.go` - auth route registration.
- `src/server/internal/http/router.go` - route composition and service construction.
- `src/server/internal/admin/audit_store.go` - audit log persistence.
- `src/server/internal/admin/service.go` - current `LogAction` helper.
- `src/server/migrations/0022_audit_logs.sql` - existing audit log schema.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `src/server/internal/tenant` already owns organization validation, SQL storage, and admin handler integration.
- `src/server/internal/admin` already provides `audit_logs` and `LogAction`, though Phase 10 sensitive membership mutations need stronger fail-closed audit semantics.
- `src/server/internal/http/auth_middleware.go` already signs session cookies with HMAC and can be extended for CSRF token derivation.
- `src/server/internal/http/route_surface_test.go` already protects admin/app auth boundaries and should be extended for CSRF/rate-limit expectations.
- `src/server/internal/http/server_test.go` already supplies DB-backed integration patterns gated by `TEST_DATABASE_URL`.

### Established Patterns
- Go services own validation; SQL stores own persistence.
- HTTP handlers use `decodeRequestJSON`, `writeSuccess`, `writeError`, `parseQueryInt`, and session context helpers.
- Admin route registration happens directly in `router.go` and uses `authMiddleware.requireAdmin`.
- App routes are cookie-session routes under `/api/v1/app/*`.
- DB-backed tests are required to prove commercial persistence behavior; unit-only tests are insufficient for Phase 10 closure.

### Integration Points
- Membership tables reference both `organizations(id)` and `users(id)`.
- Invitation acceptance needs authenticated user email from `auth.Session.User.Email`.
- CSRF middleware must be applied without breaking public login/register and safe GET routes.
- Rate limits must run before expensive bcrypt work on login/register/password reset paths.
- Session rotation must update the signed cookie response path in HTTP handlers.
</code_context>

<deferred>
## Deferred Ideas

- Phase 11 owns tenant-scoping existing Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, and Marketplace publisher data.
- v05 owns all Relay endpoint billing and production fail-closed endpoint classification.
- v06 owns Stripe production rollout, Marketplace payout accounting, refunds, invoices, and settlement UX.
- v07 owns Kubernetes/equivalent production orchestration, backup/restore, observability, alerts, dashboards, and runbooks.
- v08 owns full product completeness, RAG/Knowledge promise alignment, durable Agent workflows, and public commercial onboarding/pricing docs.
</deferred>

---

*Phase: 10-membership-roles-and-auth-security*
*Context gathered: 2026-05-28*
