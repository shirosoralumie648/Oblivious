---
last_mapped_commit: c0e55fdbb3aaed7da80a0f7f2399237aed13bca3
mapped_dirty_worktree: true
---

# Codebase Concerns

**Analysis Date:** 2026-05-02

## High Priority

### Admin/Marketplace Backend Is Richer Than Routed API

**What:** Phase 3 backend packages exist, but the active router exposes only a small admin subset and no marketplace routes.

**Evidence:**
- `src/server/internal/admin/channel_service.go`, `route_service.go`, `plan_service.go`, `audit_store.go`, and `user_service.go` implement admin domains.
- `src/server/internal/marketplace/service.go`, `store.go`, `search.go`, and `publisher_analytics.go` implement marketplace behavior.
- `src/server/internal/http/router.go` registers `/api/v1/admin/stats` and `/api/v1/admin/users*` only.
- `src/server/internal/http/router.go` has no routes for `marketplace`, `published_agents`, `reviews`, `categories`, plans, routes, channels, or audit logs.

**Impact:** Planning from service/store files alone overstates shipped API coverage. Frontend work cannot reach most backend Phase 3 capabilities without route handlers.

**Fix approach:** Add focused HTTP handlers and route registration for admin channels/routes/plans/audit/reviews and marketplace publish/search/install/review/category flows. Add handler tests as each route is exposed.

### Review Queue Store Methods Are Stubs

**What:** The admin review queue interface is present, but the SQLStore implementation returns "not implemented".

**Evidence:**
- `src/server/internal/admin/store.go` methods `ListPendingReviews`, `ApproveAgent`, and `RejectAgent` are stubs.
- Marketplace review methods exist separately in `src/server/internal/marketplace/store.go`.

**Impact:** Admin review actions can compile through the interface but fail at runtime if exposed through HTTP.

**Fix approach:** Either delegate admin review methods to marketplace SQL operations or remove the duplicate admin review queue path and route reviews through a marketplace service owned by admin handlers.

### Admin UI Contract Mismatches Current API Envelope

**What:** Admin pages use direct `fetch` and assume response shapes that do not match the backend envelope.

**Evidence:**
- `src/web/src/routes/admin/AdminUsersPage.tsx` expects `data.data` to be an array.
- `src/server/internal/http/admin_handler.go` returns `data: { users, total }` for `listUsers`.
- `src/web/src/routes/admin/AdminHomePage.tsx` and `AdminUsersPage.tsx` bypass `createHttpClient`.

**Impact:** `/admin/users` can render incorrectly even when the backend route works. Error handling and envelope parsing are inconsistent with the rest of the frontend.

**Fix approach:** Add `src/web/src/features/admin/api.ts`, use `createHttpClient`, type `ListUsersResponse`, and update pages/tests around the actual `{ users, total }` payload.

### Marketplace UI Installs MCP Servers Through The Wrong Contract

**What:** `MarketplacePage` is a static MCP catalog and posts command/args payloads to the MCP server endpoint, but the backend handler requires a URL-based MCP server request.

**Evidence:**
- `src/web/src/routes/workspace/MarketplacePage.tsx` sends `{ name, command, args, description }` to `/api/v1/app/mcp-servers`.
- `src/server/internal/http/mcp_handler.go` decodes `AddServerRequest` with `Name`, `URL`, and optional `AuthToken`, then requires `URL`.
- The actual marketplace backend under `src/server/internal/marketplace/` is not routed.

**Impact:** Marketplace install actions fail or do not install what the UI implies. The UI is closer to an MCP catalog mock than the Phase 3 marketplace product.

**Fix approach:** Decide whether this page is an MCP server catalog or an agent marketplace. For MCP, align payload and UX with `AddServerRequest`. For agent marketplace, expose marketplace HTTP routes and replace static data with backend search/install APIs.

## Medium Priority

### Router Composition Is Too Large

**What:** `src/server/internal/http/router.go` is over 700 lines and constructs services, handlers, and route matching inline.

**Impact:** Route additions are easy to misplace, duplicate, or leave partially wired. Admin/marketplace gaps are harder to spot because service creation and route registration share one long function.

**Fix approach:** Keep `NewRouter` as composition root, but split route registration into focused helpers such as `registerAgentRoutes`, `registerMemoryRoutes`, `registerAdminRoutes`, and `registerMarketplaceRoutes`.

