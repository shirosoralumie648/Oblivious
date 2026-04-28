---
last_mapped_commit: f4dc5e48826c9893706249151aa081638e295dc1
---

# Coding Conventions

**Analysis Date:** 2026-04-28

## Language Split

The codebase has two distinct language domains with separate conventions:

| Layer | Language | Location |
|-------|----------|----------|
| Frontend (web) | TypeScript (ESNext modules) | `src/web/src/` |
| Backend (server) | Go 1.25 | `src/server/` |

## TypeScript / React (Frontend)

### Naming Patterns

**Files:**
- Components and routes: `PascalCase.tsx` (e.g., `ChatPage.tsx`, `ConsoleLayout.tsx`)
- Modules, services, stores, utilities: `camelCase.ts` (e.g., `client.ts`, `store.ts`, `routerFuture.ts`)
- Test files: `<Name>.test.ts` or `<Name>.test.tsx` — always co-located with source
- Behavior files: `<Name>.behavior.test.tsx` for interaction/integration-style component tests
- One exception: `appContext.tsx` uses camelCase despite containing a component

**Functions:**
- Factory pattern: `create<Name>()` returns an object implementing a typed interface
  ```typescript
  // Store pattern: createAuthStore()
  export function createAuthStore(initialState: AuthState): AuthStore { ... }

  // API pattern: createChatApi()
  export function createChatApi(client: HttpClient): ChatApi { ... }

  // HttpClient pattern: createHttpClient()
  export function createHttpClient(options?: HttpClientOptions): HttpClient { ... }
  ```
- No classes used — plain functions and objects
- All exported functions use named exports (no `export default`)

**Types/Interfaces:**
- PascalCase types (e.g., `AuthState`, `AppContextValue`, `HttpClient`, `ChatApi`)
- Types declared in the same file as their usage, not in separate type files
- Exceptions: `src/web/src/types/api.ts` contains shared API response types
- Union types for status enums: `type AuthStatus = 'idle' | 'loading' | 'authenticated' | 'unauthenticated'`
- `type` keyword preferred over `interface` for most declarations, though `interface` also appears

**Props:**
- Props types suffixed with `Props`: `AppProvidersProps`, `AppContextProviderProps`
- Inline props destructuring in function signatures:
  ```typescript
  export function AppProviders({ children }: AppProvidersProps) { ... }
  ```

### Code Style

**Formatting:**
- No ESLint, Prettier, Biome, or EditorConfig detected at repository level or in `src/web/`
- TypeScript strict mode enabled (`"strict": true` in `tsconfig.json`)
- Semicolons: used consistently at statement ends
- Single quotes for strings (no double quotes in import paths or JSX)
- Consistent 2-space indentation

**Linting:**
- No explicit lint tool configured
- Enforced at build time via `tsc --noEmit` as part of the build script
- TypeScript strict mode catches type errors at compile time

### Import Organization

**Order:**
1. React/core library imports (`'react'`, `'react-dom'`, `'react-router-dom'`)
2. Internal modules via relative paths (`'../../app/providers'`, `'../features/auth/store'`)
3. Type-only imports use `import type` syntax but may be mixed with value imports

**Pattern observed in `src/web/src/app/router.tsx`:**
```typescript
import { createBrowserRouter, createMemoryRouter, type RouteObject } from 'react-router-dom';
import { routerFuture } from './routerFuture';
import { ProtectedRoute } from '../features/auth/ProtectedRoute';
```

**No path aliases configured** — all imports use relative paths (no `@/` or `~/` prefix). The `tsconfig.json` has no `paths` configuration.

### Error Handling

**Frontend error pattern:**
- Custom `HttpError` class (`src/web/src/services/http/errors.ts`) with `status` and `message`
- Envelope response format: `{ ok, data, error }` unwrapped by `unwrapEnvelope()` (`src/web/src/services/http/envelope.ts`)
- Try/catch with silent catch for non-critical failures:
  ```typescript
  try {
    const session = await authApi.me();
    applySession(session);
  } catch {
    store.clearUser();
  }
  ```
