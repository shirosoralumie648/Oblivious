# Technology Stack

**Analysis Date:** 2026-05-12

## Scope

**Active mainline:**
- Root application code lives in `src/server`, `src/web`, `config`, `scripts`, and `.github/workflows`; `README.md` defines this boundary and `pnpm-workspace.yaml` includes only `src/web`.
- `lobehub/` and `new-api/` are repository-local reference trees. Keep them out of root workspace changes unless a phase explicitly targets those directories.

**Reference trees:**
- `lobehub/` is an upstream-style TypeScript/Next.js/Bun/pnpm workspace with its own `lobehub/package.json`, `lobehub/pnpm-workspace.yaml`, and `lobehub/Dockerfile`.
- `new-api/` is an upstream-style Go/Gin service with an embedded Vite/Bun frontend under `new-api/web`.

## Languages

**Primary:**
- Go `1.25.0` - active backend module in `src/server/go.mod`, entrypoints in `src/server/cmd/server/main.go` and `src/server/cmd/migrate/main.go`.
- TypeScript `^5.6.3` - active React frontend in `src/web/src`, configured by `src/web/tsconfig.json`.

**Secondary:**
- SQL - PostgreSQL migrations in `src/server/migrations` and relay migration reference in `src/server/internal/relay/migrations/001_init_relay.sql`.
- Shell - root workflow commands in `scripts/check.sh`, `scripts/test.sh`, `scripts/dev.sh`, `scripts/deploy-validate.sh`, and `scripts/deploy-smoke.sh`.
- YAML - GitHub Actions in `.github/workflows/ci.yml`, Docker Compose in `docker-compose.yml`, and Kubernetes manifests in `deploy/kubernetes`.
- CSS/HTML - Vite entry assets in `src/web/index.html`, `src/web/src/theme/global.css`, and `src/web/src/theme/tokens.css`.
- JSON - package/config metadata in `package.json`, `src/web/package.json`, `src/web/components.json`, and `.mcp.json`.

**Reference-only:**
- TypeScript `^5.9.3` and React `^19.2.3` in `lobehub/package.json`.
- Go `1.25.1` in `new-api/go.mod`, with `new-api/Dockerfile` using a Go builder image separately.
- TypeScript `4.4.2` and React `^18.2.0` in `new-api/web/package.json`.

## Runtime

**Environment:**
- Active backend uses Go modules from `src/server/go.mod`; CI reads this file through `actions/setup-go` in `.github/workflows/ci.yml`.
- Active server container builds with `golang:1.25-bookworm` and runs on `alpine:3.21` in `Dockerfile.server`.
- Active frontend builds with Node.js 20 and pnpm in `Dockerfile.web`; CI also uses Node.js 20 in `.github/workflows/ci.yml`.
- Active web runtime is nginx `1.27-alpine` serving `src/web/dist` and proxying `/api/` and `/v1/` to `oblivious-server` in `Dockerfile.web`.
- `README.md` lists Go `1.22` as a prerequisite, while `src/server/go.mod` and `Dockerfile.server` define the active build toolchain. Use the module and Dockerfile versions when running builds.

**Package Manager:**
- Root package manager: pnpm `10.6.0` from `package.json`.
- Lockfile: `pnpm-lock.yaml` present at repo root.
- Root workspace: `pnpm-workspace.yaml` includes only `src/web`.
- Go dependencies use `src/server/go.mod` and `src/server/go.sum`.
- Scripts pin repo-local caches through `COREPACK_HOME=.tmp/corepack`, `GOCACHE=.tmp/go-build`, and `GOMODCACHE=.tmp/go-mod` in `scripts/check.sh`, `scripts/test.sh`, and `scripts/dev.sh`.
- Reference `lobehub/` declares pnpm `10.33.0` in `lobehub/package.json`.
- Reference `new-api/web` uses Bun via `new-api/web/bun.lock` and `new-api/Dockerfile`.

## Frameworks

