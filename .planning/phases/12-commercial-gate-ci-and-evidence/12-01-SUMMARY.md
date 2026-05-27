---
phase: 12-commercial-gate-ci-and-evidence
plan: 01
status: complete
completed_at: 2026-05-28T05:10:00+08:00
requirements: [CI-01, DOC-03]
commits:
  - dc6f9be feat(12): enforce db backed ci evidence
---

# Phase 12 Plan 01 Summary

## Result

Implemented DB-backed CI evidence and commercial gate documentation for v04 Commercial Foundation closeout.

Delivered:
- `.github/workflows/ci.yml` server job now provisions `postgres:16`, sets `TEST_DATABASE_URL`, and runs with `OBLIVIOUS_REQUIRE_TEST_DATABASE=true`.
- `scripts/test.sh` now supports required-DB mode that fails loudly when `TEST_DATABASE_URL` is missing while preserving explicit local skip behavior.
- `docs/release/commercial-gates.md` defines the commercial readiness gates for tenant/identity, Relay authority, billing/monetization, product completeness, security, operations, and verification.
- README, current system contracts, RC checklist, and docs quality gates now reference DB-backed CI requirements and the commercial gate contract.
- `12-VERIFICATION.md` records exact commands, environment class, DB-backed coverage, skip semantics, and residual commercial work.

## Verification

Environment:
- PostgreSQL test container: local disposable Postgres on `127.0.0.1:32770`
- `TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable`
- Go proxy override: `GOPROXY=https://mirrors.aliyun.com/goproxy/,direct`
- Go checksum DB override: `GOSUMDB=sum.golang.google.cn`

Passed:
- `if OBLIVIOUS_REQUIRE_TEST_DATABASE=true bash scripts/test.sh server; then echo "expected required-DB failure" >&2; exit 1; fi`
- `bash scripts/test.sh server`
- `TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true bash scripts/test.sh server`
- `bash scripts/check.sh docs`
- `GOPROXY=... GOSUMDB=... TEST_DATABASE_URL=... bash scripts/test.sh all`
- `GOPROXY=... GOSUMDB=... TEST_DATABASE_URL=... bash scripts/check.sh all`
- Targeted `rg` checks for `postgres:16`, `TEST_DATABASE_URL`, `OBLIVIOUS_REQUIRE_TEST_DATABASE`, `commercial-gates.md`, and all seven commercial gate names.

## Requirement Closure

| Requirement | Evidence |
|-------------|----------|
| CI-01 | CI server job provisions PostgreSQL and runs shared server tests with `TEST_DATABASE_URL` plus `OBLIVIOUS_REQUIRE_TEST_DATABASE=true`; script fail-fast smoke proves missing DB cannot silently skip when required |
| DOC-03 | `docs/release/commercial-gates.md` maps commercial gates to required evidence, v04/v05-v08 ownership, and claim rules; docs gate asserts the file and references stay present |

## Residual Commercial Work

- Phase 12 closes v04 Commercial Foundation only.
- v05 Relay Billing Completeness remains required.
- v06 Billing And Marketplace Operations remains required.
- v07 Production Operations remains required.
- v08 Product Completeness remains required.
- The overall commercial-complete SaaS objective remains active and must not be marked complete yet.

## Next Phase Readiness

v05 can now start from a reliable tenant/security/CI foundation. The next commercial-program step should plan Relay Billing Completeness: `/v1/*` endpoint classification, production fail-closed behavior, Relay-only provider access checks, and per-endpoint auth/rate-limit/billing/audit behavior.

---

*Summary written: 2026-05-28*