- Error messages extracted from API responses when available, falling back to `statusText`

### Logging

- No structured logging framework detected in frontend
- `console.warn` and `console.error` treated as test failures via `src/web/src/test/setup.ts`
- The test setup spies on `console.warn` and `console.error`, and throws if any unexpected calls are detected during a test

### Component Design

**React v18 patterns:**
- Functional components with hooks only
- `useMemo` for expensive computations (`createAppRouter` in `src/web/src/app/App.tsx`)
- `useRef` for stable references across renders (store instances in `src/web/src/app/appContext.tsx`)
- `useSyncExternalStore` for external store subscriptions
- Context + Provider pattern for dependency injection
- `React.StrictMode` enabled on root

**Layout components:**
- Each layout renders `<Outlet />` for nested routes
- Layouts are thin wrappers — navigation and chrome only, minimal business logic

### State Management

**Pattern: external stores + React context**
- Stores follow the "external store" pattern compatible with `useSyncExternalStore`
- Factories return objects with `getState()` and `subscribe()` methods
- Example: `src/web/src/features/auth/store.ts`, `src/web/src/store/app.ts`
- Context providers wire stores to React tree (`src/web/src/app/appContext.tsx`)
- No global state library (no Redux, Zustand, etc.)

### Module Design

- **No barrel/index files** — each module is directly imported by name
- API modules export a typed interface and a factory function:
  ```typescript
  export type ChatApi = { ... };
  export function createChatApi(client: HttpClient): ChatApi { ... };
  ```
- Services follow the same pattern: type + factory (`createHttpClient`, `createAuthStore`)
- Named exports only — no default exports anywhere

## Go (Backend)

### Naming Patterns

**Files:**
- snake_case: `auth_middleware.go`, `relay_gateway.go`, `circuitbreaker.go`
- Test files: `<name>_test.go` co-located with source

**Packages:**
- Single lowercase word: `auth`, `chat`, `relay`, `config`, `http`, `ws`, `mcp`
- Domain-driven: each package corresponds to a bounded context
- Package `http` aliased as `stdhttp` to avoid collision with `net/http`:
  ```go
  import stdhttp "net/http"
  ```

**Types:**
- PascalCase exported types: `Service`, `Config`, `Session`, `Conversation`
- Acronyms in ALLCAPS: `SQLStore`, `HTTPReplyGenerator`, `CORSAllowedOrigins`, `LLMBaseURL`
- JSON tags always present on exported struct fields: `json:"createdAt"`

**Functions/Methods:**
- PascalCase for exported: `NewService`, `NewRouter`, `Register`, `CreateConversation`
- camelCase for unexported: `applyMiddleware`, `withRecover`, `writeJSON`, `writeError`, `combineHandlers`
- Constructor pattern: `New<Type>()` returns `*<Type>`:
  ```go
  func NewService(store Store) *Service { ... }
  func NewRouter(cfg config.Config, database *sql.DB) stdhttp.Handler { ... }
  func NewSQLStore(db *sql.DB) *SQLStore { ... }
  ```

**Variables:**
- camelCase for local and package-level variables
- Sentinel errors with `Err` prefix: `ErrInvalidCredentials`, `ErrNoAvailableChannel`, `ErrCircuitOpen`
- Error variables defined as package-level `var` blocks in `errors.go` files

### Code Style

**Formatting:**
- Standard `gofmt` formatting (Go standard)
- Line comments: `// description` (no block comments observed)
- Struct fields aligned with consistent indentation

**No golangci-lint or additional linter configuration detected at project level**

### Import Organization

**Standard Go convention observed:**
1. Standard library imports 
2. Third-party package imports
3. Internal project imports (`oblivious/server/internal/...`)

