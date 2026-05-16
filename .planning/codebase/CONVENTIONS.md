# Coding Conventions

**Analysis Date:** 2026-05-16

This codebase has two distinct conventions surfaces:
- **Frontend (TypeScript / React):** `src/web/` — Vite + React 18 + Vitest
- **Backend (Go):** `src/server/` — `net/http` + `database/sql`

Both are exercised by the shared `scripts/check.sh` / `scripts/test.sh` entry points and gated by `.github/workflows/ci.yml`.

## Naming Patterns

**Files (Frontend):**
- React components and route views: `PascalCase.tsx` — e.g. `src/web/src/routes/admin/AdminUsersPage.tsx`, `src/web/src/components/shared/ConfirmDialog.tsx`
- shadcn / `components/ui/` primitives: `kebab-case.tsx` (single-word lower-case files) — e.g. `src/web/src/components/ui/dropdown-menu.tsx`, `src/web/src/components/ui/sheet.tsx`
- Non-component TypeScript modules: `camelCase.ts` — e.g. `src/web/src/app/appContext.tsx`, `src/web/src/app/routerFuture.ts`, `src/web/src/features/auth/useAuthBootstrap.ts`
- Test files: co-located with the source under test using the suffix `.test.ts` / `.test.tsx` — e.g. `src/web/src/services/http/client.test.ts`
- Behavior-flavored tests use `.behavior.test.tsx` — e.g. `src/web/src/routes/workspace/ChatPage.behavior.test.tsx`
- Playwright E2E specs live in `src/web/e2e/` with the suffix `.spec.ts` — e.g. `src/web/e2e/admin-marketplace.spec.ts`
- Test fixtures live under `src/web/e2e/fixtures/` with `camelCase.ts` — e.g. `src/web/e2e/fixtures/adminMarketplace.ts`

**Files (Backend / Go):**
- Standard Go `snake_case.go` — e.g. `src/server/internal/admin/channel_service.go`, `src/server/internal/http/auth_middleware_test.go`
- Tests live next to the source with the `_test.go` suffix (Go convention) — e.g. `src/server/internal/agent/service_test.go`
- Package directories use single lower-case words — e.g. `internal/admin`, `internal/relay/channel`

**Functions:**
- TypeScript: `camelCase` (e.g. `createHttpClient`, `unwrapEnvelope`, `createAuthStore`, `useAppContext`, `formatConsoleArgs`)
- React hooks: `useXxx` (e.g. `useAuthBootstrap`, `useAppContext`)
- Factory functions: `createXxx` (e.g. `createHttpClient`, `createAuthStore`, `createAdminApi`, `createAuthBootstrapController`)
- React components: `PascalCase` (e.g. `AdminUsersPage`, `ProtectedRoute`, `ConfirmDialog`)
- Go: `PascalCase` for exported, `camelCase` for unexported (e.g. `NewService`, `Load`, `testConfig`, `testDatabase`)

**Variables:**
- TypeScript: `camelCase` — `consoleWarnSpy`, `unexpectedConsoleCalls`, `routeState`
- Go: `camelCase` for locals (`databaseURL`, `originsRaw`, `recorder`, `request`); `PascalCase` for exported package-level names
- React `useState`-style pairs: `[value, setValue]` (not enforced but consistent in admin pages)

**Types:**
- TypeScript: `PascalCase` for `type` aliases and interfaces — `HttpClient`, `HttpClientOptions`, `ApiUser`, `ApiEnvelope<T>`, `AuthState`, `AuthStatus`, `UserPreferences`
- Prefer `type X = { ... }` over `interface X { ... }`. All public shapes in `src/web/src/types/api.ts` and `src/web/src/types/admin.ts` use `type` aliases.
- Go: `PascalCase` structs (`Config`, `Service`, `Agent`, `MemorySearcher`)
- API-facing TypeScript types are prefixed with `Api` (e.g. `ApiUser`, `ApiSession`, `ApiWorkspace`, `ApiEnvelope`, `ApiEnvelopeError`) — see `src/web/src/types/api.ts`

**Constants / IDs in fixtures:**
- Snake-case prefixed identifiers reflecting backend ID space: `user_admin`, `session_admin`, `channel_openai_primary`, `route_primary`, `agent_release_helper`, `kb_1`, `ch_1`, `rt_1` — see `src/web/e2e/fixtures/adminMarketplace.ts`

## Code Style

