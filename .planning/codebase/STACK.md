# Technology Stack

**Analysis Date:** 2026-05-16

## Languages

**Primary:**
- Go 1.25 — Backend API, relay/router, migrations (`src/server/`, declared in `src/server/go.mod`)
- TypeScript 5.6 — Frontend web app (`src/web/src/`, declared in `src/web/tsconfig.json`)

**Secondary:**
- SQL (PostgreSQL dialect) — Schema migrations in `src/server/migrations/0001_*.sql` through `0024_*.sql`
- Bash — Build, test, deploy scripts (`scripts/dev.sh`, `scripts/check.sh`, `scripts/test.sh`, `scripts/deploy-validate.sh`, `scripts/deploy-smoke.sh`, `scripts/verify-quality-gates.sh`)
- Dockerfile — Container builds (`Dockerfile.server`, `Dockerfile.web`)

## Runtime

**Environment:**
- Go runtime: 1.25 (pinned in `src/server/go.mod` line 3 and `Dockerfile.server` `GO_IMAGE=golang:1.25-bookworm`)
- Node.js: 20 (pinned in `.github/workflows/ci.yml` `node-version: 20` and `Dockerfile.web` `NODE_IMAGE=node:20-bookworm-slim`)
- Alpine 3.21 — Server runtime base image (`Dockerfile.server` `ALPINE_IMAGE=alpine:3.21`)
- Nginx 1.27 (alpine) — Web runtime base image, also serves as reverse proxy for `/api/*` and `/v1/*` (`Dockerfile.web` lines 26-53)

**Package Manager:**
- pnpm 10.6.0 — JS/TS workspace (declared in root `package.json` `packageManager` field, activated via corepack)
- Go modules — Server dependencies (`src/server/go.mod`, `src/server/go.sum`)
- Lockfiles: `pnpm-lock.yaml` (244 KB, present), `src/server/go.sum` (present)

## Frameworks

**Core (backend):**
- `github.com/gin-gonic/gin v1.12.0` — HTTP router/middleware for the App API and Relay handlers (`src/server/internal/http/`, `src/server/internal/relay/handler/router.go`)
- `github.com/gorilla/websocket v1.5.3` — WebSocket upgrade for chat streaming and Realtime API (`src/server/internal/relay/handler/chat.go`, `src/server/internal/relay/handler/realtime.go`, `src/server/internal/ws/`)
- `github.com/hibiken/asynq v0.26.0` — Redis-backed async task queue for billing timeout/polling jobs (`src/server/internal/relay/billing_worker.go`)
- `github.com/lib/pq v1.10.9` — PostgreSQL `database/sql` driver (used in `src/server/internal/db/`)

**Core (frontend):**
- React 18.3.1 + React DOM 18.3.1 (`src/web/package.json`)
- React Router DOM 6.28.0 — Client routing (`src/web/src/routes/`)
- Vite 5.4.10 — Dev server and production build (`src/web/vite.config.ts`)
- TailwindCSS 3 + PostCSS 8.5 + Autoprefixer 10.5 (`src/web/tailwind.config.ts`, `src/web/postcss.config.mjs`)
- shadcn 4.6.0 (config `src/web/components.json`, style `radix-maia`, icon library `remixicon`) layered on `radix-ui` 1.4.3
- `next-themes` 0.4.6 — Dark/light theming
- `class-variance-authority` 0.7.1, `clsx` 2.1.1, `tailwind-merge` 3.5.0 — Class composition
- `cmdk` 1.1.1 — Command palette
- `sonner` 2.0.7 — Toast notifications
- `@remixicon/react` 4.9.0, `@fontsource-variable/figtree` 5.2.10, `@fontsource-variable/geist-mono` 5.2.7 — Icons and fonts
- `tw-animate-css` 1.4.0 — Animation utilities

**Testing:**
- Vitest 2.1.4 + jsdom 25.0.1 — Web unit tests (config in `src/web/vite.config.ts` `test:` block, setup file `src/web/src/test/setup.ts`)
- `@testing-library/react` 16.1.0 + `@testing-library/jest-dom` 6.6.3 — DOM assertions
- `@playwright/test` 1.52.0 — Browser E2E (`src/web/playwright.config.ts`, tests in `src/web/e2e/`)
- Go `testing` stdlib + `httptest` — Server unit and HTTP integration tests (`*_test.go` files across `src/server/internal/`)
- Integration tests gated by `TEST_DATABASE_URL` (`scripts/test.sh` lines 43-49, `src/server/internal/http/server_test.go:18`)

