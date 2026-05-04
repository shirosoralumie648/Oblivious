---
last_mapped_commit: 98576468acf0d72bbca7e61317dc83cd5c6ad7a9
mapped_dirty_worktree: true
analysis_date: 2026-05-04
mapper: sequential-fallback
---

# Testing Patterns

## Test Inventory

Approximate current test file counts:

- Backend: 35 Go `*_test.go` files under `src/server/`.
- Frontend/unit: 32 Vitest test files reported by `bash scripts/test.sh all`.
- Frontend/E2E: `src/web/e2e/admin-marketplace.spec.ts` with 3 Playwright tests.

## Primary Commands

Use these commands for current release work:

```bash
bash scripts/check.sh docs
bash scripts/check.sh web
bash scripts/check.sh server
bash scripts/check.sh all
bash scripts/test.sh web
bash scripts/test.sh server
bash scripts/test.sh all
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e
bash scripts/deploy-validate.sh
```

## Check Gate

`scripts/check.sh` defines:

- `docs`: runs `scripts/verify-quality-gates.sh`, verifies env vars in `config/.env.example`, `docs/architecture/current-system-contracts.md`, and `src/server/internal/config/config.go`, and checks workspace boundary.
- `web`: runs `pnpm --dir src/web build`, which runs `tsc --noEmit && vite build`.
- `server`: runs `go test ./... -count=1` in `src/server`.
- `all`: docs, web, server in sequence.

Known caveat: server tests using `httptest.NewServer` need local socket/listen ability. In this sandbox they required the approved non-sandbox path.

## Test Gate

`scripts/test.sh` defines:

- `web`: `pnpm --dir src/web test`
- `server`: `go test ./... -count=1`, then optional DB-backed `go test ./internal/http`
- `all`: web then server

`TEST_DATABASE_URL` behavior:

- When unset, `scripts/test.sh server` prints `Skipping server integration tests: TEST_DATABASE_URL not set.`
- Release evidence must record this as an explicit skip, not silently treat DB-backed coverage as run.
- When set, the script runs `go test ./internal/http`.

## Backend Test Patterns

Backend tests are colocated and mostly standard-library based:

- HTTP/middleware/router tests in `src/server/internal/http/*_test.go`.
- Chat gateway/service tests in `src/server/internal/chat/*_test.go`.
- Agent service/store tests in `src/server/internal/agent/*_test.go`.
- Knowledge service/store tests in `src/server/internal/knowledge/*_test.go`.
- Marketplace service tests in `src/server/internal/marketplace/service_test.go`.
- Relay billing/routing/rate/health/pricing/token tests in `src/server/internal/relay/*_test.go`.
- Quota, notification, metrics, task, console, WebSocket tests under their feature packages.

Patterns:

- Prefer fake stores and `httptest` for boundary behavior.
- Use package-level unit tests for service/store behavior.
- Use broad `go test ./... -count=1` as the server release gate.
- Use DB-backed tests only with explicit `TEST_DATABASE_URL`.

## Frontend Unit Test Patterns

Vitest + Testing Library tests are colocated:

- App/router/context tests under `src/web/src/app/`.
- Auth tests under `src/web/src/features/auth/`.
- API client tests under `src/web/src/features/admin/` and `src/web/src/features/marketplace/`.
- Layout tests under `src/web/src/features/layouts/`.
- Route tests under `src/web/src/routes/*/*.test.tsx`.
- HTTP client tests under `src/web/src/services/http/`.

The web package script is `pnpm --dir src/web test`, and CI runs it through `bash scripts/test.sh web`.

## E2E Patterns

- Playwright config: `src/web/playwright.config.ts`.
- Main spec: `src/web/e2e/admin-marketplace.spec.ts`.
- Fixtures: `src/web/e2e/fixtures/adminMarketplace.ts`.
- Current E2E scope:
  - Admin navigation exposes release management pages.
  - Marketplace browse/detail/install workflow.
  - Marketplace publish and my-agents workflow.
- E2E avoids live providers, Stripe, and external LLM calls by route fixtures.
- CI installs Chromium with `pnpm --dir src/web exec playwright install --with-deps chromium`.

## Deployment Validation

Deployment validation is not complete until a real runtime stack starts:

```bash
bash scripts/deploy-validate.sh
```

That script:

1. Checks `docker` and `docker compose`.
2. Checks `docker info`.
3. Runs `docker compose config`.
4. Runs `docker compose build`.
5. Runs `docker compose up -d`.
6. Runs `BASE_URL=... bash scripts/deploy-smoke.sh`.

Current host blocker:

- `docker info` fails with permission denied on `/var/run/docker.sock`.
- `dockerd` requires root privileges.
- Docker Desktop build socket/buildx also fails with permission errors.
- `kubectl` is not installed.

Therefore DEPLOY-01 remains blocked despite configuration and script coverage.

## CI Coverage

`.github/workflows/ci.yml` includes jobs for:

- release docs/assets
- web build/test
- Playwright E2E
- server check/test

CI does not currently prove Docker compose startup or Kubernetes apply. Those are release/operator gates in `docs/release/rc-checklist.md`.

## Verification Evidence Last Observed

Current planning state records:

- `bash scripts/check.sh all` passed.
- `bash scripts/test.sh all` passed: Web 32 files / 110 tests, server `go test ./... -count=1`, explicit `TEST_DATABASE_URL` skip.
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e` passed 3/3.
- `bash scripts/check.sh docs` passed.
- `docker compose config` passed.
- `bash scripts/deploy-validate.sh` fails early because Docker daemon is unreachable.
