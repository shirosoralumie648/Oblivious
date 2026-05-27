# Phase 12: Commercial Gate CI and Evidence - Context

**Gathered:** 2026-05-28
**Status:** Ready for planning
**Source:** Current goal context, `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md`, `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`

<domain>
## Phase Boundary

Phase 12 closes v04 Commercial Foundation by making the tenant/security evidence reproducible and by defining the commercial gate contract that later v05-v08 milestones must satisfy before claiming commercial readiness.

This phase owns:
- CI wiring that runs DB-backed server HTTP integration tests and fails loudly if the database is unavailable in CI.
- Release/documentation updates that make `bash scripts/check.sh all` and `bash scripts/test.sh all` v04 gates with DB-backed coverage expectations.
- Commercial gate documentation mapping tenant, Relay, billing, product, security, operations, and verification gates to required evidence.
- Phase 12 verification artifacts that record exact commands, environment class, migration status, passed checks, skipped checks, and accepted residual debt.

This phase does not implement v05 Relay endpoint billing, v06 Stripe/Marketplace payouts, v07 production operations, or v08 product completeness.
</domain>

<decisions>
## Implementation Decisions

### CI DB-backed integration policy
- CI server jobs must set `TEST_DATABASE_URL` against a PostgreSQL service and must run `bash scripts/test.sh server` with a required-DB mode.
- Required-DB mode must fail before `go test` if `TEST_DATABASE_URL` is missing; the CI job must not pass by silently skipping `src/server/internal/http` integration tests.
- Local developer runs may keep the existing explicit skip behavior when `TEST_DATABASE_URL` is unset, because not every local shell has PostgreSQL available.
- `scripts/test.sh` should expose required-DB behavior through an environment flag rather than a CI-only hard-coded branch, so local release verification can opt into the same gate.

### Commercial gate documentation
- Add a dedicated commercial gate document instead of overloading the older RC checklist.
- The gate document must say the project is not commercial complete until tenant/identity, Relay authority, billing/monetization, product completeness, security, operations, and verification gates all have current repository evidence, automated checks, and runtime smoke where applicable.
- v04 may claim only Commercial Foundation completion after CI and evidence gates pass; it must not claim final commercial readiness.
- v05-v08 must remain visible as required future milestones.

### Evidence model
- Phase 12 verification must record exact commands, whether they used local Postgres or CI service Postgres, migration status, pass/fail status, and any skip reason.
- Missing DB-backed proof is a blocker for CI-01, not acceptable residual debt.
- Missing v05-v08 commercial gates are residual commercial-program work, not blockers to v04 completion.

### Existing docs that need promotion
- `README.md` should point readers to the commercial gate document and distinguish local optional DB skips from CI required DB coverage.
- `docs/release/rc-checklist.md` can remain as historical/release-candidate gate material, but it must point to the commercial gate contract and avoid implying RC readiness is commercial readiness.
- `docs/architecture/current-system-contracts.md` should describe the required-DB CI mode alongside the current test commands.
- `scripts/verify-quality-gates.sh` should enforce the new gate document and CI/script assertions.

### the agent's Discretion
- The exact name of the required-DB environment variable is implementation discretion, but it must be clear, documented, and asserted by quality gates.
- The CI job may keep the existing `server` job name if it clearly provisions PostgreSQL and runs required DB-backed tests.
</decisions>

<canonical_refs>
## Canonical References

Downstream agents MUST read these before planning or implementing.

### Commercial Objective
- `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md` - Defines final commercial-complete gates and v04-v08 decomposition.
- `.planning/PROJECT.md` - Current commercial foundation state and non-MVP objective.
- `.planning/REQUIREMENTS.md` - CI-01 and DOC-03 requirements plus future v05-v08 requirements.
- `.planning/ROADMAP.md` - Phase 12 goal, success criteria, and verification expectations.
- `.planning/STATE.md` - Current status after Phase 11 completion.

### Existing Evidence And Gate Surfaces
- `.github/workflows/ci.yml` - Current CI jobs; server job currently lacks PostgreSQL service wiring.
- `scripts/test.sh` - Current local test entry point; server integration tests skip when `TEST_DATABASE_URL` is unset.
- `scripts/check.sh` - Current release check entry point.
- `scripts/verify-quality-gates.sh` - Current docs/release asset assertions.
- `docs/release/rc-checklist.md` - Existing release gate ledger and integration skip semantics.
- `docs/architecture/current-system-contracts.md` - Current behavior and command contract.
- `README.md` - Top-level command and docs navigation.

### Prior v04 Evidence
- `.planning/phases/09-tenant-model-and-migration-ledger/09-01-SUMMARY.md` - Tenant model and migration ledger DB-backed evidence.
- `.planning/phases/10-membership-roles-and-auth-security/10-01-SUMMARY.md` - Membership/auth security DB-backed evidence.
- `.planning/phases/11-tenant-scope-across-core-domains/11-01-SUMMARY.md` - Tenant scope and cross-tenant denial DB-backed evidence.
</canonical_refs>

<specifics>
## Specific Ideas

- Use an environment flag such as `OBLIVIOUS_REQUIRE_TEST_DATABASE=true` in CI.
- Add a PostgreSQL service to `.github/workflows/ci.yml` with `postgres:16`, `POSTGRES_USER=oblivious`, `POSTGRES_PASSWORD=oblivious`, `POSTGRES_DB=oblivious_test`, and a `pg_isready` health check.
- Set `TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:5432/oblivious_test?sslmode=disable` in the server job.
- Add `docs/release/commercial-gates.md` with a table for gate, evidence required, current v04 status, and owning future milestone.
- Phase 12 verification should include targeted `rg` checks for `OBLIVIOUS_REQUIRE_TEST_DATABASE`, `services:`, `postgres:16`, `commercial-gates.md`, `CI-01`, `DOC-03`, and the commercial gate names.
</specifics>

<deferred>
## Deferred Ideas

- v05 Relay Billing Completeness owns `/v1/*` endpoint classification, production fail-closed unsupported endpoints, per-endpoint billing/rate-limit/audit behavior, and direct-provider bypass checks.
- v06 Billing And Marketplace Operations owns Stripe production routes/webhooks, subscriptions, invoices, refunds, top-ups, Marketplace settlement, payout state, and moderation.
- v07 Production Operations owns Kubernetes/equivalent orchestration proof, backup/restore smoke, structured logs, tracing, metrics, alerts, dashboards, runbooks, release, and rollback.
- v08 Product Completeness owns real or disabled built-in MCP tools, durable Agent workflows, Knowledge behavior matching product copy, commercial UX, docs, onboarding, pricing, operator guides, and full commercial journeys.
</deferred>

---

*Phase: 12-commercial-gate-ci-and-evidence*
*Context gathered: 2026-05-28*
