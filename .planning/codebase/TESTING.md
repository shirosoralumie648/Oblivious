---
last_mapped_commit: f4dc5e48826c9893706249151aa081638e295dc1
---

# Testing Patterns

**Analysis Date:** 2026-04-28

## Test Frameworks

### Frontend (TypeScript/React)

**Runner:** Vitest 2.1.4
**Config:** `src/web/vite.config.ts` (in `test` key)

```typescript
test: {
  environment: 'jsdom',
  globals: true,
  setupFiles: ['./src/test/setup.ts']
}
```

**Component Testing:** @testing-library/react 16.1.0 + @testing-library/jest-dom 6.6.3
**Setup File:** `src/web/src/test/setup.ts` — imports jest-dom matchers, spies on console.warn/console.error

**Run Commands:**
```bash
pnpm --dir src/web test           # Vitest run (one-shot)
bash scripts/test.sh              # Root-level wrapper (runs both web and server tests)
```

### Backend (Go)

**Runner:** Go standard `testing` package (no external test framework)
**No testify, ginkgo, or other assertion libraries** — plain `t.Fatal`, `t.Fatalf`, `t.Errorf`

**Run Commands:**
```bash
cd src/server && go test ./...                         # Run all tests
bash scripts/test.sh                                   # Root-level wrapper
```

## Test File Organization

### Location: Co-located

**Frontend:**
```
src/web/src/
├── features/auth/
│   ├── store.ts
│   ├── store.test.ts          # Co-located test
│   ├── useAuthBootstrap.ts
│   └── useAuthBootstrap.test.ts
├── services/http/
│   ├── client.ts
│   └── client.test.ts         # Co-located test
├── routes/console/
│   ├── ConsoleHomePage.tsx
│   └── ConsoleHomePage.test.tsx  # Co-located test
```

**Backend:**
```
src/server/internal/
├── chat/
│   ├── service.go
│   └── service_test.go        # Co-located test
├── relay/
│   ├── router.go
│   └── router_test.go         # Co-located test
├── config/
│   ├── config.go
│   └── config_test.go         # Co-located test
```

### Naming Conventions

| Type | Pattern | Examples |
|------|---------|----------|
| Unit test (web) | `<Name>.test.ts` or `<Name>.test.tsx` | `store.test.ts`, `ConsoleHomePage.test.tsx` |
| Behavior test (web) | `<Name>.behavior.test.tsx` | `ChatPage.behavior.test.tsx` |
| Unit test (go) | `<name>_test.go` | `service_test.go`, `config_test.go` |

### Test Counts

- Frontend: 20 test files
- Backend: 30 test files

## Test Structure

### Frontend (Vitest)

**Suite Organization:**
```typescript
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';

describe('auth store', () => {
  it('tracks loading and unauthenticated flows', () => {
    const store = createAuthStore();
    expect(store.getState()).toEqual({ status: 'idle', user: null, preferences: null });
    store.startLoading();
    expect(store.getState().status).toBe('loading');
  });

  it('notifies subscribers when the state changes', () => {
    const store = createAuthStore();
    const listener = vi.fn();
    store.subscribe(listener);
    store.startLoading();
    expect(listener).toHaveBeenCalledTimes(1);
  });
});
```

**Patterns:**
- `describe` blocks group related tests by component/module name
- `it` blocks describe behavior in present tense: "renders...", "throws...", "notifies..."
- Tests are isolated — each `it` creates its own instances
- `beforeEach`/`afterEach` used for mock cleanup
- Static mock data defined within test blocks, not in shared fixtures

### Backend (Go)

**Suite Organization:**
```go
func TestRouter_SelectsHealthyChannel(t *testing.T) {
    pool := NewChannelPool()
    healthyCh := &types.Channel{ID: "healthy", BaseURL: "http://healthy", Enabled: true}
    pool.AddChannel(healthyCh, 1)

    router := NewRouter(pool, lb, cbs, tb, hc)
    ch := router.SelectChannel(context.Background(), "chat")

    if ch == nil {
        t.Fatal("should return a channel")
    }
}
```