Example from `src/server/internal/http/server.go`:
```go
import (
    "database/sql"
    "fmt"
    "log"
    stdhttp "net/http"
    "time"

    "github.com/google/uuid"

    "oblivious/server/internal/config"
    "oblivious/server/internal/relay"
)
```

### Error Handling

**Patterns:**
- **Sentinel errors** for known conditions (`src/server/internal/relay/errors.go`, `src/server/internal/auth/service.go`):
  ```go
  var ErrInvalidCredentials = errors.New("invalid credentials")
  var ErrNoAvailableChannel = errors.New("relay: no available channel")
  ```
- **Custom error types** with `errors.As` support (`src/server/internal/relay/`):
  ```go
  var re *RouterError
  if !errors.As(err, &re) { ... }
  ```
- **Error wrapping** via `fmt.Errorf`:
  ```go
  return Config{}, fmt.Errorf("invalid SERVER_PORT: %q", portRaw)
  ```
- **Explicit error returns** — every function that can fail returns `(T, error)`
- **Log and continue** for non-fatal failures:
  ```go
  if err := ...; err != nil {
      log.Printf("warning: ...: %v", err)
  }
  ```

### Logging

- Standard library `log` package: `log.Printf`, `log.Fatalf`
- Structured logging format via key=value pairs in messages:
  ```go
  log.Printf("method=%s path=%s status=%d duration=%s request_id=%s", ...)
  ```
- Warning prefix for non-fatal errors: `log.Printf("warning: ...")`

### Dependency Injection

**Manual constructor injection:**
- Services accept `Store` interfaces as constructor parameters
- HTTP handlers accept service instances
- All wiring happens in `NewRouter()` (`src/server/internal/http/router.go`) — single composition root
- Example chain:
  ```go
  authStore := auth.NewSQLStore(database)
  authService := auth.NewService(authStore)
  authHandler := newAuthHandler(authService, authMiddleware, preferencesService)
  ```

### API Response Format

**Envelope pattern in `src/server/internal/http/response.go`:**
```go
type Envelope struct {
    OK    bool          `json:"ok"`
    Data  any           `json:"data"`
    Error *ErrorPayload `json:"error"`
}
```
- `writeSuccess(w, status, data)` wraps data in `{ ok: true, data: ..., error: null }`
- `writeError(w, status, code, message)` wraps errors in `{ ok: false, data: null, error: { code, message } }`

### Middleware Pattern

**Functional composition** (`src/server/internal/http/middleware.go`):
```go
type middleware func(http.Handler) http.Handler

func applyMiddleware(handler http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler { ... }
```
Individual middlewares are `with*` functions: `withRecover`, `withRequestID`, `withLogging`, `withCORS`

### Context Usage

- **Typed context keys** using string `contextKey` type:
  ```go
  type contextKey string
  const requestIDContextKey contextKey = "request-id"
  const sessionContextKey contextKey = "session"
  ```
- Context passed as first parameter to all methods that need it

### Channel / Option Pattern

**RelayGateway uses functional options** (`src/server/internal/chat/relay_gateway.go`):
```go
type RelayGatewayOption func(*RelayGateway)
func WithRelayURL(url string) RelayGatewayOption { ... }
func NewRelayGateway(opts ...RelayGatewayOption) *RelayGateway { ... }
```

## File Size Guidance

The CLAUDE.md project configuration specifies "Keep files under 500 lines." Current codebase has several files exceeding this:
- `src/server/internal/task/store.go` (732 lines)
- `src/server/internal/http/router.go` (726 lines)
- `src/web/src/routes/workspace/SoloPage.tsx` (719 lines)
- `src/server/internal/memory/service.go` (664 lines)

New code should target the 500-line guideline; existing files above this are known exceptions.

## Comments

**When to comment:**
- TODO comments for planned future work (used in relay handlers)
- No JSDoc/TSDoc comments observed in the codebase
- Comments are sparse — code is expected to be self-documenting through clear naming
- Go comments follow standard `//` convention, no `/* */` block comments

---

*Convention analysis: 2026-04-28*
