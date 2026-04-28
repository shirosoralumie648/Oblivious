---
last_mapped_commit: f4dc5e48826c9893706249151aa081638e295dc1
---
# Codebase Concerns

**Analysis Date:** 2026-04-28

## Tech Debt

### Relay Handler Layer: Missing Auth, Billing, and Streaming

**What:** The entire `src/server/internal/relay/handler/` package (14 files) contains 7 TODO markers for unimplemented features. These are not stubs — the code compiles and passes traffic, but critical cross-cutting concerns are missing.

**Files:**
- `src/server/internal/relay/handler/realtime.go:45` — Auth middleware not wired
- `src/server/internal/relay/handler/realtime.go:47` — PreBill pre-deduction not implemented
- `src/server/internal/relay/handler/realtime.go:104` — Post-connection billing settlement missing
- `src/server/internal/relay/handler/batch.go:61` — PreBill for batch submissions
- `src/server/internal/relay/handler/batch.go:89` — Batch polling via Asynq not registered
- `src/server/internal/relay/handler/responses.go:66` — Responses SSE streaming not implemented (returns empty 200)
- `src/server/internal/relay/handler/files.go:168` — File mapping persistence to `relay_files` table not implemented (only fmt.Printf logging)

**Impact:** Relay API traffic flows without authentication, billing, or proper file tracking. The Responses SSE endpoint is a silent no-op. Batch submissions have no async result polling.

**Fix approach:** Implement in dependency order: (1) wire auth middleware first, (2) implement PreBill/BillingHook, (3) implement Responses SSE streaming modeled after `chat_handler.go`, (4) add Asynq polling for batch jobs.

### Hardcoded localhost Relay URLs

**What:** Multiple components construct relay URLs targeting `localhost` with configurable ports. These break in multi-host or containerized deployments.

**Files:**
- `src/server/internal/chat/relay_gateway.go:51` — Default: `http://localhost:8080/v1`
- `src/server/internal/memory/embedder.go:28` — Default: `http://localhost:8080/v1`
- `src/server/internal/http/router.go:50,72,83` — Constructed: `http://localhost:{port}/v1`

**Impact:** Cannot run relay and server on separate hosts without code changes. Localhost hardcoding prevents container orchestration where services run on different containers.

**Fix approach:** Add a `RELAY_URL` config field (`src/server/internal/config/config.go`) and use it consistently across all relay clients. Remove all hardcoded `localhost` defaults.

### HTTP Client Created Per Request (No Connection Pooling)

**What:** Every relay handler method creates a new `http.Client{}` instance per upstream request instead of sharing a pooled client. This is observed in 17 locations across the relay handler package.

**Files (representative):**
- `src/server/internal/relay/handler/batch.go:75` — `client := &http.Client{Timeout: 60 * time.Second}`
- `src/server/internal/relay/handler/batch.go:118` — `client := &http.Client{Timeout: 30 * time.Second}`
- `src/server/internal/relay/handler/responses.go:123` — `client := &http.Client{Timeout: 60 * time.Second}`
- `src/server/internal/relay/handler/completions.go:103`
- `src/server/internal/relay/handler/embeddings.go:106`
- `src/server/internal/relay/handler/chat.go:142`
- `src/server/internal/relay/handler/audio.go:187,212`
- `src/server/internal/relay/handler/moderations.go:104`
- `src/server/internal/relay/handler/files.go:98,154`
- `src/server/internal/relay/handler/fine_tuning.go:68`
- `src/server/internal/relay/handler/images.go:144`
- `src/server/internal/relay/handler/common.go:109`
- `src/server/internal/relay/handler/assistants.go:87`

**Impact:** Each upstream request incurs TLS handshake and TCP connection setup overhead. At high throughput this wastes CPU, memory (ephemeral ports, TIME_WAIT sockets), and adds latency.

**Fix approach:** Create a single shared `http.Client` with connection pooling (default transport already pools) and inject it into all handlers via constructor or a shared module-level variable. Set sensible `MaxIdleConnsPerHost` and `IdleConnTimeout`.

### Crude Token Estimation in Chat Service

**What:** `src/server/internal/chat/service.go:375-391` estimates token count as `runeCount / 4` with a ceiling adjustment. This is inaccurate:

- Non-English text (CJK characters) undercounts significantly (1 CJK character ~1-2 tokens, not 0.25)
- Code undercounts (symbols are individual tokens)
- Multi-byte emoji overcounts

**Files:** `src/server/internal/chat/service.go:382` — `tokenCount := runeCount / 4`

**Impact:** Token usage tracking and quota enforcement are unreliable. Users may exceed quotas or be incorrectly charged. Relay billing based on upstream-reported tokens is fine, but local estimation for pre-flight checks is wrong.

