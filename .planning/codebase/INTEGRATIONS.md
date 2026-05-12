# External Integrations

**Analysis Date:** 2026-05-12

## Scope

**Active mainline:**
- Root release scope is `src/server`, `src/web`, `config`, `scripts`, and `.github/workflows`; `README.md` and `scripts/check.sh` exclude `lobehub/` and `new-api/` from root workspace and CI.
- Treat `lobehub/` and `new-api/` integrations as reference-only unless a phase explicitly targets those subtrees.

## APIs & External Services

**OpenAI-compatible model gateways:**
- Direct chat-completions fallback - `src/server/internal/chat/gateway.go`
  - SDK/Client: Go `net/http`
  - Auth: `LLM_API_KEY`
  - Base URL: `LLM_BASE_URL`
  - Model default: `MODEL_DEFAULT_NAME`
- Internal relay gateway - `src/server/internal/chat/relay_gateway.go`, `src/server/internal/http/router.go`, `src/server/internal/http/server.go`
  - SDK/Client: Go `net/http` against local `/v1`
  - Auth: `OBLIVIOUS_INTERNAL_AUTH_TOKEN` for trusted app-to-relay headers
  - Control flags: `RELAY_ENABLED`, `RELAY_DEFAULT_MODEL`
- Default OpenAI relay channel - `src/server/internal/http/server.go`
  - SDK/Client: relay channel pool and `src/server/internal/relay/channel/openai_adapter.go`
  - Auth: `OPENAI_API_KEY`
  - Base URL: `OPENAI_BASE_URL`
- Admin-configured relay channels - `src/server/internal/admin/channel_store.go`, `src/server/internal/relay/store.go`
  - SDK/Client: Go `net/http`
  - Auth: API keys submitted through admin channel APIs and stored in the `channels.api_key_encrypted` column created by `src/server/migrations/0013_channels.sql`
  - Admin API: `/api/v1/admin/channels*` in `src/server/internal/http/router.go`

**OpenAI-compatible relay surface:**
- Routes are declared in `src/server/internal/relay/handler/router.go`.
  - Chat/responses: `/v1/chat/completions`, `/v1/responses`
  - Realtime: `/v1/realtime`
  - Embeddings: `/v1/embeddings`
  - Images/videos/audio/moderations/completions: `/v1/images/*`, `/v1/videos`, `/v1/audio/*`, `/v1/moderations`, `/v1/completions`
  - Batch/files/fine-tuning/assistants/threads/runs: `/v1/batch`, `/v1/files*`, `/v1/fine_tuning/jobs*`, `/v1/assistants*`, `/v1/threads*`
- The web container proxies `/v1/` to the server in `Dockerfile.web`.
- Relay channel routing, circuit breaking, rate limiting, and billing hooks live under `src/server/internal/relay`.

**MCP servers and tools:**
- User-configured MCP servers are stored through `src/server/internal/mcp/client.go` and `src/server/migrations/0015_mcp_servers.sql`.
  - SDK/Client: JSON-RPC over HTTP using Go `net/http`
  - Auth: per-server bearer token stored as `auth_token_encrypted`
  - API routes: `/api/v1/app/mcp-servers*` in `src/server/internal/http/router.go`
- Built-in tool adapters exist in `src/server/internal/mcp/builtin.go`.
  - `web_search` is a placeholder and does not call an external search API.
  - `http_request` can make outbound HTTP requests to user-provided URLs.
  - `calculator` and `datetime` are local helpers.
- Repo MCP client configuration exists in `.mcp.json` for `@claude-flow/cli`; it is not part of the app runtime.

**Stripe billing:**
- Stripe checkout helper is implemented in `src/server/internal/stripe/checkout.go`.
  - SDK/Client: `github.com/stripe/stripe-go/v83`
  - Auth: `STRIPE_SECRET_KEY`
  - Redirect config: `STRIPE_SUCCESS_URL`, `STRIPE_CANCEL_URL`
- Stripe webhook handler is implemented in `src/server/internal/stripe/webhook.go`.
  - SDK/Client: `github.com/stripe/stripe-go/v83/webhook`
  - Auth: `STRIPE_WEBHOOK_SECRET`
  - Events handled: `checkout.session.completed`, `invoice.payment_succeeded`, `customer.subscription.deleted`
  - Idempotency storage: `audit_logs` table from `src/server/migrations/0022_audit_logs.sql`
- No active Stripe route registration is present in `src/server/internal/http/router.go`; wire a route explicitly before relying on Stripe webhooks in the active app.

**Prometheus metrics:**
- `/metrics` is exposed through `promhttp.Handler()` in `src/server/internal/http/router.go`.
  - SDK/Client: `github.com/prometheus/client_golang`
  - Custom metrics: `src/server/internal/metrics/prometheus.go`
  - Auth: None detected on the `/metrics` route.

