# Testing Patterns

**Analysis Date:** 2026-05-16

The repo runs three parallel test suites, each gated independently in CI (`.github/workflows/ci.yml`):
1. **Web unit / component tests** — Vitest + Testing Library (jsdom)
2. **Web end-to-end** — Playwright (Chromium) against a built `vite preview` server
3. **Server tests** — Go's built-in `testing` package, with optional integration tests against a real Postgres

All three are orchestrated through `scripts/test.sh` and `scripts/check.sh` so local and CI behavior stay aligned.

## Test Framework

**Runner — Web:**
- Vitest `^2.1.4` — `src/web/package.json`
- Config lives inside `src/web/vite.config.ts` (no separate `vitest.config.*`):
  ```ts
  test: {
    environment: 'jsdom',
    globals: true,
    exclude: ['e2e/**', 'node_modules/**', 'dist/**'],
    setupFiles: ['./src/test/setup.ts'],
  }
  ```
- TypeScript awareness: `src/web/tsconfig.json` declares `"types": ["vitest/globals"]`, so `describe`/`it`/`expect`/`vi` are globally typed.

**Runner — E2E:**
- Playwright `@playwright/test ^1.52.0` — `src/web/package.json`
- Config: `src/web/playwright.config.ts`
  - `testDir: './e2e'`
  - `fullyParallel: false` — tests share state via mocked routes
  - `reporter`: GitHub + list in CI; list locally (driven by `process.env.CI`)
  - `use.baseURL: 'http://127.0.0.1:4173'`
  - `use.trace: 'retain-on-failure'`
  - `webServer.command: 'pnpm build && pnpm preview --host 127.0.0.1 --port 4173'`
  - `webServer.timeout: 120_000`
  - Single project `chromium` using `devices['Desktop Chrome']`

**Runner — Server:**
- Go standard `testing` package (no Ginkgo / Testify is required; basic `t.Fatalf` / `t.Helper`).
- Driver: `go test ./... -count=1` from `src/server/`.

**Assertion Libraries:**
- Web unit: `expect` from Vitest + `@testing-library/jest-dom/vitest` matchers (imported once in `src/web/src/test/setup.ts`: `import '@testing-library/jest-dom/vitest';`).
- E2E: `expect` from `@playwright/test`.
- Server: plain `t.Fatalf` / `t.Errorf` comparisons.

**Component testing utilities:**
- `@testing-library/react ^16.1.0` — `render`, `screen`, `fireEvent`, `waitFor`
- `@testing-library/jest-dom ^6.6.3` — DOM matchers (`toBeInTheDocument`, `toBeVisible`, etc.)
- `jsdom ^25.0.1` — DOM environment for Vitest

**Run Commands:**
```bash
# All suites (skips gracefully if src/server or src/web missing)
bash scripts/test.sh                  # alias of scripts/test.sh all
pnpm test                             # same, via root package.json

# Web unit tests only
bash scripts/test.sh web
pnpm test:web                         # pnpm --dir src/web test  →  vitest run

# Web E2E only
pnpm test:e2e                         # pnpm --dir src/web test:e2e  →  playwright test

# Server tests only
bash scripts/test.sh server
pnpm test:server                      # go test ./... (skips integration if TEST_DATABASE_URL unset)

# Release gates / build verification
bash scripts/check.sh                 # all
bash scripts/check.sh docs            # quality-gates + env contract check (no compile)
bash scripts/check.sh web             # tsc --noEmit && vite build
bash scripts/check.sh server          # go test ./... -count=1
```

## Test File Organization

**Web — co-located unit / component tests:**
- Pattern: source `Foo.tsx` lives beside its test `Foo.test.tsx` in the same directory.
- Examples:
  - `src/web/src/services/http/client.ts` ↔ `src/web/src/services/http/client.test.ts`
  - `src/web/src/routes/admin/AdminUsersPage.tsx` ↔ `src/web/src/routes/admin/AdminUsersPage.test.tsx`
  - `src/web/src/features/auth/store.ts` ↔ `src/web/src/features/auth/store.test.ts`
- Behavior-flavored extra coverage uses `.behavior.test.tsx` (e.g. `src/web/src/routes/workspace/ChatPage.behavior.test.tsx`) — used when a page has a separate user-flow surface beyond the basic render test.

**Web — E2E (Playwright):**
- Directory: `src/web/e2e/` (excluded from Vitest via `exclude: ['e2e/**', ...]`).
- Specs: `src/web/e2e/admin-marketplace.spec.ts` — the single end-to-end spec at the moment, covering admin navigation, marketplace browse/install, and publish flows.
- Fixtures: `src/web/e2e/fixtures/adminMarketplace.ts` — registers mocked API routes for the preview server.

