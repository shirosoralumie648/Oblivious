# Codebase Concerns

**Analysis Date:** 2026-05-16

## 0. Overview

This audit synthesizes signal from TODO/marker scans (`docs/reports/2026-04-04-*` and `2026-04-05-*`), the archaeology report (`ARCHAEOLOGY_REPORT.md`), current status (`CURRENT_STATUS.md`), architecture decisions (`docs/architecture/knowledge-evolution-decision.md`, `docs/architecture/solo-runtime-decision.md`), release prep (`docs/release/*`), planning state (`.planning/STATE.md`, `.planning/ROADMAP.md`, `.planning/RETROSPECTIVE.md`), and live grep of `src/server` + `src/web`.

Key framing for v03.3:

- Project is mid-flight in Phase 7 ("Frontend, E2E, and Deployment Gate Alignment"). Phase 6 (backend hardening) shipped in commit `ef81374`.
- A non-trivial uncommitted churn surface still exists in the worktree (CI, Docker, compose, env, web theme/types/vite, plus untracked Playwright config, new workspace pages, and historical root reports).
- The most expensive bugs in this repo are NOT runtime defects — they are **authority-of-truth** problems: historical reports keep injecting old realities into planning. Phase 6 closed many of the old "implementation is missing" concerns; what remains is structural and operational.
- Source code TODOs in the main line are concentrated in the Relay handler subsystem (`src/server/internal/relay/handler/*`); the rest of `src/server` and all of `src/web/src` are marker-free.

Severity scale used below:

- **Blocker** — must be resolved before milestone close or release.
- **High** — affects current Phase 7/8 scope or production safety.
- **Medium** — structural debt; safe to defer one milestone but compounding.
- **Low** — hygiene / cleanup; track in backlog.

## 1. Blocker

### B-01: Dirty worktree mixes Phase 7 source changes with historical/reference files

- Area: deployment / docs / planning hygiene
- Files: tracked modifications in `.github/workflows/ci.yml`, `Dockerfile.server`, `Dockerfile.web`, `docker-compose.yml`, `config/.env.example`, `scripts/deploy-validate.sh`, `package.json`, `pnpm-lock.yaml`, `src/web/package.json`, `src/web/vite.config.ts`, `src/web/tailwind.config.ts`, `src/web/src/theme/tokens.css`, `src/web/src/types/api.ts`; untracked root-level `ARCHAEOLOGY_REPORT.md`, `CLAUDE.md`, `CURRENT_STATUS.md`, `ROADMAP.md`, `docs/API.md`, several `docs/reports/2026-04-04-*` and `2026-04-05-*` files, `docs/architecture/knowledge-evolution-decision.md`, `docs/architecture/solo-runtime-decision.md`, `docs/superpowers/plans/`, `docs/superpowers/specs/2026-04-06-m0-baseline-freeze-closeout-design.md`, `src/web/e2e/`, `src/web/playwright.config.ts`, `src/web/src/routes/workspace/KnowledgePageView.tsx`, `src/web/src/routes/workspace/MarketplacePage.tsx`, `src/web/src/routes/workspace/SoloPageView.tsx`.
- Impact: Phase 5 (`05-COMMIT-BOUNDARIES.md`) explicitly separated planning / backend / frontend / deployment / historical-reference slices. A naive `git add -A` will collapse those slices and lose the Phase 5 boundary work. Root-level files (`ARCHAEOLOGY_REPORT.md`, `CURRENT_STATUS.md`, `ROADMAP.md`) duplicate `.planning/*` content and must not override `.planning` state per `.planning/STATE.md` §Worktree Context.
- Fix approach: For Phase 7 commits, stage only frontend/E2E/deployment/CI files. Quarantine root historical reports and the `docs/reports/2026-04-*` materials into a separate "historical material" commit (or leave untracked) — they are explicitly flagged in `CURRENT_STATUS.md` §11 as no longer the execution baseline. Use the per-slice plan in `.planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-COMMIT-BOUNDARIES.md`.

### B-02: Realtime relay endpoint is unauthenticated

