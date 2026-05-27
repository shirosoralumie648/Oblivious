# Phase 11: Tenant Scope Across Core Domains - Patterns

**Mapped:** 2026-05-28
**Purpose:** Current code patterns and change points for TENANT-04/TENANT-05.

## Existing Authority Columns

| Domain | Current authority | Representative files | Phase 11 target |
|--------|-------------------|----------------------|-----------------|
| Session | `sessions.workspace_id` | `src/server/internal/auth/store.go`, `src/server/internal/http/auth_middleware.go` | Add `sessions.organization_id`; expose `auth.Session.OrganizationID` |
| Chat | `conversations.workspace_id` | `src/server/internal/chat/service.go`, `src/server/internal/chat/store.go` | Filter by active `organization_id`; retain workspace for compatibility |
| Knowledge | `knowledge_bases.workspace_id` | `src/server/internal/knowledge/service.go`, `src/server/internal/knowledge/store.go` | Filter bases/docs/chunks by active `organization_id` |
| Console | `usage_records.workspace_id` | `src/server/internal/console/store.go` | Aggregate by active `organization_id` |
| Agent | `agents.user_id`, `agent_conversations.user_id` | `src/server/internal/agent/service.go`, `src/server/internal/agent/store.go` | Filter agents/conversations/messages by active `organization_id`; keep `user_id` as creator/attribution |
| Memory | `memory_documents.user_id`, `memory_chunks.user_id` | `src/server/internal/memory/service.go` | Filter by active `organization_id` and current `user_id`; no organization-shared memory behavior is added in Phase 11 |
| MCP | `mcp_servers.user_id` | `src/server/internal/mcp/client.go`, `src/server/internal/http/mcp_handler.go` | Filter server CRUD/connect/tools by active `organization_id` |
| Quota | `quotas.user_id`, `billing_sessions.user_id`, `subscriptions.user_id`, `topup_orders.user_id` | `src/server/internal/quota/service.go`, `src/server/internal/http/quota_handler.go` | Carry `organization_id` through quota, billing session, subscription, and top-up rows |
| Marketplace publisher | `published_agents.owner_id`, `agent_installs.user_id`, `agent_reviews.user_id` | `src/server/internal/marketplace/*.go`, `src/server/internal/http/marketplace_handler.go` | Add publisher/install/review organization ownership while retaining user attribution |
| Admin tenant data | `organizations`, `organization_memberships`, `audit_logs` | `src/server/internal/tenant/*.go`, `src/server/internal/admin/audit_store.go` | Organization data remains tenant-aware; audit entries include organization resource IDs or organization ID |

## Established HTTP/Test Patterns

- Authenticated app routes use `authMiddleware.requireSession(...)`.
- Admin routes use `authMiddleware.requireAdmin(...)`.
- Cookie-auth mutating routes require `X-CSRF-Token` after Phase 10.
- Handlers get the actor through `sessionFromContext(r)` or `sessionOrUnauthorized(w, r)`.
- DB-backed HTTP tests live in `src/server/internal/http/server_test.go` and skip only when `TEST_DATABASE_URL` is absent.
- Test helpers already exist for registration, login, CSRF token extraction, admin promotion, organization create, invitation, and membership acceptance.

## Migration Pattern

- New migration files are append-only and ledgered by filename/checksum.
- Use `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` for existing tables.
- Backfill before `ALTER COLUMN ... SET NOT NULL`.
- Use deterministic backfill IDs for legacy organization rows so reruns are safe.
- Add query-shape indexes after columns are populated.

## Implementation Pattern

- Services own validation and authorization rules.
- SQL stores own tenant filters and must include `organization_id` in representative list/detail/update/delete queries.
- Handlers should not accept client-supplied tenant authority except the explicit organization-select route, which verifies membership before changing session state.
- Existing route shapes should stay stable for Phase 11; frontend can continue using `/api/v1/app/*` while the active session organization selects the tenant.

## Test Pattern

Every cross-tenant test should:
1. Register or create two users.
2. Ensure each user has or selects a different organization.
3. Create one resource in organization A.
4. Attempt representative read from organization B and assert denial.
5. Attempt representative write/delete/update from organization B and assert denial.
6. Query the database to prove the original row was not changed by the denied operation.

## Known Risk Areas

- `auth.SQLStore.CreateConversation` still references `conversations.user_id`, a legacy path that does not match the current migration schema. Do not rely on it for Phase 11 tenant scoping.
- Agent public access currently allows conversations on another user's public agent. Phase 11 rule: public Agent use creates a conversation in the caller's active organization while the published agent retains its owner organization.
- Memory search currently filters by user ID only. If memory becomes organization-shared, update tests and product copy; if it remains user-private inside an organization, encode both filters intentionally.
- Marketplace public browse/search is global by design, so TENANT-05 tests should target publisher-owned, install, review, and analytics paths rather than public search.
- Console usage currently relies on `usage_records.workspace_id`; Phase 11 must ensure chat usage writes `organization_id` and console queries aggregate by organization.