### Relay Realtime And Responses Streaming Are Incomplete

**What:** Some Relay endpoint families still have explicit TODOs or incomplete streaming behavior.

**Evidence:**
- `src/server/internal/relay/handler/responses.go` has a TODO for Responses SSE streaming.
- `src/server/internal/relay/handler/realtime.go` has TODOs for auth, pre-billing, and close-time settlement.

**Impact:** `/v1/responses` streaming and realtime behavior can appear available while missing production-critical auth/billing semantics.

**Fix approach:** Model Responses streaming after the chat handler path, and make realtime auth/billing explicit before advertising it as supported.

### Relay URLs Are Hardcoded To Localhost In App-Originated Clients

**What:** Chat and memory clients construct Relay URLs from `localhost:{port}`.

**Evidence:**
- `src/server/internal/http/router.go` builds `http://localhost:{port}/v1` for chat, agent fallback gateway, and memory embedder.
- `src/server/internal/chat/relay_gateway.go` and `src/server/internal/memory/embedder.go` also default to localhost.

**Impact:** Containerized or split-host deployments break unless the server can call itself through localhost. This also complicates tests that want an injected Relay URL.

**Fix approach:** Add a `RELAY_URL` config field, document it in env/contracts, and inject it into chat/memory gateway constructors.

### WebSocket Origin Check Is Open

**What:** The WebSocket upgrader allows all origins.

**Evidence:**
- `src/server/internal/ws/handler.go` has `CheckOrigin: func(r *http.Request) bool { return true }`.

**Impact:** Browser-origin protection depends entirely on session cookies and route auth. Cross-origin WebSocket attempts are not restricted by configured CORS origins.

**Fix approach:** Reuse `CORS_ALLOWED_ORIGINS` or an explicit WebSocket origin allowlist and add handler tests.

## Lower Priority / Follow-Up

### Test Scripts Do Not Cover All Active Backend Packages

**What:** Root server checks run a focused package list, not `go test ./...`.

**Evidence:**
- `scripts/check.sh server` runs config/chat/knowledge/task/console only.
- `scripts/test.sh server` runs the same list plus `./internal/http` only when `TEST_DATABASE_URL` exists.

**Impact:** Admin, marketplace, agent, memory, relay, quota, metrics, notification, and ws regressions can be missed by default scripts.

**Fix approach:** Keep focused scripts for speed, but add a release or pre-merge command that runs `go test ./...` with repo-local caches. Expand focused checks when a package becomes product-critical.

### Builtin Tools Are Placeholder Implementations

**What:** Some builtin MCP tools return placeholder text instead of real behavior.

**Evidence:**
- `src/server/internal/mcp/builtin.go` describes `web_search` and `calculator` as simplified placeholder implementations.

**Impact:** Agent tool-loop demos can pass structurally while delivering non-real tool output.

**Fix approach:** Either hide placeholders from production tool catalogs or replace them with real integrations and tests.

### Global WebSocket Hub Makes Isolation Harder

**What:** `ws.DefaultHub()` is a global singleton and `/api/v1/ws` always uses it.

**Evidence:**
- `src/server/internal/ws/hub.go` defines `defaultHub` and `DefaultHub`.
- `src/server/internal/http/router.go` calls `ws.ServeWS(ws.DefaultHub(), ...)`.

**Impact:** Tests and multi-instance deployments have less control over hub lifecycle and broadcast isolation.

**Fix approach:** Inject a hub into router/server construction when WebSocket behavior becomes central to product flows.

### Planning State Can Drift From Code

**What:** `.planning/STATE.md` still labels Phase 3.1 as pending discussion, while code includes Phase 3 backend artifacts, Admin UI pages, Marketplace UI, and migrations through `0024`.

**Impact:** Workflow routing can understate or overstate real implementation depending on which artifact is read first.

**Fix approach:** For GSD routing, inspect `.planning/phases/`, route registration, migrations, and tests before trusting the headline in `.planning/STATE.md`.

## Security Checklist For Next Phases

- Do not store provider keys in docs or generated mapping files.
- Keep `SESSION_SECRET` required and signed cookie verification intact.
- Tighten WebSocket origin checks before production release.
- Ensure every admin route uses `requireAdmin`.
- Ensure every marketplace owner mutation validates ownership in service code.
- Ensure every Relay endpoint that can consume quota goes through billing or is explicitly marked unsupported.