- Area: security / backend
- Files: `src/server/internal/relay/handler/realtime.go:45` (`// 3. TODO: 鉴权（后续 Auth 中间件实现）`), and the surrounding `Handle()` method.
- Impact: The realtime WebSocket bridge proxies arbitrary traffic to the upstream provider with no auth check, no PreBill quota gate (`realtime.go:47`), and no post-close settlement (`realtime.go:104`). If this route is registered in any production-facing relay router, it is an unbounded provider-cost and abuse vector. The `_ = connectionID` discard (line 43) means even the idempotency hook is wired but unused.
- Fix approach: Either remove `realtime` from the registered route surface until Phase D billing/auth lands, or wrap it in `requireSession` plus a hard quota guard before exposing it externally. Treat as a release blocker until both `TODO: 鉴权` and `TODO: PreBill` resolve.

### B-03: Relay billing TODOs leave non-chat endpoints unmetered

- Area: security / billing / backend
- Files:
  - `src/server/internal/relay/handler/batch.go:61` — `// TODO: PreBill 预扣（Plan D 实现 BillingHook 后启用）`
  - `src/server/internal/relay/handler/batch.go:89` — `// TODO: 提取 batch_id，注册 Asynq polling 任务（Plan D）`
  - `src/server/internal/relay/handler/realtime.go:47` — PreBill TODO
  - `src/server/internal/relay/handler/realtime.go:104` — settlement TODO
  - `src/server/internal/relay/handler/files.go:168` — `// TODO: 存入 relay_files 表（Plan D 或后续实现）` (replaced today by a `fmt.Printf`)
  - `src/server/internal/relay/handler/responses.go:66` — `// TODO: 实现 Responses SSE 流式处理` (function returns nil with headers set but no body produced)
- Impact: The project's core value statement (`.planning/PROJECT.md` §"Core Value") is *"所有 AI 调用必须经过 Relay，确保计费、限流、监控统一"*. Today, only Chat/Completions enforce that contract through `Quota` + `BillingHook`. Batch, Realtime, Files, and Responses routes can fan out provider cost while bypassing quota — directly violating the project's foundational invariant. Files mapping persistence is replaced by `fmt.Printf("file mapped: ...")` (`files.go:169`), so file lifecycle is observability-only.
- Fix approach: Classify each `/v1/*` Relay endpoint as (a) native + billed, (b) passthrough + billed via post-response hook, or (c) stub/partial — and gate (c) behind a feature flag or `requireAdmin`. Add per-endpoint billing tests before advertising production support. See also CONCERNS §3.1.

## 2. High

### H-01: Router composition is monolithic; modular registration files are dead code

- Area: backend / maintainability
- Files: `src/server/internal/http/router.go` (1053 lines, single `NewRouter` function); modular siblings `src/server/internal/http/routes_auth.go`, `routes_chat.go`, `routes_console.go`, `routes_knowledge.go`, `routes_preferences.go`, `routes_task.go` define `register*Routes(...)` functions that are never called by `router.go` or `cmd/server/main.go` (verified by grep).
- Impact: Two routing surfaces coexist — an active monolith and dead modular helpers. Future edits can land in `routes_*.go` and silently have no runtime effect. Merge-conflict risk on `router.go` is high because Phase 6 already added Agent, Memory, MCP, Notification, Quota, WebSocket, Admin, and Marketplace registration to the same function. The file is at 1053 lines, ~3x the threshold flagged in `CURRENT_STATUS.md` §8.2.
- Fix approach: Either (a) wire `register*Routes(...)` into `NewRouter` and shrink the monolith, with route-surface tests asserting equivalence; or (b) delete the unused `routes_*.go` files. Half-measures (leaving both) are the worst outcome.

### H-02: SOLO runtime is template-driven, not a real agent execution engine

- Area: backend / product truth
- Files: `src/server/internal/task/runtime.go:11-55,128-225` (state advancement + `buildRuntimeResultSummary` template), `src/server/internal/task/store.go` (732 lines), `src/web/src/routes/workspace/SoloPage.tsx` (719 lines).
- Impact: Per `docs/architecture/solo-runtime-decision.md`, the team has committed to **Route A: keep SOLO as a restricted runtime MVP**, not upgrade to a true agent runtime. The risk is not the code — it is the narrative drift. `ARCHAEOLOGY_REPORT.md` §3.2 calls this out as "implementation fraud" risk: code volume looks like a real executor; behavior is a fixed state pusher with templated `result_summary`. Any external doc, README, or release note describing SOLO as "agent orchestration" violates the decision record.
- Fix approach: Enforce the decision in language: `docs/API.md`, README, marketing copy, and `docs/architecture/current-system-contracts.md` must say "restricted runtime MVP" or "structured task scaffold" — not "agent runtime". No new abstractions in `task/runtime.go` until the Exit Criteria in `solo-runtime-decision.md` §5 are met.

