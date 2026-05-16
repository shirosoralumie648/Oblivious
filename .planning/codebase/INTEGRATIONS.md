# External Integrations

**Analysis Date:** 2026-05-16

## APIs & External Services

**LLM Providers (Relay layer):**
- **OpenAI** — Primary upstream provider for `/v1/*` relay endpoints.
  - Adapter: `src/server/internal/relay/channel/openai_adapter.go` (`OpenAIAdapter`, provider key `"openai"`)
  - Capabilities advertised: chat, streaming, embeddings, images, audio, realtime, assistants (`openai_adapter.go` lines 26-35)
  - Default channel auto-provisioned at startup when `OPENAI_API_KEY` is set (`src/server/internal/http/server.go` lines 58-78, models `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`, `gpt-3.5-turbo`)
  - Default relay model: `gpt-4o-mini` (`config.go` line 99)
  - Auth: `OPENAI_API_KEY` env var → `Authorization: Bearer <key>` header set by adapter
  - Base URL: `OPENAI_BASE_URL` env var, default `https://api.openai.com` (`config.go` lines 104-107)
- **Generic LLM upstream** — Pre-relay legacy path used by `MODEL_DEFAULT_NAME=demo-reply` and embeddings.
  - Configured via `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_TIMEOUT_MS` (`src/server/internal/config/config.go` lines 75-89)
  - Used by `src/server/internal/memory/embedder.go` calling `POST {relayURL}/embeddings` for `text-embedding-3-small` (model wired in `src/server/internal/http/router.go:88`)

**OpenAI-Compatible Relay Surface (outbound, exposed by Oblivious itself):**
The server publishes an OpenAI-compatible `/v1/*` API mounted from `src/server/internal/relay/handler/router.go`. Handlers cover the OpenAI API surface:
- `/v1/chat/completions` (`handler/chat.go`)
- `/v1/completions` (`handler/completions.go`)
- `/v1/embeddings` (`handler/embeddings.go`)
- `/v1/audio/*` (`handler/audio.go`)
- `/v1/images/*` (`handler/images.go`)
- `/v1/moderations` (`handler/moderations.go`)
- `/v1/assistants/*` (`handler/assistants.go`)
- `/v1/batches/*` (`handler/batch.go`)
- `/v1/files/*` (`handler/files.go`)
- `/v1/fine_tuning/*` (`handler/fine_tuning.go`)
- `/v1/realtime` WebSocket bridge (`handler/realtime.go`)
- `/v1/responses` (`handler/responses.go`)

Returns OpenAI-shaped responses; gated by `RELAY_ENABLED=true` (default true).

**Payments:**
- **Stripe** — Checkout sessions + webhook receiver.
  - SDK: `github.com/stripe/stripe-go/v83` v83.2.1 (`src/server/go.mod` line 13)
  - Checkout: `src/server/internal/stripe/checkout.go` — `CreateCheckoutSession` supports both `payment` and `subscription` modes
  - Webhook: `src/server/internal/stripe/webhook.go` — Handles `checkout.session.completed`, `invoice.paid`, `customer.subscription.deleted`; verifies signatures via `stripe/webhook` package; idempotency tracked through `audit_logs` table with `action='stripe.webhook'`
  - Auth: `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET` env vars (`checkout.go` lines 32-35)
  - Redirects: `STRIPE_SUCCESS_URL`, `STRIPE_CANCEL_URL`

## Data Storage

**Databases:**
- **PostgreSQL 16** — System of record (`docker-compose.yml` `postgres:16`)
  - Connection: `DATABASE_URL` env var (required, format `postgres://...`)
  - Test connection: `TEST_DATABASE_URL` env var (used by `src/server/internal/http/server_test.go:18` and `scripts/test.sh` lines 43-49)
  - Driver: `github.com/lib/pq` v1.10.9 via `database/sql` (`src/server/internal/db/`)
  - Migration runner: `src/server/cmd/migrate/main.go`, applies files from `src/server/migrations/0001_*.sql` … `0024_*.sql`
  - Domain tables span: users, conversations, messages, knowledge bases + documents + chunks, tasks, channels, agents, MCP servers, quotas, audit logs, marketplace, categories/tags, plan extensions
- **pgvector** — Vector storage extension enabled by migration `src/server/migrations/0016_pgvector.sql`; HNSW indexes added by `0020_memory_hnsw.sql`. Used by `src/server/internal/memory/` for embedding similarity search.

