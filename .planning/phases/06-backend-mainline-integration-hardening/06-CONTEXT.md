# Phase 6: Backend Mainline Integration Hardening - Context

**Gathered:** 2026-05-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 6 hardens the backend slice already classified by Phase 5. It covers route registration, service wiring, auth/session contracts, Relay-backed Chat and Agent behavior, and targeted Go verification for ROUTE-01, CHAT-06, and AUTH-01.

This phase does not own frontend pages, Playwright, Docker/CI, deployment runtime proof, or contract documentation reconciliation. Those remain Phase 7 and Phase 8 work. Historical/reference material stays out of scope unless it directly proves a backend contract.

</domain>

<decisions>
## Implementation Decisions

### Route Registry And Auth Boundaries
- **D-01:** Keep `src/server/internal/http/router.go` as the active backend composition root unless the plan deliberately completes the route-split refactor. Partial route files may remain, but they must not create drift between intended routes and live routes.
- **D-02:** If route split files are wired in, remove matching inline registrations in the same backend commit and prove route parity with tests. Do not leave duplicate or conflicting route registration paths.
- **D-03:** All app route groups introduced or changed in this slice must require a session: Agent, Memory, MCP, Notification, Quota, Console, Preferences, Chat, Knowledge, Task, and WebSocket.
- **D-04:** Admin routes must continue using `requireAdmin`; Phase 6 must not weaken role checks while normalizing session/user fields.
- **D-05:** `/healthz` and `/metrics` are not changed by default in this phase. If a security issue is found, record it explicitly rather than folding a metrics policy change into unrelated backend wiring.
- **D-06:** Notification mutation routes must be reviewed for user ownership. `markRead` and `delete` need either user-scoped service/store checks or targeted tests proving users cannot modify another user's notification.
- **D-07:** WebSocket remains cookie-session authenticated via `authMiddleware.currentSession` and user ID binding. Anonymous WebSocket access and token-based WebSocket auth are out of scope for this phase.

### Relay-First Chat And Agent Contract
- **D-08:** When Relay is enabled, Chat and Agent model calls must go through the local Relay gateway and carry internal metadata. Production behavior must not silently bypass Relay through a direct provider fallback.
- **D-09:** Any direct HTTP model fallback must be treated as a development/demo fallback only, or must fail closed in production. The plan should make this explicit before closing CHAT-06.
- **D-10:** Use the existing `chat.WithRelayRequestMetadata` / `RelayRequestMetadata` contract to pass user ID, workspace ID, and request ID into Relay calls before they leave the app service layer.
- **D-11:** Preserve structured tool-call support for Agent. The `StructuredReplyGenerator` path, `RunWithTools`, assistant tool-call messages, role `tool` messages, and tool call serialization must remain covered by tests.
- **D-12:** Streaming function-call deltas are not required in Phase 6. The current strategy of non-streamed structured tool loops plus word-level final answer streaming is acceptable if tests protect it.
- **D-13:** Keep app-level `usage_records` for console visibility separate from Relay quota/billing settlement. Do not collapse these into one mechanism unless the plan also covers console and quota regressions.

### Auth, Session, And User Preferences Contract
- **D-14:** `auth.User` responses should consistently expose `id`, `email`, `name`, and `role` across register, login, session lookup, and `/api/v1/auth/me`.
- **D-15:** Default role remains `user`; admin authority comes only from persisted role and `requireAdmin`.
- **D-16:** Default display name should be stable. If no explicit name exists, use email or an existing deterministic default rather than returning inconsistent empty fields across auth flows.
- **D-17:** User preferences should keep existing defaults for `defaultMode`, `modelStrategy`, `defaultAgentModel`, `sidebarCollapsed`, and notification settings while preserving per-user isolation.

