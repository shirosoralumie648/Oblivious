<!-- refreshed: 2026-05-16 -->
# Architecture

**Analysis Date:** 2026-05-16

## System Overview

Oblivious is a relay-first, multi-channel LLM gateway. The single load-bearing rule is: every AI call (chat completions, responses, embeddings, images, audio, moderations, completions, batch, files, fine-tuning, assistants, threads, runs, realtime) goes through the Relay layer at `/v1/*`. The application server (`src/server`) and the web SPA (`src/web`) consume the Relay through the same OpenAI-compatible surface.

```text
┌───────────────────────────────────────────────────────────────────────────┐
│                              Browser                                       │
│                       React 18 + React Router 6 SPA                        │
│                          `src/web/src/main.tsx`                            │
└────────┬────────────────────────────────────────────────────┬─────────────┘
         │ /api/v1/* (envelope JSON, SESSION cookie)          │ /api/v1/ws
         │                                                    │ (WebSocket)
         ▼                                                    ▼
┌───────────────────────────────────────────────────────────────────────────┐
│                       Application HTTP Surface                             │
│         Go net/http ServeMux + middleware (recover, request-id,            │
│                              logging, CORS)                                │
│                  `src/server/internal/http/router.go`                      │
│                  `src/server/internal/http/server.go`                      │
└────────┬──────────────────────────────────────────┬───────────────────────┘
         │ in-process call                          │ HTTP self-call to /v1
         ▼                                          ▼
┌───────────────────────────────────────────┐  ┌────────────────────────────┐
│        Bounded Domain Services            │  │       Relay Engine          │
│  auth, userprefs, chat, knowledge, task,  │  │  gin.Engine mounted under   │
│  agent, memory, mcp, notification, quota, │  │           /v1/*             │
│  console, admin, marketplace, ws, usage,  │  │   `src/server/internal/     │
│  stripe                                   │  │           relay/relay.go`   │
│  `src/server/internal/<bounded-context>/` │  │                             │
└────────┬──────────────────────────────────┘  │  Pool → LoadBalancer →      │
         │                                     │  Router (CB + TokenBucket   │
         │ chat.RelayGateway                   │  + Health + Billing) →      │
         │ memory.RelayEmbedder                │  Channel Adapter            │
         │ (HTTP self-call to /v1)             │                             │
         └────────────────────────────────────►│  `relay/router.go`          │
                                               │  `relay/pool.go`            │
                                               │  `relay/loadbalancer.go`    │
                                               │  `relay/handler/*.go`       │
                                               │  `relay/channel/openai_*`   │
                                               └────────────┬────────────────┘
                                                            │ HTTPS
                                                            ▼
                                               ┌────────────────────────────┐
                                               │ Upstream LLM Providers     │
                                               │ (OpenAI-compatible)         │
                                               └────────────────────────────┘

┌───────────────────────────────────────────────────────────────────────────┐
│                                Storage                                     │
│  PostgreSQL (lib/pq) — `src/server/migrations/*.sql`, `internal/db/db.go`  │
│  Optional Redis (asynq) — Stripe + billing timeout safety net               │
└───────────────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| Server entrypoint | Load config, open DB, build server, signal-aware shutdown | `src/server/cmd/server/main.go` |
| Migrate entrypoint | Apply SQL migrations in `migrations/` | `src/server/cmd/migrate/main.go` |
| HTTP server factory | Build main router and conditionally mount Relay gin engine under `/v1/*` | `src/server/internal/http/server.go` |
| Main router | Wire bounded contexts to `/api/v1/*` routes; apply middleware chain | `src/server/internal/http/router.go` |
| Auth middleware | Session cookie validation, `requireSession` / `requireAdmin` guards | `src/server/internal/http/auth_middleware.go` |
| Cross-cutting middleware | `withRecover`, `withRequestID`, `withLogging`, `withCORS` | `src/server/internal/http/middleware.go` |
| Relay engine | gin engine that owns every `/v1/*` route; OpenAI-compatible surface | `src/server/internal/relay/relay.go` |
| Relay route table | 35-route OpenAI-compatible table with Native / Passthrough / FileProxy strategies | `src/server/internal/relay/handler/router.go` |
| Relay routing core | `Router.Route` and `RouteWithBilling`: TokenBucket → LoadBalancer → CircuitBreaker → upstream call → billing settle/refund | `src/server/internal/relay/router.go` |
| Channel pool | In-memory channel registry hydrated from DB on boot | `src/server/internal/relay/pool.go`, `relay/store.go` |
| Channel adapter | Translates ProviderRequest into upstream OpenAI-compatible HTTP call | `src/server/internal/relay/channel/openai_adapter.go` |
| Chat domain | Conversations, messages, conversation config, convert-to-task | `src/server/internal/chat/service.go`, `chat/store.go` |
| Chat → Relay gateway | Self-HTTP call from chat service into Relay `/v1/chat/completions` | `src/server/internal/chat/relay_gateway.go` |
| Agent runtime | Tool-calling loop (default max 10 iterations) over `chat.ChatGateway` + memory + MCP | `src/server/internal/agent/runner.go`, `agent/executor.go`, `agent/service.go` |
| Task / SOLO runtime | Limited-scope state machine: draft → awaiting_confirmation → running → paused → completed/cancelled | `src/server/internal/task/runtime.go`, `task/service.go` |
| Memory | Document CRUD, chunker, embedder, user-isolated search; embedder is a Relay client | `src/server/internal/memory/service.go`, `memory/embedder.go`, `memory/chunker.go` |
| MCP | Built-in tools + remote MCP server client; injected into agent runner | `src/server/internal/mcp/client.go`, `mcp/builtin.go` |
| Knowledge | Knowledge base + document CRUD + text-match retrieval (no embeddings yet) | `src/server/internal/knowledge/service.go`, `knowledge/store.go` |
| Quota | Pre-authorize / settle / refund quota sessions; injected into Relay billing | `src/server/internal/quota/service.go` |
| Console | Read-only usage / access / models / billing summaries | `src/server/internal/console/service.go`, `console/store.go` |
| Admin | Channels, model routes, plans, users, audit logs, marketplace reviews | `src/server/internal/admin/*.go` |
| Marketplace | Discovery, publish, install, reviews, publisher stats, search | `src/server/internal/marketplace/service.go`, `marketplace/search.go` |
| Notification + WS | Persistent notifications + realtime push via gorilla/websocket hub | `src/server/internal/notification/service.go`, `ws/hub.go`, `ws/handler.go` |
| Usage recorder | Writes per-request usage rows used by console | `src/server/internal/usage/store.go` |
| Stripe | Checkout + webhook stubs for quota top-up | `src/server/internal/stripe/checkout.go`, `stripe/webhook.go` |
| Web SPA bootstrap | `ReactDOM.createRoot` mounts `<App />` with theme CSS imports | `src/web/src/main.tsx` |
| Web router | `createBrowserRouter` / `createMemoryRouter` with marketing / workspace / console / admin trees | `src/web/src/app/router.tsx` |
| App context | Auth bootstrap + preferences update; falls back to test-safe value with no provider | `src/web/src/app/appContext.tsx`, `app/providers.tsx` |
| HTTP client | Envelope-aware `get/post/put/delete` over `fetch`, throws `HttpError` on non-2xx | `src/web/src/services/http/client.ts`, `services/http/envelope.ts` |
| Feature API modules | Per-domain typed clients (`auth/api.ts`, `chat/api.ts`, `console/api.ts`, etc.) | `src/web/src/features/<area>/api.ts` |
| Route guards | `ProtectedRoute`, `AdminRoute` — drive workspace/console/admin trees | `src/web/src/features/auth/ProtectedRoute.tsx`, `features/auth/AdminRoute.tsx` |

## Pattern Overview

**Overall:** Relay-first layered monolith with bounded contexts.

**Key Characteristics:**
- Relay boundary is a hard contract: anything talking to an LLM speaks OpenAI-compatible HTTP against `/v1/*`, including in-process consumers (`chat.RelayGateway`, `memory.RelayEmbedder`) which self-call `http://localhost:<port>/v1` rather than reach inside the relay package.
- App API surface is a flat `/api/v1/*` envelope (`{ok, data, error}`) served by `net/http`; Relay surface is a 35-route OpenAI-compatible gin engine mounted onto the same listener.
- Domain services are wired manually in `NewRouter` (no DI framework); each context exposes `Service` + `Store` and a thin HTTP handler.
- Persistence is a single PostgreSQL database with SQL migrations under `src/server/migrations/`. Optional Redis powers asynq billing-timeout safety net.
- Frontend is a single Vite-built React 18 SPA with React Router 6 data-router; the SPA is served by nginx in production with `/api` and `/v1` reverse-proxied to the server.

## Layers

**Entrypoint layer:**
- Purpose: Process bootstrap, signal handling, graceful shutdown.
- Location: `src/server/cmd/server/main.go`, `src/server/cmd/migrate/main.go`.
- Contains: `main()` only.
- Depends on: `internal/config`, `internal/db`, `internal/http`.
- Used by: Container runtime (`Dockerfile.server`).

**HTTP transport layer:**
- Purpose: Translate HTTP into bounded-context service calls; enforce auth, CORS, recovery, logging, request-id.
- Location: `src/server/internal/http/`.
- Contains: `server.go` (factory), `router.go` (route table), `middleware.go`, `auth_middleware.go`, `response.go` (envelope writers), one `<context>_handler.go` per bounded context.
- Depends on: All bounded-context services, `internal/relay`, `internal/ws`.
- Used by: `cmd/server/main.go`.

**Relay layer:**
- Purpose: Single authoritative path to upstream LLM providers; multi-channel pooling, load balancing, circuit breaking, rate limiting, billing.
- Location: `src/server/internal/relay/`.
- Contains: `relay.go` (engine), `router.go` (routing + billing), `pool.go`, `loadbalancer.go`, `circuitbreaker.go`, `tokenbucket.go`, `healthchecker.go`, `billing.go`, `billing_worker.go`, `pricing.go`, `retry.go`, `tokenizer.go`, `store.go`, `handler/*.go` (per-API-type handlers), `channel/openai_adapter.go`, `types/*`.
- Depends on: `internal/quota` (via `QuotaManager` interface), upstream providers.
- Used by: `internal/http/server.go` mounts it; `internal/chat/relay_gateway.go` and `internal/memory/embedder.go` consume it over HTTP.

**Bounded-context service layer:**
- Purpose: Own a single domain's data and rules.
- Location: `src/server/internal/<context>/` — `auth`, `userprefs`, `chat`, `knowledge`, `task`, `agent`, `memory`, `mcp`, `notification`, `quota`, `console`, `admin`, `marketplace`, `usage`, `stripe`, `metrics`.
- Contains: `service.go` (use cases), `store.go` (SQL persistence), `*_test.go`, occasionally `types.go`.
- Depends on: `internal/db` (`*sql.DB`), peers via injected interfaces.
- Used by: Matching handler in `internal/http/`.

**WebSocket layer:**
- Purpose: Authenticated realtime fan-out for notifications.
- Location: `src/server/internal/ws/`.
- Contains: `hub.go` (default singleton hub), `handler.go` (`ServeWS`).
- Used by: `/api/v1/ws` route in `router.go` after `authMiddleware.currentSession`.

**Web SPA layers (`src/web/src/`):**
- `app/`: Composition root — `main.tsx`, `App.tsx`, `router.tsx`, `appContext.tsx`, `providers.tsx`.
- `routes/<area>/`: Page-level components consumed by the router (`marketing/`, `workspace/`, `console/`, `marketplace/`, `admin/`).
- `features/<domain>/`: Domain logic — `api.ts` per domain plus reusable components, stores, hooks (`auth/store.ts`, `auth/useAuthBootstrap.ts`, `layouts/*Layout.tsx`).
- `services/http/`: Single envelope-aware HTTP client + stream + upload + error types.
- `components/ui/`: Headless primitives (shadcn-style, Radix-backed).
- `components/shared/`: Cross-domain composites (`DataTable`, `EmptyState`, `MetricCard`, ...).
- `types/api.ts`, `types/admin.ts`: Backend contract types mirroring server responses.
- `store/app.ts`: Cross-cutting client state.

## Data Flow

### Primary Request Path — User Chat Message

1. Browser issues `POST /api/v1/app/conversations/{id}/messages` via `createHttpClient` (`src/web/src/services/http/client.ts:20`) using the session cookie.
2. `net/http` ServeMux dispatches to `chatHandler.sendMessage` after `authMiddleware.requireSession` (`src/server/internal/http/router.go:198`).
3. `chat.Service` persists the user message via `chat.SQLStore`, then asks its injected `ChatGateway` for a reply (`src/server/internal/chat/service.go`).
4. When `RELAY_ENABLED=true`, the gateway is `chat.RelayGateway`; it issues a `POST http://localhost:<port>/v1/chat/completions` self-call (`src/server/internal/chat/relay_gateway.go`).
5. The Relay gin engine routes to `ChatHandler.Handle` (`src/server/internal/relay/handler/chat.go`), which invokes `Router.RouteWithBilling` (`relay/router.go:181`).
6. `Router.RouteWithBilling` calls `billingHook.PreBill` (quota pre-authorize), then `Route` → `TokenBucket.TryAcquire` → `LoadBalancer.Select` → `CircuitBreaker` gate → upstream provider via `OpenAIAdapter`.
7. On success it records `usage` via `BillingHook.PostBill` (settle quota); on error it calls `Refund` and enqueues a Redis-asynq safety-net task when `billingRedisAddr` is set.
8. Response bubbles back into `chat.Service`, which stores the assistant message + writes a `usage` row via `usage.NewSQLRecorder`, then returns to the handler.
9. Handler writes the envelope (`response.go: writeJSON`/`writeError`) and the browser updates Zustand-style auth store / page state.

### Direct Relay Path — External `/v1/*` Consumer

1. Any HTTP client (test, integration, external app) calls `POST /v1/chat/completions` directly.
2. `combineHandlers` in `src/server/internal/http/server.go:86` recognizes the `/v1` prefix and forwards to the Relay gin engine, bypassing `/api/v1/*` middleware and the application session cookie.
3. Same Relay routing flow (steps 5–7 above). When the request carries `HeaderInternalAuth == OBLIVIOUS_INTERNAL_AUTH_TOKEN`, the handler attaches a trusted user id via `types.WithTrustedUserID` so billing settles against that user.

### Realtime Notification Flow

1. Server-side producer (any service) calls `ws.DefaultHub().Send(userID, payload)`.
2. Browser holds an authenticated WebSocket on `/api/v1/ws` (`src/server/internal/http/router.go:694`), registered against the hub keyed by `session.User.ID`.
3. Hub fans payloads out to all sockets registered for that user (`src/server/internal/ws/hub.go`).

**State Management:**
- Backend state lives in PostgreSQL exclusively; in-memory state (`ChannelPool`, `CircuitBreaker` map, `TokenBucket`, WS hub) is rebuilt on boot from the database where applicable.
- Frontend uses `useSyncExternalStore` over a small auth store (`src/web/src/features/auth/store.ts`) plus React Router data-router state; there is no Redux/RTK.

## Key Abstractions

**Channel:**
- Purpose: One upstream provider credential + model list + weights + RPM/TPM limits.
- Examples: `src/server/internal/relay/types/types.go`, persisted via `src/server/internal/relay/store.go`, hydrated into `ChannelPool` on boot (`src/server/internal/http/server.go:28`).
- Pattern: Aggregate root managed by Admin (`/api/v1/admin/channels`) and the Relay routing core.

**RouteChannel:**
- Purpose: Selected channel + scoring context for one routing decision.
- Examples: `src/server/internal/relay/types/types.go` (returned by `LoadBalancer.Select`).

**BillingSession:**
- Purpose: PreBill → PostBill / Refund lifecycle binding a single LLM call to a quota reservation.
- Examples: `src/server/internal/relay/billing.go`, used in `Router.RouteWithBilling` (`relay/router.go:195`).

**ChatGateway interface:**
- Purpose: Polymorphic LLM caller (`RelayGateway`, `HTTPReplyGenerator`, `CompositeGateway`).
- Examples: `src/server/internal/chat/gateway.go`, satisfied by `chat/relay_gateway.go`, consumed by `chat/service.go` and `agent/runner.go`.
- Pattern: Strategy + composition (`CompositeGateway` falls back from Relay to local LLM in non-production).

**Envelope (JSON success/error):**
- Purpose: Uniform `{ok, data, error}` response on every `/api/v1/*` route.
- Examples: `src/server/internal/http/response.go` (`writeJSON`, `writeError`); unwrap on the SPA in `src/web/src/services/http/envelope.ts`.

**AuthState (frontend):**
- Purpose: `idle | loading | authenticated | unauthenticated` state machine driving `ProtectedRoute` and `useAppContext`.
- Examples: `src/web/src/features/auth/store.ts`, `features/auth/useAuthBootstrap.ts`.

## Entry Points

**Application server (`oblivious-server`):**
- Location: `src/server/cmd/server/main.go`.
- Triggers: Container start (`Dockerfile.server` CMD) or `pnpm dev:server`.
- Responsibilities: Load config, open DB pool, build combined `net/http`+gin handler via `NewServer`, listen on `:SERVER_PORT`, install SIGINT/SIGTERM handler with 5s graceful shutdown.

**Migration runner (`oblivious-migrate`):**
- Location: `src/server/cmd/migrate/main.go`.
- Triggers: Container init job; manual invocation.
- Responsibilities: Apply numbered SQL files from `src/server/migrations/` in order.

**Web SPA bootstrap:**
- Location: `src/web/src/main.tsx` → `src/web/src/app/App.tsx` → `src/web/src/app/router.tsx`.
- Triggers: Browser load of `index.html` (nginx-served in production via `Dockerfile.web`).
- Responsibilities: Mount `<App />`, create router once with `useMemo`, wrap in `AppProviders` (auth bootstrap + preferences). `MainEntries` is left undefined to trigger `createBrowserRouter`; tests pass `initialEntries` to use `createMemoryRouter`.

**Relay engine mount:**
- Location: `src/server/internal/http/server.go:30` (constructs `Relay`, mounts under `/v1/*` via `combineHandlers`).
- Triggers: Only when `cfg.RelayEnabled == true` (default true).
- Responsibilities: Build `ChannelPool` from DB, ensure a default OpenAI channel when `OPENAI_API_KEY` is set and the pool is empty, wire `quota.Service` into the billing hook, expose the gin engine.

**WebSocket entry:**
- Location: `/api/v1/ws` registered in `src/server/internal/http/router.go:694`.
- Triggers: Authenticated client connection.
- Responsibilities: Upgrade to WS via `ws.ServeWS(ws.DefaultHub(), …)` keyed by `session.User.ID`.

**Background asynq billing worker:**
- Location: `src/server/internal/relay/billing_worker.go`.
- Triggers: Redis-backed delayed task enqueued by `RouteWithBilling` on error.
- Responsibilities: Safety-net refund / settle when upstream may have consumed resources despite a client-side error.

## Architectural Constraints

- **Single mainline:** `src/server` and `src/web` are the only mainline runtime trees. `new-api/` and `lobehub/` are non-mainline reference checkouts and must not be imported, built, or tested by mainline CI. See `docs/architecture/current-system-contracts.md` §2.
- **Relay boundary:** No service may bypass `/v1/*` and call an LLM provider directly. `chat.RelayGateway` and `memory.RelayEmbedder` deliberately go out and back through HTTP so the Relay's load balancing, circuit breaker, token bucket, and billing always apply. `chat.HTTPReplyGenerator` exists only as a non-production local fallback inside `CompositeGateway`.
- **Threading:** `net/http` and gin both serve concurrent requests; per-request goroutines. `ChannelPool`, `BillingHook`, `CircuitBreaker`, `TokenBucket`, and `ws.Hub` are designed for concurrent access via internal locking. `billing_worker.go` uses asynq's worker pool.
- **Global state:** `handler.globalRouter` (`src/server/internal/relay/handler/router.go:10`) is a package-level pointer set by `relay.NewRelay` via `handler.SetRouter`; this is intentional but constrains running two relay instances per process. `ws.DefaultHub()` is a process singleton.
- **Self-call coupling:** Relay clients inside the same process target `http://localhost:<SERVER_PORT>/v1`. The port is read from `cfg.Port` in `NewRouter`. Running the application without `RELAY_ENABLED` falls back to direct `chat.HTTPReplyGenerator`, which violates the Relay boundary and must only be used in dev.
- **SOLO runtime is bounded:** Per `docs/architecture/solo-runtime-decision.md`, `task/runtime.go` is a constrained state machine, not a real agent executor. Do not grow it into one without an explicit roadmap decision.
- **Envelope discipline:** Every `/api/v1/*` handler must use `writeJSON`/`writeError`. Direct `w.Write` on JSON without an envelope breaks the SPA's `unwrapEnvelope` and the contract in `docs/architecture/current-system-contracts.md` §3.
- **CORS:** Only origins in `CORS_ALLOWED_ORIGINS` get CORS headers; an empty list disables CORS entirely (no headers, no preflight). See `src/server/internal/http/middleware.go:78`.

## Anti-Patterns

### Calling an LLM provider from inside a service

**What happens:** A service imports an HTTP client or SDK and calls `https://api.openai.com/v1/...` directly.
**Why it's wrong:** Bypasses channel pool, load balancing, circuit breaker, rate limit, billing, and audit; breaks the load-bearing project invariant ("all AI calls MUST go through Relay").
**Do this instead:** Inject a `chat.ChatGateway` and use `chat.NewRelayGateway(...)` (see `src/server/internal/http/router.go:50`). For embeddings, use `memory.NewRelayEmbedder` (`src/server/internal/memory/embedder.go`).

### Adding a new `/api/v1/*` route by writing handler logic directly in the mux

**What happens:** New code stuffs business logic into the `HandlerFunc` body inside `router.go`.
**Why it's wrong:** `router.go` is already >1000 lines; mixing logic into routing breaks the bounded-context separation and the 500-line file rule in `CLAUDE.md`.
**Do this instead:** Add a `<context>_handler.go` next to the existing handlers in `src/server/internal/http/`, expose methods on it, and bind them in `router.go` with the same `authMiddleware.requireSession(...)` shape used by `chatHandler` / `taskHandler`.

### Returning raw JSON without the envelope

**What happens:** A handler writes `w.Write([]byte(\`{"foo":1}\`))` or `json.NewEncoder(w).Encode(payload)` directly.
**Why it's wrong:** SPA's `unwrapEnvelope` (`src/web/src/services/http/envelope.ts`) throws; the route also fails the `docs/architecture/current-system-contracts.md` §3 contract.
**Do this instead:** Use `writeJSON(w, status, payload)` or `writeError(w, status, code, message)` from `src/server/internal/http/response.go`.

### Reaching into the Relay package from a domain service

**What happens:** A service imports `oblivious/server/internal/relay` to call `Router.Route` in-process.
**Why it's wrong:** Couples domain code to the relay engine internals, breaks the HTTP boundary that the Relay relies on for context propagation (`HeaderInternalAuth`, `HeaderInternalUserID`), and makes the LLM call invisible to gin middleware.
**Do this instead:** Self-call the local Relay over HTTP via `chat.RelayGateway` or `memory.RelayEmbedder`, mirroring `src/server/internal/http/router.go:50` and `:86`.

### Mounting a workspace route without `ProtectedRoute`

**What happens:** A new authenticated page is added under `MarketingLayout` instead of inside the `<ProtectedRoute>` subtree.
**Why it's wrong:** Bypasses the auth-state machine in `src/web/src/features/auth/ProtectedRoute.tsx` and lets unauthenticated visitors hit pages that assume a session.
**Do this instead:** Add the route under the `<ProtectedRoute>` block in `src/web/src/app/router.tsx` and reuse `WorkspaceLayout` / `ConsoleLayout` / `AdminLayout` as appropriate. Admin pages must additionally sit under `<AdminRoute>`.

## Error Handling

**Strategy:** Domain services return typed `error`; HTTP handlers translate to a small set of stable error codes from `docs/architecture/current-system-contracts.md` §3 (`invalid_request`, `invalid_credentials`, `unauthorized`, `method_not_allowed`, `not_found`, `internal_error`).

**Patterns:**
- `withRecover` middleware (`src/server/internal/http/middleware.go:39`) converts panics into `500 internal_error`.
- `writeError` in `response.go` is the single egress for failures and always writes the `{ok:false, data:null, error:{code,message}}` envelope.
- Relay layer defines its own error type `relay.RouterError` (`src/server/internal/relay/router.go:161`) carrying HTTP code + `Retry-After`; Relay handlers translate these to OpenAI-shaped error JSON via `relay/handler/common.go`.
- Frontend `HttpError` (`src/web/src/services/http/errors.ts`) wraps non-2xx responses with the envelope's `error.message` when available.

## Cross-Cutting Concerns

**Logging:** Backend uses `log` package via `withLogging` (`middleware.go:66`) — one log line per request with `method`, `path`, `status`, `duration`, `request_id`. Request IDs are propagated via the `X-Request-Id` response header and `requestIDContextKey` in `context.Context`.

**Validation:** Performed inside each handler before calling the service (e.g., method check via `writeError(..., "method_not_allowed", ...)`; JSON decode + struct validation in the handler body). No central validator is used by the main router; the Relay layer relies on gin's binding for select endpoints.

**Authentication:** HMAC-signed session token stored in `oblivious_session` HttpOnly cookie (`src/server/internal/http/auth_middleware.go`). `authMiddleware.requireSession` and `authMiddleware.requireAdmin` wrap every authenticated route; current user is exposed to handlers via `authMiddleware.currentSession(r)`.

**Authorization:** Bounded contexts scope queries by `user_id` / `workspace_id` from the session. Admin endpoints (`/api/v1/admin/*`) require `requireAdmin`. Marketplace publish/install/review require session (`serveWithSession`).

**Metrics:** Prometheus handler exposed at `/metrics` via `promhttp.Handler()` (`src/server/internal/http/router.go:40`). Bounded contexts emit counters from `internal/metrics/`.

**Internal-call trust:** When a service self-calls Relay it must set `HeaderInternalAuth` to `OBLIVIOUS_INTERNAL_AUTH_TOKEN` and `HeaderInternalUserID` to the acting user, allowing `types.WithTrustedUserID` to attribute billing correctly.

---

*Architecture analysis: 2026-05-16*
