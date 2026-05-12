# Testing Patterns

**Analysis Date:** 2026-05-12

## Test Framework

**Runner:**
- Go standard `testing` for `src/server`, configured by `src/server/go.mod` and run through `scripts/test.sh`.
- Vitest 2.x for `src/web`, configured in `src/web/vite.config.ts` with `jsdom`, globals, and `src/web/src/test/setup.ts`.
- Playwright for `src/web/e2e`, configured in `src/web/playwright.config.ts`.
- Vitest for `lobehub/`, configured in `lobehub/vitest.config.mts` and package-level files such as `lobehub/packages/database/vitest.config.server.mts`.
- Go standard `testing` plus `testify` for `new-api/`, with dependencies in `new-api/go.mod`.

**Assertion Library:**
- Use Go `testing` assertions with `t.Fatalf` and `t.Fatal` in mainline backend tests such as `src/server/internal/chat/service_test.go`.
- Use `expect` from Vitest and `@testing-library/jest-dom` in `src/web/src/**/*.test.tsx`, initialized by `src/web/src/test/setup.ts`.
- Use Playwright `expect` in `src/web/e2e/admin-marketplace.spec.ts`.
- Use `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require` in New API tests such as `new-api/service/task_billing_test.go`.

**Run Commands:**
```bash
bash scripts/test.sh              # Run mainline web and server tests
bash scripts/test.sh web          # Run src/web Vitest tests
bash scripts/test.sh server       # Run src/server Go tests and DB-gated integration tests
bash scripts/check.sh             # Run release docs checks, web build, and server checks
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test      # Run src/web unit tests directly
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e  # Run src/web Playwright tests
cd src/server && GOCACHE=../../.tmp/go-build GOMODCACHE=../../.tmp/go-mod go test ./... -count=1
cd lobehub && pnpm test-app                              # Run LobeHub app Vitest suite
cd lobehub && pnpm test-app:coverage                     # Run LobeHub app coverage
cd new-api && go test ./...                              # Run New API Go tests
```

## Test File Organization

**Location:**
- Co-locate mainline Go tests with the package under test, such as `src/server/internal/relay/retry_test.go` beside `src/server/internal/relay/retry.go`.
- Co-locate mainline React/Vitest tests beside route, feature, service, and app modules, such as `src/web/src/services/http/client.test.ts` beside `src/web/src/services/http/client.ts`.
- Keep browser-level tests under `src/web/e2e/`, with reusable Playwright fixtures under `src/web/e2e/fixtures/`.
- Keep LobeHub tests either co-located as `*.test.ts` or under `__tests__` directories, such as `lobehub/apps/desktop/src/main/controllers/__tests__/AuthCtr.test.ts`.
- Keep New API Go tests beside package code, such as `new-api/controller/token_test.go` and `new-api/service/error_test.go`.

**Naming:**
- Use `*_test.go` for Go tests in `src/server` and `new-api`.
- Use `.test.ts` and `.test.tsx` for Vitest tests in `src/web/src` and `lobehub`.
- Use `.spec.ts` for Playwright tests in `src/web/e2e`.
- Use descriptive test names that state behavior, such as `TestChatHandlerUpdateConversationConfigAcceptsKnowledgeBaseIDs` in `src/server/internal/http/chat_handler_test.go`.

**Structure:**
```text
src/server/internal/<domain>/<unit>.go
src/server/internal/<domain>/<unit>_test.go
src/web/src/<area>/<module>.ts
src/web/src/<area>/<module>.test.ts
src/web/src/routes/<domain>/<Page>.tsx
src/web/src/routes/<domain>/<Page>.test.tsx
src/web/e2e/<workflow>.spec.ts
src/web/e2e/fixtures/<workflow>.ts
lobehub/packages/<package>/src/**/__tests__/*.test.ts
new-api/<package>/<unit>_test.go
```

## Test Structure

