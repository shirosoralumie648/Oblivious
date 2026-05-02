---
last_mapped_commit: c0e55fdbb3aaed7da80a0f7f2399237aed13bca3
mapped_dirty_worktree: true
---

# External Integrations

**Analysis Date:** 2026-05-02

## AI Provider Relay

**OpenAI-compatible Relay:**
- Public API surface is mounted at `/v1/*` when `RELAY_ENABLED=true` in `src/server/internal/http/server.go`.
- Gin routes are registered by `src/server/internal/relay/handler/router.go`.
- Channel selection, circuit breaking, rate limiting, retry/fallback, and quota billing live in `src/server/internal/relay/router.go`.
- Provider adaptation currently centers on OpenAI-compatible upstreams through `src/server/internal/relay/channel/openai_adapter.go`.
- Channel configuration is stored in PostgreSQL tables managed by `src/server/migrations/0013_channels.sql` and loaded through `src/server/internal/relay/store.go`.

**Default development channel:**
- `src/server/internal/http/server.go` creates a default OpenAI channel when the channel pool is empty and `OPENAI_API_KEY` is configured.
- Do not document real provider keys in planning docs. Use env var names only.

**App-originated LLM calls:**
- Chat uses `chat.NewRelayGateway` when relay is enabled (`src/server/internal/http/router.go`).
- The fallback direct generator is `chat.NewHTTPReplyGenerator` in `src/server/internal/chat/gateway.go`.
- Agent runtime reuses the chat gateway via `src/server/internal/agent/service.go`.

## Memory And Embeddings

**Vector memory path:**
- Memory documents and chunks are handled by `src/server/internal/memory/service.go`.
- Embeddings call Relay through `src/server/internal/memory/embedder.go`.
- Text chunking is implemented in `src/server/internal/memory/chunker.go`.
- pgvector schema starts in `src/server/migrations/0016_pgvector.sql`.
- HNSW index replacement is append-only in `src/server/migrations/0020_memory_hnsw.sql`.

**API surface:**
- Memory endpoints are registered inline in `src/server/internal/http/router.go`.
- Handler implementation is `src/server/internal/http/memory_handler.go`.
- Search is per authenticated user through `memory.Search(ctx, session, request)`.

## MCP Tools

**External MCP servers:**
- MCP server records are stored by `src/server/internal/mcp/client.go` through `mcp_servers`.
- HTTP JSON-RPC calls cover `initialize`, `tools/list`, and `tools/call`.
- App endpoints are implemented in `src/server/internal/http/mcp_handler.go`.
- Agent tool execution reaches MCP through `src/server/internal/agent/executor.go`.

**Builtin tools:**
- Builtins are defined in `src/server/internal/mcp/builtin.go`.
- Available names are `web_search`, `calculator`, `datetime`, and `http_request`.
- `web_search` and `calculator` are placeholder implementations; do not rely on them as real integrations until replaced.

## PostgreSQL

**Primary database:**
- The server opens a single `*sql.DB` from `DATABASE_URL` in `src/server/cmd/server/main.go`.
- Stores are plain SQL implementations under domain packages, for example `src/server/internal/chat/store.go`, `src/server/internal/agent/store.go`, `src/server/internal/marketplace/store.go`, and `src/server/internal/admin/*_store.go`.
- Migrations are sequential SQL files in `src/server/migrations/`.

**Schema areas:**
- Auth/workspaces/chat: `0001_phase1_foundation.sql`.
- Preferences/conversation config/usage/tasks/knowledge: `0002` through `0012`.
- Relay channels/routes/agents/MCP/quota/admin/marketplace: `0013` through `0024`.

## Authentication And Sessions

**Cookie sessions:**
- Login/register/logout are handled by `src/server/internal/http/auth_handler.go`.
- Session cookies are HMAC-signed using `SESSION_SECRET` in `src/server/internal/http/auth_middleware.go`.
- Authenticated API routes use `authMiddleware.requireSession`.
- Admin API routes use `authMiddleware.requireAdmin` and require `session.User.Role == "admin"`.

**Frontend auth bootstrap:**
- `src/web/src/app/appContext.tsx` calls `createAuthApi(createHttpClient())`.
- `src/web/src/features/auth/useAuthBootstrap.ts` controls the bootstrap flow.
- Protected route shells are `src/web/src/features/auth/ProtectedRoute.tsx` and `src/web/src/features/auth/AdminRoute.tsx`.

## Admin And Marketplace

**Admin backend:**
- Admin service/store packages are under `src/server/internal/admin/`.
- Channel, route, plan, user, and audit store/service files exist, but `src/server/internal/http/router.go` currently exposes only stats and user routes.
- Admin review queue methods in `src/server/internal/admin/store.go` still return "not implemented".

**Marketplace backend:**
- Marketplace domain logic is under `src/server/internal/marketplace/`.
- Database tables are created by `src/server/migrations/0023_marketplace.sql` and `src/server/migrations/0024_categories_tags.sql`.
- No marketplace HTTP routes are currently registered in `src/server/internal/http/router.go`.

**Marketplace frontend:**
- `src/web/src/routes/workspace/MarketplacePage.tsx` is a static MCP server catalog that posts to `/api/v1/app/mcp-servers`.
- It does not yet consume `src/server/internal/marketplace/` APIs because those APIs are not routed.

## Billing, Quota, And Payments

**Quota billing lifecycle:**
- `src/server/internal/quota/service.go` manages balances, packages, subscriptions, topups, and billing sessions.
- `src/server/internal/relay/billing.go` adapts relay usage to quota preconsume/settle/refund.
- `src/server/internal/http/server.go` wires `quota.Service` into `relay.Router`.

**Stripe:**
- `github.com/stripe/stripe-go/v83` is present.
- `src/server/internal/stripe/webhook.go` exists, but no Stripe webhook route is visible in `src/server/internal/http/router.go`.

**Async billing worker:**
- `src/server/internal/relay/billing_worker.go` uses Asynq for timeout/polling safety tasks.
- The relay constructor passes an empty Redis address, so timeout queueing is disabled unless wiring is added.

## Observability And Realtime

**Metrics:**
- `/metrics` is registered in `src/server/internal/http/router.go`.
- Prometheus helpers/tests live under `src/server/internal/metrics/`.

**WebSocket notifications:**
- Authenticated app WebSocket route is `/api/v1/ws`.
- Hub implementation is `src/server/internal/ws/hub.go`.
- Upgrade handler is `src/server/internal/ws/handler.go`.
- Current origin check returns true; tighten this before production exposure.

## Reference Integrations

- `new-api/` is a reference for relay/provider behavior and has its own `go.mod` and Docker setup.
- `lobehub/` is a reference for frontend/product behavior and has its own pnpm/Bun ecosystem.
- Treat these as upstream reference trees unless a task explicitly targets imported source synchronization.