**Caching / Queue Broker:**
- **Redis 7** (AOF-persisted) — `docker-compose.yml` `redis:7` with `--appendonly yes`
  - Used exclusively as the asynq broker for relay billing tasks
  - Address configured per-call to `NewBillingWorker(redisAddr, …)` in `src/server/internal/relay/billing_worker.go:52`
  - Test default address `localhost:6379` (`src/server/internal/relay/billing_worker_test.go:68`)

**File Storage:**
- Local filesystem only via `/v1/files/*` relay handler (`src/server/internal/relay/handler/files.go`); maps `local_id` → `openai_id` per upload (line 167). No cloud blob storage detected.

## Authentication & Identity

**Auth Provider:**
- **Custom session-cookie auth** — No external IdP.
  - Implementation: `src/server/internal/auth/service.go`, `src/server/internal/auth/store.go`
  - Password hashing: `golang.org/x/crypto/bcrypt` with `bcrypt.DefaultCost` (`auth/service.go:62`)
  - Session cookie: name controlled by `SESSION_COOKIE_NAME` (default `oblivious_session`), `Secure` flag controlled by `SESSION_COOKIE_SECURE`, signed with `SESSION_SECRET` (required env var — server refuses to start without it, `config.go:64-67`)
  - Admin role: enforced via session middleware; admin role added by migration `src/server/migrations/0019_admin_role.sql`
- Public app endpoints: `POST /api/v1/auth/login`, `POST /api/v1/auth/register`, `GET /healthz`, `GET /metrics` (per `docs/API.md`)
- Authenticated endpoint requires the session cookie; admin endpoints additionally require the admin role.

## Monitoring & Observability

**Metrics:**
- **Prometheus** — `github.com/prometheus/client_golang` v1.23.2
  - Endpoint: `GET /metrics` (`docs/API.md` Base URLs table)
  - Definitions: `src/server/internal/metrics/prometheus.go` — request counters/histograms, channel gauges, billing counters

**Error Tracking:**
- None detected. No Sentry/Rollbar/Bugsnag SDK in `package.json`, `src/web/package.json`, or `src/server/go.mod`.

**Logs:**
- Server uses stdlib `log` package (`src/server/cmd/server/main.go`, `src/server/internal/http/server.go`)
- No structured logger (zap/zerolog/logrus) detected

**Tracing:**
- None detected. No OpenTelemetry SDK in dependency lists.

## CI/CD & Deployment

**CI Pipeline:**
- GitHub Actions — `.github/workflows/ci.yml`
- Jobs: `release-gates`, `web`, `e2e`, `server` (see STACK.md "CI Pipeline" section)
- pnpm cache via `pnpm/action-setup@v4` and `actions/setup-node@v4` with `cache: pnpm`
- Go cache via `actions/setup-go@v5` with `go-version-file: src/server/go.mod`
- Playwright Chromium installed in the `e2e` job

**Hosting / Deployment:**
- Self-hosted via Docker Compose stack (`docker-compose.yml`) for local validation
- Kubernetes manifests under `deploy/` (gitignored secret `deploy/kubernetes/secret.yaml`)
- Smoke validation: `scripts/deploy-validate.sh` → `scripts/deploy-smoke.sh` against `BASE_URL` (default `http://127.0.0.1:8080`)
- Optional registry mirror: `OBLIVIOUS_IMAGE_REGISTRY_PREFIX` rewrites pull paths in `docker-compose.yml`
- Optional Go proxy: `OBLIVIOUS_GOPROXY` / `OBLIVIOUS_GOSUMDB` build args for `Dockerfile.server`

**Reverse Proxy:**
- Nginx 1.27 inside `oblivious-web` container proxies `/api/` and `/v1/` to `oblivious-server:8080`, serves SPA fallback to `/index.html` (`Dockerfile.web` lines 25-53). Single ingress at port 4173 on host.

## Environment Configuration

**Required server env vars (server refuses to start without these):**
- `DATABASE_URL` (`config.go:60-62`)
- `SESSION_SECRET` (`config.go:64-67`)

**Required server env vars (validated by `scripts/check.sh docs` for documentation consistency):**
- Frontend: `WEB_PORT`, `WEB_API_BASE_URL`
- Backend: `SERVER_PORT`, `APP_ENV`, `CORS_ALLOWED_ORIGINS`, `DATABASE_URL`, `SESSION_SECRET`, `SESSION_COOKIE_NAME`, `SESSION_COOKIE_SECURE`, `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_TIMEOUT_MS`, `MODEL_DEFAULT_NAME`