**Frontend-to-backend API:**
- Active frontend uses same-origin relative API paths through `src/web/src/services/http/client.ts`.
  - SDK/Client: browser `fetch`
  - Auth: session cookie from backend auth middleware
  - Routes: `/api/v1/*` and `/v1/*` proxied by `Dockerfile.web`

**Reference-only integrations:**
- `lobehub/package.json` includes OpenAI, Anthropic, Google GenAI, Hugging Face, Ollama, AWS Bedrock/S3, Azure AI, Vercel, Upstash QStash/Workflow, Stripe, Langfuse, PostHog, Resend, Nodemailer, Discord/Slack/Telegram adapters, and MCP SDK dependencies.
- `lobehub/docker-compose/dev/docker-compose.yml` and `lobehub/docker-compose/deploy/docker-compose.yml` define PostgreSQL/ParadeDB, Redis, RustFS, MinIO client initialization, and SearXNG services for the reference app.
- `new-api/go.mod` includes AWS Bedrock, Stripe, Redis, WebAuthn, JWT, GORM, SQLite/MySQL/Postgres drivers, Pyroscope, and OpenAI-compatible relay dependencies.
- `new-api/web/package.json` includes axios, Turnstile UI package support, Telegram login UI, i18next, and Vite/Bun tooling.

## Data Storage

**Databases:**
- PostgreSQL - active system of record
  - Connection: `DATABASE_URL`
  - Client: `database/sql` with `github.com/lib/pq` in `src/server/internal/db/db.go`
  - Migrations: `src/server/migrations`
  - Docker service: `postgres` in `docker-compose.yml`
  - Kubernetes manifests: `deploy/kubernetes/postgres.yaml` and `deploy/kubernetes/secret.example.yaml`
- pgvector - active vector extension for memory/RAG data
  - Migration: `src/server/migrations/0016_pgvector.sql`
  - HNSW index migration: `src/server/migrations/0020_memory_hnsw.sql`
  - Embedding dimension: `text-embedding-3-small`-sized vectors in `memory_chunks`
- Relay/admin tables - active relay channel, model routing, quota, package, subscription, audit, marketplace, MCP, and memory tables
  - Migrations: `src/server/migrations/0013_channels.sql` through `src/server/migrations/0024_categories_tags.sql`
- Reference `lobehub/` PostgreSQL/Drizzle storage
  - Connection: `DATABASE_URL`
  - Client: Drizzle config in `lobehub/drizzle.config.ts`
  - Migrations: `lobehub/packages/database/migrations`
- Reference `new-api/` storage
  - Clients: GORM with SQLite, MySQL, and PostgreSQL drivers in `new-api/go.mod`
  - Runtime initialization: `new-api/main.go`

**File Storage:**
- Active mainline uses local filesystem only for relay file-proxy code in `src/server/internal/relay/handler/files.go`.
  - Storage path is provided to `NewFilesHandler`; no top-level object storage service is configured in `docker-compose.yml`.
  - File mapping persistence is a placeholder in `saveFileMapping`.
- Reference `lobehub/` uses RustFS/S3-compatible storage in `lobehub/docker-compose/dev/docker-compose.yml`, `lobehub/docker-compose/deploy/docker-compose.yml`, and AWS S3 packages in `lobehub/package.json`.

**Caching:**
- Active Docker stack provisions Redis `7` in `docker-compose.yml`.
- Active server code includes Redis-backed asynq billing worker support in `src/server/internal/relay/billing_worker.go`.
- Active `src/server/internal/config/config.go` does not expose a Redis address env var, and `src/server/internal/relay/relay.go` constructs billing with an empty Redis address.
- Reference `lobehub/` and `new-api/` both include Redis integrations in their own manifests and compose files.

## Authentication & Identity

**Auth Provider:**
- Active app uses custom email/password auth.
  - Implementation: `src/server/internal/auth/service.go`, `src/server/internal/auth/store.go`, `src/server/internal/http/auth_handler.go`, `src/server/internal/http/auth_middleware.go`
  - Password hashing: bcrypt from `golang.org/x/crypto/bcrypt`
  - Session persistence: `sessions` table from `src/server/migrations/0001_phase1_foundation.sql`
  - Cookie auth: HMAC-signed HttpOnly cookie using `SESSION_SECRET`, `SESSION_COOKIE_NAME`, and `SESSION_COOKIE_SECURE`
  - Admin authorization: `requireAdmin` in `src/server/internal/http/auth_middleware.go` checks `session.User.Role`
