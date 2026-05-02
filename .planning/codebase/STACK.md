---
last_mapped_commit: c0e55fdbb3aaed7da80a0f7f2399237aed13bca3
mapped_dirty_worktree: true
---

# Technology Stack

**Analysis Date:** 2026-05-02

## Languages

**Primary:**
- Go 1.25.0 - backend API, Relay, Agent runtime, Memory/RAG, Admin, Marketplace, and migration tooling under `src/server/`.
- TypeScript 5.6.3 - React SPA under `src/web/`.

**Secondary:**
- SQL - PostgreSQL migrations under `src/server/migrations/` and relay-specific migration seed under `src/server/internal/relay/migrations/`.
- Bash - repo automation in `scripts/check.sh`, `scripts/test.sh`, and `scripts/dev.sh`.

## Runtime

**Backend:**
- Go module: `src/server/go.mod`.
- Main server entrypoint: `src/server/cmd/server/main.go`.
- Migration entrypoint: `src/server/cmd/migrate/main.go`.
- Primary HTTP runtime: Go `net/http` via `src/server/internal/http/router.go`.
- Relay runtime: Gin engine mounted under `/v1/*` by `src/server/internal/http/server.go`.

**Frontend:**
- Node.js 20 in CI (`.github/workflows/ci.yml`).
- Vite 5.4.10 dev/build runtime (`src/web/vite.config.ts`).
- React 18.3.1 SPA entrypoint at `src/web/src/main.tsx`.

**Package Managers:**
- pnpm 10.6.0 at the repo root (`package.json` and `pnpm-lock.yaml`).
- Go modules for backend dependencies (`src/server/go.mod`, `src/server/go.sum`).
- Workspace membership is intentionally narrow: `pnpm-workspace.yaml` includes only `src/web`.

## Frameworks

**Backend:**
- `net/http` - main API router, middleware chain, auth routes, app routes, console routes, admin routes.
- `github.com/gin-gonic/gin` v1.12.0 - Relay OpenAI-compatible API surface under `/v1/*`.
- `github.com/gorilla/websocket` v1.5.3 - authenticated app WebSocket at `/api/v1/ws`.
- `github.com/prometheus/client_golang` v1.23.2 - `/metrics` endpoint.
- `github.com/lib/pq` v1.10.9 - PostgreSQL driver and array support.
- `github.com/google/uuid` v1.6.0 - UUID generation in admin/relay defaults.
- `golang.org/x/crypto` v0.48.0 - bcrypt password hashing.
- `github.com/stripe/stripe-go/v83` v83.2.1 - Stripe webhook/service package exists under `src/server/internal/stripe/`.
- `github.com/hibiken/asynq` v0.26.0 - relay billing timeout/polling worker support.
- `github.com/pkoukk/tiktoken-go` v0.1.8 - token estimation/pricing support.

**Frontend:**
- React 18.3.1 with React Router DOM 6.28.0 (`src/web/src/app/router.tsx`).
- Vite 5.4.10 and `@vitejs/plugin-react`.
- Tailwind CSS 3 plus shadcn/Radix styling assets (`src/web/tailwind.config.ts`, `src/web/src/theme/tokens.css`, `src/web/components.json`).
- `@fontsource-variable/figtree` and `@fontsource-variable/geist-mono` for UI typography.
- `@remixicon/react` is available for icons; use it where existing UI does.

## Testing

**Backend:**
- Go standard `testing` package only. Tests live next to packages as `*_test.go`.
- Current unit focus includes config, chat, knowledge, task, console, agent, memory, relay, quota, metrics, notification, and ws packages.

**Frontend:**
- Vitest 2.1.4 with jsdom from `src/web/vite.config.ts`.
- Testing Library React and jest-dom configured through `src/web/src/test/setup.ts`.

## Configuration

**Backend environment:**
- Required: `DATABASE_URL`, `SESSION_SECRET`.
- Common runtime: `SERVER_PORT`, `APP_ENV`, `CORS_ALLOWED_ORIGINS`, `SESSION_COOKIE_NAME`, `SESSION_COOKIE_SECURE`.
- LLM fallback: `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_TIMEOUT_MS`, `MODEL_DEFAULT_NAME`.
- Relay: `RELAY_ENABLED`, `RELAY_DEFAULT_MODEL`, `OPENAI_API_KEY`, `OPENAI_BASE_URL`.
- Example file: `config/.env.example`.
- Loader: `src/server/internal/config/config.go`.

**Frontend environment:**
- `WEB_PORT` and `WEB_API_BASE_URL` are documented in `config/.env.example` and checked by `scripts/check.sh docs`.
- The current `createHttpClient` default uses relative paths; route code should prefer injected API clients over hardcoded global `fetch`.

## Datastores

**Primary database:**
- PostgreSQL via `database/sql`.
- App migrations: `src/server/migrations/0001_phase1_foundation.sql` through `src/server/migrations/0024_categories_tags.sql`.
- Relay seed migration: `src/server/internal/relay/migrations/001_init_relay.sql`.

**Vector search:**
- pgvector table/index support in `src/server/migrations/0016_pgvector.sql`.
- HNSW replacement index in `src/server/migrations/0020_memory_hnsw.sql`.

**Reference trees:**
- `lobehub/` and `new-api/` are imported/reference projects, not pnpm workspace members.
- Do not add them to `pnpm-workspace.yaml` unless the project intentionally changes from reference-source integration to monorepo development.

## Standard Commands

```bash
bash scripts/check.sh docs
bash scripts/check.sh web
bash scripts/check.sh server
bash scripts/test.sh web
bash scripts/test.sh server
cd src/server && GOCACHE=../../.tmp/go-build GOMODCACHE=../../.tmp/go-mod go test ./...
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test
```

## Operational Notes

- Use repo-local caches (`.tmp/corepack`, `.tmp/go-build`, `.tmp/go-mod`) as the scripts do.
- Keep new backend code in `src/server/internal/<domain>/` with a service/store split.
- Keep new frontend product surfaces under `src/web/src/routes/` and reusable request logic under `src/web/src/features/<domain>/api.ts`.
- Keep migrations append-only; do not rewrite old applied migration files to change behavior.
