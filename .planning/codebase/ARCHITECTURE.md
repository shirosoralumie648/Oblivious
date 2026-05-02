---
last_mapped_commit: c0e55fdbb3aaed7da80a0f7f2399237aed13bca3
mapped_dirty_worktree: true
---

# Architecture

**Analysis Date:** 2026-05-02

## System Shape

Oblivious is a React SPA plus Go backend for a multi-tenant AI platform. The core backend rule is that AI provider traffic should pass through the Relay layer so billing, quota, routing, and monitoring stay centralized.

```text
React SPA (`src/web`)
  |
  | relative HTTP calls to `/api/v1/*`; workspace/admin/console routes in React Router
  v
Go `net/http` API (`src/server/internal/http/router.go`)
  |-- auth/session middleware
  |-- app APIs: chat, knowledge, tasks, agents, memory, MCP, quota, notifications
  |-- console APIs: usage, access, models, billing
  |-- admin APIs: stats and users currently routed
  |-- websocket: `/api/v1/ws`
  |
  | when `RELAY_ENABLED=true`
  v
Gin Relay engine mounted at `/v1/*` (`src/server/internal/relay`)
  |
  | channel pool, load balancer, circuit breaker, token bucket, billing hook
  v
OpenAI-compatible upstream providers
```

## Entry Points

**Backend:**
- `src/server/cmd/server/main.go` loads config, opens PostgreSQL, builds `http.NewServer`, and handles shutdown.
- `src/server/internal/http/server.go` wraps the main handler and mounts Relay under `/v1/*`.
- `src/server/internal/http/router.go` constructs the app router, service graph, and middleware chain.
- `src/server/cmd/migrate/main.go` is the migration entrypoint.

**Frontend:**
- `src/web/src/main.tsx` mounts React.
- `src/web/src/app/App.tsx` provides the root app component.
- `src/web/src/app/router.tsx` declares marketing, workspace, console, and admin route trees.
- `src/web/src/app/appContext.tsx` owns auth bootstrap and preference updates.

## Backend Layering

**HTTP layer:**
- Route registration is mostly inline in `src/server/internal/http/router.go`.
- Handler structs live in `src/server/internal/http/*_handler.go`.
- Middleware and envelope helpers live in `src/server/internal/http/middleware.go`, `src/server/internal/http/auth_middleware.go`, and `src/server/internal/http/response.go`.

**Service layer:**
- Domain packages follow a service/store split under `src/server/internal/<domain>/`.
- Services accept `context.Context` and authenticated `auth.Session` where user ownership matters.
- Stores are concrete SQL implementations using `database/sql`.

**Domain packages:**
- Auth: `src/server/internal/auth/`.
- Chat and LLM gateway: `src/server/internal/chat/`.
- Agent runtime and tool loop: `src/server/internal/agent/`.
- Knowledge bases: `src/server/internal/knowledge/`.
- Memory/RAG: `src/server/internal/memory/`.
- MCP server/client/tool bridge: `src/server/internal/mcp/`.
- Tasks: `src/server/internal/task/`.
- Quota and billing sessions: `src/server/internal/quota/`.
- Admin management: `src/server/internal/admin/`.
- Marketplace: `src/server/internal/marketplace/`.
- Relay: `src/server/internal/relay/`.

## Relay Architecture

**Mounting:**
- `src/server/internal/http/server.go` calls `combineHandlers(mainHandler, relayInstance.Engine())`.
- Any request whose path starts with `/v1` is served by the Gin Relay engine.

**Routing:**
- `src/server/internal/relay/relay.go` builds the dependency graph: channel pool, load balancer, circuit breakers, token bucket, health checker, pricing store, and billing hook.
- `src/server/internal/relay/router.go` selects channels and runs `RouteWithBilling`.
- `src/server/internal/relay/channel/openai_adapter.go` translates provider requests.
- `src/server/internal/relay/handler/*.go` implements OpenAI-compatible endpoint families.

**Billing:**
- `src/server/internal/http/server.go` sets `relayInstance.Router().SetQuotaManager(quotaService)`.
- `src/server/internal/relay/billing.go` maps usage to `quota.Service.PreConsume`, `Settle`, and `Refund`.
- Trusted internal identity is carried through Relay context headers/types in `src/server/internal/relay/types/`.

## Agent, Memory, And MCP Flow

**Agent message flow:**
1. HTTP calls reach `src/server/internal/http/agent_handler.go`.
2. `agent.Service` validates ownership in `src/server/internal/agent/service.go`.
3. `agent.Runner` persists user/assistant/tool messages in `src/server/internal/agent/runner.go`.
4. `chat.ChatGateway` sends model requests through Relay or fallback generator.

**Tool loop:**
- Enabled tools are resolved from agent config.
- `src/server/internal/agent/executor.go` dispatches to builtin MCP tools or external MCP server tools.
- Structured tool-call plumbing is in `src/server/internal/chat/relay_gateway.go` and `src/server/internal/agent/runner.go`.

**Memory injection:**
- `agent.Runner.buildChatMessages` calls `memory.Search` when agent memory is enabled.
- `src/server/internal/memory/service.go` embeds and chunks documents on write.
- Search results are user-scoped through `session.User.ID`.

## Frontend Architecture

**Route shells:**
- Marketing pages use `src/web/src/features/layouts/MarketingLayout.tsx`.
- Authenticated workspace pages use `src/web/src/features/layouts/WorkspaceLayout.tsx`.
- Console pages use `src/web/src/features/layouts/ConsoleLayout.tsx`.
- Admin pages use `src/web/src/features/layouts/AdminLayout.tsx` behind `AdminRoute`.

**Feature APIs:**
- Stable API-client wrappers exist for auth, chat, console, knowledge, and tasks:
  - `src/web/src/features/auth/api.ts`
  - `src/web/src/features/chat/api.ts`
  - `src/web/src/features/console/api.ts`
  - `src/web/src/features/knowledge/api.ts`
  - `src/web/src/features/tasks/api.ts`
- New frontend code should use `createHttpClient` from `src/web/src/services/http/client.ts` rather than direct global `fetch`.

**State:**
- Global auth/preferences state is isolated in `src/web/src/app/appContext.tsx` and `src/web/src/features/auth/store.ts`.
- Pages mostly own local view state with React hooks.

## Data Model

**Migration ordering:**
- Base auth/workspace/chat schema begins in `src/server/migrations/0001_phase1_foundation.sql`.
- Knowledge, task, relay, agent, MCP, quota, admin, and marketplace tables are added append-only through `0024_categories_tags.sql`.
- `src/server/migrations/0020_memory_hnsw.sql` intentionally replaces an older vector index without editing `0016_pgvector.sql`.

**Store ownership:**
- Each domain store owns its own SQL queries and table joins.
- Cross-domain behavior should be coordinated in services, not by importing another package's SQL internals.

## Middleware And Response Contract

- `applyMiddleware` in `src/server/internal/http/router.go` applies recovery, request ID, logging, and CORS.
- App/API responses should use `writeSuccess` and `writeError` from `src/server/internal/http/response.go`.
- Frontend unwrapping is centralized in `src/web/src/services/http/envelope.ts`.

## Architectural Tensions

- `src/server/internal/http/router.go` is a 700+ line composition root and route table. It is useful as a single wiring point, but route growth is making it harder to review safely.
- Admin/marketplace domain services are richer than the exposed HTTP routes. Keep planning based on both code and route registration, not service files alone.
- Reference trees `lobehub/` and `new-api/` are not active app entrypoints. They should inform design decisions without being treated as shipped Oblivious code.