**Build/Dev:**
- `@vitejs/plugin-react` 4.3.4 — React Fast Refresh / JSX (`src/web/vite.config.ts`)
- `tsc --noEmit` — Type-check gate before Vite build (`src/web/package.json` `build` script)
- Corepack — Pinned pnpm activation (`Dockerfile.web` line 12)
- BuildKit cache mounts — `--mount=type=cache` for Go module cache and pnpm store in Dockerfiles
- Docker Compose v2 — Local stack orchestration (`docker-compose.yml`)

## Key Dependencies

**Critical (server):**
- `github.com/stripe/stripe-go/v83 v83.2.1` — Stripe Checkout sessions and webhook verification (`src/server/internal/stripe/checkout.go`, `src/server/internal/stripe/webhook.go`)
- `github.com/prometheus/client_golang v1.23.2` — `/metrics` endpoint (`src/server/internal/metrics/prometheus.go`)
- `github.com/pkoukk/tiktoken-go v0.1.8` — Token counting for relay billing (`src/server/internal/relay/tokenizer.go`)
- `github.com/google/uuid v1.6.0` — UUID generation (`src/server/internal/http/server.go:8`, used across services)
- `golang.org/x/crypto v0.48.0` — bcrypt password hashing (`src/server/internal/auth/service.go:12`, `src/server/internal/auth/store.go:10`)
- `github.com/redis/go-redis/v9 v9.14.1` — Indirect via asynq (Redis broker)

**Critical (web):**
- `radix-ui` 1.4.3 + `shadcn` 4.6.0 — UI primitive layer (taupe base color, CSS variables, no prefix)
- Tailwind theme reads from CSS custom properties defined in `src/web/src/theme/tokens.css`

**Infrastructure:**
- PostgreSQL 16 — Primary datastore (`docker-compose.yml` `postgres:16`)
- Redis 7 with AOF — asynq broker for relay billing workers (`docker-compose.yml` `redis:7` with `--appendonly yes`)
- pgvector — Vector similarity migration `src/server/migrations/0016_pgvector.sql`, HNSW index migration `0020_memory_hnsw.sql`

## Configuration

**Environment:**
- Source of truth: `config/.env.example` (referenced by `README.md` step 2 and verified by `scripts/check.sh` lines 35-62)
- Required by server (validated in `src/server/internal/config/config.go`):
  - `SERVER_PORT` (default 8080)
  - `APP_ENV` (default `development`)
  - `CORS_ALLOWED_ORIGINS` (comma-separated)
  - `DATABASE_URL` (required, no default)
  - `SESSION_SECRET` (required, no default)
  - `SESSION_COOKIE_NAME` (default `oblivious_session`)
  - `SESSION_COOKIE_SECURE` (boolean)
  - `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_TIMEOUT_MS` (default 30000), `MODEL_DEFAULT_NAME` (default `demo-reply`)
  - `RELAY_ENABLED` (default true), `RELAY_DEFAULT_MODEL` (default `gpt-4o-mini`)
  - `OPENAI_API_KEY`, `OPENAI_BASE_URL` (default `https://api.openai.com`)
- Stripe env vars consumed in `src/server/internal/stripe/checkout.go` lines 32-35: `STRIPE_SECRET_KEY`, `STRIPE_SUCCESS_URL`, `STRIPE_CANCEL_URL`, `STRIPE_WEBHOOK_SECRET`
- Web env vars referenced by `scripts/check.sh`: `WEB_PORT`, `WEB_API_BASE_URL`
- `.env` and `.env.*` files are gitignored (`.gitignore` lines for `.env`, `.env.*`) and dockerignored (`.dockerignore`)

