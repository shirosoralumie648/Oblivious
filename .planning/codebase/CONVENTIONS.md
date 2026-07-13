# Coding Conventions

**Analysis Date:** 2026-07-13

## Operating Principle

Oblivious is organized around a Go backend, React/Vite frontend, release-evidence scripts, and strict planning/docs artifacts. New code should preserve tenant isolation, Relay-centered AI/provider flow, explicit verification boundaries, and repo-local versus target/live evidence separation.

## Go Backend Conventions

### Package placement

- Put HTTP route wiring and handlers under `src/server/internal/http/`; use focused handler files rather than expanding `src/server/internal/http/router.go` unless adding route registration.
- Put domain services under `src/server/internal/<domain>/`, following existing domains such as `src/server/internal/agent/`, `src/server/internal/workflow/`, `src/server/internal/knowledge/`, `src/server/internal/marketplace/`, `src/server/internal/quota/`, and `src/server/internal/relay/`.
- Put gRPC runtime adapters under `src/server/internal/grpc/<service>v1/` or package-level adapters under `src/server/pkg/<service>/`; keep generated protobuf output under `api/proto/` and `src/server/internal/grpc/*/*.pb.go`.
- Put persistence tests beside the store implementation, for example `src/server/internal/agent/store_test.go`, `src/server/internal/workflow/store.go`, and `src/server/internal/knowledge/store_test.go`.

### Service and store style

- Keep orchestration in service types and persistence in store types. Use files like `src/server/internal/agent/service.go`, `src/server/internal/workflow/service.go`, and `src/server/internal/marketplace/settlement.go` as examples of domain ownership.
- Prefer explicit constructor dependencies over global state. When behavior touches billing, quota, Relay, or tenants, pass the required service/store dependency rather than looking it up indirectly.
- Preserve table-driven tests with `t.Run` in Go packages; most backend coverage follows `*_test.go` files under the same package tree.
- Use `context.Context` through service and store boundaries, especially for SQL, Relay, workflow, and agent execution paths.

### Error and response handling

- Keep HTTP response envelopes and error contracts consistent with existing handlers under `src/server/internal/http/`.
- Add OpenAPI and route-surface updates when changing public API behavior. Contract sources live in `docs/api/openapi.yaml`, `docs/api/route-surface-manifest.json`, and `scripts/verify_openapi_contract.py`.
- Prefer narrow handler tests in `src/server/internal/http/*_test.go` for route behavior, auth, tenant scoping, query serialization, and response shape.

## Tenant, Auth, And Commercial Boundaries

- Treat active organization / tenant scope as required for customer data. Existing coverage includes tenant-oriented profile names in `scripts/verify-commercial-db-evidence.sh`.
- Use session/auth boundaries from `src/server/internal/auth/` and `src/server/internal/http/auth_middleware_test.go` rather than ad hoc user IDs.
- For tenant-sensitive changes, add tests against the relevant store and HTTP route. Examples include `src/server/internal/tenant/`, `src/server/internal/quota/`, `src/server/internal/http/server_test.go`, and `src/server/internal/http/commercial_journey_test.go`.
- Do not close tenant-isolation work with frontend fixture tests only.

## Relay And Provider Conventions

- All AI/provider execution should route through Relay or an explicit policy boundary. Relay code is concentrated under `src/server/internal/relay/`.
- Outbound URL/tool behavior belongs behind `src/server/internal/outboundpolicy/`, `src/server/internal/mcp/websearch/`, or `src/server/internal/agent/tools/` rather than scattered handler code.
- Provider and payment behavior should use existing provider-specific packages such as `src/server/internal/stripe/`, `src/server/internal/payment/`, `src/server/internal/billing/`, and `src/server/internal/marketplace/`.
- When adding provider behavior, include quota/billing/request-log assertions and update release evidence profiles where appropriate.

## TypeScript / React Conventions

### Frontend structure

- Keep route pages under `src/web/src/routes/`; workspace pages live under `src/web/src/routes/workspace/`, admin pages under `src/web/src/routes/admin/`, and marketing/auth pages under `src/web/src/routes/marketing/`.
- Keep API clients and domain types under `src/web/src/features/<domain>/`, for example `src/web/src/features/admin/api.ts`, `src/web/src/features/agents/agentsApi.ts`, and `src/web/src/features/workflows/workflowsApi.ts`.
- Keep shared app wiring under `src/web/src/app/`; router tests under `src/web/src/app/router.test.tsx` should not become the only proof for backend behavior.
- Use the `@` alias from `src/web/vite.config.ts` for frontend source imports when it improves clarity.

### Component and state style

- Prefer typed API client functions over inline `fetch` calls inside pages.
- Keep user-visible loading, empty, recovery, quota, budget, settlement, and review boundary copy in route components where users interact with it.
- Keep schema validation and form behavior close to the feature page or feature API; frontend dependencies include `zod`, `react-hook-form`, `@hookform/resolvers`, `swr`, `zustand`, `react-router-dom`, and `@xyflow/react`.
- Avoid expanding already-large pages such as `src/web/src/routes/workspace/WorkflowsPage.tsx` without extracting smaller helpers or feature-level API utilities.

## Release Evidence And Docs Conventions

- Put operator-facing release instructions under `docs/release/`; key files include `docs/release/rc-checklist.md`, `docs/release/commercial-completion-audit.md`, and `docs/release/release-rollback-runbook.md`.
- Put architecture contracts under `docs/architecture/`; `docs/architecture/current-system-contracts.md` is the high-level evidence map for runtime contracts.
- Put product-facing docs under `docs/product/`.
- Put verifier implementations under `scripts/` and keep shell wrappers fail-closed with `set -euo pipefail`.
- Keep target/live proof outside git. Use `config/.env.example` and `deploy/kubernetes/secret.example.yaml` as examples only.

## Environment And Tooling Conventions

- Use `scripts/check.sh` for quality checks and `scripts/test.sh` for test entrypoints. Both set `COREPACK_HOME`, `GOCACHE`, and `GOMODCACHE` defaults under `.tmp/`.
- Use `TEST_DATABASE_URL` for DB-backed Go tests. `scripts/test.sh` runs unit tests and explicitly skips integration tests when the variable is absent unless `OBLIVIOUS_REQUIRE_TEST_DATABASE=true`.
- Use `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` when a system Chrome/Chromium install is preferred for browser E2E.
- Keep package-manager updates synchronized across `pnpm-lock.yaml`, root `package-lock.json`, and `src/web/package-lock.json` when the dependency security gate inspects them.

## Naming And Placement Rules

- Go packages use lowercase directory names under `src/server/internal/`; exported symbols should be reserved for cross-package use.
- HTTP route tests should name the route behavior explicitly, as seen in `src/server/internal/http/*_test.go`.
- Frontend test files should sit beside the page/feature when unit-level, or under `src/web/e2e/` when browser-level.
- Release scripts should use verb-object names such as `verify-*.sh`, `collect-*-evidence.sh`, `assemble-*.sh`, and keep paired fixture scripts with the `-fixtures.sh` suffix.

## Anti-Patterns

- Do not bypass Relay for new AI/provider calls.
- Do not treat `reference/` code as product implementation without explicit reference-analysis scope.
- Do not read or commit real `.env`, provider keys, Kubernetes secrets, target manifests, backup dumps, or downloaded target artifacts.
- Do not count local fixture scripts as target/live evidence.
- Do not add frontend-only fixture proof for backend persistence, billing, provider rails, tenant isolation, or release readiness.

---

*Convention analysis: 2026-07-13*
