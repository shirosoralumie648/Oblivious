# Coding Conventions

**Analysis Date:** 2026-05-12

## Naming Patterns

**Files:**
- Use Go package/domain filenames under `src/server/internal/<domain>/`, with implementation files such as `src/server/internal/chat/service.go`, persistence files such as `src/server/internal/chat/store.go`, and co-located tests such as `src/server/internal/chat/service_test.go`.
- Use Go command entrypoints under `src/server/cmd/<command>/main.go`, including `src/server/cmd/server/main.go` and `src/server/cmd/migrate/main.go`.
- Use React route components as PascalCase `.tsx` files under `src/web/src/routes/`, such as `src/web/src/routes/workspace/ChatPage.tsx`, with tests beside them as `src/web/src/routes/workspace/ChatPage.test.tsx`.
- Use React feature modules under `src/web/src/features/<domain>/` with compact role names such as `src/web/src/features/auth/store.ts`, `src/web/src/features/auth/api.ts`, and `src/web/src/features/auth/useAuthBootstrap.ts`.
- Use shadcn-style lowercase UI component filenames under `src/web/src/components/ui/`, such as `src/web/src/components/ui/button.tsx` and `src/web/src/components/ui/dialog.tsx`.
- Use shared application components as PascalCase filenames under `src/web/src/components/shared/`, such as `src/web/src/components/shared/DataTable.tsx`.
- Keep E2E specs under `src/web/e2e/` with `.spec.ts` names, such as `src/web/e2e/admin-marketplace.spec.ts`.
- Treat `lobehub/` as an upstream pnpm workspace with package-local conventions, including `lobehub/packages/*/src/**`, `lobehub/apps/desktop/src/main/**/__tests__/*.test.ts`, and root config files such as `lobehub/eslint.config.mjs`.
- Treat `new-api/` as an upstream Go/React project with Go tests beside packages such as `new-api/service/error_test.go` and JS web code under `new-api/web/src/`.

**Functions:**
- Use PascalCase for exported Go constructors, methods, and types in `src/server/internal/chat/service.go`, such as `NewService`, `CreateConversation`, `ListMessages`, and `Conversation`.
- Use lowerCamelCase for unexported Go handlers and helpers in `src/server/internal/http/chat_handler.go`, such as `newChatHandler`, `sendMessage`, and `toMessageOverrides`.
- Use `New<Type>` for Go dependency constructors in `src/server/internal/chat/gateway.go` and `src/server/internal/relay/router.go`.
- Use `With...` and `...FromContext` for context helpers in `src/server/internal/chat/gateway.go`, such as `WithRelayRequestMetadata` and `relayRequestMetadataFromContext`.
- Use `create...` factory functions in TypeScript service modules, such as `createHttpClient` in `src/web/src/services/http/client.ts`, `createChatApi` in `src/web/src/features/chat/api.ts`, and `createAuthStore` in `src/web/src/features/auth/store.ts`.
- Use `use...` names for React hooks in `src/web/src/features/auth/useAuthBootstrap.ts` and `lobehub/src/hooks/useFetchTopics.test.ts`.
- Use PascalCase for React component functions in `src/web/src/routes/workspace/ChatPage.tsx` and `src/web/src/features/layouts/WorkspaceLayout.tsx`.

**Variables:**
- Use Go `ctx` for `context.Context` parameters and pass it first, as in `src/server/internal/chat/service.go` and `src/server/internal/task/service.go`.
- Preserve initialism casing in Go names: `ID`, `URL`, `API`, and `LLM` appear in `src/server/internal/config/config.go`, `src/server/internal/chat/gateway.go`, and `src/server/internal/relay/types/types.go`.
- Use explicit `recorder` and `request` names in HTTP tests with `httptest`, as in `src/server/internal/http/chat_handler_test.go` and `src/server/internal/http/server_test.go`.
- Use `is...`, `has...`, and `can...` boolean names in TypeScript and Go helpers, such as `hasEnabledTools` in `src/server/internal/agent/store_test.go` and `cancelled` in `src/web/src/routes/workspace/ChatPage.tsx`.
- Use `next...` names for derived React state before assignment, as in `nextConversations`, `nextMessages`, and `nextConversationConfig` in `src/web/src/routes/workspace/ChatPage.tsx`.