**Suite Organization:**
```go
func TestRetry_RetriesOnFailure(t *testing.T) {
	callCount := 0
	fn := func(ctx context.Context) (*types.ProviderResponse, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("temporary error")
		}
		return &types.ProviderResponse{StatusCode: http.StatusOK}, nil
	}

	resp, err := Retry(fn, 3, "chat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls, got %d", callCount)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
```

```typescript
describe('auth store', () => {
  it('notifies subscribers when the state changes', () => {
    const store = createAuthStore();
    const listener = vi.fn();
    const unsubscribe = store.subscribe(listener);

    store.startLoading();

    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    store.startLoading();

    expect(listener).toHaveBeenCalledTimes(1);
  });
});
```

**Patterns:**
- Use arrange/act/assert inside each test in `src/server/internal/chat/service_test.go`, `src/web/src/features/auth/store.test.ts`, and `new-api/controller/token_test.go`.
- Use table-driven tests with `t.Run` for input matrices in `new-api/common/url_validator_test.go` and `new-api/service/error_test.go`.
- Use `t.Helper()` on Go setup helpers such as `testDatabase` in `src/server/internal/http/server_test.go` and `setupTokenControllerTestDB` in `new-api/controller/token_test.go`.
- Use `beforeEach` for reusable mock reset in `src/web/src/test/setup.ts`, `src/web/e2e/admin-marketplace.spec.ts`, and `lobehub/src/hooks/useFetchTopics.test.ts`.

## Mocking

**Framework:** Go fakes, `httptest`, Vitest `vi`, Testing Library, Playwright route interception

**Patterns:**
```go
type fakeUsageRecorder struct {
	records []UsageRecord
}

func (f *fakeUsageRecorder) RecordChatUsage(ctx context.Context, record UsageRecord) error {
	f.records = append(f.records, record)
	return nil
}
```

```typescript
const fetchFn = vi.fn(async () =>
  new Response(JSON.stringify({ ok: true, data: { requests: 3 }, error: null }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' }
  })
);

const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
```

```typescript
await page.route('**/api/v1/**', async (route) => {
  const request = route.request();
  const url = new URL(request.url());
  const { pathname } = url;
  const method = request.method();
  // Fulfill matching API routes with JSON envelopes.
});
```

**What to Mock:**
- Mock Go service interfaces at package boundaries, such as `chat.Store`, `chat.ReplyGenerator`, and `UsageRecorder` in `src/server/internal/chat/service_test.go`.
- Mock HTTP dependencies with `httptest.NewServer`, `httptest.NewRequest`, and `httptest.NewRecorder` in `src/server/internal/http/*_test.go` and `src/server/internal/relay/router_test.go`.
- Mock browser fetch through dependency injection in `src/web/src/services/http/client.test.ts`.
- Mock app API responses through Playwright route handlers in `src/web/e2e/fixtures/adminMarketplace.ts`.
- Mock external modules with `vi.mock` and `vi.hoisted` in LobeHub tests such as `lobehub/src/hooks/useFetchTopics.test.ts`.
- Mock New API persistence with in-memory SQLite and Gorm setup helpers in `new-api/controller/token_test.go` and `new-api/service/task_billing_test.go`.

**What NOT to Mock:**
- Do not mock pure normalization or parsing helpers; test them directly, as in `src/server/internal/knowledge/store_test.go` and `src/web/src/features/auth/workspaceLanding.test.ts`.
- Do not mock the root app router when validating route contracts; use `createAppRouter` and `RouterProvider` in `src/web/src/routes/workspace/ChatPage.test.tsx`.
- Do not mock the mainline integration database path when testing end-to-end HTTP persistence in `src/server/internal/http/server_test.go`; gate it behind `TEST_DATABASE_URL`.

## Fixtures and Factories

