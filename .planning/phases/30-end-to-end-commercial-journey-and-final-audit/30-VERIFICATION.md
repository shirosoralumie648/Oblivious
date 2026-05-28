# Phase 30 Verification - End-to-End Commercial Journey and Final Audit

**Date:** 2026-05-29
**Requirements:** PROD-06, AUDIT-01
**Result:** PASS
**Environment class:** Local worktree on branch `gsd/phase-10-membership-auth-security`; PostgreSQL pgvector test database on `127.0.0.1:32771`; Docker compose runtime; disposable pgvector backup/restore databases; Playwright using `/usr/bin/google-chrome`.
**Skipped checks:** None in strict verifier.

## Strict Commercial Completion Verifier

Command:

```bash
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome \
TEST_DATABASE_URL='postgres://oblivious:oblivious@127.0.0.1:32771/oblivious_test?sslmode=disable' \
COMMERCIAL_COMPLETION_RUN_DEPLOY=true \
COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true \
OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 \
PG_CLIENT_IMAGE=oblivious-postgres-pgvector:pg16 \
bash scripts/verify-commercial-completion.sh
```

Result: PASS.

Evidence from the strict run:

- Docs gate passed through `bash scripts/check.sh docs`.
- Relay security gate passed through `bash scripts/check.sh relay-security`.
- Focused frontend commercial suites passed: 33 test files, 130 tests.
- Playwright commercial journey passed: 1 Chromium test.
- Backend DB commercial journey passed: `ok oblivious/server/internal/http 0.501s`.
- Deployment validation passed: Docker compose build, data services, migrations, app stack, `/healthz`, `/metrics`, `/api/v1/auth/me` as 401, and `/v1/chat/completions` as 401.
- Backup/restore smoke passed: 32 migrations restored and verified; tenant, membership, quota, billing, payment, webhook, lifecycle, invoice, refund, Marketplace order/settlement/payout/governance/abuse, and audit fixture rows verified.
- Verifier summary reported `skipped checks: none` and `RESULT: strict verifier passed`.

## Additional Focused Checks

Backend commercial journey was also rerun directly after making the test idempotent:

```bash
cd src/server && TEST_DATABASE_URL='postgres://oblivious:oblivious@127.0.0.1:32771/oblivious_test?sslmode=disable' OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run TestCommercialHTTPJourney -count=1
```

Result: PASS, `ok oblivious/server/internal/http 0.507s`.

Browser commercial journey was run directly:

```bash
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e --grep "commercial journey"
```

Result: PASS, 1 Chromium test.

Verifier help was checked:

```bash
bash scripts/verify-commercial-completion.sh --help
```

Result: PASS, help documents `TEST_DATABASE_URL`, `COMMERCIAL_COMPLETION_RUN_DEPLOY=true`, `COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true`, and the non-final nature of `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true`.

## Phase 30 Gate Mapping

| Gate | Evidence | Result |
| --- | --- | --- |
| Tenant and Identity | Backend commercial journey exercises signup/session, organization scope, admin promotion, shared tenant records, and tenant-bound billing/Marketplace evidence. | PASS |
| Relay Authority | Backend journey records Chat and Knowledge embedding calls through the fake Relay with trusted organization headers; Relay security check passed. | PASS |
| Billing and Monetization | Subscription checkout, top-up checkout, signed Stripe fixtures, billing sessions, payment intents, webhook events, subscriptions, top-ups, settlements, payouts, and refunds are inspected. | PASS |
| Product Completeness | Browser journey covers onboarding, Chat, Knowledge citations, SOLO approval/retry, Marketplace paid install/publish/my-agents, Admin dashboard, billing, and reviews. | PASS |
| Security | CSRF-protected mutating routes, admin boundaries, signed webhook fixtures, and Relay-only provider boundary are included in strict verifier evidence. | PASS |
| Operations | Deploy validation and backup/restore smoke both ran in strict mode without skips. | PASS |
| Verification | `scripts/verify-commercial-completion.sh` completed with no skipped checks; docs and diff hygiene are required after this closeout update. | PASS |

## Closure Decision

`PROD-06` is complete because the strict verifier proved the end-to-end commercial journey across tenant setup, provider/channel route configuration, subscription/top-up, Chat, Agent/SOLO, Knowledge, Marketplace, billing inspection, deployment, backup, and restore.

`AUDIT-01` is complete because `docs/release/commercial-completion-audit.md` maps every commercial gate and explicit user objective surface to evidence, and the strict verifier has now supplied no-skip runtime proof.

The Product Completeness Gate and final commercial readiness may close for this repository state.