**Types:**
- Use PascalCase Go structs and interfaces with JSON tags for API contracts in `src/server/internal/chat/service.go`, `src/server/internal/http/response.go`, and `src/server/internal/relay/api_types.go`.
- Keep unexported Go request DTOs lowerCamelCase in handler packages, such as `sendMessageRequest` and `updateConversationConfigRequest` in `src/server/internal/http/chat_handler.go`.
- Use PascalCase TypeScript `type` aliases for app contracts, such as `HttpClient`, `HttpClientOptions`, `AuthState`, and `AuthStore` in `src/web/src/services/http/client.ts` and `src/web/src/features/auth/store.ts`.
- Use explicit union types for finite UI state, such as `AuthStatus` in `src/web/src/features/auth/store.ts`.
- In `lobehub/`, use exported interfaces and TSDoc for package API boundaries, as shown by `SSRFOptions` in `lobehub/packages/ssrf-safe-fetch/index.ts`.

## Code Style

**Formatting:**
- Format Go with `gofmt`; existing files under `src/server/internal/` use tab-indented Go formatting and grouped imports.
- Format mainline TypeScript with the style already present in `src/web/src/`: single quotes, semicolons, two-space indentation, and trailing commas only where TypeScript formatting already inserts them.
- Use strict TypeScript settings from `src/web/tsconfig.json`: `strict: true`, `jsx: react-jsx`, `moduleResolution: Bundler`, and `@/*` path mapping.
- Use LobeHub's shared formatter for `lobehub/`; `lobehub/prettier.config.mjs` exports `prettier` from `@lobehub/lint`.
- Use New API's web formatter for `new-api/web/`; `new-api/web/.prettierrc.mjs` extends `@so1ve/prettier-config` and `new-api/web/package.json` sets single quotes for JS/JSX.

**Linting:**
- Use `bash scripts/check.sh` as the root release check entrypoint; it validates docs, workspace boundaries, `src/web` build, and `src/server` tests from `scripts/check.sh`.
- Mainline `src/web/` does not define a standalone ESLint config; rely on `pnpm --dir src/web build` for TypeScript compile checks and `pnpm --dir src/web test` for behavior checks.
- Use LobeHub's root lint stack in `lobehub/eslint.config.mjs`, `lobehub/stylelint.config.mjs`, and `lobehub/package.json`; it enforces type-import style, MDX rules, Stylelint, circular import checks, and no-console rules outside allowed areas.
- Use New API's web lint stack in `new-api/web/.eslintrc.cjs`; it enforces AGPL header blocks and maximum one empty line for `new-api/web/**/*.js` and `new-api/web/**/*.jsx`.

## Import Organization

**Order:**
1. Go standard library imports first, as in `src/server/internal/http/chat_handler.go`.
2. Go third-party imports next, as in `src/server/internal/http/server_test.go`.
3. Go local module imports last, using `oblivious/server/...` in `src/server/internal/chat/service_test.go`.
4. TypeScript external imports first, then absolute aliases, then relative imports, as in `src/web/src/routes/workspace/ChatPage.tsx`.
5. Type-only imports use `import type` in TypeScript, as in `src/web/src/features/auth/store.ts` and `lobehub/packages/ssrf-safe-fetch/index.ts`.

**Path Aliases:**
- Use `@/*` for `src/web/src/*` imports in `src/web/vite.config.ts` and `src/web/tsconfig.json`.
- Use LobeHub aliases from `lobehub/vitest.config.mts`, including `@/`, `~test-utils`, and package-specific replacements.
- Use Go module paths `oblivious/server/...` for mainline backend imports from `src/server/go.mod`.
- Use Go module paths `github.com/QuantumNous/new-api/...` inside `new-api/`, from `new-api/go.mod`.

