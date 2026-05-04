---
last_mapped_commit: 98576468acf0d72bbca7e61317dc83cd5c6ad7a9
mapped_dirty_worktree: true
analysis_date: 2026-05-04
mapper: sequential-fallback
---

# Technology Stack

## Scope

Active mainline:

- `src/server/` - Go backend, HTTP API, Relay, Agent, Memory, Admin, Marketplace, migrations.
- `src/web/` - React/Vite frontend, browser routes, Admin/Marketplace UI, tests and E2E.
- `scripts/`, `config/`, `docs/`, `.github/workflows/`, `Dockerfile.*`, `docker-compose.yml`, `deploy/kubernetes/` - release and deployment surface.

Reference/imported trees:

- `lobehub/` and `new-api/` exist in the checkout but are not active workspace members. `pnpm-workspace.yaml` and release docs keep active work scoped to `src/web` and `src/server`.

## Backend

- Language/runtime: Go module `oblivious/server` in `src/server/go.mod`.
- Go version declared: `go 1.25.0`.
- HTTP framework: mostly standard `net/http` in `src/server/internal/http/router.go`; Relay sub-router uses Gin in `src/server/internal/relay/handler/router.go`.
- Database: PostgreSQL through `github.com/lib/pq`; migrations live in `src/server/migrations/0001_*.sql` through `0024_categories_tags.sql`.
- Cache/queue dependencies: Redis client dependency `github.com/redis/go-redis/v9` and async queue dependency `github.com/hibiken/asynq`.
- Metrics: Prometheus client in `src/server/internal/metrics/prometheus.go`; `/metrics` is mounted in `src/server/internal/http/router.go`.
- WebSocket: Gorilla WebSocket in `src/server/internal/ws/`.
- Payments/billing dependency: Stripe SDK `github.com/stripe/stripe-go/v83`, with local code in `src/server/internal/stripe/`.
- Relay dependencies: `github.com/pkoukk/tiktoken-go` for token accounting, relay channel adapters under `src/server/internal/relay/channel/`.
- Security/auth: password/session code under `src/server/internal/auth/`; crypto dependency `golang.org/x/crypto`.
- IDs: `github.com/google/uuid` is a direct dependency.

## Frontend

- Package manager: pnpm `10.6.0`, declared in root `package.json`.
- Workspace: root `pnpm-workspace.yaml` includes `src/web` only.
- Runtime/build: Node 20 expected by CI and Dockerfile, Vite `^5.4.10`, TypeScript `^5.6.3`.
- UI framework: React `^18.3.1`, React Router `^6.28.0`.
- Styling: Tailwind CSS v3, project tokens in `src/web/src/theme/tokens.css` and global styles in `src/web/src/theme/global.css`.
- UI primitives: local shadcn/radix-style components in `src/web/src/components/ui/`; shared product components in `src/web/src/components/shared/`.
- Icons: `@remixicon/react`; typography uses `@fontsource-variable/figtree` and `@fontsource-variable/geist-mono`.
- Tests: Vitest `^2.1.4`, Testing Library, jsdom, Playwright `^1.52.0`.

## Scripts And Gates

- Root scripts in `package.json` call shell entry points:
  - `bash scripts/dev.sh`
  - `bash scripts/check.sh`
  - `bash scripts/test.sh`
- `scripts/check.sh`:
  - `docs`: runs `scripts/verify-quality-gates.sh`, env/docs consistency checks, and workspace boundary checks.
  - `web`: runs `pnpm --dir src/web build`.
  - `server`: runs `go test ./... -count=1` in `src/server`.
- `scripts/test.sh`:
  - `web`: runs `pnpm --dir src/web test`.
  - `server`: runs `go test ./... -count=1`; runs `go test ./internal/http` only when `TEST_DATABASE_URL` is set.
- `scripts/deploy-smoke.sh` polls `/healthz`.
- `scripts/deploy-validate.sh` is the real Docker compose gate: compose config, build, up, then `deploy-smoke`.

## CI

`.github/workflows/ci.yml` defines:

- `release-gates`: `bash scripts/check.sh docs`
- `web`: pnpm install, `bash scripts/check.sh web`, `bash scripts/test.sh web`
- `e2e`: installs Chromium and runs `pnpm --dir src/web test:e2e`
- `server`: setup Go from `src/server/go.mod`, `bash scripts/check.sh server`, `bash scripts/test.sh server`

## Deployment Stack

- `Dockerfile.server`: Go builder image `golang:1.25-bookworm`; runtime `alpine:3.21`; builds `./cmd/server` and `./cmd/migrate`; exposes `8080`; healthchecks `/healthz`.
- `Dockerfile.web`: Node 20/pnpm builder; nginx `1.27-alpine` runtime; proxies `/api/` and `/v1/` to `oblivious-server:8080`; healthchecks `/healthz`.
- `docker-compose.yml`: `postgres:16`, `redis:7`, `oblivious-server`, `oblivious-web`.
- `deploy/kubernetes/`: namespace, configmap, secret example, PostgreSQL, Redis, server, and web manifests.

## Runtime Requirements

Required env vars for backend runtime are defined in `src/server/internal/config/config.go` and `config/.env.example`:

- Required: `DATABASE_URL`, `SESSION_SECRET`
- Defaults or optional: `SERVER_PORT`, `APP_ENV`, `CORS_ALLOWED_ORIGINS`, `SESSION_COOKIE_NAME`, `SESSION_COOKIE_SECURE`, `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_TIMEOUT_MS`, `MODEL_DEFAULT_NAME`, `RELAY_ENABLED`, `RELAY_DEFAULT_MODEL`, `OPENAI_API_KEY`, `OPENAI_BASE_URL`

## Current Runtime Blocker

The codebase has deployment configuration and validation scripts, but DEPLOY-01 is not complete because the current host cannot run a real Docker/Kubernetes stack:

- Docker client is installed, but daemon access fails with permission denied on `/var/run/docker.sock`.
- Starting `dockerd` requires root privileges.
- Docker Desktop build socket/buildx also reports permission errors.
- `kubectl` is not installed.

Remediation steps are in `docs/release/deployment-runtime-remediation.md`.
