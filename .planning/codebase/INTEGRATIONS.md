---
last_mapped_commit: f4dc5e48826c9893706249151aa081638e295dc1
---

# External Integrations

**Analysis Date:** 2026-04-28

## APIs & External Services

### AI Model Providers (via Relay)

**OpenAI API:**
- What it's used for: Primary LLM provider for chat completions, embeddings, image generation, audio, and all 22 API types
- SDK/Client: Custom HTTP client via `src/server/internal/relay/channel/openai_adapter.go`
- Auth: `OPENAI_API_KEY` env var
- Endpoint: `OPENAI_BASE_URL` env var (default: `https://api.openai.com`)
- The Relay system (`src/server/internal/relay/`) proxies requests to upstream AI providers through a configurable channel pool with load balancing, circuit breaking, and billing

**LLM Fallback Provider:**
- What it's used for: Direct LLM calls when relay is disabled (`RELAY_ENABLED=false`)
- Client: `src/server/internal/chat/gateway.go` (HTTPReplyGenerator)
- Auth: `LLM_API_KEY` env var
- Endpoint: `LLM_BASE_URL` env var

### MCP Protocol (Model Context Protocol)

**MCP Server Integration:**
- What it's used for: Connect to external MCP servers (JSON-RPC 2.0 over HTTP) to expose tools to agents
- Client: `src/server/internal/mcp/client.go` - Full MCP client implementing initialize, tools/list, tools/call
- Protocol version: `2024-11-05` (hardcoded in `Client.Connect`)
- Auth: Bearer token per server (`AuthToken` field)
- Servers stored in `mcp_servers` PostgreSQL table
- Supported operations: connect, disconnect, list tools, execute tool, status check

### Sub-project: new-api External Services

The `new-api/` sub-project (separate Go application at `github.com/QuantumNous/new-api`) integrates with:

**Stripe:**
- What it's used for: Subscription payment processing
- SDK: `github.com/stripe/stripe-go/v81`
- Auth: `STRIPE_API_SECRET`, `STRIPE_WEBHOOK_SECRET` (env vars)
- Controller: `new-api/controller/subscription_payment_stripe.go`

**AWS Bedrock:**
- What it's used for: AWS Bedrock model provider (Claude, etc.)
- SDK: `github.com/aws/aws-sdk-go-v2/service/bedrockruntime`

**Email (Outlook):**
- What it's used for: Email notifications via OAuth
- Implementation: `new-api/common/email-outlook-auth.go`, `new-api/common/email.go`

**WebAuthn/Passkey:**
- What it's used for: Passwordless authentication
- SDK: `github.com/go-webauthn/webauthn`

**OTP/TOTP:**
- What it's used for: Two-factor authentication
- SDK: `github.com/pquerna/otp`

**Payment Gateways (in new-api):**
- E-Pay: `github.com/Calcium-Ion/go-epay` (`new-api/controller/subscription_payment_epay.go`)
- Creem: `new-api/controller/subscription_payment_creem.go`

**Analytics (in new-api):**
- Google Analytics (configurable via `GOOGLE_ANALYTICS_ID`)
- Umami Analytics (configurable via `UMAMI_WEBSITE_ID`, `UMAMI_SCRIPT_URL`)

## Data Storage

### Databases

**PostgreSQL (main server - `src/server/`):**
- Connection: `DATABASE_URL` env var (required)
- Client: `database/sql` stdlib + `github.com/lib/pq` driver (`src/server/internal/db/db.go`)
- Used for: users, sessions, workspaces, conversations, messages, knowledge bases, documents, agents, tasks, quotas, billing sessions, subscriptions, packages, topup orders, MCP servers, notifications, user preferences, relay channels, memory documents/chunks, usage records
- No ORM used; raw SQL queries throughout all store implementations

**PostgreSQL / MySQL / SQLite (new-api sub-project):**
- Connection: `SQL_DSN` env var
- ORM: `gorm.io/gorm` v1.25.2
- SQLite default: `one-api.db` (embedded, for development)
- PostgreSQL and MySQL drivers available

### File Storage

- **Local filesystem only** for the main server
- No cloud storage (S3, GCS) integration detected
- new-api has disk cache support (`new-api/common/disk_cache.go`)
- new-api has embed file system (`new-api/common/embed-file-system.go`)

### Caching

**Main server:**
- No dedicated caching layer
- In-memory state only: WebSocket Hub connections (`src/server/internal/ws/hub.go`), Channel pool (`src/server/internal/relay/pool.go`), MCP server connections (`src/server/internal/mcp/client.go`)

**new-api sub-project:**
- **Redis** - Connection via `REDIS_CONN_STRING` env var
- Client: `github.com/go-redis/redis/v8`
- Used for: caching, task queues, session storage
- Implementation: `new-api/common/redis.go`