**Formatting:**
- No project-level Prettier, Biome, ESLint, or `.editorconfig` is committed (verified by `find` on repo root and `src/web/`). Style is enforced by convention, by the TypeScript compiler (`tsc --noEmit` in `pnpm --dir src/web build`), and by `go test ./...`.
- Indentation: 2 spaces in TypeScript / TSX; tabs in Go (Go default).
- String quotes: single quotes in TypeScript (`'./types/api'`, `'authenticated'`); backticks for templates.
- Trailing commas: present in object/array literals across `src/web/src/types/`, `src/web/src/features/admin/api.test.ts`, etc.
- Semicolons: required (all `.ts` / `.tsx` source files terminate statements with `;`).
- Line width: not enforced; most files stay <120 chars.

**Linting:**
- TypeScript strict mode is the linter: `src/web/tsconfig.json` sets `"strict": true`, `"noEmit": true`, `"isolatedModules": true`, `"useDefineForClassFields": true`, `"jsx": "react-jsx"`, `"moduleResolution": "Bundler"`, `"types": ["vitest/globals"]`.
- Path alias: `@/*` → `./src/*` (`src/web/tsconfig.json` and `src/web/vite.config.ts`).
- Build (`pnpm --dir src/web build`) runs `tsc --noEmit && vite build` — typecheck failures break CI via `bash scripts/check.sh web` in `.github/workflows/ci.yml`.
- Go: no `golangci-lint` config committed; correctness is gated by `go test ./... -count=1` invoked from `scripts/check.sh server` and `scripts/test.sh server`.

## Import Organization

**Order (observed in TypeScript):**
1. External packages (`react`, `react-router-dom`, `@testing-library/react`, `vitest`, `@playwright/test`)
2. Blank line
3. Internal aliases / relative imports (`../../types/api`, `./errors`, `./envelope`, `../routes/...`)

**`import type` usage:**
- Type-only imports use `import type { ... }` consistently — e.g. `import type { ApiEnvelope } from '../../types/api';` in `src/web/src/services/http/envelope.ts`, `import type { Page, Route } from '@playwright/test';` in `src/web/e2e/fixtures/adminMarketplace.ts`.

**Path Aliases:**
- `@/*` → `src/web/src/*` (declared in both `tsconfig.json` and `vite.config.ts`). In practice, source files usually use relative paths (`../../types/api`); the alias is reserved for shadcn-generated UI primitives under `src/web/src/components/ui/`.

**Imports (Go):**
- Standard library first, blank line, then external (`github.com/lib/pq`, `golang.org/x/crypto/bcrypt`), blank line, then internal (`oblivious/server/internal/...`). See `src/server/internal/http/server_test.go`.

## API Typing Patterns

**Envelope contract:**
- All backend responses follow the `ApiEnvelope<T>` shape from `src/web/src/types/api.ts`:
  ```ts
  type ApiEnvelope<T> = {
    ok: boolean;
    data: T | null;
    error: { code: string; message: string } | null;
  };
  ```
- Unwrapping is centralized in `src/web/src/services/http/envelope.ts` (`unwrapEnvelope<T>`) and used by the only HTTP client factory (`src/web/src/services/http/client.ts`).
- 204 responses return `undefined as T`; envelope `data: null` also returns `undefined`.

**HTTP client:**
- Single factory `createHttpClient(options: HttpClientOptions): HttpClient` in `src/web/src/services/http/client.ts` exposes `get<T> / post<T> / put<T> / delete<T>`.
- Always accepts `Accept: application/json`; only sets `Content-Type: application/json` when a body is present.
- `fetchFn` is injectable for tests — every `*.test.ts` against this client passes a `vi.fn()` cast to `typeof fetch`.

**Feature API modules:**
- Each feature defines a `createXxxApi(client: HttpClient)` factory returning a typed object — e.g. `createAdminApi` in `src/web/src/features/admin/api.ts`, `createChatApi`, `createTasksApi`, `createAuthApi`. This makes mocking trivial in component tests (see `src/web/src/routes/admin/AdminUsersPage.test.tsx`).
- Server collection responses use varied keys (`channels`, `routes`, `entries`, `plans`, `data`); the API layer normalizes them — see the `ChannelListPayload` / `RouteListPayload` shapes in `src/web/src/features/admin/api.ts` and the normalization test in `src/web/src/features/admin/api.test.ts`.

## Error Handling

**Frontend:**
- Single error class: `HttpError` in `src/web/src/services/http/errors.ts` — carries `status: number` and message.
- `src/web/src/services/http/client.ts` decodes JSON error bodies, prefers `payload.error.message` from the envelope, falls back to `response.statusText`. Non-JSON bodies are swallowed silently with a comment ("Keep the default message when the error body is not JSON.").
- `unwrapEnvelope` throws `new HttpError(500, payload.error?.message ?? 'HTTP request failed')` when `ok: false`.
- Streaming: `src/web/src/services/http/stream.ts` throws `new Error('Unable to open stream')` for missing response body.
- Plain `throw new Error(...)` is rare; the only non-`HttpError` plain throws in production code live in `src/web/src/routes/workspace/MarketplacePage.tsx:138` and `src/web/src/services/http/stream.ts:9`.

