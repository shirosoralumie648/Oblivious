# Phase 11: Tenant Scope Across Core Domains - Context

**Gathered:** 2026-05-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 11 applies organization tenant identity to the core application data domains and proves representative cross-tenant reads and writes fail. It proves TENANT-04 and TENANT-05.

This phase builds on Phase 9 first-class organizations and Phase 10 memberships/security. It does not complete Relay endpoint billing classification, Stripe production rollout, Marketplace payouts, Kubernetes or equivalent production orchestration, Knowledge RAG upgrades, durable Agent workflow expansion, or Phase 12 CI/commercial-gate documentation.
</domain>

<decisions>
## Implementation Decisions

### Tenant context source
- **D-01:** The active organization is server authority, not a client payload field. Store it on the authenticated session as `sessions.organization_id` and expose it in `auth.Session`.
- **D-02:** Add the user-facing organization switch route `POST /api/v1/app/organizations/{id}/select`. It must verify active membership before updating the session organization.
- **D-03:** Registration and legacy login paths must always resolve an active organization. New users get a default organization and owner membership. Existing users/workspaces are backfilled into deterministic legacy organizations.
- **D-04:** Domain handlers and services may accept path IDs for resources, but tenant authority comes from the active organization in `auth.Session` after membership verification.
- **D-05:** During this phase, preserve existing `workspace_id` and `user_id` columns for backwards compatibility and attribution, but they are no longer allowed to be the security boundary for tenant-scoped data.

### Data model
- **D-06:** Add `organization_id` to tenant-owned parent tables and to child tables where direct tenant filtering prevents accidental joins from becoming the only isolation mechanism.
- **D-07:** Chat tenant scope covers `workspaces`, `sessions`, `conversations`, `messages`, `conversation_configs`, `conversation_knowledge_bindings`, and `usage_records`.
- **D-08:** Knowledge tenant scope covers `knowledge_bases`, `knowledge_documents`, and `knowledge_document_chunks`.
- **D-09:** Agent tenant scope covers `agents`, `agent_conversations`, and `agent_messages`. Agent owner `user_id` remains attribution, not isolation.
- **D-10:** Memory tenant scope covers `memory_documents` and `memory_chunks`. Memory search must filter by organization and authenticated user rules intentionally, not by user only.
- **D-11:** MCP tenant scope covers `mcp_servers`; remote server CRUD/connect/tool listing must require the active organization to match.
- **D-12:** Quota/Console tenant scope covers `quotas`, `billing_sessions`, `subscriptions`, `topup_orders`, and `usage_records`. Phase 11 does not change the v05 Relay endpoint billing model, but it must carry organization identity through existing quota and usage records.
- **D-13:** Marketplace publisher scope covers `published_agents`, `agent_versions`, `agent_installs`, and `agent_reviews`. Keep public browse/search global, but publisher-owned and installed/reviewed data must be scoped to the active organization.
- **D-14:** Admin channels, model routes, and platform package catalog remain platform-global in Phase 11. Admin organization/membership/audit views and Marketplace review data must show organization identity where the underlying data is tenant-owned.

### Authorization semantics
- **D-15:** Active members can use core app data for the selected organization. Membership management remains governed by Phase 10 owner/admin/member rules.
- **D-16:** Cross-tenant reads should return 403 when the caller is authenticated but not a member of the resource organization; 404 is acceptable only when existing handler contracts already use not-found semantics and tests lock that behavior.
- **D-17:** Cross-tenant writes must be denied before mutation. Tests must prove the database row is unchanged or absent after the denied request.
- **D-18:** Client-supplied `workspaceId`, `userId`, `organizationId`, `ownerId`, or `publisherId` in request bodies cannot grant access or choose tenant scope.

### Migration and rollout
- **D-19:** Use a single append-only migration, expected as `0027_tenant_scope_core_domains.sql`, with idempotent `ADD COLUMN IF NOT EXISTS`, backfill, indexes, and `SET NOT NULL` only after every row is populated.
- **D-20:** Backfill organizations from existing workspaces using deterministic IDs/slugs so existing data remains reachable after the migration.
- **D-21:** Backfill user-owned tables without `workspace_id` through the user's oldest workspace/default organization. If a user has no workspace, create a deterministic personal organization and owner membership first.
- **D-22:** Add indexes for `(organization_id, id)` and list-query shapes used by each migrated domain. Cross-tenant tests should fail if an organization filter is removed from representative queries.

### Verification
- **D-23:** Unit-only tests are insufficient. Phase 11 completion requires DB-backed HTTP integration tests across representative read and write paths for Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin organization data, and Marketplace publisher data.
- **D-24:** Broad closeout must run `bash scripts/test.sh all` and `bash scripts/check.sh all` with `TEST_DATABASE_URL` enabled and restricted-network Go proxy overrides if needed.
- **D-25:** Phase 11 summary must explicitly state that Relay billing completion, Stripe, Marketplace payout accounting, production orchestration, and product completeness remain v05-v08 work.
</decisions>

<specifics>
## Specific Ideas