### Verification Strategy
- **D-18:** Phase 6 planning should start with focused failing or gap-covering Go tests for route/auth/session/Relay contracts, then implement or harden only the backend paths needed to pass them.
- **D-19:** Minimum targeted verification should include `go test` over `internal/http`, `internal/auth`, `internal/chat`, `internal/agent`, `internal/memory`, `internal/mcp`, `internal/notification`, `internal/quota`, `internal/relay`, `internal/userprefs`, and `internal/ws`.
- **D-20:** Before Phase 6 closeout, broaden to `cd src/server && go test ./... -count=1`. If DB-backed tests require `TEST_DATABASE_URL`, record the skip or runtime prerequisite explicitly.
- **D-21:** Do not commit the backend integration slice until the exact staged backend paths match the Phase 5 commit-boundary group.

### Agent's Discretion
- The planner may decide whether to finish wiring `routes_*.go` helpers or keep inline registration, as long as the live route surface is intentional and tested.
- The planner may choose fake stores, httptest routes, or DB-backed integration tests based on the least brittle way to prove the contract.
- The planner may add narrowly scoped backend tests in existing test files or new package-local test files.

</decisions>

<specifics>
## Specific Ideas

- Treat the current dirty backend files as user-owned input, not generated clutter.
- Phase 6 should explicitly inspect route groups for Agent, Memory, MCP, Notification, Quota, and WebSocket, because those are the highest-risk additions in the current backend slice.
- The current `RelayRequestMetadata` helper appears to be the intended path for trusted app-to-Relay headers; the plan should make sure Chat and Agent actually use it.
- Notification ownership and production Relay fallback are likely risk points worth testing early.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Active planning
- `.planning/PROJECT.md` - v03.3 scope, active requirements, and Relay-first core value.
- `.planning/REQUIREMENTS.md` - ROUTE-01, CHAT-06, and AUTH-01 definitions.
- `.planning/ROADMAP.md` - Phase 6 goal, success criteria, and likely verification commands.
- `.planning/STATE.md` - current state, dirty worktree constraints, and v03.2 deployment baseline.
- `.planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-WORKTREE-INVENTORY.md` - backend integration file list and handoff.
- `.planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-COMMIT-BOUNDARIES.md` - explicit backend staging group and do-not-stage rules.

### Codebase maps
- `.planning/codebase/STACK.md` - active stack, package boundaries, and verification commands.
- `.planning/codebase/ARCHITECTURE.md` - backend composition root, route surface, and current architectural risks.
- `.planning/codebase/INTEGRATIONS.md` - Relay, MCP, Auth, Quota, WebSocket, and external integration boundaries.

### Backend route and auth surface
- `src/server/internal/http/router.go` - active route registry and service wiring.
- `src/server/internal/http/server.go` - Relay mounting under `/v1` and quota manager wiring.
- `src/server/internal/http/auth_middleware.go` - session and admin guards.
- `src/server/internal/http/routes_auth.go` - route-split auth registration candidate.
- `src/server/internal/http/routes_chat.go` - route-split chat registration candidate.
- `src/server/internal/http/routes_console.go` - route-split console registration candidate.
- `src/server/internal/http/routes_knowledge.go` - route-split knowledge registration candidate.
- `src/server/internal/http/routes_preferences.go` - route-split preferences registration candidate.
- `src/server/internal/http/routes_task.go` - route-split task registration candidate.
- `src/server/internal/http/agent_handler.go` - Agent HTTP handler.
- `src/server/internal/http/memory_handler.go` - Memory HTTP handler.
- `src/server/internal/http/mcp_handler.go` - MCP HTTP handler.
- `src/server/internal/http/notification_handler.go` - Notification HTTP handler.
- `src/server/internal/http/quota_handler.go` - Quota HTTP handler.

