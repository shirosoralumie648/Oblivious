# Phase 19 Summary: Marketplace Settlement and Governance

## Status

Complete.

## Requirements Closed

- **MARKET-03:** Marketplace publisher revenue, platform fee, payout state, and refund impact are modeled before paid Marketplace operation is enabled.
- **MARKET-04:** Marketplace moderation and abuse workflows cover publish, approve, reject, takedown, appeal, and audit paths.

## Delivered

- Added `src/server/migrations/0030_marketplace_settlement_governance.sql` with:
  - `marketplace_orders` for paid install order/payment/install state.
  - `marketplace_settlements` for gross revenue, platform fee, publisher net, refund impact, hold, and payout linkage.
  - `marketplace_payouts` for provider-neutral local payout-state modeling.
  - `marketplace_governance_events` for append-only moderation and abuse workflow evidence.
  - `marketplace_abuse_reports` for open/resolved/dismissed abuse report state.
- Added `marketplace.SettlementService` for paid install checkout creation, verified checkout application, refund impact, and local payout-state transitions.
- Preserved free Marketplace installs while paid installs now create pending orders/payment intents and do not create `agent_installs` before verified webhook evidence.
- Wired `checkout.session.completed` with `checkout_kind=marketplace_install` through a Stripe-to-Marketplace adapter so duplicate webhooks create one install, one order effect, one settlement, and one lifecycle transition.
- Wired `refund.created` with `checkout_kind=marketplace_install` to update generic refund records plus Marketplace order/settlement refund state idempotently.
- Added `marketplace.GovernanceService` and HTTP routes for admin takedown, publisher appeal, admin reinstate, abuse report submission, and abuse report resolve/dismiss.
- Extended publisher stats with settlement-backed gross revenue, platform fees, net revenue, refunded amount, pending settlement, available, payout-pending, and paid-out amounts.
- Kept external payout provider execution out of scope; payout state is local-only in Phase 19.

## Tests Added

- `src/server/internal/marketplace/settlement_test.go`
  - `TestSettlementCreatePaidInstallCheckoutCreatesPendingOrderAndIntent`
  - `TestSettlementApplyPaidInstallCheckoutCompletedCreatesInstallAndSettlementOnce`
  - `TestSettlementApplyRefundAdjustsOrderAndSettlementOnce`
  - `TestSettlementPayoutStateIsLocalOnly`
  - `TestSettlementPublisherStatsIncludesSettlementAmounts`
- `src/server/internal/marketplace/governance_test.go`
  - `TestGovernanceTakedownPreventsNewInstallsAndPreservesHistory`
  - `TestGovernanceAppealAndReinstateRecordEvents`
  - `TestGovernanceAbuseReportLifecycle`
- `src/server/internal/http/stripe_handler_test.go`
  - `TestMarketplacePaidInstallDoesNotInstallBeforeWebhook`
  - `TestStripeWebhookRouteAppliesMarketplaceInstallSettlementOnce`
  - `TestStripeRefundUpdatesMarketplaceSettlementOnce`
- `src/server/internal/http/admin_marketplace_handler_test.go`
  - `TestMarketplaceGovernanceTakedownAppealAndReinstate`
  - `TestMarketplaceAbuseReportLifecycle`
  - `TestMarketplacePublisherStatsIncludesSettlementAmounts`

## TDD Evidence

- RED: `TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Marketplace.*(Takedown|Appeal|Abuse|PublisherStats)|StripeRefundUpdatesMarketplaceSettlementOnce' -count=1` failed because governance/abuse routes returned 404 and Marketplace refund webhooks left marketplace order/settlement refund state unchanged.
- GREEN: the same route coverage passed after wiring governance routes and Marketplace refund application through the Stripe lifecycle adapter.

## Verification

- `cd src/server && TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test -p 1 ./internal/marketplace ./internal/http -run 'Settlement|Governance|Abuse|Payout|PublisherStats|Marketplace.*(Paid|Settlement|Refund|Takedown|Appeal|Abuse|PublisherStats)|Stripe.*Marketplace' -count=1`
- `cd src/server && TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test -p 1 ./internal/marketplace ./internal/stripe ./internal/http -count=1`
- `bash scripts/check.sh docs`
- `GOPROXY=https://mirrors.aliyun.com/goproxy/,direct GOSUMDB=sum.golang.google.cn bash scripts/check.sh all`
- `git diff --check`

## Verification Note

The shared PostgreSQL test database must be used serially for package-level DB-backed tests. Running `./internal/marketplace` and `./internal/http` concurrently against the same database causes DDL race failures while test helpers rebuild schema. Use `go test -p 1` for multi-package DB-backed verification.

## Remaining v06 Work

Phase 19 does not complete v06. The following remain required:

- Phase 20: admin billing inspection for sessions, webhook events, subscriptions, top-ups, invoices, refunds, settlements, and payout state.
- Phase 20: v06 closeout evidence mapping `ADMIN-BILL-01` and `DOC-05` to code, tests, database proof, docs, and residual v07/v08 work.

## Remaining Commercial Program Work

The overall commercial-complete SaaS objective remains open:

- v07 Production Operations: Kubernetes or equivalent production orchestration, backup/restore smoke, observability, alerting, dashboards, and runbooks.
- v08 Product Completeness: real or disabled built-in MCP tools, durable Agent workflows, Knowledge behavior matching product copy, commercial UX, public docs, onboarding, pricing, and operator guides.