**Optional / feature-flag env vars:**
- `RELAY_ENABLED` (default `true`), `RELAY_DEFAULT_MODEL` (default `gpt-4o-mini`)
- `OPENAI_API_KEY`, `OPENAI_BASE_URL` (default `https://api.openai.com`) — when set, auto-creates default OpenAI channel on first boot
- `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_SUCCESS_URL`, `STRIPE_CANCEL_URL`
- `TEST_DATABASE_URL` — enables HTTP integration suite

**Secrets location:**
- `.env`, `.env.*` files gitignored (`.gitignore`) and dockerignored (`.dockerignore`)
- Reference template at `config/.env.example` (the only file inside `config/`)
- `docker-compose.yml` inlines a placeholder `SESSION_SECRET=change-me-in-production` and empty `OPENAI_API_KEY=` / `LLM_API_KEY=` — these are development defaults only

## Webhooks & Callbacks

**Incoming:**
- **Stripe webhook** — Handled by `src/server/internal/stripe/webhook.go`
  - Verifies signature with `STRIPE_WEBHOOK_SECRET` via `github.com/stripe/stripe-go/v83/webhook`
  - Handled event types:
    - `checkout.session.completed` (`webhook.go:83`)
    - `invoice.paid` (`webhook.go:159`)
    - `customer.subscription.deleted` (`webhook.go:166`)
  - Idempotency: dedup against `audit_logs WHERE action='stripe.webhook' AND resource_id=$1` (`webhook.go:207-228`)

**Outgoing:**
- HTTP requests to LLM providers via `OpenAIAdapter.DoRequest` (`src/server/internal/relay/channel/openai_adapter.go`) and `memory/embedder.go` for `/embeddings`
- HTTP requests to user-configured MCP servers (see below)
- Stripe API calls via `stripe-go` SDK

## Message Queues

- **asynq** (Redis-backed) — `github.com/hibiken/asynq` v0.26.0
  - Worker: `src/server/internal/relay/billing_worker.go`
  - Task types defined as constants `BillingTimeoutPayload`, `BillingTimeoutQueue` (referenced at `billing_worker.go:71-75`)
  - Two task families:
    - Billing timeout — `EnqueueBillingTimeoutTask(redisAddr, task, delay)` at `billing_worker.go:65`
    - Billing polling — `EnqueueBillingPollingTask(redisAddr, task)` at `billing_worker.go:79`
  - Purpose: ensure async settle/refund of relay billing when upstream completion is delayed

## MCP Servers

- **Model Context Protocol** integration is first-class but custom-built.
  - User-managed external MCP servers: created/listed/managed via `src/server/internal/mcp/client.go` (`Server` struct with `URL`, `AuthToken`, `Status`)
  - Persistence: migration `src/server/migrations/0015_mcp_servers.sql`
  - Built-in MCP tools: `src/server/internal/mcp/builtin.go`
  - Outgoing transport: HTTP via `http.Client` (`client.go` line 42)
  - Tool execution surfaced through agent endpoints `GET /api/v1/app/agents/:agentId/tools` (`docs/API.md`)
- **Repo-level developer MCP** (not part of the application, only for Claude tooling): `.mcp.json` declares `claude-flow` MCP server invoked via `npx -y @claude-flow/cli@latest mcp start`. This is editor-side tooling, not a runtime dependency.

## Third-Party Services Summary

| Service | Purpose | SDK / Client | Auth Env Var | Where |
|---------|---------|--------------|--------------|-------|
| OpenAI | LLM provider for relay | `OpenAIAdapter` (`src/server/internal/relay/channel/openai_adapter.go`) | `OPENAI_API_KEY` | `src/server/internal/relay/` |
| OpenAI (embeddings) | `text-embedding-3-small` for memory/knowledge | stdlib `net/http` | `LLM_API_KEY` | `src/server/internal/memory/embedder.go` |
| Stripe | Checkout + subscription billing | `stripe-go/v83` | `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET` | `src/server/internal/stripe/` |
| PostgreSQL | Primary datastore | `lib/pq` | `DATABASE_URL` | `src/server/internal/db/` |
| Redis | asynq broker | `redis/go-redis/v9` (indirect via asynq) | none (URL passed to `NewBillingWorker`) | `src/server/internal/relay/billing_worker.go` |
| User MCP servers | Tool execution for agents | stdlib `net/http` | per-server `AuthToken` | `src/server/internal/mcp/client.go` |

No detected integrations with: AWS, GCP, Azure, Cloudflare, Anthropic, Google Gemini, Vercel, Supabase, Auth0, Clerk, Firebase, Twilio, SendGrid, Mailgun, Sentry, Datadog, New Relic, S3, GCS, or any object-storage SDK.

---

*Integration audit: 2026-05-16*