**Server (Go):**
- Pattern: `_test.go` next to source (`internal/agent/service.go` ↔ `internal/agent/service_test.go`).
- Integration tests gated by `TEST_DATABASE_URL`: live in `src/server/internal/http/` and skip if the env var is absent (`src/server/internal/http/server_test.go`).

**Structure summary:**
```
src/web/
├── e2e/
│   ├── admin-marketplace.spec.ts
│   └── fixtures/
│       └── adminMarketplace.ts
├── playwright.config.ts
├── vite.config.ts                # also holds vitest config
└── src/
    ├── test/setup.ts             # vitest setupFiles entry
    ├── app/
    │   ├── appContext.tsx
    │   ├── appContext.test.tsx
    │   ├── router.tsx
    │   └── router.test.tsx
    ├── features/{admin,auth,...}/*.test.{ts,tsx}
    ├── routes/{admin,console,marketplace,workspace}/*.test.tsx
    └── services/http/client.test.ts

src/server/internal/
├── admin/service_test.go
├── agent/{service,store}_test.go
├── chat/{gateway,relay_gateway,service}_test.go
├── config/config_test.go
├── http/                         # router/integration (Postgres) tests
│   ├── admin_marketplace_handler_test.go
│   ├── auth_middleware_test.go
│   ├── chat_handler_test.go
│   ├── knowledge_handler_test.go
│   ├── middleware_test.go
│   ├── notification_handler_test.go
│   ├── route_surface_test.go
│   └── server_test.go
└── task/{runtime,service}_test.go
```

## Test Structure

**Vitest suite shape:**
```ts
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listUsers = vi.fn();
vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({ listUsers, /* ... */ }),
}));

import { AdminUsersPage } from './AdminUsersPage';

describe('AdminUsersPage', () => {
  beforeEach(() => {
    listUsers.mockReset();
  });

  it('renders users with role, plan, status, and usage', async () => {
    listUsers.mockResolvedValue({ data: [activeUser], total: 1 });
    render(<AdminUsersPage />);
    expect(await screen.findByRole('heading', { name: 'Users' })).toBeInTheDocument();
  });
});
```
Pattern is consistent across `src/web/src/routes/admin/*.test.tsx`.

**Hoisted shared state:** When a test needs state that the mocked module closes over (e.g. router params, app context), use `vi.hoisted`:
```ts
const routeState = vi.hoisted(() => ({ conversationId: undefined as string | undefined }));
const appContext = vi.hoisted(() => ({ authState: { /* ... */ } }));
```
See `src/web/src/routes/workspace/ChatPage.behavior.test.tsx`.

**Store / service tests (`src/web/src/features/auth/store.test.ts`):**
- No DOM. Instantiate the factory under test (`createAuthStore()`), call mutators, assert via `store.getState()`, attach `vi.fn()` listeners and assert call counts.

**Go suite shape (`src/server/internal/agent/service_test.go`, `src/server/internal/http/server_test.go`):**
- Package-local `fakeXxx` types implement the dependency interfaces (`fakeGateway`, `fakeStore`).
- Helper functions like `testConfig()` and `testDatabase(t *testing.T) *sql.DB` build fixtures; `t.Helper()` is called so failures point at the caller.
- Integration tests register cleanup via `t.Cleanup(func() { database.Close() })`.
- `t.Skip(...)` is used to bypass integration tests when prerequisites (`TEST_DATABASE_URL`) are missing.

**Setup / teardown patterns:**
- Frontend: `beforeEach`/`afterEach` defined globally in `src/web/src/test/setup.ts` (see Console policy below). Per-test resets use `mockReset()` inside `beforeEach`.
- Go: `t.Helper()` plus `t.Cleanup()` rather than `TestMain`.

**Strict console policy (`src/web/src/test/setup.ts`):**
- Every test wraps `console.warn` / `console.error` with `vi.spyOn(...).mockImplementation(...)` recording calls into `unexpectedConsoleCalls`.
- In `afterEach`, any recorded call **throws**: `"Unexpected console calls detected during the test."`
- Effect: any React warning, prop-types violation, or unhandled promise rejection inside a test will fail it. This is the de-facto lint for runtime mistakes.

## Mocking

**Framework:** Vitest's `vi.mock`, `vi.fn`, `vi.spyOn`, `vi.hoisted`, `vi.importActual`.

**Module mocking pattern:**
```ts
vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listUsers, updateUser, disableUser, enableUser,
  }),
}));
```
The exported factory is replaced by one that returns the test's `vi.fn()` references — see `src/web/src/routes/admin/AdminUsersPage.test.tsx`.

**Partial module mock with passthrough:**
```ts
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => navigate, useParams: () => ({ conversationId: routeState.conversationId }) };
});
```
Used in `src/web/src/routes/workspace/ChatPage.behavior.test.tsx`.