### H-03: Knowledge "retrieval" is text matching, not RAG — documentation drift risk

- Area: backend / product truth
- Files: `src/server/internal/knowledge/store.go` (572 lines), `src/server/internal/http/knowledge_handler.go`, `src/web/src/routes/workspace/KnowledgePage.tsx` (425 lines).
- Impact: Per `docs/architecture/knowledge-evolution-decision.md`, Knowledge is locked to **Route A: text retrieval Beta**, NOT embedding-based RAG. Migration `0016_pgvector.sql` is present and migration `0020_memory_hnsw.sql` exists, which creates pressure to claim "RAG is shipping". `CURRENT_STATUS.md` §3.2 explicitly rates this as a "definition inflation" risk. Tail-call signal: external docs may already use "RAG" — verify before Phase 8 doc reconciliation.
- Fix approach: Phase 8 (`DOC-02`) must grep `docs/` for "RAG" / "retrieval-augmented" and downgrade to "text retrieval Beta" wherever it appears. Knowledge code is allowed to use pgvector tables for memory-only purposes (per migration `0016`), but `KnowledgePage` and `/retrieve` must not be described as embedding-backed.

### H-04: MCP built-in tools are placeholder strings, not real implementations

- Area: backend / product
- Files:
  - `src/server/internal/mcp/builtin.go:59` — `web_search` returns `"Search results for: %s (placeholder - integrate with search API)"`
  - `src/server/internal/mcp/builtin.go:95` — calculator returns `"Result of '%s' = (placeholder - implement expression parser)"`
- Impact: Agent tool-loop demos appear functional; downstream LLM gets fake "search results" and fake "calculator results". This contaminates Agent test fixtures and any user-facing demo. Built-in tools register in `client.go:456` and are exposed as defaults, so an end user calling `web_search` gets a confidence-looking placeholder string and may believe the agent actually searched the web.
- Fix approach: Either (a) replace with real providers (web search API, expression parser like `govaluate`), (b) gate behind `requireAdmin` and label clearly in the UI/API response as a demo, or (c) remove from the default tool registry. Add tests asserting tool output structure includes a `provider` field that names the implementation.

### H-05: Stripe code exists but is not wired into the router

- Area: backend / billing
- Files: `src/server/internal/stripe/checkout.go`, `src/server/internal/stripe/webhook.go`. Grep of `src/server/internal/http/router.go`, `routes_*.go`, and `cmd/server/main.go` shows **no** `stripe` imports or webhook handler mounts.
- Impact: Stripe webhook signature verification + checkout helpers are implemented and committed, but unreachable from the running server. Webhook idempotency, subscription updates, and `audit_logs` integration (migration `0022_audit_logs.sql` exists) cannot be exercised in production. Any release note implying "Stripe billing is wired" is false.
- Fix approach: Decide explicitly: (a) wire the webhook into `router.go` under `/api/v1/billing/stripe/webhook` with `STRIPE_WEBHOOK_SECRET` env binding, register checkout under `requireAdmin`, and add an integration test; or (b) move `src/server/internal/stripe/*` to `docs/superpowers/specs/` as reference design and document the scope cut. Currently it is Schrödinger's billing.

### H-06: Phase 7 frontend churn (Playwright + new pages) is untracked and unverified

- Area: frontend / E2E / deployment gate
- Files: untracked `src/web/e2e/admin-marketplace.spec.ts`, `src/web/e2e/fixtures/adminMarketplace.ts`, `src/web/playwright.config.ts`, `src/web/src/routes/workspace/KnowledgePageView.tsx`, `src/web/src/routes/workspace/SoloPageView.tsx`, `src/web/src/routes/workspace/MarketplacePage.tsx`. Tracked diffs: `src/web/package.json`, `src/web/vite.config.ts`, `src/web/tailwind.config.ts`, `src/web/src/theme/tokens.css`, `src/web/src/types/api.ts`.
- Impact: Phase 7 success criteria (`.planning/ROADMAP.md` §"Phase 7") require Playwright to run cleanly and frontend types to match backend Phase 6 contracts. Today there is no evidence those run. The legacy `src/web/src/routes/workspace/MarketplacePage.tsx` is also tracked as backlog 999.2 (`.planning/ROADMAP.md` §"Phase 999.2") because the canonical `/marketplace` route is now served by `src/web/src/routes/marketplace/*` — keeping the workspace duplicate alive doubles the search surface.
- Fix approach: Run `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test`, `pnpm --dir src/web build`, and `pnpm --dir src/web test:e2e` before any Phase 7 close. Decide MarketplacePage workspace page fate (delete vs archive) per backlog 999.2. Validate that `src/web/src/types/api.ts` matches the user/role/preferences contract finalized in Phase 6.

