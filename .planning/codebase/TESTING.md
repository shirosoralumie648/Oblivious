---
last_mapped_commit: c0e55fdbb3aaed7da80a0f7f2399237aed13bca3
mapped_dirty_worktree: true
---

# Testing Patterns

**Analysis Date:** 2026-05-02

## Test Entry Points

**Root scripts:**
```bash
bash scripts/check.sh docs
bash scripts/check.sh web
bash scripts/check.sh server
bash scripts/check.sh all
bash scripts/test.sh web
bash scripts/test.sh server
bash scripts/test.sh all
```

**Backend direct:**
```bash
cd src/server && GOCACHE=../../.tmp/go-build GOMODCACHE=../../.tmp/go-mod go test ./...
```

**Frontend direct:**
```bash
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test
COREPACK_HOME=.tmp/corepack pnpm --dir src/web build
```

## CI

`.github/workflows/ci.yml` defines three jobs:

- `release-gates` runs `bash scripts/check.sh docs`.
- `web` installs pnpm dependencies, runs `bash scripts/check.sh web`, then `bash scripts/test.sh web`.
- `server` runs `bash scripts/check.sh server`, then `bash scripts/test.sh server`.

CI uses:
- Node.js 20.
- pnpm 10.6.0.
- Go version from `src/server/go.mod`.

## Script Behavior

**`scripts/check.sh`:**
- Creates repo-local caches under `.tmp/`.
- `docs` verifies release assets, env/docs consistency, and workspace boundaries.
- `web` runs `pnpm --dir src/web build`.
- `server` runs a focused package set: `./internal/config ./internal/chat ./internal/knowledge ./internal/task ./internal/console`.

**`scripts/test.sh`:**
- `web` runs `pnpm --dir src/web test`.
- `server` runs the same focused backend package set as `check.sh`.
- `server` then runs `go test ./internal/http` only when `TEST_DATABASE_URL` is set.

## Backend Tests

**Framework:**
- Go standard `testing` package only.
- No testify/ginkgo-style assertion library is used in active backend tests.

**Placement:**
- Tests are co-located with packages as `*_test.go`.
- Examples:
  - `src/server/internal/chat/service_test.go`
  - `src/server/internal/agent/service_test.go`
  - `src/server/internal/memory/embedder_test.go`
  - `src/server/internal/relay/billing_test.go`
  - `src/server/internal/ws/hub_test.go`

**Patterns:**
- Use small fake stores/gateways for service tests.
- Prefer table tests for validation/config behavior.
- Use direct service calls for ownership and state-machine behavior.
- Use `httptest` around handlers/router when testing HTTP behavior.
- Use repo-local Go caches to avoid global cache permission issues.

**Integration boundary:**
- HTTP integration tests are gated behind `TEST_DATABASE_URL` in `scripts/test.sh`.
- Keep tests that require PostgreSQL separate from pure unit tests.
- When adding DB-backed tests, make setup/teardown explicit and avoid relying on a developer's local default database.

## Frontend Tests

**Framework:**
- Vitest 2.1.4 with jsdom.
- Testing Library React.
- jest-dom matchers loaded by `src/web/src/test/setup.ts`.

**Placement:**
- Co-locate tests beside route/component/store modules.
- Examples:
  - `src/web/src/app/router.test.tsx`
  - `src/web/src/features/auth/store.test.ts`
  - `src/web/src/features/layouts/WorkspaceLayout.test.tsx`
  - `src/web/src/routes/workspace/ChatPage.behavior.test.tsx`
  - `src/web/src/services/http/client.test.ts`

**Patterns:**
- Test route rendering with `createAppRouter(initialEntries)` from `src/web/src/app/router.tsx`.
- Inject fake APIs or clients rather than mocking global state when possible.
- Test HTTP envelope behavior through `src/web/src/services/http/client.test.ts`.
- Behavior tests can use `.behavior.test.tsx` for richer user flows.

## Coverage And Gaps

**Well-covered areas:**
- Auth bootstrap/store and route protection.
- Workspace and console layout shells.
- Chat, knowledge, task service behavior.
- Relay primitives: billing, retry, circuit breaker, health checker, load balancer, token bucket, tokenizer.
- Agent service and store helper behavior.
- Memory embedder behavior.
- WebSocket hub behavior.

**Known gaps to close before relying on Phase 3 surfaces:**
- `src/web/src/routes/admin/AdminHomePage.tsx` and `src/web/src/routes/admin/AdminUsersPage.tsx` do not have route/page tests.
- `src/web/src/routes/workspace/MarketplacePage.tsx` has no tests and currently uses a static catalog with direct `fetch`.
- `src/server/internal/marketplace/` has no visible tests in the current file list.
- Admin channel/route/plan/audit services exist but are not covered by the focused `scripts/check.sh server` package list.
- `scripts/check.sh server` and `scripts/test.sh server` do not run `go test ./...`; use the broader command for shared backend or release-risk changes.

## Recommended Test Additions

**For Admin UI/API work:**
- Add typed admin API client tests under `src/web/src/features/admin/`.
- Add page behavior tests for `/admin` and `/admin/users`.
- Add backend handler tests for all newly exposed admin routes.
- Add service/store tests for channel, route, plan, audit, and review-queue paths.

**For Marketplace work:**
- Add backend service/store tests in `src/server/internal/marketplace/`.
- Add HTTP handler tests once marketplace routes are registered.
- Add frontend tests for search/filter/install/review flows.

**For Relay changes:**
- Keep unit tests close to the changed primitive.
- Add handler-level tests for endpoint families when changing request/response mapping.
- Include quota lifecycle assertions when changing billing paths.

## Verification Rule Of Thumb

- Docs-only changes: run `bash scripts/check.sh docs` when the docs/env contracts are touched.
- Frontend UI changes: run `pnpm --dir src/web test` and `pnpm --dir src/web build`.
- Backend domain changes: run package tests for the touched domain plus `go test ./...` when shared interfaces, router wiring, migrations, or Relay are affected.
- Database schema changes: run migration tooling against a disposable database and include rollback/re-run expectations in the phase UAT.
