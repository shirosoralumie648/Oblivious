# Project Milestones: Oblivious

## v03.2 Quality and Release (Shipped: 2026-05-14)

**Delivered:** Release-candidate quality gates, documentation, and Docker runtime validation for the active `src/server` and `src/web` mainline.

**Phases completed:** Phase 04 Quality and Release (4 plans, 4 requirements)

**Key accomplishments:**

- Broadened backend release checks to `go test ./... -count=1`, covering Admin, Marketplace, Relay, Agent, Memory, and Quota boundaries.
- Added Admin/Marketplace browser E2E with deterministic Playwright route fixtures.
- Reconciled API documentation, current system contracts, RC checklist, README release guidance, and docs quality-gate assertions.
- Added Dockerfiles, Docker compose stack, Kubernetes manifests, env examples, and `/healthz` smoke tooling for the active mainline.
- Cleared DEPLOY-01 through real Docker compose image build, stack startup, health checks, and smoke validation using documented restricted-network overrides.

**Verification:**

- `bash scripts/check.sh all` passed in the approved non-sandbox path.
- `bash scripts/test.sh all` passed in the approved non-sandbox path: 32 web test files / 110 tests, server `go test ./... -count=1`, and explicit `TEST_DATABASE_URL` integration skip.
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e` passed 3/3 Admin/Marketplace browser tests.
- `docker compose config` passed.
- `OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh` built server/web images, started PostgreSQL, Redis, server, and web, passed `http://127.0.0.1:8080/healthz`, and cleaned up the stack.

**Environment limitations recorded:**

- Direct Docker Hub access from the Docker daemon still times out in this host environment; the validated local path uses the documented image registry prefix.
- Default Go module routes were unreliable during image builds; the validated path uses the documented Aliyun Go proxy and China-accessible checksum DB.
- `kubectl` is not installed, so Kubernetes remains an unexecuted alternate deployment path, not a blocker after Docker compose validation passed.

**Known deferred items at close:**

- Phase 01 summary reconstruction remains in backlog 999.1.
- Legacy `src/web/src/routes/workspace/MarketplacePage.tsx` cleanup remains in backlog 999.2.
- Future milestone close policy for living `.planning/REQUIREMENTS.md` remains in backlog 999.2; v03.2 requirements were archived, but the living file is preserved for cross-phase context.

**Archives:**

- `.planning/milestones/v03.2-ROADMAP.md`
- `.planning/milestones/v03.2-REQUIREMENTS.md`
- `.planning/milestones/v03.2-MILESTONE-AUDIT.md`

**What's next:** Run `$gsd:new-milestone` to define the next milestone.

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