## Error Handling

**Patterns:**
- Return Go errors instead of panicking in production code under `src/server/internal/`; `src/server/internal/config/config.go` validates env values with `fmt.Errorf`.
- Convert HTTP handler failures into the shared envelope from `src/server/internal/http/response.go` using `writeError` with stable codes such as `invalid_request`, `unauthorized`, and `internal_error`.
- Preserve wrapped backend context with `%w` where stores return lower-level failures, as in `src/server/internal/userprefs/store.go`.
- In `src/web/src/services/http/client.ts`, throw `HttpError` for non-OK HTTP responses and unwrap successful backend envelopes with `unwrapEnvelope`.
- In React route code such as `src/web/src/routes/workspace/ChatPage.tsx`, keep async load failures local to the view and reset visible state rather than leaking raw exceptions to the UI.
- In `new-api/service/error.go`, use typed response wrappers and status mapping helpers such as `ResetStatusCode` for upstream-provider error normalization.
- In `lobehub/packages/ssrf-safe-fetch/index.ts`, rethrow errors with a descriptive message and `cause` after logging SSRF-specific failures.

## Logging

**Framework:** console/log package per subtree

**Patterns:**
- Mainline Go code uses minimal standard logging; `src/server/internal/ws/hub.go` logs hub initialization with `log.Println`.
- Mainline web tests fail unexpected `console.warn` and `console.error` calls through `src/web/src/test/setup.ts`; explicitly mock or suppress expected console output in the test that needs it.
- LobeHub's ESLint config in `lobehub/eslint.config.mjs` disables `no-console` only for `e2e/**/*`, `**/*.test.ts`, `**/*.test.tsx`, `packages/agent-tracing/**/*`, and `apps/cli/**/*`.
- New API uses its project logger in service code such as `new-api/service/task_billing.go` and `new-api/service/text_quota.go`; keep context-aware logger calls instead of ad hoc prints in that subtree.

## Comments

**When to Comment:**
- Prefer self-explanatory names in mainline Go and TypeScript; add comments only for non-obvious behavior such as the integration-test database setup in `src/server/internal/http/server_test.go`.
- Use short comments to describe security-sensitive or environment-sensitive behavior, as in `lobehub/packages/ssrf-safe-fetch/index.ts`.
- Keep generated or migration notes close to the code they explain, such as SQL migration filenames under `src/server/migrations/`.

**JSDoc/TSDoc:**
- Use TSDoc for exported package APIs in `lobehub/packages/*`, as shown by `SSRFOptions` and `ssrfSafeFetch` in `lobehub/packages/ssrf-safe-fetch/index.ts`.
- Mainline `src/web/src/` uses TypeScript types rather than broad JSDoc; prefer explicit exported types in `src/web/src/types/api.ts` and API modules.

## Function Design

**Size:** Keep service functions focused on one workflow step in `src/server/internal/<domain>/service.go`; move normalization and conversion helpers into private functions such as `normalizeKnowledgeBaseIDs` in `src/server/internal/chat/service.go`.

**Parameters:** Pass dependencies through constructors or interfaces, not globals, in mainline backend code. `chat.NewService` in `src/server/internal/chat/service.go` accepts a `Store`, `ReplyGenerator`, default model, and usage recorder.

**Return Values:** Return typed domain values plus `error` in Go, as in `src/server/internal/chat/service.go`. Return typed promises in TypeScript clients, as in `src/web/src/services/http/client.ts`.

## Module Design

**Exports:** Use named exports in mainline TypeScript modules, such as `createHttpClient` in `src/web/src/services/http/client.ts` and `createAuthStore` in `src/web/src/features/auth/store.ts`.

**Barrel Files:** Mainline `src/web/src/` does not use broad barrel files; import directly from the feature, service, or type module needed. LobeHub packages may expose package-level entrypoints such as `lobehub/packages/ssrf-safe-fetch/index.ts`.

---

*Convention analysis: 2026-05-12*