### H-07: Auth lacks CSRF, rate limiting, password policy, and session rotation

- Area: security
- Files: `src/server/internal/auth/service.go`, `src/server/internal/http/auth_middleware.go`, `src/server/internal/http/auth_handler.go`. Grep for `CSRF`, `rate.?limit`, `brute` in `src/server` returns zero results.
- Impact: Auth uses bcrypt + HttpOnly cookie session, but there is no CSRF token on state-changing endpoints (`/api/v1/auth/logout`, all `PUT`/`POST` `/api/v1/app/*`), no login rate limiting, no password complexity check at registration, and no session rotation on privilege change. `SESSION_SECRET` is required by `config.Load()` (`src/server/internal/config/config.go:64-67`) but the auth session ID itself is opaque random — `SESSION_SECRET` is not used to sign or verify anything verifiable in source. Acceptable for Beta but unacceptable for any "production release" claim.
- Fix approach: Track as deferred-but-documented. Add a release-gate item in `docs/release/rc-checklist.md` reading "Auth hardening not in scope; release is Beta-grade". Before any external launch, layer: per-IP login limiter, CSRF double-submit cookie on `requireSession` mutating routes, and minimum password entropy check. Make `SESSION_SECRET` actually sign the session cookie OR delete the config requirement to remove the security-theater appearance.

### H-08: `internal/http/server_test.go` requires live PostgreSQL; CI may silently skip integration coverage

- Area: testing / release gate
- Files: `src/server/internal/http/server_test.go` (561 lines, gated on `TEST_DATABASE_URL`), `scripts/test.sh:44-45` (prints `Skipping server integration tests: TEST_DATABASE_URL not set.`).
- Impact: `bash scripts/test.sh server` passes without proving HTTP persistence path when `TEST_DATABASE_URL` is unset. Phase 6 closed with `TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32768/oblivious_test?sslmode=disable go test ./... -count=1` — but that requires a running disposable Postgres. CI workflow (`.github/workflows/ci.yml`, currently dirty) must be inspected to confirm a service container provides Postgres; otherwise CI is structurally permissive.
- Fix approach: Phase 7 verification (`.planning/ROADMAP.md` §"Phase 7") should include `docker compose config` and an explicit CI assertion that `TEST_DATABASE_URL` is set during `server` job. Release evidence must record either the exact `TEST_DATABASE_URL` value class or `"omitted; integration tests skipped by explicit rule"` per `docs/release/rc-checklist.md` §"Integration-Test Skip Semantics".

## 3. Medium

### M-01: Large page components compress state, actions, and rendering

- Area: frontend / maintainability
- Files:
  - `src/web/src/routes/workspace/SoloPage.tsx` — 719 lines (test file at 883 lines)
  - `src/web/src/routes/workspace/SoloPage.test.tsx` — 883 lines
  - `src/web/src/routes/admin/AdminChannelsPage.tsx` — 475 lines
  - `src/web/src/routes/workspace/KnowledgePage.tsx` — 425 lines
  - `src/web/src/routes/admin/AdminPlansPage.tsx` — 377 lines
  - `src/web/src/routes/workspace/ChatPage.tsx` — 318 lines
- Impact: Single-file pages absorb data fetching, state, actions, derived UI, and export logic. `KnowledgePageView.tsx` (221 lines, untracked) and `SoloPageView.tsx` (391 lines, untracked) suggest a view-extraction refactor is in flight but not committed. Risk: refactor lands mid-Phase 7 without tests catching behavior drift.
- Fix approach: Land the `*PageView.tsx` extractions explicitly with a paired diff to `*Page.tsx` so the page becomes a container only (data fetching + state) and the view becomes a presentation component. Keep `*.test.tsx` pointed at the container so behavior coverage does not regress.

### M-02: Backend hotspot files exceed the 500-line threshold from CLAUDE.md

