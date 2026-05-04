---
last_mapped_commit: 98576468acf0d72bbca7e61317dc83cd5c6ad7a9
mapped_dirty_worktree: true
analysis_date: 2026-05-04
mapper: sequential-fallback
---

# Architecture

## High-Level Shape

Oblivious is a Go + React application with a single active backend mainline under `src/server` and a single active frontend mainline under `src/web`.

The central architectural rule is recorded in `.planning/PROJECT.md`: all LLM calls must route through Relay so billing, quota, routing, and monitoring stay centralized.

```
React/Vite web app
  -> /api/v1/* application APIs
  -> /v1/* Relay-compatible APIs
Go net/http server
  -> service/store modules
  -> PostgreSQL migrations and SQL stores
  -> Relay engine, quota, usage, agent/memory/MCP
```

## Backend Entry Points

- `src/server/cmd/server/main.go` loads config, opens PostgreSQL through `internal/db`, constructs `internal/http.NewServer`, and listens with graceful shutdown.
- `src/server/cmd/migrate/main.go` loads config, opens PostgreSQL, reads `src/server/migrations/*.sql`, sorts them, and applies statements in order.
- `src/server/internal/http/server.go` wraps the router in server configuration.
- `src/server/internal/http/router.go` is the main composition root: it constructs auth, preferences, chat, agent, memory, MCP, quota, admin, marketplace, notification, console, metrics, and WebSocket handlers.

## Backend Layers

- HTTP handlers live under `src/server/internal/http/`.
- Domain services and SQL stores are organized per feature:
  - `src/server/internal/auth/`
  - `src/server/internal/chat/`
  - `src/server/internal/agent/`
  - `src/server/internal/memory/`
  - `src/server/internal/mcp/`
  - `src/server/internal/quota/`
  - `src/server/internal/admin/`
  - `src/server/internal/marketplace/`
  - `src/server/internal/knowledge/`
  - `src/server/internal/task/`
  - `src/server/internal/console/`
  - `src/server/internal/notification/`
  - `src/server/internal/userprefs/`
  - `src/server/internal/usage/`
- Shared infrastructure:
  - `src/server/internal/config/config.go`
  - `src/server/internal/db/db.go`
  - `src/server/internal/metrics/`
  - `src/server/internal/ws/`

## Relay Architecture

Relay has two surfaces:

- Main app integration in `src/server/internal/http/router.go`, where Chat, Agent, and Memory are configured to use Relay URLs when `RELAY_ENABLED` is true.
- Relay-specific route engine in `src/server/internal/relay/handler/router.go`, using Gin and route metadata for OpenAI-compatible endpoints.

Key relay packages:

- `src/server/internal/relay/relay.go` - relay orchestration
- `src/server/internal/relay/router.go` - model/channel routing
- `src/server/internal/relay/billing.go` and `billing_worker.go` - quota/usage integration
- `src/server/internal/relay/channel/openai_adapter.go` - provider adapter
- `src/server/internal/relay/tokenizer.go`, `pricing.go`, `retry.go`, `circuitbreaker.go`, `loadbalancer.go`, `healthchecker.go`
- `src/server/internal/relay/handler/*` - endpoint handlers for chat, responses, embeddings, images, audio, files, batches, fine-tuning, assistants, realtime, moderation

## Main HTTP Route Groups

Mounted in `src/server/internal/http/router.go`:

- Health and metrics: `/healthz`, `/metrics`
- Auth: `/api/v1/auth/login`, `/register`, `/me`, `/logout`
- Preferences: `/api/v1/app/me/preferences`
- Chat: `/api/v1/app/conversations`, `/api/v1/app/conversations/{id}/messages`, `/config`, `/convert-to-task`
- Knowledge: `/api/v1/app/knowledge-bases`, document routes below each knowledge base
- Tasks: `/api/v1/app/tasks`
- Agent: `/api/v1/app/agents`, `/api/v1/app/agents/{id}`, `/api/v1/app/agents/conversations/{id}`
- Memory: `/api/v1/app/memory/documents`, `/api/v1/app/memory/search`
- MCP: `/api/v1/app/mcp-servers`
- Console: `/api/v1/console/usage`, `/access`, `/models`, `/billing`
- Quota/packages: `/api/v1/app/quota`, `/packages`, `/quota/topup`
- Notifications: `/api/v1/app/notifications/*`
- WebSocket: `/api/v1/ws`
- Admin: `/api/v1/admin/stats`, channels, routes, plans, users, audit logs, reviews
- Marketplace: `/api/v1/marketplace/featured`, curated, categories, search, agents, installs, my-agents, publisher stats

## Agent, Memory, MCP Flow

- `agent.Service` is constructed with a chat gateway in `router.go`.
- When Relay is enabled, `chat.NewRelayGateway` points at local `/v1` and `chat.NewCompositeGateway` falls back to local generator behavior.
- `memory.Service` uses `memory.NewRelayEmbedder` pointed at local `/v1` for embeddings and is injected into `agent.Service`.
- `mcp.Client` is constructed with SQL store and injected into `agent.Service` for tool discovery/calls.
- Quota hooks live under Relay billing and quota service tests.

## Frontend Architecture

- `src/web/src/app/router.tsx` defines the route tree.
- Auth gates:
  - `ProtectedRoute` wraps workspace, console, marketplace, and admin surfaces.
  - `AdminRoute` gates `/admin`.
- Layouts:
  - `MarketingLayout` for `/`, `/login`, `/register`
  - `WorkspaceLayout` for `/chat`, `/knowledge`, `/solo`, `/marketplace`, `/settings`
  - `ConsoleLayout` for `/console/*`
  - `AdminLayout` for `/admin/*`
- API clients are feature-scoped under `src/web/src/features/*/api.ts`.
- Shared UI components live under `src/web/src/components/shared/`; low-level UI primitives live under `src/web/src/components/ui/`.

## Data And Migration Architecture

- Migrations are append-only SQL files under `src/server/migrations/`.
- Current migration range: `0001_phase1_foundation.sql` through `0024_categories_tags.sql`.
- Notable domains:
  - foundation/auth/session/conversation in early migrations
  - knowledge/task/memory/pgvector
  - channels, agents, MCP servers, quotas
  - admin role, plans, audit logs, marketplace, categories/tags
- Historical note from project memory: old `0016_pgvector.sql` used IVFFlat, and the repair path was append-only HNSW migration (`0020_memory_hnsw.sql`), not mutating old migrations.

## Deployment Architecture

- Docker compose runs `postgres`, `redis`, `oblivious-server`, and `oblivious-web`.
- Kubernetes manifests mirror that stack in `deploy/kubernetes/`.
- `Dockerfile.web` uses nginx to serve static assets and proxy `/api/` and `/v1/` to the server.
- `scripts/deploy-validate.sh` is the intended local deployment proof, but current host daemon access blocks execution.

## Current Architectural Risk

The architecture map must distinguish "configuration exists" from "runtime proven":

- TEST-01, TEST-02, and DOC-01 are verified.
- DEPLOY-01 has Docker/Kubernetes assets and a one-command validation script, but no real stack startup evidence in this environment.
