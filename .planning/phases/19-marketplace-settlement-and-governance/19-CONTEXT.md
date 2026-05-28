# Phase 19 Context: Marketplace Settlement and Governance

## Milestone

v06 Billing And Marketplace Operations.

## Why This Phase Exists

Phase 18 closed `PAY-03`: verified Stripe events now update subscription, invoice, top-up, failed-payment, plan-change, and refund state through append-only billing lifecycle transitions. That still does not make paid Marketplace operation safe. The product can publish, approve, browse, install, and review agents, but paid Marketplace installs do not yet create an order, platform fee, publisher settlement, payout state, refund impact, or governance trail.

The commercial-complete program requires Marketplace publisher revenue, platform fees, payout state, refund impact, moderation, and abuse workflows before paid Marketplace operation can be enabled. Phase 19 consumes the Phase 18 billing lifecycle foundation and adds those Marketplace-specific money-movement and governance boundaries.

## Requirements

- **MARKET-03:** Marketplace publisher revenue, platform fee, payout state, and refund impact are modeled before paid Marketplace operation is enabled.
- **MARKET-04:** Marketplace moderation and abuse workflows cover publish, approve, reject, takedown, appeal, and audit paths.

## Current Evidence And Gaps

Existing Marketplace assets:

- `src/server/internal/marketplace/service.go` supports publish, update, delete, review queue, approve, reject, install, uninstall, user reviews, categories, versions, and publisher stats.
- `src/server/internal/marketplace/store.go` persists `published_agents`, `agent_versions`, `agent_installs`, and `agent_reviews`.
- `src/server/internal/http/marketplace_handler.go` exposes public browse/search/detail plus session-protected publish, install, review, my-agents, installs, and publisher stats routes.
- `src/server/internal/http/admin_handler.go` exposes admin review list, approve, and reject routes.
- Phase 18 added `payment_intents`, `billing_lifecycle_events`, `billing_invoices`, and `billing_refunds` as the payment authority/state foundation.

Current gaps:

- Paid agents can be published with `pricing_type` and `pricing_amount`, but installing a paid agent does not create a paid order or block installation before verified payment.
- There is no Marketplace order model linking a buyer, publisher, agent, payment intent, checkout session, install, gross amount, platform fee, publisher net, and refund state.
- There is no settlement ledger for publisher revenue, platform fee, payout eligibility, payout state, or refund reversal.
- Phase 18 refund records do not affect Marketplace orders or publisher settlement state.
- Moderation is limited to approve/reject. There is no takedown, appeal, reinstate, abuse report, or append-only Marketplace governance event trail.
- Publisher stats show install counts and active users, but not revenue, platform fees, pending/available/paid payouts, or refund impact.
- Admin billing inspection remains Phase 20, but Phase 19 must store enough structured Marketplace state for Phase 20 to expose.

## Boundaries

Included:

- Add Marketplace settlement/governance schema for orders, settlements, payouts, governance events, and abuse reports.
- Treat paid Marketplace install as payment-backed: create a pending order and payment intent, return checkout, and do not create `agent_installs` until a verified payment event applies the order.
- Keep free Marketplace install behavior working through the existing install path.
- Extend Stripe checkout metadata and lifecycle application for `checkout_kind=marketplace_install`.
- Apply refund impact to Marketplace orders and settlements using Phase 18 `billing_refunds` / payment-intent refund state.
- Add takedown, appeal, reinstate, and abuse-report workflows with audit/governance events.
- Extend publisher stats with gross revenue, platform fee, net revenue, pending/available/paid settlement amounts, and refund totals.
- Add DB-backed tests for paid install, settlement idempotency, refund reversal, moderation transitions, abuse reports, and publisher financial stats.

Excluded:

- Admin billing/settlement UI pages and broad admin inspection APIs. Those remain Phase 20.
- Live Stripe network tests. Tests continue to use fake checkout creators and signed webhook fixtures.
- Actual provider payout execution against Stripe Connect or any external payout provider. Phase 19 models payout state and records provider IDs, but external payout execution remains disabled until production operations and provider onboarding are proven.
- v06 closeout, v07 production operations, and v08 product completeness.

## Settlement Design

Phase 19 should add a narrow Marketplace settlement boundary inside `src/server/internal/marketplace`:

- `SettlementService` validates paid install checkout requests, calculates platform fee and publisher net amount, creates pending `marketplace_orders`, creates a `payment_intents` row with `kind=marketplace_install`, and returns a checkout request.
- `SettlementStore` owns SQL writes for Marketplace orders, settlements, payout records, governance events, and abuse reports.
- Paid install completion should be driven only by a verified Stripe webhook event. The lifecycle path should parse `checkout_kind=marketplace_install`, complete the local payment intent, mark the order paid, create the `agent_installs` row, create one settlement record, and record one lifecycle/governance transition.
- Refund handling should locate Marketplace orders by local or provider payment intent, update order refund totals/status, reverse or adjust settlement amounts once, and leave an auditable trail.
- Platform fee should be deterministic. Use `platform_fee_bps = 2000` (20%) unless the agent/order metadata specifies a different configured value in the future.
- Payout modeling should remain provider-neutral: payout rows track pending/processing/paid/failed/cancelled state and provider IDs, but no live payout API is called in this phase.

## Governance Design

Marketplace governance should use append-only events rather than relying only on the mutable `published_agents.status` field:

- `marketplace_governance_events` records publish, approve, reject, takedown, appeal, reinstate, abuse report, abuse resolve, and payout state actions.
- `marketplace_abuse_reports` records reporter, agent, reason, details, status, resolution, and reviewer.
- Admin takedown changes public availability and prevents new installs without deleting settlement history.
- Publisher appeal records the appeal and moves the abuse/governance review state without automatically restoring the agent.
- Admin reinstate restores approved/public availability only through an explicit governance event.

## Verification Targets

Focused RED/GREEN:

- `cd src/server && go test ./internal/marketplace -run 'Settlement|Governance|Abuse|Payout|PublisherStats' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Marketplace.*(Paid|Settlement|Refund|Takedown|Appeal|Abuse|PublisherStats)' -count=1`

Broader phase verification:

- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/marketplace ./internal/stripe ./internal/http -count=1`
- `bash scripts/check.sh docs`
- `GOPROXY=https://mirrors.aliyun.com/goproxy/,direct GOSUMDB=sum.golang.google.cn bash scripts/check.sh all`
- `git diff --check`

## Residual Risk After This Phase

Phase 19 should close `MARKET-03` and `MARKET-04` only. The product remains non-commercial-complete until Phase 20 exposes admin billing/settlement evidence and closes v06, v07 proves production operations, and v08 removes remaining product placeholders and completes customer-facing commercial journeys.