**Fix approach:** The project already depends on `github.com/pkoukk/tiktoken-go` (indirect, via go.sum). Use `tiktoken-go` with the model-specific encoding (e.g., `cl100k_base` for GPT-4) instead of the rune-based heuristic. Note: `src/server/internal/relay/tokenizer.go` exists and may already implement this — wire it into `chat/service.go`.

### Oversized Files Exceeding 500-Line Guideline

**What:** Several files exceed the project's self-imposed 500-line limit stated in `CLAUDE.md`.

**Files (source only, excluding test files):**
- `src/server/internal/http/router.go` — 726 lines (route registration + service wiring monolith)
- `src/server/internal/task/store.go` — 732 lines (SQL store with many queries)
- `src/server/internal/memory/service.go` — 664 lines
- `src/server/internal/knowledge/store.go` — 572 lines
- `src/server/internal/quota/service.go` — 511 lines
- `src/web/src/routes/workspace/SoloPage.tsx` — 719 lines (frontend)
- `src/web/src/routes/workspace/SoloPage.test.tsx` — 883 lines (frontend test)

**Impact:** Large files are harder to review, test, and maintain. `router.go` is particularly concerning since it wires ALL services in a single function.

**Fix approach:** For `router.go`: extract route registration into separate files by domain (`routes_auth.go`, `routes_chat.go` already exist but still call back to the monolith). Create a `RouteRegistrar` interface that each domain implements. For large stores: split query methods into query objects or repository pattern with separate files per entity.

### MongoDB Driver Unused Dependency

**What:** `go.mongodb.org/mongo-driver/v2 v2.5.0` appears in `src/server/go.mod` as an indirect dependency but no code references MongoDB.

**Files:** `src/server/go.mod` — listed under `require` with `// indirect`

**Impact:** Unnecessary dependency bloats binary size, increases supply chain attack surface, and slows builds.

**Fix approach:** Run `go mod tidy` and verify the dependency is truly unused. If so, it should be removed automatically. If something transitively pulls it, investigate why.

## Known Bugs

### Responses SSE Streaming Returns Empty 200

**Symptoms:** `POST /v1/responses` with `stream: true` returns HTTP 200 with SSE headers but no data.

**Files:** `src/server/internal/relay/handler/responses.go:61-67`

**Trigger:** Any Responses API request with `"stream": true`.

**Workaround:** Use non-streaming mode only (synchronous requests work correctly).

**Fix approach:** Implement SSE streaming similar to the Chat completions streaming handler. The non-streaming path in `executeRequest()` already works.

### Potential Goroutine Leak in Realtime WebSocket Proxy

**Symptoms:** If one WebSocket direction experiences a panic or the `WriteMessage` blocks indefinitely (no write deadline), one goroutine hangs while the other exits, leaving `wg.Wait()` blocked forever.

**Files:** `src/server/internal/relay/handler/realtime.go:77-100`

**Trigger:** Unstable network connection where one side of the WebSocket proxy stalls without triggering a read error.

**Workaround:** The current code lacks write deadlines and a context-based cancellation mechanism. No workaround exists in production.

**Fix approach:** Add write deadlines (`SetWriteDeadline`) on both connections. Pass a context with timeout to the goroutines and use `select` to abort on context cancellation. Consider using `context.WithCancel` derived from the gin request context.

### Race on Deferred vs. Goroutine Connection Close in Realtime Handler

**What:** `realtime.go` defers `upstreamConn.Close()` (line 63) and `clientConn.Close()` (line 70), but the proxy goroutines also call `Close()` on the peer connection (lines 82, 95). While `Close()` is safe to call multiple times on a `websocket.Conn`, this pattern creates confusion about ownership and could mask bugs.

**Files:** `src/server/internal/relay/handler/realtime.go:63,70,77-100`

**Fix approach:** Consolidate connection lifecycle: let the goroutines own closing their respective read-sides and signal completion via a channel. The outer function should only close connections if the goroutines haven't already, using a `sync.Once`.

## Security Considerations

### WebSocket CORS Allows All Origins

**What:** Both WebSocket upgrader instances set `CheckOrigin` to always return `true`, allowing any website to open WebSocket connections.

**Files:**
- `src/server/internal/relay/handler/realtime.go:14` — `CheckOrigin: func(r *http.Request) bool { return true }`
- `src/server/internal/relay/handler/chat.go:18` — `CheckOrigin: func(r *http.Request) bool { return true }`