**Build:**
- `src/web/vite.config.ts` — Vite + Vitest config, alias `@` → `./src`
- `src/web/tsconfig.json` — TS strict mode, ES2020 target, JSX `react-jsx`, path alias `@/*`
- `src/web/tailwind.config.ts` — Tailwind theme bound to CSS variables in `src/web/src/theme/tokens.css`
- `src/web/postcss.config.mjs` — PostCSS pipeline (tailwindcss + autoprefixer)
- `src/web/components.json` — shadcn registry config (style `radix-maia`, base color `taupe`)
- `src/web/playwright.config.ts` — Playwright config (chromium only, base URL `http://127.0.0.1:4173`, webServer runs `pnpm build && pnpm preview`)
- `pnpm-workspace.yaml` — Workspace packages: `src/web` only (mainline boundary; `lobehub/`, `new-api/` excluded)

## Platform Requirements

**Development:**
- Go 1.22+ documented in `README.md` (actual `go.mod` declares 1.25)
- Node.js 20+, pnpm 10.6.0, PostgreSQL 14+ (`README.md` Prerequisites)
- Docker + Docker Compose v2 — Required by `scripts/deploy-validate.sh` lines 10-23

**Production / Deployment:**
- Multi-stage Docker images:
  - `oblivious-server` — Go builder → Alpine runtime, port 8080, healthcheck `GET /healthz` (`Dockerfile.server`)
  - `oblivious-web` — Node builder → Nginx runtime, port 80, proxies `/api/` and `/v1/` to `oblivious-server:8080` (`Dockerfile.web`)
- Optional image-registry mirror prefix via `OBLIVIOUS_IMAGE_REGISTRY_PREFIX` (`docker-compose.yml`)
- Optional Go module proxy override via `OBLIVIOUS_GOPROXY` / `OBLIVIOUS_GOSUMDB`
- Persisted volumes: `oblivious-postgres-data`, `oblivious-redis-data` (`docker-compose.yml` lines 84-86)
- Kubernetes deploy directory exists at `deploy/` (referenced by `.gitignore` rule `deploy/kubernetes/secret.yaml`)

## Build / Test / Lint Commands

```bash
# Root orchestration (run from repo root)
pnpm install --frozen-lockfile   # Install web workspace deps
bash scripts/dev.sh              # Start web + server (auto-detects presence)
bash scripts/check.sh            # docs + web build + server `go test ./... -count=1`
bash scripts/check.sh docs       # Release-asset + env consistency checks (run by CI release-gates job)
bash scripts/check.sh web        # `pnpm --dir src/web build`
bash scripts/check.sh server     # `go test ./... -count=1` from src/server
bash scripts/test.sh             # web Vitest + server unit + integration (if TEST_DATABASE_URL set)
bash scripts/test.sh web         # `pnpm --dir src/web test` (vitest run)
bash scripts/test.sh server      # `go test ./... -count=1` + integration package
bash scripts/deploy-validate.sh  # Build images via docker compose, start stack, run smoke

# Web (run from src/web)
pnpm dev                         # Vite dev server
pnpm build                       # tsc --noEmit && vite build
pnpm test                        # vitest run (jsdom, globals)
pnpm test:e2e                    # playwright test
pnpm preview                     # Serve built dist

# Server (run from src/server)
go run ./cmd/server              # Start API on SERVER_PORT
go run ./cmd/migrate             # Apply SQL migrations from src/server/migrations/
go test ./... -count=1           # Unit + contract tests
go test ./internal/http          # HTTP integration suite (requires TEST_DATABASE_URL)
```

No dedicated linter is wired into CI; quality gates are: type-check (`tsc --noEmit`), build success, and `go test`. There is no `eslint`, `prettier`, or `gofmt` step in `.github/workflows/ci.yml` or in `scripts/check.sh`.

## CI Pipeline

Defined in `.github/workflows/ci.yml`, four jobs run on `push` to `main`/`master` and on every PR:

| Job | Steps | Source |
|-----|-------|--------|
| `release-gates` | `bash scripts/check.sh docs` | Lines 11-16 |
| `web` | pnpm install, `bash scripts/check.sh web`, `bash scripts/test.sh web` | Lines 18-36 |
| `e2e` | pnpm install, `playwright install --with-deps chromium`, `pnpm --dir src/web test:e2e` | Lines 38-56 |
| `server` | `setup-go` from `src/server/go.mod`, `bash scripts/check.sh server`, `bash scripts/test.sh server` | Lines 58-68 |

All jobs run on `ubuntu-latest`. The server job does not run integration tests (no `TEST_DATABASE_URL`).

---

*Stack analysis: 2026-05-16*
