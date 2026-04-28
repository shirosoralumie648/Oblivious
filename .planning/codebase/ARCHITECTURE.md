---
last_mapped_commit: f4dc5e48826c9893706249151aa081638e295dc1
---

<!-- refreshed: 2026-04-28 -->
# Architecture

**Analysis Date:** 2026-04-28

## System Overview

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                        Web Client (React SPA)                            │
│  `src/web/`  —  Vite + React Router v7  —  features/routes pattern      │
├────────────────┬──────────────────┬───────────────────┬──────────────────┤
│  Marketing     │  Workspace       │  Console          │  Admin           │
│  /, /login,    │  /chat, /solo,   │  /console,        │  /admin,         │
│  /register     │  /knowledge,     │  /console/billing │  /admin/users    │
│  Layouts:      │  /settings       │  Layout:          │  Layout:         │
│  `Marketing`   │  Layout:         │  `Console`        │  `Admin`         │
│                │  `Workspace`     │                   │                  │
└───────┬────────┴────────┬─────────┴──────────┬────────┴────────┬─────────┘
        │                 │                    │                 │
        ▼                 ▼                    ▼                 ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                       HTTP API (`/api/v1/*`)                             │
│  `src/server/internal/http/router.go`  —  Single NewRouter() function    │
│  All routes registered inline in one function ~726 lines                 │
├──────────────────────────────────────────────────────────────────────────┤
│  Auth   │  Chat   │  Agent  │  Knowledge │  Task  │  Console  │  Memory  │
│  MCP    │  Quota  │  Admin  │  Notify    │  WS    │           │          │
└────────┬┴────────┬┴────────┬┴───────────┬┴───────┬┴──────────┬┴─────────┘
         │         │         │             │        │            │
         ▼         ▼         ▼             ▼        ▼            ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                     Service Layer (business logic)                        │
│  `src/server/internal/{auth,chat,agent,knowledge,task,console,mcp,...}/`  │
│  Each: service.go + store.go  —  Store interface + Service struct        │
└──────────────────────────────────┬───────────────────────────────────────┘
                                   │
         ┌─────────────────────────┼─────────────────────────┐
         ▼                         ▼                         ▼
┌──────────────────┐   ┌────────────────────┐   ┌──────────────────────┐
│  PostgreSQL DB   │   │  LLM Gateway       │   │  Relay Subsystem     │
│  `internal/db/`  │   │  (HTTPReplyGen /   │   │  `internal/relay/`   │
│  19 migrations   │   │   RelayGateway)    │   │  Gin-based, 35 API   │
│  raw SQL driver  │   │  `internal/chat/`  │   │  routes, load        │
│                  │   │                    │   │  balancing, billing  │
└──────────────────┘   └────────────────────┘   └──────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| `http` | HTTP routing, handlers, middleware, response envelope | `src/server/internal/http/router.go` |
| `auth` | Registration, login, logout, session management, bcrypt | `src/server/internal/auth/service.go` |
| `chat` | Conversations, messages, LLM gateway abstraction, streaming | `src/server/internal/chat/service.go` |
| `agent` | AI agent with tool-loop execution, Runner state machine, memory injection | `src/server/internal/agent/service.go` |
| `knowledge` | Knowledge base / document CRUD, text retrieval | `src/server/internal/knowledge/service.go` |
| `memory` | Document embedding (pgvector), chunking, semantic search | `src/server/internal/memory/service.go` |
| `task` | Long-running task state machine, budget tracking | `src/server/internal/task/runtime.go` |
| `console` | Workspace-level usage, billing, model, access summaries | `src/server/internal/console/service.go` |
| `relay` | OpenAI-compatible API proxy with load balancing, circuit breaking, billing | `src/server/internal/relay/relay.go` |
| `mcp` | MCP server registry, client for tool execution, builtin tools | `src/server/internal/mcp/client.go` |
| `ws` | Real-time WebSocket hub, per-user broadcast | `src/server/internal/ws/hub.go` |
| `config` | Environment variable parsing into typed Config struct | `src/server/internal/config/config.go` |
| `db` | PostgreSQL connection pool via `database/sql` with `lib/pq` | `src/server/internal/db/db.go` |
| `quota` | Quota checking and package management | `src/server/internal/quota/service.go` |
| `notification` | User notification CRUD, unread counts | `src/server/internal/notification/service.go` |
| `admin` | Admin stats, user listing, quota management | `src/server/internal/admin/service.go` |
| `userprefs` | User preference persistence (default mode, onboarding, model strategy) | `src/server/internal/userprefs/service.go` |
| `usage` | Usage recording store | `src/server/internal/usage/store.go` |
| `metrics` | Prometheus metrics exposition | `src/server/internal/metrics/prometheus.go` |

