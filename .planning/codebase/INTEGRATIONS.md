---
last_mapped_commit: 98576468acf0d72bbca7e61317dc83cd5c6ad7a9
mapped_dirty_worktree: true
analysis_date: 2026-05-04
mapper: sequential-fallback
---

# Integrations

## Internal Integration Map

The active app integrates several backend domains through `src/server/internal/http/router.go`:

- Auth/session (`internal/auth`) protects app, console, marketplace private actions, and admin routes.
- Chat (`internal/chat`) connects conversations to Relay or local generator behavior.
- Agent (`internal/agent`) uses the configured chat gateway, Memory service, and MCP client.
- Memory (`internal/memory`) embeds through local Relay `/v1` when Relay is enabled.
- MCP (`internal/mcp`) provides server/tool discovery and built-in tools.
- Relay (`internal/relay`) centralizes provider-compatible `/v1/*` behavior and billing hooks.
- Quota (`internal/quota`) supports packages, top-up, preconsume/settle/refund behavior.
- Admin (`internal/admin`) manages channels, routes, plans, users, audit logs, and reviews.
- Marketplace (`internal/marketplace`) supports featured/curated/search/detail/install/publish/my-agents flows.
- Notification, console, knowledge, task, usage, userprefs, metrics, and WebSocket modules are composed in the same router.

## Database

- PostgreSQL is required through `DATABASE_URL`.
- SQL driver: `github.com/lib/pq`.
- Migrations live in `src/server/migrations/`.
- Docker compose uses `postgres:16`.
- Kubernetes manifests include `deploy/kubernetes/postgres.yaml` with PVC, Deployment, Service, and readiness probe.

Notable schema domains:

- users/sessions/preferences
- conversations/messages/config/capabilities/usage
- knowledge bases/documents/chunks
- tasks and authorization/tool boundaries
- relay channels/model routes
- agents/conversations/messages
- MCP servers
- pgvector/memory HNSW
- quotas
- admin role/plans/audit logs
- marketplace/categories/tags

## Redis / Queue

- Docker compose includes `redis:7`.
- Kubernetes manifests include `deploy/kubernetes/redis.yaml`.
- Go dependencies include `github.com/redis/go-redis/v9` and `github.com/hibiken/asynq`.
- Redis is part of the release stack even though not every runtime path is proven in the current environment.

## Relay Provider Integration

Provider/env vars:

- `OPENAI_API_KEY`
- `OPENAI_BASE_URL`
- `RELAY_ENABLED`
- `RELAY_DEFAULT_MODEL`
- `LLM_BASE_URL`
- `LLM_API_KEY`
- `LLM_TIMEOUT_MS`
- `MODEL_DEFAULT_NAME`

Relay-compatible endpoints in `src/server/internal/relay/handler/router.go` include:

- `/v1/chat/completions`
- `/v1/responses`
- `/v1/realtime`
- `/v1/embeddings`
- `/v1/images/*`
- `/v1/videos`
- `/v1/audio/*`
- `/v1/moderations`
- `/v1/completions`
- `/v1/batch` and `/v1/batches`
- `/v1/files`
- `/v1/fine_tuning/jobs`
- `/v1/assistants`, `/v1/threads`, `/v1/threads/:id/runs`

`Dockerfile.web` proxies `/v1/` to `http://oblivious-server:8080`.

## Stripe / Payments

- Go dependency: `github.com/stripe/stripe-go/v83`.
- Code paths: `src/server/internal/stripe/checkout.go`, `src/server/internal/stripe/webhook.go`.
- Current Phase 4 release gates avoid live Stripe dependency.
- Production payment/revenue share is out of scope in `.planning/REQUIREMENTS.md`.

## Auth And Sessions

- Auth routes in `src/server/internal/http/router.go`:
  - `/api/v1/auth/login`
  - `/api/v1/auth/register`
  - `/api/v1/auth/me`
  - `/api/v1/auth/logout`
- Config:
  - `SESSION_SECRET`
  - `SESSION_COOKIE_NAME`
  - `SESSION_COOKIE_SECURE`
- Admin routes use `requireAdmin`; app routes use `requireSession`.

## Frontend-Backend Integration

API clients:

- `src/web/src/features/auth/api.ts`
- `src/web/src/features/chat/api.ts`
- `src/web/src/features/knowledge/api.ts`
- `src/web/src/features/tasks/api.ts`
- `src/web/src/features/console/api.ts`
- `src/web/src/features/admin/api.ts`
- `src/web/src/features/marketplace/api.ts`
- HTTP core: `src/web/src/services/http/client.ts`, `envelope.ts`, `errors.ts`, `stream.ts`, `upload.ts`

Frontend routes consume backend groups:

- `/admin/*` -> `/api/v1/admin/*`
- `/marketplace/*` -> `/api/v1/marketplace/*`
- `/console/*` -> `/api/v1/console/*`
- workspace routes -> `/api/v1/app/*`

## Observability

- `/healthz` returns status from the main server.
- `/metrics` exposes Prometheus metrics.
- Docker and Kubernetes probes use `/healthz`.
- `scripts/deploy-smoke.sh` polls `/healthz`.

## Deployment Integrations

- Docker compose stack:
  - `postgres`
  - `redis`
  - `oblivious-server`
  - `oblivious-web`
- Kubernetes:
  - `namespace.yaml`
  - `configmap.yaml`
  - `secret.example.yaml`
  - `postgres.yaml`
  - `redis.yaml`
  - `server.yaml`
  - `web.yaml`
- Real cluster secrets must be created outside git from `deploy/kubernetes/secret.example.yaml`.

## External Runtime Blockers

Current host integration blockers:

- Docker daemon permission denied.
- Docker Desktop build socket/buildx permission denied.
- `dockerd` requires root.
- `kubectl` missing.

Until these are repaired, `scripts/deploy-validate.sh` and Kubernetes smoke cannot prove actual runtime integration.
