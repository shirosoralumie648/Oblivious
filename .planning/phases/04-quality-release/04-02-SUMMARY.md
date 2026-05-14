---
phase: 04-quality-release
plan: 02
subsystem: testing
tags: [playwright, e2e, admin, marketplace, release-gate]

requires:
  - phase: 03.1-admin-marketplace-ui
    provides: Admin and Marketplace browser surfaces to validate
provides:
  - Playwright browser E2E gate for Admin and Marketplace workflows
  - Deterministic mocked API fixtures for Admin, Marketplace, and auth routes
  - CI E2E job and RC checklist command for release-candidate validation
affects: [04-quality-release, TEST-02, release-candidate]

tech-stack:
  added:
    - "@playwright/test"
  patterns:
    - Browser E2E uses Playwright route fixtures instead of live APIs
    - Chromium-only initial release gate

key-files:
  created:
    - src/web/playwright.config.ts
    - src/web/e2e/admin-marketplace.spec.ts
    - src/web/e2e/fixtures/adminMarketplace.ts
  modified:
    - package.json
    - src/web/package.json
    - .github/workflows/ci.yml
    - docs/release/rc-checklist.md
    - scripts/verify-quality-gates.sh

key-decisions:
  - "Use deterministic Playwright route fixtures for all Admin and Marketplace API calls."
  - "Run E2E as a separate CI job so browser install cost is visible."
  - "Keep the first Phase 4 browser gate scoped to Chromium."

patterns-established:
  - "E2E fixtures return the same response envelope expected by the web HTTP client."
  - "Release browser tests do not require live LLM provider keys, Stripe, or external services."

requirements-completed: [TEST-02]

duration: 25min
completed: 2026-05-02
---

# Phase 04 Plan 02: Admin and Marketplace E2E Gate Summary

**The Admin and Marketplace browser workflows now have a repeatable Playwright release gate with mocked API data and no live external service dependency.**

## Performance

- **Duration:** 25 min
- **Completed:** 2026-05-02
- **Tasks:** 4 completed
- **Files created/modified:** 9

## Accomplishments

- Added a Playwright configuration that builds and previews the Vite app on `127.0.0.1:4173`.
- Added deterministic Admin, Marketplace, and auth route fixtures under `src/web/e2e/fixtures/adminMarketplace.ts`.
- Added browser specs covering Admin navigation, Marketplace browse/detail/install, Marketplace publish, and my-agents views.
- Added a root E2E script and a web package E2E script.
- Added a separate CI `e2e` job that installs Chromium and runs `pnpm --dir src/web test:e2e`.
- Updated the RC checklist with the exact E2E command.
- Aligned the docs quality-gate script with the already-completed 04-01 server release gate command.

## Files Created/Modified

- `src/web/playwright.config.ts` - Playwright Chromium project with build/preview web server.
- `src/web/e2e/fixtures/adminMarketplace.ts` - Mocked `/api/v1/auth/me`, Admin, and Marketplace responses.
- `src/web/e2e/admin-marketplace.spec.ts` - Browser workflow coverage for Admin and Marketplace release paths.
- `src/web/package.json` - Adds `test:e2e` and `@playwright/test`.
- `package.json` - Adds root `test:e2e` script.
- `.github/workflows/ci.yml` - Adds separate E2E job.
- `docs/release/rc-checklist.md` - Adds the release-candidate E2E command.
- `scripts/verify-quality-gates.sh` - Asserts the current `go test ./... -count=1` server release gate instead of the pre-04-01 narrow package list.

## Verification

Passed:

```bash
COREPACK_HOME=.tmp/corepack pnpm --dir src/web install --frozen-lockfile
COREPACK_HOME=.tmp/corepack pnpm --dir src/web exec playwright install chromium
COREPACK_HOME=.tmp/corepack pnpm --dir src/web exec playwright test --list
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test
COREPACK_HOME=.tmp/corepack pnpm --dir src/web build
bash scripts/check.sh docs
```

Observed results:

- Playwright listed 3 tests in `admin-marketplace.spec.ts`.
- Playwright E2E passed: 3 tests passed.
- Web Vitest passed: 32 files, 110 tests.
- Web build passed: `tsc --noEmit && vite build`.
- Docs check passed after quality-gate assertions were aligned to the current server release gate.

## CI Decision

E2E is wired as a separate GitHub Actions job. This keeps the browser install cost explicit while still making the gate repeatable for release-candidate validation.

## Deviations from Plan

None. The E2E gate uses mocked responses for the planned Admin and Marketplace surfaces and does not call live LLM, Stripe, or provider services.

## Issues Encountered

`bash scripts/check.sh docs` initially failed because `scripts/verify-quality-gates.sh` still asserted the old focused server test command from before 04-01. The script now checks for `go test ./... -count=1`, and `bash scripts/check.sh docs` exits 0.

Playwright emitted Node warnings about `NO_COLOR` being ignored because `FORCE_COLOR` was set, but the tests exited 0.

## User Setup Required

Run Chromium install once before local E2E if Playwright browsers are not already installed:

```bash
COREPACK_HOME=.tmp/corepack pnpm --dir src/web exec playwright install chromium
```

## Self-Check: PASSED

- `04-02-SUMMARY.md` exists.
- Requirement `TEST-02` is complete for this release gate.
- Plan-level verification commands passed.
- No real provider or payment secrets are present in E2E fixtures or CI wiring.

## Next Phase Readiness

Ready for `04-03` documentation and release checklist reconciliation. Remaining Phase 4 work is DOC-01 and DEPLOY-01.

---
*Phase: 04-quality-release*
*Completed: 2026-05-02*