**Risk:** Cross-site WebSocket hijacking (CSWSH). Any website can open WebSocket connections to the relay, potentially tunneling authenticated sessions if the user has cookies in their browser — though this is mitigated if auth is not yet wired on relay endpoints.

**Current mitigation:** The main HTTP router's CORS middleware (`src/server/internal/http/middleware.go:73-117`) correctly validates origins against a configured allowlist. However, WebSocket upgrade requests bypass this entirely since they use the `gorilla/websocket` upgrader directly.

**Recommendations:**
1. Wire the same CORS origin check into the WebSocket upgrader's `CheckOrigin` function.
2. Once auth middleware is implemented on relay handlers, add session validation in `CheckOrigin`.

### Relay API Exposed Without Authentication

**What:** The entire relay handler layer (`src/server/internal/relay/handler/`) has no authentication wired. The TODO at `realtime.go:45` acknowledges this but the code is live and reachable.

**Files:** All 14 files in `src/server/internal/relay/handler/`

**Risk:** Anyone who can reach the relay port can proxy requests to upstream AI providers (OpenAI, etc.) through the relay, consuming API credits and bypassing billing.

**Current mitigation:** Authentication is implemented for the main `/api/v1/app/*` routes via `authMiddleware.requireSession`. The relay layer presumably runs on a separate/internal port not exposed to the internet — but this is not enforced in code.

**Recommendations:** Prioritize wiring `authMiddleware` (or an API-key-based auth) into the relay handler chain. At minimum, gate the relay router behind the same session middleware used for the app API.

### Session ID Generation Uses crypto/rand But No Rate Limiting on Login

**What:** Session IDs are generated using `crypto/rand` (good), and passwords are hashed with bcrypt (good). However, there is no visible rate limiting on the `/api/v1/auth/login` endpoint.

**Files:**
- `src/server/internal/auth/service.go:90-97` — `NewID` function
- `src/server/internal/auth/store.go:69` — bcrypt comparison
- `src/server/internal/http/router.go:111` — login endpoint (no rate limit middleware)

**Risk:** Brute-force attacks on the login endpoint could succeed over time, though bcrypt's `DefaultCost` (10) slows each attempt. No account lockout after repeated failures.

**Recommendations:** Add rate limiting middleware specifically on auth endpoints. Track failed login attempts per IP and per email in the database. Implement account lockout after N consecutive failures.

### Session Expiry Not Enforced on Active Use

**What:** Sessions expire after 24 hours from creation (`src/server/internal/auth/store.go:82`) with no sliding expiration or refresh mechanism. A user actively using the system for over 24 hours will be logged out mid-session.

**Files:** `src/server/internal/auth/store.go:25,82` — `time.Now().Add(24 * time.Hour)`

**Risk:** Poor UX for long-running sessions. Not a security vulnerability per se, but may encourage users to implement workarounds (staying logged in, disabling security features).

**Recommendations:** Implement sliding expiration — update `expires_at` on each authenticated request, or issue refresh tokens for longer-lived sessions.

## Performance Bottlenecks

### No Request Context Propagation in Relay Upstream Calls

**What:** Relay handlers use `http.NewRequest` (which creates requests with `context.Background()`) instead of `http.NewRequestWithContext` propagating the gin request context. If the client disconnects, the upstream request continues consuming resources.

**Files:** All relay handler files — e.g., `src/server/internal/relay/handler/batch.go:66`, `src/server/internal/relay/handler/responses.go:116`, etc.

**Cause:** The standard library's `http.NewRequest` does not accept a context parameter. `NewRequestWithContext` must be used explicitly.

**Improvement path:** Replace all `http.NewRequest` calls with `http.NewRequestWithContext(c.Request.Context(), ...)` to propagate cancellation. This prevents wasted upstream API calls when clients disconnect.

### No Write Deadlines on WebSocket Proxy

**What:** The realtime WebSocket proxy (`src/server/internal/relay/handler/realtime.go:77-100`) writes messages without deadlines. A slow or stalled client can cause the `WriteMessage` call to block indefinitely, holding a goroutine and memory.

**Files:** `src/server/internal/relay/handler/realtime.go:85,98`

**Improvement path:** Add `SetWriteDeadline(time.Now().Add(30 * time.Second))` before each `WriteMessage` call. Alternatively, use a context-based approach with `WriteMessage` replacement that respects context cancellation.

## Fragile Areas

### http/router.go — Monolithic Route Registration

**Files:** `src/server/internal/http/router.go` (726 lines)

**Why fragile:** Every service is instantiated and every route is registered in a single `NewRouter` function. Adding a new domain requires modifying this central file. The function has deep nesting (up to 5 levels of if/switch) and mixes service wiring with HTTP routing.