### Backend service contracts
- `src/server/internal/auth/service.go` - auth user/session types.
- `src/server/internal/auth/store.go` - register/login/session persistence.
- `src/server/internal/userprefs/service.go` - preference defaults.
- `src/server/internal/userprefs/store.go` - preference persistence.
- `src/server/internal/chat/gateway.go` - reply interfaces, structured replies, and Relay metadata types.
- `src/server/internal/chat/relay_gateway.go` - Relay request, streaming, structured reply, and internal metadata header behavior.
- `src/server/internal/chat/service.go` - Chat message, stream, config, and usage-recording behavior.
- `src/server/internal/agent/service.go` - Agent ownership and service entry points.
- `src/server/internal/agent/runner.go` - Agent streaming, memory injection, and tool-call loop.
- `src/server/internal/memory/service.go` - Memory ownership and vector search service.
- `src/server/internal/mcp/client.go` - MCP server ownership and tool invocation client.
- `src/server/internal/notification/service.go` - Notification ownership and mutation behavior.
- `src/server/internal/quota/service.go` - quota and billing session service behavior.
- `src/server/internal/ws/handler.go` - WebSocket upgrade and user binding.
- `src/server/internal/ws/hub.go` - WebSocket hub behavior.

### Migrations and tests
- `src/server/migrations/0013_channels.sql` through `src/server/migrations/0019_admin_role.sql` - backend slice migrations from the Phase 5 inventory.
- `src/server/internal/http/server_test.go` - existing HTTP integration baseline.
- `src/server/internal/http/auth_middleware_test.go` - auth middleware baseline.
- `src/server/internal/chat/relay_gateway_test.go` - Relay metadata, streaming, and structured reply tests.
- `src/server/internal/chat/service_test.go` - chat usage recording baseline.
- `src/server/internal/agent/service_test.go` - Agent stream/tool behavior baseline.
- `src/server/internal/agent/store_test.go` - tool-call serialization baseline.
- `src/server/internal/notification/service_test.go` - notification service tests.
- `src/server/internal/quota/service_test.go` - quota service tests.
- `src/server/internal/ws/hub_test.go` - WebSocket hub tests.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `authMiddleware.requireSession`, `requireAdmin`, and `currentSession` are the shared route guard primitives.
- `chat.RelayGateway`, `CompositeGateway`, `StructuredReplyGenerator`, and `RelayRequestMetadata` are existing Relay-first building blocks.
- `agent.Runner` already supports plain streaming, structured tool loops, memory injection, and tool-call serialization.
- Existing Go tests cover auth middleware, chat service usage recording, relay gateway streaming/structured replies, agent tool loops, quota, notifications, and WebSocket hub behavior.

### Established Patterns
- Backend code follows handler -> service -> store, with route registration centralized in `router.go`.
- User ownership checks generally happen in services for Agent and Memory, while some handlers also validate session context.
- App HTTP responses use `writeSuccess` and `writeError` envelopes.
- Root workspace and CI exclude `lobehub/` and `new-api`; Phase 6 should not edit those reference trees.

### Integration Points
- Route/service wiring starts in `src/server/internal/http/router.go` and `src/server/internal/http/server.go`.
- Relay-first Chat/Agent behavior crosses `chat.Service`, `chat.RelayGateway`, `agent.Service`, `agent.Runner`, and `relay.Router`.
- Auth/session consistency crosses `auth.Service`, `auth.SQLStore`, `authMiddleware`, `authHandler`, and frontend-visible `/api/v1/auth/me` envelopes.
- Quota and billing cross `quota.Service`, `relay.Router().SetQuotaManager`, and chat/agent usage records.
- Memory and MCP plug into Agent through `SetMemory` and `SetMCPClient`.

</code_context>

<deferred>
## Deferred Ideas

- Frontend API types, route pages, Playwright, CI, Docker, and deployment validation remain Phase 7.
- API, architecture, release docs, README, and final verification evidence remain Phase 8.
- Metrics route authentication, Stripe webhook route registration, and broader production security hardening can become follow-up work if Phase 6 finds risk beyond ROUTE-01/CHAT-06/AUTH-01.
- Legacy workspace Marketplace route cleanup remains backlog 999.2 unless Phase 7 promotes it.

</deferred>

---

*Phase: 06-backend-mainline-integration-hardening*
*Context gathered: 2026-05-14*
