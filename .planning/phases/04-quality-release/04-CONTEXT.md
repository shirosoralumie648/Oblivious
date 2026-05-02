# Phase 4: 质量与发布 - Context

**Gathered:** 2026-05-02
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 4 delivers release-candidate readiness for the already shipped mainline: integration tests, E2E coverage, API/release documentation, and deploy validation for the current `src/server` + `src/web` stack.

This phase must not add new product capabilities. It should prove and document the existing Relay, Agent, Memory, Quota, Admin, and Marketplace surfaces well enough for a release candidate.

Requirements: TEST-01, TEST-02, DOC-01, DEPLOY-01
</domain>

<decisions>
## Implementation Decisions

### Test Matrix
- **D-01:** Treat `go test ./... -count=1` as the broad backend release gate when shared interfaces, router wiring, migrations, Relay, Admin, Marketplace, Memory, Agent, or Quota behavior is touched.
- **D-02:** Keep existing fast script entry points, but Phase 4 planning should close the mismatch where `scripts/check.sh server` and `scripts/test.sh server` only run a focused package subset.
- **D-03:** Integration tests should emphasize boundary behavior over unit duplication: Admin/Marketplace HTTP routing, role/session checks, Relay billing/quota lifecycle, Agent tool-loop persistence, Memory search isolation, and migration-backed store behavior.
- **D-04:** Database-backed tests must be explicit about `TEST_DATABASE_URL`; if no disposable database is available, the release checklist must record the skip reason rather than silently treating integration coverage as complete.

### Browser E2E Scope
- **D-05:** Use browser E2E to cover critical user workflows, not every component state. The minimum flows are Admin channel/route/plan/user/review navigation and Marketplace browse/search/detail/install/publish/my-agents.
- **D-06:** Prefer stable seeded or mocked test data for E2E. Do not depend on real provider API keys, live Stripe, or external LLM calls.
- **D-07:** E2E should verify route wiring, auth/admin protection, loading/error affordances, and successful action feedback. Visual regression is optional and should not block the initial Phase 4 plan.

### API Documentation And RC Checklist
- **D-08:** `docs/API.md` should become the canonical API index for the current release surface and must include Admin, Marketplace, Memory, MCP, Quota, Notification, Agent, Chat, Console, Auth, and `/v1/*` Relay surfaces that are actually routed.
- **D-09:** `docs/architecture/current-system-contracts.md` should stay as the mainline contract document. Update it only for verified current behavior, not intended future behavior.
- **D-10:** `docs/release/rc-checklist.md` should be the release gate checklist and must name exact commands, required env vars, optional skip conditions, known accepted debt, and the expected evidence artifact for each gate.

### Deployment Validation
- **D-11:** Deployment work should target the active Oblivious mainline, not the imported `lobehub/` or `new-api/` reference trees.
- **D-12:** Docker/Kubernetes config should be minimal but runnable: web, server, PostgreSQL, Redis where required, migrations, env examples, health check, and a documented smoke command.
- **D-13:** Secrets must stay as env var names or placeholders in docs/config. Do not include provider keys, database passwords, Stripe secrets, or session secrets in planning artifacts or committed config.
- **D-14:** Deployment validation should prove startup and health, then point to the same test/check scripts used by CI. Do not create a separate release path that diverges from `.github/workflows/ci.yml`.

### Agent's Discretion
- Exact E2E framework choice if it fits the existing Vite/React toolchain and can run headless in CI.
- Exact disposable database strategy for integration tests, as long as setup/teardown is documented and does not rely on a developer's default local database.
- Whether to split Phase 4 into separate plan files by test/docs/deploy or combine docs and deploy in one plan, as long as each requirement maps clearly.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone Scope
- `.planning/PROJECT.md` — v03.2 goal and active requirement scope.
- `.planning/REQUIREMENTS.md` — TEST-01, TEST-02, DOC-01, DEPLOY-01 definitions and traceability.
- `.planning/ROADMAP.md` — Phase 4 goal, tasks, success criteria, and backlog boundaries.
- `.planning/STATE.md` — current milestone state, accepted deferred items, and latest v03.1 verification evidence.
- `.planning/MILESTONES.md` — v03.1 shipped baseline and known deferred items.

