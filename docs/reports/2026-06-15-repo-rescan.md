# Repository Rescan - 2026-06-15

## Current Truth

- Branch: `main`; this scan starts from commit `5dd9b0b test(frontend): prove console billing subscription checkout`.
- Worktree status during the scan: dirty only because this slice adds `src/web/e2e/console-access.spec.ts`, `src/web/e2e/fixtures/consoleAccess.ts`, and documentation updates for the rescan/evidence checkpoint.
- The project is still **not complete** against the four 2026-06-04 fusion specs.
- The current completion matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate after this rescan remains **84/100**. The new Console Access browser proof narrows Frontend, Billing, API contract, and Security evidence for user-facing API-token lifecycle behavior, but it does not close target-environment proof, broad tenant-isolation/security depth, live provider/payment rails, deployment validation, or final no-skip release readiness.

## What Changed In This Rescan

- Real Playwright browser coverage now exercises `/console/access` inside the Console shell.
- The journey proves active Console Access navigation, active workspace/session context, existing API-token listing, API-token creation, token usage inspection, provider/channel route-detail omission from user-visible usage rows, and scoped revoke of the original token.
- The fixture rejects mismatched create-token payloads, so the passing browser test proves the UI sends:
  - `name: "Browser key"`
  - `modelLimitsEnabled: true`
  - `modelLimits: ["gpt-4o-mini", "gpt-4.1-mini"]`
  - `userGroup: "vip"`
  - `quotaLimit: 25.5`
  - `expiresAt: "2026-06-30T00:00:00Z"`
- This is fixture-backed local browser proof, not target-environment proof for every Console/API-token path.

## Repository Inventory

- First-party tracked file distribution after this slice is intended to be:
  - `src`: 970 files
  - `.planning`: 210 files
  - `docs`: 92 files
  - `scripts`: 37 files
  - `deploy`: 42 files
- Server shape remains:
  - `src/server/internal`: 587 tracked files
  - `src/server/migrations`: 106 SQL migration files
  - largest active server domains remain `relay`, `http`, `mcp`, `admin`, `agent`, `workflow`, `knowledge`, `observability`, `channel`, `migration`, and `marketplace`
- Web shape remains:
  - `src/web/src/routes`: 80 tracked files
  - `src/web/src/features`: 51 tracked files
  - route families are mostly `workspace`, `admin`, `console`, `marketing`, and `marketplace`
- Test inventory after this slice:
  - Go test files: 226
  - Web component/API test files: 67
  - Web Playwright specs: 7 specs
  - Web E2E fixture files: 7 files
- Latest checked-in top-level migration remains `src/server/migrations/0081_admin_relay_channel_organization_scope.sql`.
- Project-local `AGENTS.md`: none at the main repo root or under first-party source; dependency caches and nested `reference/*` repositories are excluded from this first-party scan.

## Completion Matrix Snapshot

Proven rows remain:

- API gateway and relay
- Knowledge base and RAG
- Multi-channel publishing
- Database schema and migrations

Partial rows remain:

- Workflow engine
- Agent system
- Billing and monetization
- Marketplace ecosystem
- Frontend shell and core pages
- Observability metrics, logs, alerts, recovery
- API contract
- Deployment and operations
- Security and tenant isolation
- Migration strategy and release readiness

This scan does not reclassify any Partial row to Proven. Console Access browser proof improves evidence for Billing, Frontend, API contract, and Security, but broader runtime, target-environment, and final release proof remain open.

## Verification Run During This Rescan

```bash
git status --short --branch
git log --oneline -n 12
git ls-files | awk '...inventory counters...'
find . \( -path '*/node_modules/*' -o -path './.tmp/*' -o -path './reference/*' -o -path './.git/*' \) -prune -o -name AGENTS.md -print
rg -n "TODO|FIXME|XXX|stub|placeholder|Unimplemented|DisabledInProduction" src scripts deploy docs/release docs/reports -g '!src/web/test-results/**' -g '!src/web/playwright-report/**'
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/console-access.spec.ts --project=chromium
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web test src/routes/console/AccessPage.test.tsx -- --runInBand
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit
```

Result:

- `console-access.spec.ts` passed with Chromium. Build warnings about `NO_COLOR`, Lightning CSS `@theme`/`@utility`, and chunk size were non-fatal and match the existing Playwright environment behavior.
- `AccessPage.test.tsx` passed with 3 tests.
- `pnpm --dir src/web exec tsc --noEmit` passed.
- The first-party AGENTS scan found no main-root or first-party source `AGENTS.md`.
- The TODO/stub scan did not reveal a new broad implementation gap. Active first-party matches remain the known release-boundary items from the June 14 scan: disabled future Relay surfaces, generated gRPC `Unimplemented*` boilerplate, test stubs, placeholder-only release docs/config examples, and the service-template migration TODO.

## Notable Scan Findings

- The live worktree before documentation updates had only the new Console Access E2E spec and fixture as untracked files.
- Existing DB-backed evidence remains stronger than browser evidence for persistence and tenant isolation; this slice only adds browser-level proof for a user-visible Console lifecycle.
- `src/web/e2e/console-access.spec.ts` complements the earlier DB-backed Console API-token create/list/revoke and sanitized usage proof by exercising the route in the built browser app.
- The recommended browser journey queue should now treat Agent planning, Chat-to-SOLO, Marketplace paid provider, Workflows mobile responsive, Console Billing subscription checkout, and Console Access API-token lifecycle as covered local Playwright paths.

## Recommended Next Slices

1. Continue DB-backed security depth for remaining tenant-isolation and response-safety surfaces not covered by the current no-skip profiles.
2. Expand browser/E2E proof to the next high-value commercial journey not already covered by the six local Playwright paths above.
3. Rerun the strict commercial verifier on target infrastructure with deploy and backup/restore enabled before renewing any final readiness claim.
4. Extend Observability/recovery proof from repository-owned panic recovery into target-environment OOM/crash restart, scale, and failover evidence.
5. Keep Deployment and release readiness open until Kubernetes, backup/restore, migration replay, and provider/payment rails have target-environment proof.

## Boundary

This report is a repository rescan and evidence checkpoint, not a final completion claim. Rows marked `Partial` in `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` remain open until row-specific proof is recorded and rerun in the required environment.