**HTTP client injection (no global fetch monkey-patching):**
```ts
const fetchFn = vi.fn(async () => new Response(JSON.stringify({ ok: true, data: { requests: 3 }, error: null }), { status: 200 }));
const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
```
Pattern: pass `fetchFn` into `createHttpClient(...)`. See `src/web/src/services/http/client.test.ts`.

**Feature API mocks:**
- API tests use a `createClient(overrides: Partial<HttpClient>)` helper that returns a `HttpClient` whose methods are `vi.fn()` — see `src/web/src/features/admin/api.test.ts`. Each method is asserted with `expect(client.get).toHaveBeenNthCalledWith(...)`.

**Playwright network mocking:**
- `page.route(...)` patterns are registered via fixtures — see `registerAdminMarketplaceRoutes` exported from `src/web/e2e/fixtures/adminMarketplace.ts`. Specs call `await registerAdminMarketplaceRoutes(page)` inside `test.beforeEach`.
- The Playwright web server runs `pnpm build && pnpm preview`, so the app is exercised in production-build mode against mocked HTTP.

**Go fakes:**
- Hand-written `fakeGateway`, `fakeStore` structs satisfying the production interfaces — see `src/server/internal/agent/service_test.go`. Unused fake methods `panic("not used")` to surface accidental coupling.
- No `gomock` / `mockery` in tree.

**What to Mock:**
- External I/O: `fetch` (web), HTTP/MCP clients (Go), Postgres (only via real `TEST_DATABASE_URL`, otherwise skipped).
- Cross-module collaborators that would pull in routing or context (e.g. `react-router-dom` hooks, `useAppContext`).

**What NOT to Mock:**
- The component / module under test itself.
- Pure utility modules (`unwrapEnvelope`, `createAuthStore`) — these are exercised directly.
- React Testing Library queries — never mock `screen`; let the rendered DOM be the contract.

## Fixtures and Factories

**Inline literal fixtures (most tests):**
- Small, test-scoped objects defined at the top of the test file (`activeUser`, `preferences`, `session` in `src/web/src/features/auth/store.test.ts`).
- IDs use snake-case prefixes mirroring backend IDs (`user_1`, `plan_pro`, `kb_1`).
- Timestamps are hard-coded ISO strings (`'2026-05-02T12:00:00Z'`, `'2026-01-01T00:00:00Z'`).

**E2E shared fixtures:**
- `src/web/e2e/fixtures/adminMarketplace.ts` exports `registerAdminMarketplaceRoutes(page: Page)` plus typed mock entities (`channel`, `routeInfo`, `plan`, `releaseAgent`, `adminSession`). Tests import only the registration function.

**Go fixtures:**
- `testConfig()` builds a `config.Config` populated for local Postgres (`src/server/internal/http/server_test.go`).
- `testDatabase(t *testing.T) *sql.DB` drops and re-creates the schema each invocation — full DDL is inlined into the helper. Cleanup is registered via `t.Cleanup`.

## Coverage

**Requirements:** No coverage threshold is enforced. There is no `vitest --coverage` invocation in `package.json`, `scripts/test.sh`, or `.github/workflows/ci.yml`. The release gate is the test result, not coverage.

**View Coverage (ad hoc):**
```bash
pnpm --dir src/web exec vitest run --coverage
cd src/server && go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
```

## CI Test Gating

CI workflow: `.github/workflows/ci.yml` runs four parallel jobs on every push to `main`/`master` and on every PR. All four must pass.

| Job | Steps | Failure Means |
|-----|-------|---------------|
| `release-gates` | `bash scripts/check.sh docs` | Env contract drift between `config/.env.example`, `docs/architecture/current-system-contracts.md`, `src/server/internal/config/config.go`; or workspace boundary violation (e.g. `lobehub` / `new-api` accidentally re-added to `pnpm-workspace.yaml`). Driven by `scripts/verify-quality-gates.sh`. |
| `web` | `pnpm install --frozen-lockfile` → `bash scripts/check.sh web` → `bash scripts/test.sh web` | `tsc --noEmit` failure, Vite build failure, or any failing Vitest spec (including unexpected `console.warn` / `console.error`). |
| `e2e` | `pnpm install --frozen-lockfile` → `pnpm --dir src/web exec playwright install --with-deps chromium` → `pnpm --dir src/web test:e2e` | Any failing Playwright spec under `src/web/e2e/`. Playwright auto-runs `pnpm build && pnpm preview` per `playwright.config.ts.webServer`. |
| `server` | `actions/setup-go@v5` with `go-version-file: src/server/go.mod` → `bash scripts/check.sh server` → `bash scripts/test.sh server` | Any failing `go test ./...` package. `server_test.go` integration tests **skip** when `TEST_DATABASE_URL` is unset, so CI runs unit tests only; integration tests are only exercised when a developer sets the env var locally. |