### Prior Decisions
- `.planning/phases/03.1-admin-marketplace-ui/03.1-CONTEXT.md` — Admin/Marketplace UI scope, API client patterns, route/page expectations.
- `.planning/phases/03-admin-marketplace/03-CONTEXT.md` — Admin and Marketplace backend decisions, RBAC, audit, review, and marketplace discovery scope.
- `.planning/phases/02-agent-memory-enhancement/02-CONTEXT.md` — Agent, Memory, and Quota/Relay decisions that Phase 4 tests must protect.

### Codebase Maps
- `.planning/codebase/TESTING.md` — test entry points, current coverage shape, and verification rules.
- `.planning/codebase/STRUCTURE.md` — active backend/frontend layout and where tests/docs/deploy work should connect.
- `.planning/codebase/INTEGRATIONS.md` — Relay, Memory, MCP, PostgreSQL, Auth, Admin, Marketplace, Quota, Metrics, and WebSocket integration points.

### Release And Documentation
- `docs/API.md` — current API documentation draft to update and reconcile with live routes.
- `docs/architecture/current-system-contracts.md` — mainline contract baseline for HTTP envelope, auth, routes, and workspace boundaries.
- `docs/release/rc-checklist.md` — release-candidate checklist to strengthen.
- `config/.env.example` — env var contract for docs and deployment examples.

### Verification And CI
- `scripts/check.sh` — current docs/web/server check entry point.
- `scripts/test.sh` — current web/server test entry point and `TEST_DATABASE_URL` integration-test gate.
- `scripts/verify-quality-gates.sh` — existing quality gate asset assertions.
- `.github/workflows/ci.yml` — CI jobs that Phase 4 should align with.
- `package.json` — root script contract.
- `src/web/package.json` — frontend build/test script contract.

### Live Route And UI Wiring
- `src/server/internal/http/router.go` — current `/api/v1/*`, `/api/v1/admin/*`, `/api/v1/marketplace/*`, `/api/v1/ws`, and service wiring.
- `src/server/internal/relay/handler/router.go` — OpenAI-compatible `/v1/*` relay surface.
- `src/web/src/app/router.tsx` — current Admin and Marketplace browser routes.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `scripts/check.sh`, `scripts/test.sh`, and `scripts/verify-quality-gates.sh` already provide a release-gate skeleton.
- `.github/workflows/ci.yml` already separates docs/release gates, web build/test, and server checks/tests.
- Backend tests are co-located `*_test.go` using standard Go `testing`, `httptest`, fake stores, and table tests.
- Frontend tests use Vitest + Testing Library with `src/web/src/test/setup.ts`.
- `src/server/internal/http/admin_marketplace_handler_test.go` and the Admin/Marketplace frontend tests provide a concrete v03.1 regression baseline.

### Established Patterns
- Keep active work inside `src/server`, `src/web`, `scripts`, `docs`, `config`, and `.github`; imported `lobehub/` and `new-api/` are reference trees.
- Backend behavior follows service/store/handler separation with route registration in `src/server/internal/http/router.go`.
- Frontend API clients use `create<Name>Api` factories and route/page tests are co-located with pages.
- Docs checks assert env/docs/script consistency through `scripts/check.sh docs`.

### Integration Points
- Server integration tests can be gated by `TEST_DATABASE_URL` and should make setup/teardown explicit.
- E2E tests should connect through the active React router and API client surface, not the obsolete workspace Marketplace page noted in backlog 999.2.
- API documentation must be reconciled against `src/server/internal/http/router.go` and `src/server/internal/relay/handler/router.go`.
- Deployment config must align with `config/.env.example`, `cmd/server`, `cmd/migrate`, `/healthz`, `/metrics`, PostgreSQL migrations, and the CI command path.
</code_context>

<specifics>
## Specific Ideas

- Phase 4 should start by making the release gate explicit, then fill test/docs/deploy gaps against that gate.
- Treat v03.1 accepted cleanup debt as documented known debt unless directly needed to keep E2E route coverage coherent.
- Avoid broad cleanup of the existing dirty worktree; plans should name precise file scopes and verification commands.
</specifics>

<deferred>
## Deferred Ideas

- Full production observability and alerting beyond the existing `/metrics` surface belongs after this release-readiness milestone.
- Real payment, revenue share, and Stripe production hardening remain out of scope unless a release gate exposes a blocking contract mismatch.
- Mobile-specific release testing remains out of scope for the web-first RC.
</deferred>

---

*Phase: 04-quality-release*
*Context gathered: 2026-05-02*
