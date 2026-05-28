# Commercial Readiness Gates

Oblivious is not commercial complete until every gate below is proven with current repository evidence, automated verification, and runtime smoke where applicable. v04 Commercial Foundation closes the tenant, security, migration, CI, and evidence foundation. v05 Relay Billing Completeness closes the Relay Authority Gate evidence. v06 Billing And Marketplace Operations is in progress. None of those partial milestones alone is final commercial readiness.

| Gate | Required evidence before commercial readiness | Current status | Owning milestone |
| --- | --- | --- | --- |
| Tenant And Identity Gate | Organizations are first-class tenants; users have audited memberships, roles, invitations, and ownership transfers; tenant-scoped reads and writes reject cross-tenant access. | Foundation evidence complete after Phases 9-11 and protected by Phase 12 CI evidence. | v04 |
| Relay Authority Gate | Production cannot call upstream LLM providers outside Relay; every `/v1/*` endpoint is classified as supported, internal/admin-only, or disabled; unsupported endpoints fail closed; supported endpoints require tenant identity, rate-limit policy, audit semantics, and explicit billing settlement/refund policy. | Complete for v05 after Phase 13 route classification/fail-closed evidence, Phase 14 provider-bypass/auth/rate-limit/audit evidence, Phase 15 settlement/refund evidence, and Phase 16 Relay Authority Gate closeout evidence. This does not complete v06 money movement, v07 production operations, or v08 customer-facing product completeness. | v05 |
| Billing And Monetization Gate | Plans, quota, subscriptions, top-ups, refunds, invoices, Stripe checkout/webhooks, Marketplace settlement, platform fees, and payout state are implemented and auditable. | In progress. Phase 17 completes mounted Stripe checkout/webhook route authority, raw-body webhook signature verification, tenant-aware checkout metadata, `payment_intents`, and idempotent `stripe_webhook_events`. Phase 18 completes subscription, invoice, failed-payment, plan-change, top-up, and refund lifecycle state through `billing_lifecycle_events`, `billing_invoices`, and `billing_refunds`. Phase 19 completes Marketplace paid-install orders, settlement/platform-fee/payout-state modeling, refund impact, publisher financial stats, and governance/abuse workflows. Admin billing UI and v06 closeout remain required. | v06 |
| Product Completeness Gate | Chat, Agent, Knowledge, MCP, Admin, and Marketplace behavior matches customer-facing product copy; placeholder tools are disabled or replaced; full commercial journeys pass. | Future required work. v04 does not remove every MVP or placeholder product behavior. | v08 |
| Security Gate | CSRF, rate limits, password policy, session rotation, tenant isolation, admin boundaries, webhook signatures, and Relay cost-abuse paths have current automated evidence. | Foundation evidence complete for CSRF, auth rate limits, password policy, session rotation, and tenant isolation. Phase 14 adds Relay provider-bypass and production identity guardrails. Phase 17 adds Stripe webhook signature rejection and signed fixture idempotency evidence. | v04, v05, v06 |
| Operations Gate | Append-only migrations, backup/restore smoke, Kubernetes or equivalent orchestration proof, logs, metrics, tracing, alerts, dashboards, runbooks, release, and rollback are verified. | Migration ledger evidence complete in v04. Production orchestration, restore smoke, observability, and runbooks remain future required work. | v07 |
| Verification Gate | CI runs unit, contract, frontend, E2E, and DB-backed integration tests without silently skipping critical persistence coverage; final audit maps every commercial requirement to evidence. | Phase 12 owns DB-backed CI evidence for v04. Final commercial completion audit remains future required work. | v04, v08 |

## Claim Rules

