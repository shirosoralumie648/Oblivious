---
phase: 10-membership-roles-and-auth-security
plan: 01
status: complete
completed_at: 2026-05-28T01:44:00+08:00
requirements: [TENANT-02, TENANT-03, SEC-01, SEC-02, SEC-03]
commits:
  - 991b17b test(10): add failing membership and auth security tests
  - 31dc646 feat(10): add membership and auth security foundations
  - 79ad6cf test(10): add failing csrf and organization route tests
  - 01e6e50 test(10): add failing auth rate limit and reset route tests
  - e1d6854 feat(10): enforce organization auth security routes
  - 67c729a fix(web): restore tailwind token build config
---

# Phase 10 Plan 01 Summary

## Result

Implemented enforceable organization membership, invitation, and auth-security controls for the v04 commercial foundation.

Delivered:
- `0026_membership_auth_security.sql` with organization memberships, invitations, password reset tokens, and SQL-backed rate limits.
- `src/server/internal/tenant` membership service/store methods for multi-organization memberships, invite, accept, revoke, role update, removal, ownership transfer, and audit-backed mutations.
- App organization routes for membership listing, invitation create/revoke/accept, member role update/removal, and ownership transfer.
- CSRF enforcement for cookie-authenticated mutating routes.
- SQL-backed rate limits for login, registration, password reset, and sensitive admin/organization writes.
- Password policy enforcement, password reset confirmation, session rotation on invitation acceptance, and target-user session revocation on sensitive membership changes.
- Tailwind token config restoration so the required commercial gate `bash scripts/check.sh all` builds from the isolated worktree.

## Verification

Environment:
- PostgreSQL test container: `oblivious-phase10-postgres`
- `TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32769/oblivious_test?sslmode=disable`
- Go proxy override: `GOPROXY=https://mirrors.aliyun.com/goproxy/,direct`
- Go checksum DB override: `GOSUMDB=sum.golang.google.cn`
- Web dependencies installed with `pnpm install --frozen-lockfile` in the isolated worktree because `node_modules` was absent.

Passed:
- `cd src/server && go test ./internal/tenant ./internal/auth ./internal/http ./internal/admin -count=1`
- `cd src/server && TEST_DATABASE_URL=... go test ./internal/tenant ./internal/auth ./internal/http ./internal/admin -count=1`
- `GOPROXY=... GOSUMDB=... TEST_DATABASE_URL=... bash scripts/test.sh all`
- `GOPROXY=... GOSUMDB=... TEST_DATABASE_URL=... bash scripts/check.sh all`

Additional focused evidence:
- `TEST_DATABASE_URL=... go test ./internal/http -run 'AuthRateLimit|PasswordResetRoutes' -count=1`
- `TEST_DATABASE_URL=... go test ./internal/http -run 'OrganizationInvitationRevoke|OrganizationSessionSecurity|SensitiveOrganizationActions' -count=1`
- `rg -n 'password":"secret|[]byte\("secret"\)|"secret"' src/server/internal/http src/server/internal/auth -g '*_test.go'` returns only the intentional weak-password rejection in `auth/service_test.go`.

## Requirement Closure

| Requirement | Evidence |
|-------------|----------|
| TENANT-02 | `organization_memberships` schema, `ListMembershipsForUser`, role values `owner/admin/member`, creator owner membership, and DB-backed membership lifecycle tests |
| TENANT-03 | Invitation create/accept/revoke, member role update/removal, ownership transfer, transactional audit inserts, and HTTP/DB tests covering revoked invitation rejection and audit rows |
| SEC-01 | `security_middleware.go` CSRF guard plus route-surface and integration tests proving cookie-auth mutating routes reject missing CSRF |
| SEC-02 | `auth_rate_limits` store/service plus login, registration, password reset, and sensitive admin/organization action tests returning 429 |
| SEC-03 | Password policy tests, password reset session revocation, invitation acceptance session rotation, and role-change target session revocation tests |

## Deviations And Auto-Fixes

- The isolated worktree lacked the pre-existing unstaged Tailwind token config from the primary checkout, so `bash scripts/check.sh all` initially failed on `border-border`. The fix was committed as `67c729a` and matches the existing token/config repair pattern already present in the primary checkout.
- HTTP integration resets were updated to use `DROP TABLE IF EXISTS ... CASCADE` so tests can clean up tables left by older package-specific migration/test setups before rebuilding their minimal schemas.

## Residual Debt

- Phase 10 intentionally does not claim tenant-scoped Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, or Marketplace publisher data. That remains Phase 11.
- Phase 10 does not complete Relay endpoint billing classification, Stripe production rollout, Marketplace payout accounting, Kubernetes/equivalent production orchestration, RAG upgrade, or Agent workflow expansion. Those remain v05-v08 commercial program work.
- Phase 12 still owns CI wiring that proves DB-backed integration tests fail loudly when persistence is unavailable.

## Next Phase Readiness

Phase 11 can now plan tenant scope across core domains on top of:
- first-class organizations from Phase 9;
- auditable memberships and organization roles from Phase 10;
- production auth controls for CSRF, rate limits, password policy, and session invalidation.

---

*Summary written: 2026-05-28*
