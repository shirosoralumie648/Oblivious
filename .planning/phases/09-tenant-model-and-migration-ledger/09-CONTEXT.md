# Phase 9: Tenant Model and Migration Ledger - Context

**Gathered:** 2026-05-27
**Status:** Ready for planning/execution

<domain>
## Phase Boundary

Phase 9 starts v04 Commercial Foundation by introducing first-class organization tenants and an append-only migration ledger. It proves TENANT-01 and MIGR-01 only.

This phase does not implement membership roles, invitations, ownership transfer, CSRF, rate limits, password policy, session rotation, cross-domain tenant migration, Relay billing completion, Stripe, Marketplace payouts, Kubernetes validation, RAG upgrades, or Agent workflow expansion. Those are later v04 phases or later commercial milestones.
</domain>

<decisions>
## Implementation Decisions

### Tenant model
- **D-01:** Use `organizations` as the first-class tenant table name. Avoid `tenant` as the primary code/package name where it would collide with broader future tenant-scope concepts.
- **D-02:** Phase 9 organization records include `id`, `slug`, `name`, `status`, `metadata`, `created_by_user_id`, `created_at`, `updated_at`, and `archived_at`.
- **D-03:** Valid statuses are `active`, `disabled`, and `archived`. Delete should be soft-delete/archive, not physical deletion.
- **D-04:** Membership tables, invitations, owner transfer, and role enforcement are Phase 10, not Phase 9.
- **D-05:** Existing `workspaces` and user-owned domain records remain backward-compatible in Phase 9. Phase 11 owns tenant-scoping core domain data.

### Migration ledger
- **D-06:** `schema_migrations` is append-only and keyed by migration filename/version.
- **D-07:** The migration runner must create/ensure `schema_migrations` before applying migrations, compute a checksum for each migration file, skip already-applied matching migrations, and fail on checksum mismatch.
- **D-08:** Existing migrations should not be rewritten. Because most existing migrations are idempotent, a database without ledger rows may safely backfill the ledger by re-running current migrations once.
- **D-09:** `0024_categories_tags.sql` contains a plain `INSERT INTO categories`; Phase 9 implementation must either make that migration idempotent or make the runner handle it safely before it can be backfilled into a ledgered database.

### HTTP/admin boundary
- **D-10:** Admin organization management routes live under `/api/v1/admin/organizations` and require `requireAdmin`, matching existing admin route policy.
- **D-11:** Implement organization logic in a focused package such as `src/server/internal/tenant` and inject it through `router.go`; do not bloat the existing admin package with tenant domain behavior.
- **D-12:** Organization create/update requests must validate slug/name/status server-side. Slugs are lowercase DNS-like identifiers and must be unique.

### Verification
- **D-13:** Phase 9 must include DB-backed tests for migration ledger idempotency and organization CRUD. Unit-only fake-store tests are not enough to close MIGR-01 or TENANT-01.
- **D-14:** `bash scripts/check.sh all` remains the broad check, but Phase 9 cannot claim completion unless at least one `TEST_DATABASE_URL` path proves PostgreSQL-backed behavior.
</decisions>

<specifics>
## Specific Ideas

- Create `src/server/migrations/0025_tenant_foundation.sql`.
- Create `src/server/internal/tenant/` with types, SQL store, service, and DB-backed tests.
- Add `src/server/internal/http/tenant_handler.go` and route registration in `src/server/internal/http/router.go`.
- Extend `src/server/internal/http/route_surface_test.go` so `/api/v1/admin/organizations` is covered by admin route expectations.
- Prefer precise route shapes:
  - `GET /api/v1/admin/organizations`
  - `POST /api/v1/admin/organizations`
  - `GET /api/v1/admin/organizations/{id}`
  - `PUT /api/v1/admin/organizations/{id}`
  - `POST /api/v1/admin/organizations/{id}/archive`
</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before implementing.**

- `.planning/PROJECT.md` - v04 scope and commercial-complete constraints.
- `.planning/REQUIREMENTS.md` - TENANT-01 and MIGR-01 definitions.
- `.planning/ROADMAP.md` - Phase 9 success criteria and likely verification.
- `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md` - non-negotiable commercial gates.
- `src/server/cmd/migrate/main.go` - current unledgered migration runner.
- `src/server/migrations/0001_phase1_foundation.sql` through `0024_categories_tags.sql` - existing migration style and idempotency constraints.
- `src/server/internal/http/router.go` - route registration and admin guard patterns.
- `src/server/internal/http/route_surface_test.go` - route auth/admin route coverage.
- `src/server/internal/admin/*` - existing admin service/store/handler patterns.
- `src/server/internal/http/server_test.go` - DB-backed test setup and `TEST_DATABASE_URL` behavior.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `src/server/cmd/migrate/main.go` already sorts `.sql` migration files and executes them against `config.Load().DatabaseURL`.
- `src/server/internal/http/router.go` creates services/stores and registers admin routes with `authMiddleware.requireAdmin`.
- `src/server/internal/http/admin_handler.go` contains handler helpers for admin list/create/update/delete-style operations.
- `src/server/internal/http/auth_middleware.go` stores `auth.Session` in request context and already signs session cookies.
- `src/server/internal/http/server_test.go` demonstrates PostgreSQL integration tests gated by `TEST_DATABASE_URL`.

### Established Patterns
- Stores accept `context.Context` and use `database/sql`.
- Services own validation and audit-facing behavior; SQL stores own persistence.
- Admin handlers use `decodeRequestJSON`, `writeSuccess`, `writeError`, and `parseQueryInt`.
- Admin route tests use fake stores for handler behavior and `requireAdmin` boundary checks.

### Integration Points
- Migration runner changes affect Docker deploy validation and any local `go run ./cmd/migrate` path.
- Organization routes must be mounted in `router.go` so docs and route-surface tests see them.
- `schema_migrations` ledger must be compatible with PostgreSQL, because v04 requires DB-backed integration evidence.
</code_context>

<deferred>
## Deferred Ideas

- Organization membership roles, invitations, ownership transfer, and audit events are Phase 10.
- Tenant-scoped Chat/Agent/Knowledge/Memory/MCP/Quota/Console/Admin/Marketplace publisher data is Phase 11.
- CI wiring that forces DB-backed integration tests belongs to Phase 12.
- Commercial billing, Marketplace payout accounting, Kubernetes proof, and product completeness remain v05-v08.
</deferred>

---

*Phase: 09-tenant-model-and-migration-ledger*
*Context gathered: 2026-05-27*