## Authentication & Identity

### Auth Provider (main server)

**Custom session-based authentication:**
- Implementation: `src/server/internal/auth/service.go`
- Password hashing: bcrypt via `golang.org/x/crypto`
- Session management: Custom session store in PostgreSQL (`sessions` table) via `src/server/internal/auth/store.go`
- Session cookie: `SESSION_COOKIE_NAME` env var (default: `oblivious_session`), secured by `SESSION_SECRET`
- Middleware: `src/server/internal/http/auth_middleware.go` - `requireSession` and `requireAdmin` middleware
- Session lookup via cookie → database, no JWT

**No external SSO/OAuth providers detected in main server.** The auth system is entirely custom.

**new-api sub-project:**
- JWT auth: `github.com/golang-jwt/jwt/v5`
- WebAuthn/Passkey: `github.com/go-webauthn/webauthn`
- OTP/TOTP: `github.com/pquerna/otp`
- Session management: `github.com/gin-contrib/sessions`
- OAuth: Custom OAuth controller (`new-api/controller/oauth.go`)

## Monitoring & Observability

### Metrics

**Prometheus:**
- What it's used for: Application metrics exported on `GET /metrics`
- Implementation: `src/server/internal/metrics/prometheus.go`
- Metrics tracked:
  - `relay_requests_total` - Counter by channel_id, model, api_type, status
  - `relay_request_duration_seconds` - Histogram by channel_id, model, api_type
  - `relay_tokens_total` - Counter by channel_id, model, api_type, token_type
  - `relay_billing_amount_total` - Counter by channel_id, model, api_type, billing_status
  - `relay_channel_healthy` - Gauge by channel_id, model
  - `relay_channel_latency_seconds` - Histogram by channel_id
  - `relay_rate_limit_exceeded_total` - Counter by channel_id, model, api_type
- Endpoint registered: `src/server/internal/http/router.go` line 39 (`mux.Handle("/metrics", promhttp.Handler())`)

### Error Tracking

- No external error tracking service (Sentry, Datadog) detected
- Error logging via Go `log` package
- new-api has optional error log recording via `ERROR_LOG_ENABLED`

### Logs

- Standard Go `log` package for the main server
- new-api has structured logging to `/app/logs` directory

### Profiling

- new-api sub-project: `github.com/grafana/pyroscope-go` for continuous profiling

## CI/CD & Deployment

### Hosting

- No specific hosting platform configured in the main project
- new-api sub-project: Docker Compose with PostgreSQL + Redis services

### CI Pipeline

- **GitHub Actions** - `.github/workflows/ci.yml`
- Triggers: push to main/master, pull requests
- Jobs:
  - `release-gates` - Check release assets via `scripts/check.sh docs`
  - `web` - Build web app, run tests (pnpm, Node 20)
  - `server` - Run Go tests (`src/server/go.mod` version)
- Scripts driven via: `scripts/check.sh`, `scripts/test.sh`, `scripts/dev.sh`, `scripts/verify-quality-gates.sh`

## Environment Configuration

### Required env vars (main server)

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `SESSION_SECRET` | Yes | - | Session cookie signing key |
| `SERVER_PORT` | No | 8080 | HTTP listen port |
| `APP_ENV` | No | development | Environment name |
| `CORS_ALLOWED_ORIGINS` | No | (empty) | Comma-separated origins |
| `OPENAI_API_KEY` | No | - | OpenAI key for default relay channel |
| `OPENAI_BASE_URL` | No | `https://api.openai.com` | OpenAI-compatible base URL |
| `LLM_BASE_URL` | No | - | Fallback LLM endpoint |
| `LLM_API_KEY` | No | - | Fallback LLM API key |
| `LLM_TIMEOUT_MS` | No | 30000 | LLM request timeout |
| `MODEL_DEFAULT_NAME` | No | `demo-reply` | Default model name |
| `RELAY_ENABLED` | No | `true` | Enable/disable relay gateway |
| `RELAY_DEFAULT_MODEL` | No | `gpt-4o-mini` | Default relay model |
| `SESSION_COOKIE_NAME` | No | `oblivious_session` | Session cookie name |
| `SESSION_COOKIE_SECURE` | No | `false` | Secure cookie flag |

### Secrets location

- Environment variables only (no vault, no .env file loading in main server)
- `config/.env.example` template file present (structure only)
- new-api uses `github.com/joho/godotenv` with `.env.example` template

## Webhooks & Callbacks

### Incoming

**None detected in main server.** No webhook receiver endpoints implemented.

**new-api sub-project:**
- Stripe webhook: `STRIPE_WEBHOOK_SECRET` expected (for Stripe checkout session completion callbacks)

### Outgoing

**None detected in main server.**

---

*Integration audit: 2026-04-28*