**Test Data:**
```go
func seedToken(t *testing.T, db *gorm.DB, userID int, name string, rawKey string) *model.Token {
	t.Helper()

	token := &model.Token{
		UserId: userID,
		Name:   name,
		Key:    rawKey,
		Status: common.TokenStatusEnabled,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	return token
}
```

```typescript
const releaseAgent = {
  id: 'agent_release_helper',
  ownerID: 'user_admin',
  name: 'Release Helper',
  status: 'approved',
  currentVersion: '1.0.0'
};
```

**Location:**
- Mainline Go fixtures live as unexported helper types and functions inside each `src/server/internal/**/*_test.go` file.
- Mainline web shared setup lives in `src/web/src/test/setup.ts`; Playwright fixtures live in `src/web/e2e/fixtures/adminMarketplace.ts`.
- New API test helpers live inside package tests such as `new-api/controller/token_test.go` and `new-api/service/task_billing_test.go`.
- LobeHub shared test utilities live in `lobehub/tests/setup.ts` and `lobehub/tests/utils.tsx`.

## Coverage

**Requirements:** None enforced for mainline `src/server` or `src/web`.

**View Coverage:**
```bash
cd src/server && go test ./... -cover
cd src/web && pnpm exec vitest run --coverage
cd lobehub && pnpm test-app:coverage
cd lobehub/packages/database && pnpm test:coverage
```

- Mainline `src/web/package.json` has `test` and `test:e2e` scripts but no coverage script.
- Mainline `scripts/check.sh` enforces build and test completion but no percentage threshold.
- LobeHub uses V8 coverage with text/json/lcov/text-summary reporters in `lobehub/vitest.config.mts` and package-level configs under `lobehub/packages/*/vitest.config.*`.

## Test Types

**Unit Tests:**
- Backend unit tests cover services, config, relay logic, token buckets, task runtime, and helpers under `src/server/internal/**/*_test.go`.
- Frontend unit/component tests cover stores, API clients, route shells, layouts, and pages under `src/web/src/**/*.test.ts` and `src/web/src/**/*.test.tsx`.
- LobeHub package tests cover hooks, server utilities, Vite plugins, packages, and desktop controllers under `lobehub/src/`, `lobehub/packages/`, and `lobehub/apps/`.
- New API unit tests cover controllers, DTOs, relay helpers, service billing, and URL validation under `new-api/**/*_test.go`.

**Integration Tests:**
- Mainline server integration tests in `src/server/internal/http/server_test.go` exercise router, auth, conversations, and usage records against a real Postgres database when `TEST_DATABASE_URL` is set.
- New API integration-style tests use in-memory SQLite and global model DB state in `new-api/controller/token_test.go` and `new-api/service/task_billing_test.go`.

**E2E Tests:**
- Playwright E2E tests are used for the mainline admin and marketplace workflows in `src/web/e2e/admin-marketplace.spec.ts`.
- LobeHub CLI E2E tests are under `lobehub/apps/cli/e2e/*.e2e.test.ts`.

## Common Patterns

**Async Testing:**
```typescript
render(<RouterProvider future={routerFuture} router={router} />);

expect(await screen.findByText('Workspace')).toBeInTheDocument();
expect(await screen.findByText('Conversations')).toBeInTheDocument();
```

**Error Testing:**
```go
_, err := Load()
if err == nil {
	t.Fatal("expected error for missing database url")
}
```

```typescript
await expect(client.get('/api/v1/console/usage')).rejects.toBeInstanceOf(HttpError);
```

**Environment Testing:**
- Use `t.Setenv` for backend config tests in `src/server/internal/config/config_test.go`.
- Reset process env in `beforeEach` for LobeHub package tests such as `lobehub/packages/ssrf-safe-fetch/index.test.ts`.

**Console Guarding:**
- Mainline Vitest setup in `src/web/src/test/setup.ts` spies on `console.warn` and `console.error` before each test and fails after each test when unexpected calls occur.

---

*Testing analysis: 2026-05-12*