## Pattern Overview

**Overall:** Layered architecture with Store abstraction (Repository pattern via Go interfaces).

**Key Characteristics:**
- All HTTP routes defined inline in a single `NewRouter()` function spanning ~726 lines — no separate route modules
- Every internal package follows `Service` + `Store` interface pattern
- Dependencies are constructed manually in `NewRouter()` (manual DI, no framework)
- The backend operates in two modes: **Direct LLM** (HTTPReplyGenerator) and **Relay** (RelayGateway via internal proxy)
- Store implementations are almost exclusively concrete SQL structs implementing package-internal `Store` interfaces
- Web frontend uses a flat feature-based directory structure with co-located tests, API modules, and components

## Layers

**HTTP Handler Layer:**
- Purpose: Expose REST API, parse requests, delegate to services, write envelope responses
- Location: `src/server/internal/http/`
- Contains: `*_handler.go` files + `router.go`, `server.go`, `middleware.go`, `response.go`, `routes_*.go` (sub-route groups)
- Depends on: All internal service packages
- Used by: `cmd/server/main.go` (entry point), web client

**Service Layer:**
- Purpose: Business logic, validation, ownership checks, orchestration
- Location: `src/server/internal/{auth,chat,agent,console,knowledge,memory,task,mcp,notification,quota,admin,userprefs,usage}/`
- Contains: `service.go` (core logic), `store.go` (data access interface), sometimes `config.go`, `gateway.go`
- Depends on: Store interfaces, external gateways, auth sessions
- Used by: HTTP handler layer

**Store Layer:**
- Purpose: Data persistence, SQL queries, interface definitions
- Location: `store.go` within each service package (e.g., `src/server/internal/auth/store.go`)
- Contains: `Store` interface + `SQLStore` struct implementing it
- Depends on: `database/sql`, `*sql.DB`
- Used by: Service layer only

**Relay Subsystem:**
- Purpose: OpenAI-compatible API proxy with multi-provider support, load balancing, rate limiting, circuit breaking, billing
- Location: `src/server/internal/relay/`
- Contains: `relay.go` (core), `router.go` (channel selection), `pool.go` (channel registry), `loadbalancer.go`, `circuitbreaker.go`, `tokenbucket.go`, `billing.go`, `handler/` (35 API route handlers), `channel/` (provider adapters)
- Depends on: Gin framework (only place in the codebase that uses an external HTTP router)
- Used by: Mounted at `/v1/*` in `NewServer()`

**Web Frontend:**
- Purpose: User-facing SPA with React Router-based routing
- Location: `src/web/src/`
- Contains: `app/` (App, router, providers), `features/` (auth, chat, console, knowledge, layouts, tasks), `routes/` (per-route page components), `services/` (HTTP client), `store/`, `types/`, `theme/`
- Depends on: Backend API at `/api/v1/*`
- Used by: End users via browser

## Data Flow

### Primary Request Path (Chat Message)

1. User sends message via `ChatPage` UI (`src/web/src/routes/workspace/ChatPage.tsx`) -> HTTP POST to `/api/v1/app/conversations/{id}/messages`
2. `NewRouter()` dispatches to `chatHandler.sendMessage()` (`src/server/internal/http/chat_handler.go:167`)
3. `chatHandler` extracts session from context, decodes JSON body, calls `service.SendMessage()` (`src/server/internal/chat/service.go:279`)
4. Service persists user message to DB, builds conversation history, calls `replyGenerator.GenerateReply()` (`src/server/internal/chat/gateway.go:18` — ReplyGenerator interface)
5. Gateway (RelayGateway or HTTPReplyGenerator) calls LLM API either directly or through internal Relay
6. Assistant reply persisted to DB, usage recorded via UsageRecorder, full message list returned

### Relay Request Path (OpenAI API proxy)

