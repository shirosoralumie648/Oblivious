# Architecture

**Analysis Date:** 2026-05-12

## Scope

**Active mainline:**
- `src/server` is the active Go backend.
- `src/web` is the active React/Vite frontend and the only package included by `pnpm-workspace.yaml`.
- `config`, `scripts`, `.github/workflows`, `Dockerfile.server`, `Dockerfile.web`, `docker-compose.yml`, and `deploy/kubernetes` define runtime, validation, and release boundaries.

**Reference-only trees:**
- `lobehub/` and `new-api/` remain large imported reference trees. `README.md`, `pnpm-workspace.yaml`, and `scripts/check.sh` keep them outside the root workspace and root CI scope.

## High-Level Shape

Oblivious is a workspace application with a Go API, a React SPA, PostgreSQL as the system of record, and an optional OpenAI-compatible Relay mounted under `/v1`.

```text
Browser
  -> React/Vite app in `src/web`
  -> `/api/v1/*` application APIs or `/v1/*` Relay-compatible APIs
  -> Go `net/http` router in `src/server/internal/http`
  -> domain services in `src/server/internal/<domain>`
  -> SQL stores and PostgreSQL migrations in `src/server/migrations`
```

The backend follows a compact handler -> service -> store pattern. The frontend follows a route/layout -> page -> feature API client -> shared HTTP client pattern.

## Backend Entry Points

- `src/server/cmd/server/main.go` loads environment configuration, opens PostgreSQL through `src/server/internal/db/db.go`, creates `http.NewServer`, and handles graceful shutdown.
- `src/server/cmd/migrate/main.go` loads the same config, reads `src/server/migrations/*.sql`, sorts filenames, and applies all SQL files in lexical order.
- `src/server/internal/http/server.go` creates the primary router and conditionally wraps it with the Relay Gin engine when `RELAY_ENABLED` is true.
- `src/server/internal/http/router.go` is still the active composition root for app services, handlers, middleware, metrics, WebSocket, admin routes, marketplace routes, memory, MCP, agent, quota, console, knowledge, task, chat, auth, and preferences.

## Backend Layers

**HTTP layer:**
- Main app routes and handler wiring live in `src/server/internal/http/router.go`.
- Handler implementations live beside the router, for example `src/server/internal/http/chat_handler.go`, `src/server/internal/http/memory_handler.go`, and `src/server/internal/http/admin_handler.go`.
- Shared middleware, auth guards, and JSON envelopes live in `src/server/internal/http/middleware.go`, `src/server/internal/http/auth_middleware.go`, and `src/server/internal/http/response.go`.

**Domain layer:**
- Auth/session: `src/server/internal/auth`
- User preferences: `src/server/internal/userprefs`
- Chat and model gateway: `src/server/internal/chat`
- Knowledge CRUD and text retrieval: `src/server/internal/knowledge`
- SOLO task runtime: `src/server/internal/task`
- Agent runner/tool loop: `src/server/internal/agent`
- Memory documents, chunks, embeddings, and vector search: `src/server/internal/memory`
- MCP server client and built-in tool adapters: `src/server/internal/mcp`
- Relay routing, provider adapters, billing, retry, rate limits, health checks, and token accounting: `src/server/internal/relay`
- Admin control plane: `src/server/internal/admin`
- Marketplace catalog, installs, search, reviews, and publisher analytics: `src/server/internal/marketplace`
- Quota/packages: `src/server/internal/quota`
- Notifications: `src/server/internal/notification`
- Console summaries: `src/server/internal/console`
- Usage recording: `src/server/internal/usage`

**Infrastructure layer:**
- Env config: `src/server/internal/config/config.go`
- Database opening: `src/server/internal/db/db.go`
- Prometheus collectors: `src/server/internal/metrics/prometheus.go`
- WebSocket hub and handler: `src/server/internal/ws`
- SQL migrations: `src/server/migrations`

## Service Composition

`src/server/internal/http/router.go` constructs SQL stores and services directly:

- `auth.NewService(auth.NewSQLStore(database))`
- `userprefs.NewService(userprefs.NewSQLStore(database))`
- `chat.NewService` or `chat.NewServiceWithGateway` depending on Relay config
- `agent.NewService(agent.NewSQLStore(database), gateway)`, then `SetMemory` and `SetMCPClient`
- `memory.NewService(memory.NewSQLStore(database), relayEmbedder, chunker, model)` when Relay is enabled
- `mcp.NewClient(mcp.NewSQLStore(database))`
- `quota.NewService(quota.NewSQLStore(database))`
- `admin.NewService(admin.NewSQLStore(database))`
- `marketplace.NewService` and `marketplace.NewSearchService`

This makes the app easy to follow, but it also makes `router.go` a high-churn integration point.

## Route Architecture

**Application HTTP routes:**
- Health and metrics: `/healthz`, `/metrics`
- Auth: `/api/v1/auth/login`, `/register`, `/me`, `/logout`
- Preferences: `/api/v1/app/me/preferences`
- Chat: `/api/v1/app/models`, `/api/v1/app/conversations`, `/messages`, `/config`, `/convert-to-task`
- Knowledge: `/api/v1/app/knowledge-bases`, nested document routes, and `/retrieve`
- Tasks: `/api/v1/app/tasks`, `/start`, `/approve`, `/pause`, `/resume`, `/cancel`, `/budget`
- Agents: `/api/v1/app/agents`, nested conversations, messages, and tools
- Memory: `/api/v1/app/memory/documents`, `/chunks`, and `/search`
- MCP: `/api/v1/app/mcp-servers`, `/connect`, `/disconnect`, `/tools`, `/status`, `/execute`
- Console: `/api/v1/console/usage`, `/access`, `/models`, `/billing`
- Quota/packages: `/api/v1/app/quota`, `/packages`, `/quota/topup`
- Notifications: `/api/v1/app/notifications/*`
- WebSocket: `/api/v1/ws`
- Admin: `/api/v1/admin/*`
- Marketplace: `/api/v1/marketplace/*`

`src/server/internal/http/routes_auth.go`, `routes_chat.go`, `routes_knowledge.go`, `routes_preferences.go`, `routes_task.go`, and `routes_console.go` define modular registration functions, but current `NewRouter` still registers those routes inline. Treat the route files as an incomplete refactor until `router.go` actually calls them.

**Relay HTTP routes:**
- `src/server/internal/relay/handler/router.go` registers OpenAI-compatible `/v1/*` routes through Gin.
- `src/server/internal/http/server.go` uses `combineHandlers` to dispatch `/v1` requests to the Relay engine and everything else to the main `net/http` mux.
- Relay supports native and passthrough strategies for chat, responses, realtime, embeddings, images, videos, audio, moderations, completions, batch, files, fine-tuning, assistants, threads, and runs.

## Relay, Agent, Memory, MCP Flow

- Chat can use a direct HTTP reply generator or a composite gateway that tries local Relay first.
- The Agent service shares the chat gateway and can run with MCP tools through `agent.Runner`.
- When Relay is enabled, Memory uses `memory.NewRelayEmbedder` against local `/v1` with `text-embedding-3-small`.
- MCP server definitions are stored in PostgreSQL and used by `mcp.Client` to perform JSON-RPC initialize, tools/list, and tool calls.
- Quota is wired into the Relay billing lifecycle in `src/server/internal/http/server.go`.

## Frontend Architecture

- `src/web/src/main.tsx` mounts `App`.
- `src/web/src/app/App.tsx` creates the router and wraps it in `AppProviders`.
- `src/web/src/app/router.tsx` defines marketing, protected workspace, console, marketplace, and admin routes.
- `src/web/src/app/appContext.tsx` owns auth bootstrap state and preferences updates.
- `src/web/src/services/http/client.ts` is the shared fetch wrapper. It uses same-origin paths by default, unwraps backend envelopes, and throws `HttpError` on non-OK responses.
- Feature API clients live under `src/web/src/features/*/api.ts`.

## Frontend Route Groups

- Marketing: `/`, `/login`, `/register`
- Workspace: `/onboarding`, `/chat`, `/chat/:conversationId`, `/knowledge`, `/knowledge/:knowledgeBaseId`, `/solo`, `/solo/new`, `/settings`
- Marketplace: `/marketplace`, `/marketplace/agents/:agentId`, `/marketplace/publish`, `/marketplace/my-agents`
- Console: `/console`, `/console/models`, `/console/usage`, `/console/billing`, `/console/access`
- Admin: `/admin`, `/admin/channels`, `/admin/routes`, `/admin/plans`, `/admin/users`, `/admin/audit-log`, `/admin/reviews`

`ProtectedRoute` gates workspace, marketplace, console, and admin shells. `AdminRoute` additionally gates `/admin`.

## Data Architecture

- PostgreSQL is the active system of record.
- `src/server/migrations` contains migrations `0001_phase1_foundation.sql` through `0024_categories_tags.sql`.
- The schema covers auth/session/workspace, preferences, conversations, usage, knowledge, tasks, relay channels, agents, MCP servers, pgvector memory chunks, quota/packages, admin roles, plans, audit logs, marketplace, categories, and tags.
- The migration command does not maintain an explicit migration ledger in code; it executes all SQL files in sorted order. Existing migrations should be treated as append-only.

## Deployment Architecture

- `docker-compose.yml` defines `postgres`, `redis`, `oblivious-server`, and `oblivious-web`.
- `Dockerfile.server` builds the Go server and exposes `/healthz`.
- `Dockerfile.web` builds static Vite assets, serves them through nginx, and proxies `/api/` plus `/v1/` to the backend.
- `deploy/kubernetes` mirrors the same stack with namespace, config map, server, web, Postgres, Redis, and example secret manifests.
- `scripts/deploy-validate.sh` is the intended real deployment proof: render compose config, build images, start the stack, and smoke `/healthz`.

## Current Architectural Summary

The active product is no longer just chat, knowledge, and SOLO. The current worktree includes Relay, Agent, Memory, MCP, Quota, Admin, Marketplace, notifications, WebSocket, and deployment surfaces. The strongest architectural risk is that the central router has grown into both a composition root and a route registry while partial modular route files exist but are not wired.