**Core:**
- Go `net/http` - active app router and server in `src/server/internal/http/router.go` and `src/server/internal/http/server.go`.
- Gin `v1.12.0` - OpenAI-compatible relay engine under `src/server/internal/relay` and dependency in `src/server/go.mod`.
- React `^18.3.1` - active frontend UI in `src/web/src`.
- Vite `^5.4.10` - active frontend dev/build runner in `src/web/vite.config.ts`.
- React Router DOM `^6.28.0` - route tree in `src/web/src/app/router.tsx`.
- Tailwind CSS `3` - design token integration in `src/web/tailwind.config.ts`, `src/web/postcss.config.mjs`, and `src/web/src/theme/tokens.css`.
- shadcn/Radix UI - component setup in `src/web/components.json` and UI components in `src/web/src/components/ui`.
- PostgreSQL - system of record via `DATABASE_URL`, `github.com/lib/pq`, and migrations in `src/server/migrations`.

**Testing:**
- Go `go test` - backend unit and integration tests in `src/server/internal/**`.
- Vitest `^2.1.4` with jsdom - frontend unit tests configured in `src/web/vite.config.ts`.
- Testing Library - React tests through `@testing-library/react` and `@testing-library/jest-dom` in `src/web/package.json`.
- Playwright `^1.52.0` - e2e tests in `src/web/e2e`, configured by `src/web/playwright.config.ts`.

**Build/Dev:**
- `bash scripts/dev.sh` - starts active web and server together.
- `bash scripts/check.sh` - verifies docs/env consistency, web build, and server checks.
- `bash scripts/test.sh` - runs web tests, server tests, and optional DB-backed integration tests.
- Docker Compose - active local stack in `docker-compose.yml` with Postgres, Redis, server, and web services.
- Kubernetes manifests - deployment templates in `deploy/kubernetes`.
- GitHub Actions - CI gates in `.github/workflows/ci.yml`.

## Key Dependencies

**Critical:**
- `github.com/lib/pq v1.10.9` - PostgreSQL driver used by `src/server/internal/db/db.go`.
- `github.com/gin-gonic/gin v1.12.0` - relay HTTP engine in `src/server/internal/relay`.
- `github.com/gorilla/websocket v1.5.3` - app WebSocket endpoint in `src/server/internal/ws` and relay realtime handlers in `src/server/internal/relay/handler`.
- `github.com/prometheus/client_golang v1.23.2` - `/metrics` handler in `src/server/internal/http/router.go` and custom relay metrics in `src/server/internal/metrics/prometheus.go`.
- `github.com/stripe/stripe-go/v83 v83.2.1` - checkout and webhook package in `src/server/internal/stripe`.
- `golang.org/x/crypto v0.48.0` - bcrypt auth hashing in `src/server/internal/auth`.
- `github.com/pkoukk/tiktoken-go v0.1.8` - token accounting support in `src/server/internal/relay`.
- `github.com/hibiken/asynq v0.26.0` - Redis-backed billing worker code in `src/server/internal/relay/billing_worker.go`.
- `react ^18.3.1`, `react-dom ^18.3.1`, and `react-router-dom ^6.28.0` - active web app runtime in `src/web/package.json`.

**Infrastructure:**
- `@vitejs/plugin-react ^4.3.4` - Vite React integration in `src/web/vite.config.ts`.
- `tailwindcss 3`, `autoprefixer ^10.5.0`, and `postcss ^8.5.12` - active CSS pipeline in `src/web/postcss.config.mjs`.
- `radix-ui ^1.4.3`, `class-variance-authority ^0.7.1`, `clsx ^2.1.1`, and `tailwind-merge ^3.5.0` - UI component primitives and styling helpers in `src/web/package.json`.
- `@remixicon/react ^4.9.0` - icon library configured by `src/web/components.json`.
- `sonner ^2.0.7` and `next-themes ^0.4.6` - frontend notifications and theme state in `src/web/package.json`.