**Safe modification:** When adding new routes, use the existing `routes_*.go` helper files as a pattern if they exist for the domain. If not, create a new `routes_<domain>.go` file following the `routes_auth.go` or `routes_chat.go` pattern.

**Test coverage:** `src/server/internal/http/server_test.go` (550 lines) provides integration-level coverage, but individual route groups are not independently tested.

### SoloPage.tsx — Large Frontend Component

**Files:** `src/web/src/routes/workspace/SoloPage.tsx` (719 lines), `src/web/src/routes/workspace/SoloPage.test.tsx` (883 lines)

**Why fragile:** The SoloPage component handles task creation, configuration, knowledge base selection, and execution mode — mixing multiple concerns. The test file is larger than the component, suggesting complex test setup compensating for component complexity.

**Safe modification:** The component already has a `SoloPageView.tsx` (391 lines) for the view layer. Further extract task form logic, knowledge base picker, and execution configuration into separate hooks or sub-components under `src/web/src/features/tasks/`.

**Test coverage:** Covered by a large test file (883 lines) but the tests likely test many combinations of component state rather than isolated units.

### relay/handler/ — Zero Test Coverage

**Files:** All 14 source files in `src/server/internal/relay/handler/` have zero test files.

**Why fragile:** This is the layer that proxies all external AI provider traffic. Any change to URL construction, header handling, error responses, or streaming logic has no automated verification. The layer is also where auth, billing, and async processing will be added.

**Safe modification:** Write integration tests for the handler layer before making changes. Each handler should have at minimum: (1) a test for successful upstream proxying with mocked HTTP, (2) a test for upstream error propagation, (3) a test for invalid input rejection.

**Test coverage:** 0 lines / 0 files. High priority.

### Missing Tests Across Core Domains

**Files:** These packages have zero test files:
- `src/server/internal/auth/` — 2 source files, 0 tests (AUTHENTICATION!)
- `src/server/internal/db/` — 1 source file, 0 tests
- `src/server/internal/mcp/` — 2 source files, 0 tests
- `src/server/internal/memory/` — 3 source files, 0 tests
- `src/server/internal/quota/` — 1 source file, 0 tests
- `src/server/internal/usage/` — 1 source file, 0 tests
- `src/server/internal/userprefs/` — 2 source files, 0 tests
- `src/server/internal/relay/handler/` — 14 source files, 0 tests
- `src/server/internal/relay/channel/` — 2 source files, 0 tests
- `src/server/internal/relay/types/` — 1 source file, 0 tests

**Risk:** Auth bugs could compromise all user data. Memory/embedding bugs cause silent data corruption. Quota bugs lead to billing errors.

**Priority:** Auth tests are HIGHEST priority. Follow with quota and memory.

## Scaling Limits

### No Horizontal Scaling Support

**Current capacity:** Single-process server. Session state stored in PostgreSQL. WebSocket connections held in-memory via `ws.DefaultHub()` (in-process singleton in `src/server/internal/ws/hub.go`).

**Limit:** WebSocket connections are pinned to a single process. Cannot scale beyond one instance without a WebSocket-aware load balancer (e.g., Redis pub/sub for cross-instance message fanout).

**Scaling path:** For multi-instance deployments:
1. Replace `ws.DefaultHub()` with a Redis-backed pub/sub hub for WebSocket fanout.
2. Ensure session tokens are stateless (JWT) or use a shared session store (Redis).
3. Add sticky sessions at the load balancer or implement connection migration.

### Token Bucket Rate Limiting is In-Process Only

**Files:** `src/server/internal/relay/tokenbucket.go`, `src/server/internal/relay/router.go:60-62`

**Current capacity:** Per-process token bucket. Multiple instances would each have independent rate limit counters.

**Limit:** Cannot enforce global rate limits across instances.

**Scaling path:** Replace the in-process token bucket with a Redis-based rate limiter (using the already-included `go-redis/v9` dependency) for cross-instance coordination.

## Dependencies at Risk

### Go Version `1.25.0` Does Not Exist

**Files:** `src/server/go.mod:3` — `go 1.25.0`

**Risk:** Go 1.25.0 is not a released version (current stable is 1.23.x as of early 2026). This may be a typo or placeholder. Building with an actual Go toolchain may produce unexpected behavior or fail.

**Impact:** CI/CD builds may fail. Local development may behave differently from CI.

**Migration plan:** Change to `go 1.23.0` or the actual Go version used in development/CI. Verify with `go version` and update `go.mod` accordingly.

### MongoDB Driver Present But Unused

**Files:** `src/server/go.mod` — `go.mongodb.org/mongo-driver/v2 v2.5.0 // indirect`

