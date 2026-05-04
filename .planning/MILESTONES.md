# Project Milestones: Oblivious

## v03.2 Quality and Release (Blocked: 2026-05-04)

**Delivered so far:** Release-candidate quality gates, documentation, and deployment configuration for the active `src/server` and `src/web` mainline.

**Phase status:** 04-01, 04-02, and 04-03 complete; 04-04 configuration complete but DEPLOY-01 runtime validation blocked.

**Key accomplishments:**
- Broadened backend release checks to `go test ./... -count=1`.
- Added Admin/Marketplace browser E2E with deterministic route fixtures.
- Reconciled API documentation, system contracts, RC checklist, and quality-gate assertions.
- Added Dockerfiles, Docker compose stack, Kubernetes manifests, env examples, and `/healthz` deployment smoke script.

**Verification:**
- `bash scripts/check.sh all` passed in the approved non-sandbox path.
- `bash scripts/test.sh all` passed in the approved non-sandbox path: 32 web test files / 110 tests, server `go test ./... -count=1`, and explicit `TEST_DATABASE_URL` integration skip.
- `docker compose config` passed.
- `BASE_URL=http://127.0.0.1:18080 bash scripts/deploy-smoke.sh` passed against a temporary local `/healthz` stub.

**Environment limitations recorded:**
- Docker daemon access is blocked for the current user/session, so real image build and compose startup could not be executed locally.
- `kubectl` is not installed, so Kubernetes apply/dry-run validation could not be executed locally.

**Known deferred items at close:**
- Phase 01 summary reconstruction remains in backlog 999.1.
- Legacy `src/web/src/routes/workspace/MarketplacePage.tsx` cleanup remains in backlog 999.2.
- Future milestone close policy for living `.planning/REQUIREMENTS.md` remains in backlog 999.2.

**What's next:** Restore Docker daemon access or install/provide Kubernetes tooling, then run a real deployment smoke before shipping or archiving v03.2.

---

## v03.1 Admin and Marketplace UI (Shipped: 2026-05-02)

**Delivered:** A usable Admin management surface and Agent Marketplace frontend backed by the Phase 3 APIs.

**Phases completed:** 03.1 (7 plans total, 18 validation tasks)

**Key accomplishments:**
- Exposed admin and marketplace HTTP endpoints for frontend consumption.
- Built typed admin and marketplace API clients plus shared React UI primitives.
- Delivered Admin dashboard, channels, routes, plans, users, audit log, and reviews pages.
- Delivered Marketplace browse/search, detail/install/review, publish, and my-agents pages.
- Closed UAT, security, validation, and milestone audit gates.

**Stats:**
- 92 tracked files changed in the v03.1 commit range.
- 10,600 insertions and 27 deletions in the v03.1 commit range.
- 7 plans, 18 validation tasks.
- Same-day execution window: 2026-05-02 16:37 to 18:34 +0800.

**Verification:**
- Go handler suite passed.
- Vitest targeted suite passed: 12 files, 32 tests.
- TypeScript compile passed.

**Known deferred items at close:**
- Legacy `src/web/src/routes/workspace/MarketplacePage.tsx` is no longer routed by `/marketplace` and should be cleaned up later.
- Phase 01 summary reconstruction remains in the existing `999.1` backlog item.

**Git range:** `75263ec` -> `87c5c1d`

**What's next:** Start a fresh milestone for Phase 4 quality/release work, or run a small cleanup phase for accepted planning/UI debt.

---