1. External or internal client hits `/v1/chat/completions`
2. `combineHandlers()` in server.go dispatches `/v1/*` to Relay's Gin engine (`src/server/internal/http/server.go:79`)
3. Gin routes to `ChatHandler.Handle()` (`src/server/internal/relay/handler/chat.go:32`)
4. Handler builds ProviderRequest, calls `executeRequest()` which calls `router.RouteWithBilling()` (`src/server/internal/relay/chat.go:93`)
5. Relay Router selects channel via LoadBalancer (checks rate limit -> circuit breaker -> weighted/priority select)
6. BillingHook pre-bills, request proxied to upstream provider
7. Response returned, BillingHook post-bills, circuit breaker records success/failure

### Task Execution Flow

1. User converts conversation to task draft (`convertConversationToTask`)
2. Task created with budget, steps, execution mode
3. Task state machine (`src/server/internal/task/runtime.go`) manages transitions: draft -> running -> (paused/resumed) -> completed/cancelled
4. Each step marked running -> completed/paused
5. Events and result artifacts generated on completion

**State Management:**
- Session state: Cookie-based with HMAC signature, decoded per-request by `authMiddleware.currentSession()` (`src/server/internal/http/auth_middleware.go:30`)
- Application state (web): Custom `AuthStore` with pub/sub listener pattern (`src/web/src/features/auth/store.ts`)
- Channel pool: In-memory `sync.RWMutex`-guarded map with periodic DB sync (`src/server/internal/relay/pool.go`)
- WebSocket hub: Singleton `DefaultHub()` running a select-loop goroutine (`src/server/internal/ws/hub.go:182`)

## Key Abstractions

**Store Interface (Repository Pattern):**
- Purpose: Data access abstraction defined in each service package, implemented by SQLStore
- Examples: `chat.Store`, `auth.Store`, `knowledge.Store`, `memory.Store`
- Pattern: Interface in `store.go`, concrete `SQLStore` struct with `NewSQLStore(db *sql.DB)` constructor in same file

**ReplyGenerator (LLM Gateway Strategy):**
- Purpose: Abstract LLM invocation behind an interface supporting both direct API calls and internal Relay proxy
- Examples: `HTTPReplyGenerator`, `RelayGateway`, `CompositeGateway`
- Files: `src/server/internal/chat/gateway.go`, `src/server/internal/chat/relay_gateway.go`

**Channel (Relay Provider Abstraction):**
- Purpose: Represents an upstream LLM provider with model, URL, API key, rate limits
- Files: `src/server/internal/relay/pool.go`, `src/server/internal/relay/types/types.go`
- Pattern: Channels stored in pool, selected by load balancer, health-checked, circuit-breaker-protected

**Message Overrides:**
- Purpose: Allow per-message model/temperature/system-prompt overrides without changing conversation config
- Files: `src/server/internal/chat/gateway.go` (MessageOverrides struct)
- Pattern: Merged with conversation config via `mergeConversationConfig()`

**Agent Runner:**
- Purpose: Execute agent tool-loop with or without tool support, streaming support
- Files: `src/server/internal/agent/runner.go`, `src/server/internal/agent/executor.go`
- Pattern: Runner receives store, gateway, executor, memory — runs single-pass or tool-loop

## Entry Points

**Backend Server:**
- Location: `src/server/cmd/server/main.go`
- Triggers: `go run ./src/server/cmd/server` or `npm run dev:server`
- Responsibilities: Load config, open database, create HTTP server via `http.NewServer()`, listen and handle graceful shutdown

**Database Migrator:**
- Location: `src/server/cmd/migrate/main.go`
- Triggers: Run once to apply migrations
- Responsibilities: Read migration SQL files from `src/server/migrations/`, apply sequentially

**Web Client:**
- Location: `src/web/src/main.tsx`
- Triggers: `npm run dev:web` (Vite dev server)
- Responsibilities: Mount React app into DOM, render `<App />` with strict mode

**React Router:**
- Location: `src/web/src/app/router.tsx`
- Triggers: Every page navigation
- Responsibilities: Define route tree with marketing (public), workspace (authenticated), console, and admin sections; lazy route protection via `ProtectedRoute` and `AdminRoute`