**Patterns:**
- Single test functions per scenario (no sub-tests with `t.Run`)
- Setup done inline at the top of each test function
- `t.Fatal`/`t.Fatalf` for hard failures, `t.Errorf` for soft failures
- Test names: `Test<Type>_<Scenario>` or `Test<Function>`
- No shared setup/teardown helpers — each test is self-contained
- `t.Setenv()` used for environment-dependent config tests (`src/server/internal/config/config_test.go`)

### Console Spy Pattern (Frontend)

The test setup in `src/web/src/test/setup.ts` spies on `console.warn` and `console.error` in `beforeEach`, then checks for unexpected calls in `afterEach`. Any unexpected console warning or error causes the test to fail:
```typescript
beforeEach(() => {
  consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation((...args) => {
    recordUnexpectedConsole('warn', args);
  });
});
afterEach(() => {
  if (recordedCalls.length) {
    throwUnexpectedConsoleCalls(recordedCalls);
  }
});
```

## Mocking

### Frontend: vi.mock and vi.fn()

**Module-level mocking (vi.mock hoisted):**
```typescript
const getAccess = vi.fn();
const getBilling = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess,
    getBilling,
    getModels,
    getUsage
  })
}));

// Then reset between tests:
afterEach(() => {
  getAccess.mockReset();
  getBilling.mockReset();
});
```

**vi.hoisted for module-scoped mock state:**
```typescript
const routeState = vi.hoisted(() => ({
  conversationId: undefined as string | undefined
}));
```

**Partial mocking of react-router-dom:**
```typescript
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return {
    ...actual,
    useNavigate: () => navigate,
    useParams: () => ({ conversationId: routeState.conversationId })
  };
});
```

**Mocking app context:**
```typescript
vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));
```

**Function spies (vi.fn):**
```typescript
const fetchFn = vi.fn(async () =>
  new Response(JSON.stringify({ ok: true, data: { requests: 3 }, error: null }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' }
  })
);
```

**What to Mock:**
- API modules (chat, console, tasks, knowledge APIs)
- External modules (react-router-dom)
- Context providers (app/providers)
- HTTP fetch function
- Console output (automatic via setup.ts)

**What NOT to Mock:**
- Components under test (rendered for real)
- React core functionality
- Assertion matchers

### Backend: Inline Fake Structs

Go uses hand-written fake structs defined directly in test files:

```go
// Fake store implementing the Store interface
type fakeStore struct {
    config   ConversationConfig
    messages []Message
}

func (f fakeStore) CreateConversation(ctx context.Context, ...) (Conversation, error) {
    return Conversation{}, nil
}

func (f fakeStore) GetConversationConfig(ctx context.Context, ...) (ConversationConfig, error) {
    return f.config, nil
}

// Fake generator implementing ReplyGenerator interface
type fakeGenerator struct {
    reply string
}

func (f fakeGenerator) GenerateReply(ctx context.Context, ...) (string, error) {
    return f.reply, nil
}
```

**Recording fakes** capture state for assertions:
```go
type recordingStore struct {
    lastModelID          string
    lastTemperature      float64
    messages             []Message
}
```

**What to Mock:**
- Store interfaces (database layer)
- External service interfaces (ReplyGenerator, UsageRecorder, Embedder)
- HTTP servers (`httptest.NewServer`) for relay tests

**What NOT to Mock:**
- Standard library
- Config loading (tested via `t.Setenv`)

## Fixtures and Factories

**No shared fixture files or test data directories exist in either frontend or backend.**

Test data is:
- Defined inline within each test function
- Often uses literal values: `{ id: 'u1', email: 'user@example.com' }`
- For component tests, mock API responses provide complete fixture data inline

**Frontend example:**
```typescript
getAccess.mockResolvedValue({
  defaultMode: 'solo',
  modelStrategy: 'balanced',
  networkEnabledHint: true,
  onboardingCompleted: true,
  sessionExpiresAt: '2026-04-03T00:00:00Z',
  sessionId: 'session_1',
  userEmail: 'user@example.com',
  userId: 'user_1',
  workspaceId: 'workspace_1'
});
```