- Area: backend / maintainability
- Files (with line counts):
  - `src/server/internal/http/router.go` — 1053
  - `src/server/internal/marketplace/store.go` — 767
  - `src/server/internal/task/store.go` — 732
  - `src/server/internal/memory/service.go` — 664
  - `src/server/internal/http/admin_handler.go` — 621
  - `src/server/internal/knowledge/store.go` — 572
  - `src/server/internal/http/server_test.go` — 561
  - `src/server/internal/http/task_handler_test.go` — 557
  - `src/server/internal/quota/service.go` — 511
- Impact: `CLAUDE.md` §"Project Architecture" mandates "Keep files under 500 lines". Nine files violate this. `router.go` and `task/store.go` are the worst offenders and were already flagged in the 2026-04-05 technical audit and `CURRENT_STATUS.md` §6.2. Continued growth is the path of least resistance because new modules (Phase 6: agent, memory, mcp, notification, quota, ws) all register in `router.go`.
- Fix approach: After Phase 7 closes, plan a focused phase to split `router.go` into the existing `routes_*.go` (see H-01) and split `task/store.go` along the `task` / `task_step` / `task_event` / `task_artifact` table boundaries.

### M-03: Migration runner has no version tracking

- Area: database / operations
- Files: `src/server/cmd/migrate/main.go:20-49` reads `src/server/migrations/`, sorts `.sql` filenames lexicographically, executes each statement in order; no `schema_migrations` table is maintained.
- Impact: Re-running migrations against a populated DB depends on each `.sql` being idempotent (`CREATE TABLE IF NOT EXISTS`, etc). Migrations `0001`-`0024` exist; any non-idempotent statement (e.g. `ALTER TABLE ... ADD COLUMN` without `IF NOT EXISTS`) becomes a foot-gun across deployments. Editing an old migration silently breaks environments that already applied it.
- Fix approach: Add `schema_migrations(version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ)` and skip already-applied filenames. Document immutability: "never edit applied migrations; always append `NNNN_xxx.sql`". This work is sized M and should land before any prod deployment, even on the restricted-network path.

### M-04: Default error mapping collapses too many cases to `500`

- Area: backend / API contract
- Files: handler `internal/http/*_handler.go` files; `internal/auth/service.go`. The 2026-04-05 audit (§7.1) and `CURRENT_STATUS.md` §7.1 both note that `sql.ErrNoRows`, validation errors, and authentication errors generally land in `writeError(..., 500, "internal_error", ...)` instead of `404`/`400`/`401`.
- Impact: Frontend `HttpClient` cannot distinguish "not found" from "server crashed", which makes UX retry/empty-state handling impossible without inspecting `error.message` strings. This is also a CI gate gap: targeted handler tests in `internal/http/*_test.go` pass even when status code is wrong because the tests assert `200` happy paths.
- Fix approach: Introduce sentinel errors per package (`auth.ErrInvalidCredentials`, `knowledge.ErrNotFound`, etc.), map them in handlers via `errors.Is`, and add table-driven tests for 4xx codes. Sized M; pairs naturally with H-01 (router split) since the registration sites are the natural place to add the mapping.

### M-05: Heavy index/query patterns lack composite indexes

- Area: backend / performance
- Files: `src/server/migrations/0005_usage_records.sql`, `0013_channels.sql` through `0024_categories_tags.sql`; query sites in `src/server/internal/console/service.go`, `marketplace/store.go`.
- Impact: Console aggregates filter by `workspace_id + created_at` over `usage_records`; marketplace search filters by `category + tag + status`. Without composite indexes, these become seq scans as data grows. Currently invisible because dataset is small.
- Fix approach: Add `EXPLAIN` benchmarks for `console.GetUsageSummary`, marketplace search, and task listing. Add composite indexes via new append-only migrations (per M-03 immutability rule).

### M-06: `chat.RelayGateway` fallback path differs by environment

- Area: backend / determinism
- Files: `src/server/internal/http/router.go:47-63` — when `cfg.RelayEnabled` is true and `cfg.Env != "production"`, the Chat service wraps `RelayGateway` + `HTTPReplyGenerator` in a `CompositeGateway`; in production it uses Relay only.
- Impact: Dev/test traffic can succeed against the local reply generator even when Relay misconfigured. Production traffic with the same env vars will fail differently. The Phase 6 release verified Relay-first, but the composite fallback is environment-gated, so smoke testing in non-prod does not exercise the production code path.
- Fix approach: Add a release-gate item: "smoke test runs with `OBLIVIOUS_ENV=production` against the compose stack" so the composite-gateway branch is not exercised. Document this in `docs/release/rc-checklist.md`.

