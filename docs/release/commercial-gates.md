# Commercial Readiness Gates

Oblivious is not commercial complete until every gate below is proven with current repository evidence, automated verification, and runtime smoke where applicable. v04 Commercial Foundation can close the tenant, security, migration, CI, and evidence foundation, but it is not final commercial readiness.

| Gate | Required evidence before commercial readiness | Current status | Owning milestone |
| --- | --- | --- | --- |
| Tenant And Identity Gate | Organizations are first-class tenants; users have audited memberships, roles, invitations, and ownership transfers; tenant-scoped reads and writes reject cross-tenant access. | Foundation evidence complete after Phases 9-11 and protected by Phase 12 CI evidence. | v04 |
| Relay Authority Gate | Production cannot call upstream LLM providers outside Relay; every `/v1/*` endpoint is classified as supported, internal/admin-only, or disabled; unsupported endpoints fail closed. | v05 started in Phase 13 with route classification and production fail-closed behavior. `docs/release/relay-route-table.md` is the route ledger; provider-bypass, auth/rate-limit/audit, settlement, and closeout evidence remain required before this gate is complete. | v05 |
| Billing And Monetization Gate | Plans, quota, subscriptions, top-ups, refunds, invoices, Stripe checkout/webhooks, Marketplace settlement, platform fees, and payout state are implemented and auditable. | Future required work. v04 does not complete Stripe, paid Marketplace, or payout accounting. | v06 |
| Product Completeness Gate | Chat, Agent, Knowledge, MCP, Admin, and Marketplace behavior matches customer-facing product copy; placeholder tools are disabled or replaced; full commercial journeys pass. | Future required work. v04 does not remove every MVP or placeholder product behavior. | v08 |
| Security Gate | CSRF, rate limits, password policy, session rotation, tenant isolation, admin boundaries, webhook signatures, and Relay cost-abuse paths have current automated evidence. | Foundation evidence complete for CSRF, auth rate limits, password policy, session rotation, and tenant isolation; webhook and Relay cost-abuse security remain future gates. | v04, v05, v06 |
| Operations Gate | Append-only migrations, backup/restore smoke, Kubernetes or equivalent orchestration proof, logs, metrics, tracing, alerts, dashboards, runbooks, release, and rollback are verified. | Migration ledger evidence complete in v04. Production orchestration, restore smoke, observability, and runbooks remain future required work. | v07 |
| Verification Gate | CI runs unit, contract, frontend, E2E, and DB-backed integration tests without silently skipping critical persistence coverage; final audit maps every commercial requirement to evidence. | Phase 12 owns DB-backed CI evidence for v04. Final commercial completion audit remains future required work. | v04, v08 |

## Claim Rules

- A milestone may claim completion only for requirements whose current repository evidence, automated verification, and required runtime smoke are recorded.
- v04 Commercial Foundation may claim tenant/security/migration/CI foundation completion after `CI-01` and `DOC-03` pass.
- v04 must not claim final commercial readiness or commercial completeness.
- v05, v06, v07, and v08 are required future milestones, not accepted debt.
- v05 may not claim Relay Authority Gate completion until route classification, production fail-closed, provider-bypass checks, endpoint auth/rate-limit/audit, settlement/refund behavior, and v05 evidence closeout all pass.
- Any future readiness claim must link to exact files, commands, environment class, migration status, pass/fail result, skipped checks, and residual risk.

## v04 Evidence Baseline

The v04 evidence chain is:

- Phase 9: organization tenant model and append-only migration ledger.
- Phase 10: memberships, roles, invitations, ownership transfer, CSRF, rate limits, password policy, and session rotation.
- Phase 11: tenant-scoped core domains and DB-backed cross-tenant denial tests.
- Phase 12: PostgreSQL-backed CI server tests, required-DB fail-fast behavior, this commercial gate contract, and v04 verification records.

DB-backed coverage is required in CI. Local runs may skip DB-backed integration tests only when `TEST_DATABASE_URL` is absent and `OBLIVIOUS_REQUIRE_TEST_DATABASE` is not set to `true`; release evidence must record that explicit skip.

## v05 Relay Evidence Baseline

The v05 evidence chain starts with:

- Phase 13: route policy registry, production fail-closed behavior, and `docs/release/relay-route-table.md`.
- Phase 14: provider-bypass checks plus endpoint auth, rate-limit, and audit guardrails.
- Phase 15: billing settlement, idempotency, refund, streaming/realtime, file, batch, and async endpoint policy evidence.
- Phase 16: Relay Authority Gate closeout with exact commands and residual v06-v08 work.