**Risk:** Unnecessary dependency adds to binary size, build time, and supply chain attack surface. If a future refactor accidentally imports it, the code may compile against MongoDB without test coverage.

**Impact:** Currently none (not imported), but represents drift.

**Migration plan:** Run `go mod tidy` to remove unused indirect dependencies.

### React 18 with Future Router v6

**Files:** `src/web/package.json`

**Risk:** React 18 and React Router v6 are stable but the ecosystem is shifting toward React 19 and React Router v7. The `@remix-run/router` dependency comments reference "v7" TODO items throughout.

**Impact:** No immediate impact. Upgrade path exists and is well-documented.

**Migration plan:** Monitor React Router v7 stable release. The codebase already uses the `routerFuture.ts` pattern (`src/web/src/app/routerFuture.ts`) which suggests awareness of the migration path.

## Missing Critical Features

### No Request Body Size Limits

**Problem:** HTTP request bodies have no size limit enforcement. Large file uploads or malicious payloads could exhaust server memory.

**Blocks:** Safe deployment in production environments.

**Recommendation:** Add `http.MaxBytesReader` or gin middleware to limit request body sizes per endpoint. The files handler (`src/server/internal/relay/handler/files.go`) should have a higher limit than other endpoints.

### No Structured Input Validation

**Problem:** The codebase uses parameterized SQL queries (good for SQL injection prevention) but has no systematic input validation framework. Email format, string lengths, and required fields are not consistently validated before reaching the database.

**Blocks:** Robust error messages and API contract enforcement.

**Recommendation:** Add validation using `github.com/go-playground/validator/v10` (already an indirect dependency via gin). Apply struct tags to request DTOs for automatic validation.

### No Graceful Shutdown for Relay WebSocket Connections

**Problem:** The realtime WebSocket handler (`realtime.go`) has no mechanism to gracefully close connections during server shutdown. The `defer conn.Close()` pattern works for connections that finish naturally, but active connections during shutdown are terminated abruptly.

**Files:** `src/server/internal/relay/handler/realtime.go:63,70`

**Blocks:** Zero-downtime deployments. Data loss for in-flight WebSocket messages.

**Recommendation:** Register all active connections in a connection registry. During shutdown, send WebSocket close frames and drain pending writes before force-closing.

## Test Coverage Gaps

### Authentication Package — Zero Tests

**What's not tested:**
- User registration flow (bcrypt password hashing, user creation, workspace creation, session creation)
- Login flow (password comparison, session creation, last_login_at update)
- Session retrieval and expiry checking
- Logout (session deletion)
- Error cases: duplicate email, invalid credentials, expired sessions

**Files:** `src/server/internal/auth/service.go`, `src/server/internal/auth/store.go`

**Risk:** Auth bugs are the highest-impact category. A regression could lock users out (impacting revenue) or allow unauthorized access (security incident).

**Priority:** High

### Relay Handler Layer — Zero Tests (14 files)

**What's not tested:**
- All API proxy flows (chat, completions, embeddings, audio, images, moderations, files, assistants, fine-tuning, batch, realtime, responses)
- Upstream error handling and propagation
- Request body parsing and validation
- Header construction and forwarding
- SSE streaming (chat.go, once responses.go is implemented)

**Files:** All 14 files in `src/server/internal/relay/handler/`

**Risk:** This is the primary revenue path (API proxy). Untested handler logic means upstream outages, API changes, or billing errors go undetected until users report them.

**Priority:** High

### Quota and Usage — Zero Tests

**What's not tested:**
- Credit deduction and balance tracking
- Package/top-up logic
- Usage recording and aggregation

**Files:** `src/server/internal/quota/service.go`, `src/server/internal/usage/store.go`

**Risk:** Billing errors directly impact revenue and user trust.

**Priority:** High

### Memory and Embedding — Zero Tests

**What's not tested:**
- Document chunking and embedding generation
- Vector similarity search
- Relay embedder integration

**Files:** `src/server/internal/memory/service.go`, `src/server/internal/memory/embedder.go`, `src/server/internal/memory/chunker.go`

**Risk:** Embedding quality degradation goes undetected. Search result quality regressions silently reduce product value.

**Priority:** Medium

### MCP Client — Zero Tests

**What's not tested:**
- MCP server connection lifecycle
- Tool discovery and execution
- Server status monitoring
- Encrypted auth token storage

**Files:** `src/server/internal/mcp/client.go`, `src/server/internal/mcp/builtin.go`

**Risk:** MCP integration is a key differentiator. Broken tool execution blocks agent workflows.

**Priority:** Medium

---

*Concerns audit: 2026-04-28*