**Important CI behaviors:**
- `release-gates` is the deployment-gate alignment surface (Phase 7). Adding/renaming env vars without updating the three contract documents will fail this job — see the `frontend_vars` and `backend_vars` arrays in `scripts/check.sh`.
- The `e2e` job is independent from the `web` job; both must pass.
- The web Playwright config sets `fullyParallel: false`, so E2E specs run sequentially even on multi-core runners — important for shared mocked state.
- `reuseExistingServer: !process.env.CI` in `playwright.config.ts` means CI always rebuilds and re-starts the preview server.

## Test Types

**Unit Tests (Vitest):**
- Scope: pure factories, stores, API normalization layers, helper functions.
- Examples: `src/web/src/services/http/client.test.ts`, `src/web/src/services/http/envelope.ts` callers, `src/web/src/features/auth/store.test.ts`, `src/web/src/features/admin/api.test.ts`.
- Run with jsdom but typically don't render anything.

**Component Tests (Vitest + Testing Library):**
- Scope: a single route or shared component rendered under mocked context + mocked API.
- Examples: every file under `src/web/src/routes/admin/*.test.tsx`, `src/web/src/components/shared/shared-components.test.tsx`, `src/web/src/features/layouts/*.test.tsx`.
- Use `render(<Component />)`, `screen.findByRole(...)`, `fireEvent`/`waitFor`.

**Integration Tests (Go):**
- Scope: full HTTP router wired up to a real Postgres via `database/sql` + `lib/pq`.
- Files: `src/server/internal/http/*_test.go`.
- Gated by `TEST_DATABASE_URL`; the `scripts/test.sh` server target only runs them when the env var is set: `if [[ -z "${TEST_DATABASE_URL:-}" ]]; then echo "[test] Skipping server integration tests"; return; fi`.

**E2E Tests (Playwright):**
- Scope: production-built SPA driven through a real browser against mocked backend.
- Single file today: `src/web/e2e/admin-marketplace.spec.ts`.
- Mocks are registered via `page.route` in `src/web/e2e/fixtures/adminMarketplace.ts`.

## Common Patterns

**Async testing:**
```ts
it('renders users with role, plan, status, and usage', async () => {
  listUsers.mockResolvedValue({ data: [activeUser], total: 1 });
  render(<AdminUsersPage />);
  expect(await screen.findByRole('heading', { name: 'Users' })).toBeInTheDocument();
});
```
Prefer `findBy*` (which waits) over `getBy*` + manual `waitFor` for the first assertion. Use `waitFor(() => expect(spy).toHaveBeenCalledWith(...))` after user interactions.

**Error testing:**
```ts
const fetchFn = vi.fn(async () => new Response('nope', { status: 500, statusText: 'Server Error' }));
const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
await expect(client.get('/api/v1/console/usage')).rejects.toBeInstanceOf(HttpError);
```
See `src/web/src/services/http/client.test.ts`.

**Resetting per-test:**
```ts
beforeEach(() => {
  listUsers.mockReset();
  updateUser.mockReset();
  disableUser.mockReset();
  enableUser.mockReset();
});
```
Each top-level `vi.fn()` is reset, not the mock module declaration (which is hoisted once).

**Sequenced mock responses:**
```ts
listUsers
  .mockResolvedValueOnce({ data: [activeUser], total: 1 })
  .mockResolvedValueOnce({ data: [{ ...activeUser, status: 'disabled' }], total: 1 })
  .mockResolvedValue({ data: [{ ...activeUser, status: 'disabled' }], total: 1 });
```
Drives multi-step UI flows where the same endpoint is hit before and after an action — see `src/web/src/routes/admin/AdminUsersPage.test.tsx`.

**Playwright assertions:**
```ts
await page.goto('/admin');
await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();
await page.getByPlaceholder('Search agents...').fill('release');
await page.getByRole('button', { name: 'Install Agent' }).click();
await expect(page.getByText('Agent installed.')).toBeVisible();
```
Prefer role/name/placeholder queries over CSS selectors. `.first()` is used when a string is rendered in both a list and a detail header.

**Go HTTP test pattern:**
```go
router := NewRouter(testConfig(), testDatabase(t))
recorder := httptest.NewRecorder()
request := httptest.NewRequest(stdhttp.MethodGet, "/healthz", nil)
router.ServeHTTP(recorder, request)
if recorder.Code != stdhttp.StatusOK {
  t.Fatalf("expected 200, got %d", recorder.Code)
}
```
See `src/server/internal/http/server_test.go`.

---

*Testing analysis: 2026-05-16*