- MCP server auth uses per-server bearer tokens stored by `src/server/internal/mcp/client.go`.
- Relay internal auth uses `X-Oblivious-Internal-Auth` with `OBLIVIOUS_INTERNAL_AUTH_TOKEN` in `src/server/internal/chat/relay_gateway.go` and `src/server/internal/relay/handler/chat.go`.
- Reference `new-api/` includes OAuth/WebAuthn/JWT dependencies in `new-api/go.mod`.
- Reference `lobehub/` includes Better Auth/OIDC dependencies in `lobehub/package.json`.

## Monitoring & Observability

**Error Tracking:**
- Active mainline: None detected.
- Reference `lobehub/` includes Langfuse, PostHog, OpenTelemetry, Jaeger exporter, and Vercel analytics packages in `lobehub/package.json`.
- Reference `new-api/` includes Pyroscope dependency and startup call in `new-api/main.go`.

**Logs:**
- Active server logs through Go standard `log` in `src/server/cmd/server/main.go` and `src/server/internal/http/server.go`.
- Active HTTP middleware includes request ID, logging, recovery, and CORS in `src/server/internal/http/middleware.go`.
- Active audit logs are persisted in PostgreSQL through `src/server/internal/admin/audit_store.go` and `src/server/migrations/0022_audit_logs.sql`.
- Active metrics are exported through `/metrics` and custom relay collectors in `src/server/internal/metrics/prometheus.go`.

## CI/CD & Deployment

**Hosting:**
- Active local/container deployment: `docker-compose.yml`.
- Active server image: `Dockerfile.server`.
- Active web image: `Dockerfile.web`.
- Active Kubernetes deployment templates: `deploy/kubernetes`.
- Reference app deployment surfaces: `lobehub/Dockerfile`, `lobehub/docker-compose`, `new-api/Dockerfile`, and `new-api/docker-compose.yml`.

**CI Pipeline:**
- GitHub Actions in `.github/workflows/ci.yml`.
  - `release-gates`: runs `bash scripts/check.sh docs`
  - `web`: installs pnpm `10.6.0`, builds web, runs web tests
  - `e2e`: installs Chromium and runs `pnpm --dir src/web test:e2e`
  - `server`: uses `actions/setup-go` from `src/server/go.mod`, runs server checks and tests

## Environment Configuration

**Required env vars:**
- `DATABASE_URL` - required by `src/server/internal/config/config.go`
- `SESSION_SECRET` - required by `src/server/internal/config/config.go`

**Optional app env vars:**
- `SERVER_PORT`
- `APP_ENV`
- `CORS_ALLOWED_ORIGINS`
- `SESSION_COOKIE_NAME`
- `SESSION_COOKIE_SECURE`
- `LLM_BASE_URL`
- `LLM_API_KEY`
- `LLM_TIMEOUT_MS`
- `MODEL_DEFAULT_NAME`
- `RELAY_ENABLED`
- `RELAY_DEFAULT_MODEL`
- `OPENAI_API_KEY`
- `OPENAI_BASE_URL`
- `OBLIVIOUS_INTERNAL_AUTH_TOKEN`
- `TEST_DATABASE_URL`

**Optional Stripe env vars:**
- `STRIPE_SECRET_KEY`
- `STRIPE_SUCCESS_URL`
- `STRIPE_CANCEL_URL`
- `STRIPE_WEBHOOK_SECRET`

**Secrets location:**
- Local example env file exists at `config/.env.example`; note existence only and do not read secret-bearing env files.
- Kubernetes example secret manifest exists at `deploy/kubernetes/secret.example.yaml`; note existence only and do not quote secret values.
- Docker Compose environment is defined in `docker-compose.yml`; avoid copying inline placeholder credentials into committed docs.
- GitHub Actions secrets are not explicitly declared in `.github/workflows/ci.yml`.

## Webhooks & Callbacks

**Incoming:**
- Stripe webhook handler code exists in `src/server/internal/stripe/webhook.go`.
  - Expected inbound signature header: `Stripe-Signature`
  - Secret: `STRIPE_WEBHOOK_SECRET`
  - Active HTTP route: Not detected in `src/server/internal/http/router.go`
- No other incoming webhook endpoints detected in active `src/server/internal/http/router.go`.

**Outgoing:**
- OpenAI-compatible model gateway calls from `src/server/internal/chat/gateway.go`, `src/server/internal/chat/relay_gateway.go`, and `src/server/internal/relay/handler`.
- Admin channel health checks from `src/server/internal/admin/channel_store.go`.
- MCP JSON-RPC requests from `src/server/internal/mcp/client.go`.
- Built-in HTTP request tool calls from `src/server/internal/mcp/builtin.go`.
- Stripe checkout session creation from `src/server/internal/stripe/checkout.go`.

---

*Integration audit: 2026-05-12*