**No factory functions, no `__fixtures__` directories, no shared test data files.**

## Test Types

### Unit Tests (Frontend)

- **Scope:** Individual modules, stores, services
- **Approach:** London School (mock-first) — dependencies are mocked, only the unit under test is real
- **Coverage:** Each source module with logic has a corresponding test file
- **Examples:** `store.test.ts`, `client.test.ts`, `useAuthBootstrap.test.ts`

### Component Tests (Frontend)

- **Scope:** Page-level components rendered with mocked dependencies
- **Approach:** Render with `@testing-library/react`, assert against rendered output
- **Router wrapping:** Components that depend on router context are wrapped in `<MemoryRouter>`
  ```typescript
  render(
    <MemoryRouter future={routerFuture}>
      <ConsoleHomePage />
    </MemoryRouter>
  );
  ```
- **Assertions:** `screen.findByRole`, `screen.findByText`, `.toHaveAttribute`, `.toBeInTheDocument`, `.not.toBeInTheDocument`
- **Behavior tests:** `<Name>.behavior.test.tsx` for interaction flows (user input, navigation, state transitions)

### Unit Tests (Backend)

- **Scope:** Individual services, middleware, relay components
- **Approach:** Fakes injected via constructors, standard `go test` assertions
- **Coverage:** Nearly every internal package has a corresponding `*_test.go` file

### Integration Tests (Backend)

- **HTTP handler tests:** `*_handler_test.go` files in `src/server/internal/http/` test endpoints with real routing and fake services
- **Relay integration tests:** `src/server/internal/relay/*_test.go` — some use `httptest.NewServer` for HTTP-level integration

### E2E Tests

**Not used.** No Playwright, Cypress, or Selenium configuration detected.

## Coverage

**No explicit coverage requirements enforced.** Neither `vitest` nor `go test` is configured with coverage thresholds in CI.

**View Coverage:**
```bash
# Frontend
cd src/web && npx vitest --coverage

# Backend
cd src/server && go test -cover ./...
```

## Common Patterns

### Async Testing (Frontend)

```typescript
// Testing async component loading states
expect(await screen.findByText('Loading dashboard…')).toBeInTheDocument();
expect(await screen.findByText('Unable to load dashboard.')).toBeInTheDocument();

// Testing that promises reject
await expect(client.get('/api')).rejects.toBeInstanceOf(HttpError);
```

### Error Testing (Backend)

```go
// Testing that an error type is returned
var re *RouterError
if !errors.As(err, &re) {
    t.Fatalf("expected RouterError, got %T", err)
}
if re.Code != http.StatusServiceUnavailable {
    t.Fatalf("expected 503, got %d", re.Code)
}

// Testing that an error is expected
_, err := Load()
if err == nil {
    t.Fatal("expected error for missing database url")
}
```

### Component Error States (Frontend)

```typescript
// Test that component handles API failures gracefully
getModels.mockRejectedValue(new Error('network unavailable'));
render(<MemoryRouter future={routerFuture}><ConsoleHomePage /></MemoryRouter>);
expect(await screen.findByText('Top model unavailable')).toBeInTheDocument();
expect(screen.queryByText('Unable to load dashboard.')).not.toBeInTheDocument();
```

### Channel Testing (Backend)

```go
// Use httptest for upstream server simulation
ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
}))
defer ts.Close()
healthyCh.BaseURL = ts.URL
```

## Test Data Patterns

### User/Auth Test Data
```typescript
const user = { id: 'u1', email: 'user@example.com' };
const preferences: UserPreferences = {
  defaultMode: 'chat',
  modelStrategy: 'balanced',
  networkEnabledHint: false,
  onboardingCompleted: true
};
```

### Session Test Data (Go)
```go
auth.Session{
    WorkspaceID: "workspace_1",
    User: auth.User{
        ID: "user_1",
    },
}
```

---

*Testing analysis: 2026-04-28*