- A milestone may claim completion only for requirements whose current repository evidence, automated verification, and required runtime smoke are recorded.
- v04 Commercial Foundation may claim tenant/security/migration/CI foundation completion after `CI-01` and `DOC-03` pass.
- v04 must not claim final commercial readiness or commercial completeness.
- v05, v06, v07, and v08 are required future milestones, not accepted debt.
- v05 may claim Relay Authority Gate completion only after route classification, production fail-closed, provider-bypass checks, endpoint auth/rate-limit/audit, settlement/refund behavior, and v05 Relay Authority Gate closeout evidence all pass.
- v06 may not claim Billing And Monetization Gate completion until subscription lifecycle, top-up fulfillment, refunds, invoices, failed-payment states, plan changes, Marketplace settlement, payout state, moderation workflows, admin billing inspection, and v06 closeout evidence all pass. Phase 19 closes Marketplace settlement/governance evidence only; Phase 20 remains required.
- Any future readiness claim must link to exact files, commands, environment class, migration status, pass/fail result, skipped checks, and residual risk.

## v04 Evidence Baseline

The v04 evidence chain is:

- Phase 9: organization tenant model and append-only migration ledger.
- Phase 10: memberships, roles, invitations, ownership transfer, CSRF, rate limits, password policy, and session rotation.
- Phase 11: tenant-scoped core domains and DB-backed cross-tenant denial tests.
- Phase 12: PostgreSQL-backed CI server tests, required-DB fail-fast behavior, this commercial gate contract, and v04 verification records.

DB-backed coverage is required in CI. Local runs may skip DB-backed integration tests only when `TEST_DATABASE_URL` is absent and `OBLIVIOUS_REQUIRE_TEST_DATABASE` is not set to `true`; release evidence must record that explicit skip.

## v05 Relay Evidence Baseline

The v05 evidence chain is complete for the Relay Authority Gate:

- Phase 13: route policy registry, production fail-closed behavior, and `docs/release/relay-route-table.md`.
- Phase 14: provider-bypass checks plus endpoint auth, rate-limit, and audit guardrails through `scripts/verify-relay-security.sh`, supported-route policy fields, trusted internal identity enforcement, and route-decision audit events.
- Phase 15: `BILL-01` and `BILL-02` billing settlement, idempotency, refund, streaming/realtime, file, batch, and async endpoint policy evidence.
- Phase 16: Relay Authority Gate closeout evidence with exact commands, environment class, DB migration status, passed checks, skipped checks, and residual v06-v08 work.

The v05 closeout does not satisfy the Billing And Monetization Gate, Operations Gate, Product Completeness Gate, webhook-signature security work, or final Verification Gate audit. Those remain required in v06, v07, and v08.

## v06 Billing Evidence Baseline

The v06 evidence chain is in progress:

- Phase 17: Stripe checkout and webhook routes are mounted in `src/server/internal/http/router.go`; checkout is tenant/session/CSRF guarded and fake-testable; webhook ingestion verifies Stripe raw-body signatures and records idempotent provider events in `stripe_webhook_events`; checkout intent state is recorded in `payment_intents`.
- Phase 18: `PAY-03` is complete. Verified Stripe events apply subscription, invoice, top-up, failed-payment, plan-change, and refund state transitions through `src/server/internal/stripe/lifecycle.go`; `billing_lifecycle_events` provides append-only transition evidence; `billing_invoices` and `billing_refunds` persist invoice/refund state; payment-backed top-up checkout no longer credits quota before verified webhook evidence; duplicate webhook deliveries can retry lifecycle application safely through transition-key idempotency.
- Phase 19: `MARKET-03` and `MARKET-04` are complete. Paid Marketplace installs create pending orders/payment intents without installing before verified checkout evidence; verified `checkout.session.completed` creates one install, one paid order, and one settlement; `refund.created` updates Marketplace order and settlement refund state idempotently; `marketplace_payouts` models local payout state without calling an external payout API; takedown, appeal, reinstate, abuse report, resolve, and dismiss workflows record governance events; publisher stats expose settlement-backed gross, platform fee, net, refund, pending, available, payout-pending, and paid-out amounts.
- Phase 20 remains required for admin billing inspection and v06 closeout evidence.