### M-07: Frontend type contract is thin

- Area: frontend / API contract
- Files: `src/web/src/types/api.ts` (251 lines, modified). Currently defines `ApiEnvelope<T>`, auth/session/workspace shapes, `ConversationSummary`, `ConversationMessage`, and partial preference types.
- Impact: Phase 6 reshaped auth/session payloads (user.name, user.role) and preference defaults. `src/web/src/types/api.ts` must mirror those changes; otherwise the frontend `tsc --noEmit` gate passes against stale types and runtime decoding silently breaks. Phase 7 success criterion #1 (`.planning/ROADMAP.md`) is precisely "frontend API types and app pages match the backend contracts introduced or changed in Phase 6".
- Fix approach: Cross-reference each Phase 6 summary file (`.planning/phases/06-backend-mainline-integration-hardening/06-04-SUMMARY.md`) against `types/api.ts`. Add a contract test that imports a sample response JSON and type-asserts it.

## 4. Low

### L-01: Historical reports in `docs/reports/` still present despite being downgraded

- Area: docs / authority hierarchy
- Files: `docs/reports/2026-04-04-codebase-analysis.md`, `docs/reports/2026-04-04-explicit-markers.md`, `docs/reports/2026-04-04-todo-tracker.md`, `docs/reports/2026-04-05-explicit-markers.md`, `docs/reports/2026-04-05-technical-audit.md`, `docs/reports/2026-04-05-todo-tracker.md` (all untracked).
- Impact: Each file now opens with a "Historical Material Notice" header pointing readers to `CURRENT_STATUS.md` and `docs/architecture/current-system-contracts.md`, but the files themselves are still uncommitted. They will either land in the wrong commit slice (B-01) or be re-read by automation as "current". `ARCHAEOLOGY_REPORT.md` §3.3 explicitly recommends defaulting to ignore these.
- Fix approach: Commit them under an explicit "docs(history): archive 2026-04 audit reports" slice with the Historical Material Notice headers, OR move them under `docs/reports/archive/` so future grep tooling can filter by path. Do not delete — they have audit-trail value.

### L-02: Root-level duplicate files conflict with `.planning/` authority

- Area: docs / authority hierarchy
- Files: untracked `CURRENT_STATUS.md`, `ROADMAP.md`, `ARCHAEOLOGY_REPORT.md`, `CLAUDE.md` at repo root.
- Impact: `.planning/STATE.md` §"Worktree Context" already warns that these should not override `.planning/` state. The duplication invites future contributors to update the root copies and skip `.planning/`. `CLAUDE.md` at root is a Claude Code project instruction file and is required where it is; the others are debt.
- Fix approach: Decide for each file: (a) commit it as the canonical root-level summary and delete the `.planning/` equivalent, or (b) leave it untracked and rely on `.planning/` only. Current setup of "both committed in different places" is the worst option.

### L-03: Legacy workspace MarketplacePage is duplicate code

- Area: frontend / cleanup
- Files: `src/web/src/routes/workspace/MarketplacePage.tsx` (untracked). Active marketplace route tree is `src/web/src/routes/marketplace/*`.
- Impact: Tracked as backlog 999.2 in `.planning/ROADMAP.md`. Static-only data (`MARKETPLACE_SERVERS` const array of MCP servers) — looks alive but not wired through router. Searches for marketplace logic return both surfaces.
- Fix approach: Delete the file OR rename to `MarketplacePage.archived.tsx` with a top-of-file comment pointing to the active route. Resolve before milestone v03.3 archive.

### L-04: Phase 01 missing `SUMMARY.md`

- Area: planning / audit trail
- Files: `.planning/phases/01-relay-integration/PLAN.md` exists, `.planning/phases/01-relay-integration/SUMMARY.md` does not. Tracked as backlog 999.1.
- Impact: Future audits cannot trace Phase 01 outcomes without reading `PLAN.md` + `VERIFICATION.md` manually. Low risk because Phase 01 work is verified in code and downstream phases reference it.
- Fix approach: Reconstruct from `PLAN.md` + `VERIFICATION.md` per `.planning/ROADMAP.md` §"Phase 999.1". Sized S.