**Go (`src/server/`):**
- Use `fmt.Errorf` with `%w` to wrap when forwarding store errors — e.g. `src/server/internal/admin/audit_store.go:24`: `return fmt.Errorf("generate audit id: %w", err)`.
- Use plain `fmt.Errorf("…")` for validation (no wrapping). Example: `src/server/internal/admin/channel_service.go:29` `return nil, fmt.Errorf("channel id is required")`.
- Error messages are lower-case, no trailing punctuation (idiomatic Go) — e.g. `"channel name is required"`, `"action must be 'enable' or 'disable'"`.
- Sentinel errors via `errors.New` are present but rare; prefer `fmt.Errorf`.

## Logging

**Frontend:**
- No application logging framework — `console.warn` / `console.error` are **forbidden** in tests by `src/web/src/test/setup.ts` (any unexpected call throws and fails the test).
- Production source must not call `console.log`/`warn`/`error`. A repo scan confirms: zero `console.*` calls in `src/web/src/**/*.ts` / `.tsx` outside tests.
- User-visible feedback uses `sonner` toasts (`src/web/src/components/ui/sonner.tsx`).

**Backend (Go):**
- Standard library `log` package (`log.Printf`, `log.Println`) — no structured logger.
- Format: space-separated `key=value` pairs — e.g. `src/server/internal/http/middleware.go:74` `log.Printf("method=%s path=%s status=%d duration=%s request_id=%s", ...)`.
- Warning prefix `"warning: ..."` for non-fatal startup issues — e.g. `src/server/internal/http/server.go:29`.
- Component prefix in brackets for subsystems — e.g. `log.Println("[ws] hub initialized")` in `src/server/internal/ws/hub.go:199`.
- Domain-specific prefixes: `billing timeout: ...`, `billing polling: ...` (see `src/server/internal/relay/billing_worker.go`).

## Comments

**TypeScript:**
- Comments are sparse and surgical — used to explain non-obvious control flow (e.g. the swallowed JSON parse failure in `src/web/src/services/http/client.ts:42` "Keep the default message when the error body is not JSON.").
- No JSDoc/TSDoc usage observed in `src/web/src/`.
- Avoid comments for self-evident code.

**Go:**
- Exported identifiers get a single-line doc comment beginning with the identifier name (Go convention) — see `src/server/internal/agent/service.go` (`// Service Agent 服务`, `// NewService 创建 Service`).
- Comments are bilingual in places (Chinese descriptions of intent), but the code itself is English-only.

## Function Design

**Size:** Most functions stay well under 50 lines. Service constructors (e.g. `NewServiceWithMCP`, `NewServiceWithMemory` in `src/server/internal/agent/service.go`) are intentionally tiny — they delegate to `initRunner`.

**Parameters:**
- Frontend: configuration via a single optional options object (e.g. `createHttpClient(options: HttpClientOptions = {})`) rather than positional booleans.
- Go: explicit `context.Context` is the first parameter on every method that crosses I/O — `func (s *Service) CreateAgent(ctx context.Context, session auth.Session, req *CreateAgentRequest) (*Agent, error)`.

**Return Values:**
- TypeScript: prefer narrow generics on the call site (`client.get<{ requests: number }>('/api/v1/console/usage')`).
- Go: idiomatic `(value, error)` returns; never `panic` outside test fakes — `panic("not used")` in `fakeStore` is a deliberate "you wired the wrong fake" marker (see `src/server/internal/agent/service_test.go`).

## Module Design

**Exports (TypeScript):**
- Named exports only. No `default export` observed in `src/web/src/` except shadcn-generated files.
- One factory per file in `services/http/` and `features/*/api.ts`.

**Barrel Files:**
- Not used. All imports reach the implementation file directly — keeps Vite's dev graph small and aids tree-shaking.

**State management (frontend):**
- Custom factory stores using closures + `Set<Listener>` (`createAuthStore` in `src/web/src/features/auth/store.ts`); no Redux/Zustand/Jotai.
- React context for app-level wiring (`src/web/src/app/appContext.tsx`, consumed via `useAppContext`).

**Routing:**
- Centralized in `src/web/src/app/router.tsx`. Both `createBrowserRouter` and `createMemoryRouter` are exported so router tests in `src/web/src/app/router.test.tsx` can drive the same tree in-memory.

---

*Convention analysis: 2026-05-16*