**Relay Engine:**
- Location: `src/server/internal/relay/relay.go` (NewRelay)
- Triggers: Incoming requests to `/v1/*`
- Responsibilities: Initialize Gin engine, register 35 OpenAI-compatible routes, wire load balancer, circuit breakers, billing hook

## Architectural Constraints

- **Threading:** Single-threaded Go process with goroutine-per-request. WebSocket Hub runs single goroutine with channel-based messaging. ChannelPool is protected by `sync.RWMutex`.
- **Global state:** `ws.DefaultHub()` creates a singleton WebSocket hub (`src/server/internal/ws/hub.go:182`). Relay handler holds global `globalRouter` variable (`src/server/internal/relay/handler/router.go:10`).
- **Circular imports:** None detected. Internal packages import only from `auth` (Session type) and `config` (Config struct), with dependencies flowing downward.
- **No DI framework:** All dependency injection is manual in `NewRouter()` and `NewServer()`. The chain: `config.Load() -> db.Open() -> New*Store(db) -> NewService(store) -> NewHandler(service) -> route registration`.
- **Maximum file size:** The `router.go` handler file (~726 lines) exceeds the 500-line guideline stated in CLAUDE.md.

## Anti-Patterns

### Monolithic Router

**What happens:** All ~30 API routes are defined inline in a single `NewRouter()` function spanning 726 lines in `src/server/internal/http/router.go`.
**Why it's wrong:** Violates the project's 500-line file limit, makes it hard to locate specific routes, and couples all service construction together.
**Do this instead:** Follow the existing `routes_*.go` substructure pattern (e.g., `routes_auth.go`, `routes_chat.go`) — move each route group into its own function/file with `register*Routes(mux, handler, middleware)` signature.

### Dual Gateway Construction

**What happens:** When Relay is enabled, `NewRouter()` constructs a `CompositeGateway` combining `RelayGateway` and `HTTPReplyGenerator` as fallback; when disabled, only `HTTPReplyGenerator` is constructed. The same pattern is partially duplicated for agent service setup.
**Why it's wrong:** Gateway construction logic is duplicated and conditionals are embedded in route registration. The agent service also duplicates similar gateway setup logic.
**Do this instead:** Extract gateway construction into a factory function (`http.NewGateway(cfg)`) that returns a pre-built `ChatGateway`. Pass the same gateway to both chat and agent services.

### Relay Global State

**What happens:** The relay handler package uses a module-level `globalRouter` variable (`var globalRouter types.RouterInterface`) set via `SetRouter()`.
**Why it's wrong:** Global mutable state makes testing harder and creates implicit coupling between relay initialization and handler use.
**Do this instead:** Pass the `RouterInterface` through the handler constructor chain rather than storing it in a global variable.

## Error Handling

**Strategy:** Structured envelope responses with `Envelope{OK, Data, Error}` format.

**Patterns:**
- All HTTP handlers use `writeError(w, status, code, message)` for error responses (`src/server/internal/http/response.go:33`)
- All success responses use `writeSuccess(w, status, data)` (`src/server/internal/http/response.go:25`)
- Service layer returns Go errors checked by handlers; handlers convert to HTTP error envelopes
- Auth middleware returns 401/403 as envelope errors before reaching handlers
- Panic recovery in `withRecover` middleware (`src/server/internal/http/middleware.go:39`)
- Relay sends structured OpenAI-compatible error JSON via Gin

## Cross-Cutting Concerns

**Logging:** `log` package (stdlib). Request logging via `withLogging` middleware that records method, path, status, duration, request_id (`src/server/internal/http/middleware.go:61`).

**Validation:** Input validation at handler boundaries: JSON decode checks, empty string checks (e.g., `chat_handler.go:179`), trim validation. Service methods perform ownership verification (user ID matching). Conversation configs have parameter merging with defaults.

**Authentication:** Session cookie with HMAC-SHA256 signing (`src/server/internal/http/auth_middleware.go:105`). Two middleware levels: `requireSession` (standard auth) and `requireAdmin` (admin role check). Cookies are HttpOnly with configurable Secure flag.

**CORS:** Configurable allowed origins via `CORS_ALLOWED_ORIGINS` env var, applied through `withCORS` middleware.

**Metrics:** Prometheus endpoint at `/metrics` using `promhttp.Handler()`.

---

*Architecture analysis: 2026-04-28*