**Reference-only:**
- `lobehub/package.json` includes Next.js `^16.1.5`, React `^19.2.3`, Drizzle ORM `^0.45.1`, OpenAI `^4.104.0`, Anthropic SDK `^0.73.0`, AWS SDK, Vercel packages, Upstash packages, Stripe `^17.7.0`, and many LobeHub workspace packages.
- `new-api/go.mod` includes Gin `v1.9.1`, GORM, SQLite/MySQL/Postgres drivers, Redis, Stripe `v81.4.0`, WebAuthn, JWT, AWS Bedrock, and Pyroscope.
- `new-api/web/package.json` includes React `^18.2.0`, Vite `^5.2.0`, Semi UI, axios, Tailwind, i18next, and Bun-backed build scripts.

## Configuration

**Environment:**
- Active server configuration is loaded in `src/server/internal/config/config.go`.
- Required backend env vars: `DATABASE_URL`, `SESSION_SECRET`.
- Optional backend env vars: `SERVER_PORT`, `APP_ENV`, `CORS_ALLOWED_ORIGINS`, `SESSION_COOKIE_NAME`, `SESSION_COOKIE_SECURE`, `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_TIMEOUT_MS`, `MODEL_DEFAULT_NAME`, `RELAY_ENABLED`, `RELAY_DEFAULT_MODEL`, `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OBLIVIOUS_INTERNAL_AUTH_TOKEN`.
- Stripe package env vars: `STRIPE_SECRET_KEY`, `STRIPE_SUCCESS_URL`, `STRIPE_CANCEL_URL`, `STRIPE_WEBHOOK_SECRET` in `src/server/internal/stripe/checkout.go`.
- Test-only backend env var: `TEST_DATABASE_URL` in `scripts/test.sh` and `src/server/internal/http/server_test.go`.
- Frontend runtime API calls use same-origin relative paths through `src/web/src/services/http/client.ts`; `WEB_API_BASE_URL` is checked by docs/env consistency scripts but is not consumed by active frontend source.
- `.env`-style files are present as examples under `config/`, `lobehub/`, and `new-api/`; do not read secret-bearing env files when mapping or editing.

**Build:**
- Root scripts: `package.json`.
- Root workspace: `pnpm-workspace.yaml`.
- Root lockfile: `pnpm-lock.yaml`.
- Server module: `src/server/go.mod`, `src/server/go.sum`.
- Web manifest: `src/web/package.json`.
- Web build config: `src/web/vite.config.ts`, `src/web/tsconfig.json`, `src/web/tailwind.config.ts`, `src/web/postcss.config.mjs`.
- Container config: `Dockerfile.server`, `Dockerfile.web`, `docker-compose.yml`.
- Deployment config: `deploy/kubernetes/configmap.yaml`, `deploy/kubernetes/server.yaml`, `deploy/kubernetes/web.yaml`, `deploy/kubernetes/postgres.yaml`, `deploy/kubernetes/redis.yaml`, `deploy/kubernetes/secret.example.yaml`.
- CI config: `.github/workflows/ci.yml`.

## Platform Requirements

**Development:**
- Use Go from `src/server/go.mod`.
- Use Node.js 20+ and pnpm `10.6.0` for the active root workspace.
- Use PostgreSQL for normal server startup; migrations are applied by `cd src/server && go run ./cmd/migrate`.
- Use `bash scripts/check.sh` and `bash scripts/test.sh` before handoff.
- Provide `TEST_DATABASE_URL` only when running DB-backed HTTP integration tests; without it `scripts/test.sh` skips that step explicitly.

**Production:**
- Docker Compose target: `docker-compose.yml` with active services `postgres`, `redis`, `oblivious-server`, and `oblivious-web`.
- Server image: `Dockerfile.server`, exposes `8080`, includes `/healthz`, and runs `/usr/local/bin/oblivious-server`.
- Web image: `Dockerfile.web`, exposes `80`, serves static Vite output through nginx, and proxies `/api/` plus `/v1/`.
- Kubernetes target: manifests in `deploy/kubernetes` for namespace, config map, server, web, Postgres, Redis, and example secrets.
- Reference deployment surfaces in `lobehub/Dockerfile`, `lobehub/docker-compose`, `new-api/Dockerfile`, and `new-api/docker-compose.yml` are separate from the active root release path.

---

*Stack analysis: 2026-05-12*