### L-05: `log.Fatalf` on startup is correct but leaves no telemetry

- Area: backend / observability
- Files: `src/server/cmd/server/main.go:21,26,44,51`, `src/server/cmd/migrate/main.go:18,23,29,44,48`.
- Impact: Process exits on config/db/migration errors with stderr message only. No metrics emitted, no audit log. Operations runbook does not document how to detect that the server failed to start vs failed in-flight. Acceptable for current Beta operations.
- Fix approach: Backlog. Replace with structured logger and exit-code conventions. Pairs naturally with the future observability story called out in `CURRENT_STATUS.md` §"Deferred Items".

### L-06: `panic("not used")` in test mocks is fragile

- Area: testing
- Files: `src/server/internal/agent/service_test.go:54,65,69,73,77,88,92`, `src/server/internal/http/auth_middleware_test.go:19,23,27,31,35` and similar mock stubs.
- Impact: Test mocks panic if an unexpected method is called. Acceptable today, but adding a new code path that happens to call an "unused" method causes a panic in `go test`, which can mask the actual assertion failure.
- Fix approach: Replace `panic("not used")` with `t.Helper(); t.Fatalf("unexpected call: %s", "MethodName")` so the failure is attributed to the calling test.

## 5. Cross-Cutting Themes

### 5.1 Authority-of-truth drift

The single most expensive ongoing concern in this repo is documentation drift, not code defects:

- `docs/reports/2026-04-*` files describe a project state that Phase 6 has already superseded.
- Root-level `CURRENT_STATUS.md` / `ROADMAP.md` / `ARCHAEOLOGY_REPORT.md` duplicate `.planning/` content.
- Architecture decisions (`solo-runtime-decision.md`, `knowledge-evolution-decision.md`) exist as untracked files, so they have no committed authority yet.
- `docs/superpowers/specs/2026-04-06-m0-baseline-freeze-closeout-design.md` describes the M0 freeze but is untracked.

`CURRENT_STATUS.md` §11 ("基线锁定声明") declares `CURRENT_STATUS.md` + `.planning/ROADMAP.md` + `docs/architecture/current-system-contracts.md` as the only execution baseline. Phase 8 (`DOC-02`) must lock this in by committing the architecture decisions and archiving the old reports.

### 5.2 Relay subsystem is the only TODO hotspot in mainline

- Backend mainline (`src/server`, excluding reference repos): 7 TODO/FIXME markers, all in `src/server/internal/relay/handler/{batch.go, realtime.go, responses.go, files.go}`, all tagged "Plan D".
- Frontend mainline (`src/web/src`): 0 TODO/FIXME markers.
- Reference repos (`new-api/`, `lobehub/`): 124 + 118 markers respectively (per `docs/reports/2026-04-05-explicit-markers.md`) — explicitly out of mainline scope.

The Relay TODOs are not random — they all reference "Plan D" billing/auth integration that has not been planned in this milestone. They are tracked as B-02 and B-03 above.

### 5.3 SOLO and Knowledge narrative discipline

Both `docs/architecture/solo-runtime-decision.md` and `docs/architecture/knowledge-evolution-decision.md` exist precisely because the team identified "narrative inflation" as a real risk. The decisions are: do NOT upgrade, do NOT use the bigger word. Phase 8 docs reconciliation (`DOC-02`) must enforce these decisions across `docs/API.md`, README, and `current-system-contracts.md`.

## 6. Recommended Priority Order

1. **Resolve B-01** before any Phase 7 commit — bad slicing destroys Phase 5 boundary work.
2. **Quarantine B-02/B-03** — either remove `realtime`, `batch`, and `files` from the registered Relay surface, or wrap them in `requireSession` + admin gate, before any external release. Document the scope cut.
3. **Decide H-05 (Stripe)** in the same pass — wire or remove. Shipping with implemented-but-unmounted billing code is worse than not having it.
4. **Land H-06** to clear Phase 7 success criteria. Pair with M-01 (PageView extraction) and M-07 (type contract sync).
5. **Phase 8 doc reconciliation** drives L-01, L-02, H-02, H-03 narrative discipline together.
6. **Backlog after milestone:** H-01 router split, M-02 file-size remediation, M-03 migration ledger, M-04 error mapping, H-04 MCP placeholders, H-07 auth hardening.

---

*Concerns audit: 2026-05-16*