- Add `src/server/internal/tenant/scope.go` with `Scope`, `Resolver`, and `ResolveSessionScope(ctx, session)`.
- Extend `auth.Session` with `OrganizationID string`.
- Extend auth responses with `organization` and `memberships` so the web app can keep organization context.
- Add `src/server/internal/http/tenant_scope_test.go` or focused tests in `server_test.go` covering cross-tenant denial.
- Representative cross-tenant HTTP tests should create two users, two organizations, two active sessions, one resource in organization A, then attempt read/write from organization B.
- Use existing `createHTTPOrganization`, `inviteHTTPMember`, and CSRF helpers in `src/server/internal/http/server_test.go` as the base for tenant-scope integration fixtures.
</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Commercial and phase scope
- `.planning/PROJECT.md` - v04 scope and full commercial-complete constraints.
- `.planning/REQUIREMENTS.md` - TENANT-04 and TENANT-05 definitions.
- `.planning/ROADMAP.md` - Phase 11 success criteria and likely verification.
- `.planning/STATE.md` - current workflow position.
- `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md` - non-negotiable Tenant And Identity Gate and later commercial gates.

### Tenant and security foundation
- `.planning/phases/09-tenant-model-and-migration-ledger/09-CONTEXT.md` - organization and migration-ledger decisions.
- `.planning/phases/09-tenant-model-and-migration-ledger/09-01-SUMMARY.md` - implemented tenant foundation evidence.
- `.planning/phases/10-membership-roles-and-auth-security/10-CONTEXT.md` - membership/auth decisions.
- `.planning/phases/10-membership-roles-and-auth-security/10-01-SUMMARY.md` - implemented membership and auth-security evidence.
- `src/server/migrations/0025_tenant_foundation.sql` - organization schema.
- `src/server/migrations/0026_membership_auth_security.sql` - membership, invitation, rate-limit, and password reset schema.
- `src/server/internal/tenant/types.go` - current organization/membership DTOs.
- `src/server/internal/tenant/service.go` - current membership and role validation.
- `src/server/internal/tenant/store.go` - current SQL-backed organization/membership store.

### Domain surfaces to tenant-scope
- `src/server/migrations/0001_phase1_foundation.sql` - users, workspaces, sessions, conversations, messages.
- `src/server/migrations/0005_usage_records.sql` - usage records.
- `src/server/migrations/0006_knowledge_bases.sql` through `0012_knowledge_document_chunks.sql` - Knowledge data.
- `src/server/migrations/0014_agents.sql` - Agent data.
- `src/server/migrations/0015_mcp_servers.sql` - MCP server data.
- `src/server/migrations/0016_pgvector.sql` and `0020_memory_hnsw.sql` - Memory data.
- `src/server/migrations/0017_quotas.sql` and `0021_plan_extensions.sql` - quota, billing sessions, subscriptions, top-ups.
- `src/server/migrations/0023_marketplace.sql` and `0024_categories_tags.sql` - Marketplace publisher/review/install data.
- `src/server/internal/chat/service.go` and `src/server/internal/chat/store.go` - Chat service and SQL filters.
- `src/server/internal/knowledge/service.go` and `src/server/internal/knowledge/store.go` - Knowledge service and SQL filters.
- `src/server/internal/agent/service.go` and `src/server/internal/agent/store.go` - Agent service and SQL filters.
- `src/server/internal/memory/service.go` - Memory service, store, and vector search filters.
- `src/server/internal/mcp/client.go` - MCP client/store and ownership checks.
- `src/server/internal/quota/service.go` - quota and billing session persistence.
- `src/server/internal/console/service.go` and `src/server/internal/console/store.go` - console usage/model/billing summaries.
- `src/server/internal/marketplace/service.go`, `src/server/internal/marketplace/store.go`, and `src/server/internal/marketplace/publisher_analytics.go` - Marketplace publisher and install/review data.
- `src/server/internal/http/router.go` - route composition.
- `src/server/internal/http/server_test.go` - DB-backed HTTP integration test fixture.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Phase 10 already creates owner memberships when admins create organizations.
- `authMiddleware` already resolves signed session cookies and injects `auth.Session` into request context.
- `tenant.Service.GetActiveMembership` and role helpers already provide membership enforcement primitives.
- `server_test.go` already has register/login, CSRF, organization, invitation, membership, and DB reset helpers.
- Most stores already filter by a single authority column, making migration to `organization_id` mechanical but broad.

### Current tenant gaps
- Chat and Knowledge filter by `session.WorkspaceID`.
- Agent, Memory, MCP, Quota, Marketplace publisher, installs, and reviews filter by `session.User.ID` or owner/user IDs.
- Console filters by `workspace_id` through `usage_records`.
- Sessions do not currently carry an organization ID.
- Normal registration creates a user workspace but no organization or owner membership.

### Integration Points
- Auth/session changes affect every authenticated route.
- Existing frontend assumes `/api/v1/auth/me` returns `workspace`; Phase 11 can add organization fields without removing workspace fields.
- Existing Relay quota hook settles by user ID; Phase 11 should add organization ID to quota/billing records without claiming v05 endpoint-classification completion.
- Marketplace public browse/search should remain unauthenticated/global; only publisher, install, review, and owner analytics paths are tenant-owned.
</code_context>

<deferred>
## Deferred Ideas

- v05 owns all Relay endpoint billing completion and production fail-closed endpoint classification.
- v06 owns Stripe production rollout, subscription lifecycle, refunds, invoices, Marketplace payout accounting, and settlement UX.
- v07 owns production orchestration, backup/restore, observability, alerts, dashboards, runbooks, and release/rollback.
- v08 owns full product completeness, RAG promise alignment, durable Agent workflows, real/default-disabled MCP tools, public docs, onboarding, and pricing.
</deferred>

---

*Phase: 11-tenant-scope-across-core-domains*
*Context gathered: 2026-05-28*
