# Phase 18 Summary: Subscription Invoice Top-up Refund State Machine

## Status

Complete.

## Requirement Closed

- **PAY-03:** Subscription lifecycle, invoices, refunds, failed-payment states, plan changes, and top-ups are implemented as auditable state transitions.

## Delivered

- Added `src/server/migrations/0029_billing_lifecycle.sql` with:
  - `billing_lifecycle_events` for append-only, idempotent provider transition records.
  - `billing_invoices` for invoice state, amounts, provider IDs, and hosted invoice references.
  - `billing_refunds` for refund state, provider IDs, top-up linkage, and payload retention.
  - Provider reconciliation columns on `payment_intents`, `subscriptions`, and `topup_orders`.
- Added `src/server/internal/stripe/lifecycle.go` with `LifecycleService`, `SQLLifecycleStore`, and `ApplyStripeEvent` support for:
  - `checkout.session.completed`
  - `invoice.paid`
  - `invoice.payment_failed`
  - `customer.subscription.updated`
  - `customer.subscription.deleted`
  - `refund.created`
  - `charge.refunded`
- Wired the Stripe webhook handler to apply lifecycle state after signature verification and ledger recording.
- Allowed duplicate webhook deliveries to retry lifecycle application. Business effects remain idempotent through `billing_lifecycle_events.transition_key`.
- Added subscription checkout completion behavior that completes local payment intents, creates or updates organization-scoped subscriptions, updates user plan assignment, and records one lifecycle transition.
- Added payment-backed top-up checkout behavior that creates pending top-up orders without crediting quota before a verified webhook event.
- Disabled direct quota top-up crediting through `/api/v1/app/quota/topup`; callers must use `/api/v1/billing/checkout` with `kind=topup`.
- Added invoice success/failure behavior that upserts invoice state, marks subscriptions active or past due, applies pending plan changes, and records failed-payment state.
- Added subscription update/delete behavior that preserves provider subscription/customer IDs, cancellation state, and local cancellation history.
- Added refund behavior that records provider refunds, updates payment-intent refund state, reverses paid top-up quota once, and leaves Marketplace refund impact for Phase 19.
- Aligned lifecycle integration tests with explicit `TEST_DATABASE_URL` semantics and shared advisory locking so package-level DB tests can run together safely.

## Tests Added

- `src/server/internal/stripe/lifecycle_test.go`
  - `TestLifecycleApplyCheckoutSessionCompletedCreatesSubscriptionOnce`
  - `TestLifecycleApplyCheckoutSessionCompletedFulfillsTopupOnce`
  - `TestLifecycleApplyInvoicePaidAndPaymentFailedTransitions`
  - `TestLifecycleApplySubscriptionUpdatedAndDeletedTransitions`
  - `TestLifecycleApplyRefundRecordsRefundAndAdjustsTopup`
- `src/server/internal/http/stripe_handler_test.go`
  - `TestStripeWebhookRouteAppliesCheckoutCompletedSubscriptionOnce`
  - `TestStripeWebhookRouteRetriesLifecycleForRecordedDuplicateEvent`
  - `TestBillingCheckoutTopupDoesNotCreditQuotaBeforeWebhook`
  - `TestQuotaTopupEndpointNoLongerCreditsWithoutPayment`

## TDD Evidence

- RED: `TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'TestStripeWebhookRouteRetriesLifecycleForRecordedDuplicateEvent' -count=1` failed because duplicate webhook delivery returned `200` but left payment status `pending` and no lifecycle transition.
- GREEN: the same test passed after duplicate webhook deliveries were allowed to re-run lifecycle application through transition-key idempotency.

## Verification

- `cd src/server && TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe -run 'Lifecycle|CheckoutCompleted|Topup|Invoice|Refund|Subscription' -count=1`
- `cd src/server && TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook|Topup|Invoice|Refund|Subscription' -count=1`
- `cd src/server && TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe ./internal/http ./internal/quota -count=1`
- `bash scripts/check.sh docs`
- `GOPROXY=https://mirrors.aliyun.com/goproxy/,direct GOSUMDB=sum.golang.google.cn bash scripts/check.sh all`
- `git diff --check`

## Remaining v06 Work

Phase 18 does not complete v06. The following remain required:

- Phase 19: Marketplace publisher revenue, platform fee, payout state, refund impact, moderation, and abuse workflows.
- Phase 20: admin billing inspection surfaces and v06 closeout evidence.

## Remaining Commercial Program Work

The overall commercial-complete SaaS objective remains open:

- v07 Production Operations: Kubernetes or equivalent production orchestration, backup/restore smoke, observability, alerting, dashboards, and runbooks.
- v08 Product Completeness: real or disabled built-in MCP tools, durable Agent workflows, Knowledge behavior matching product copy, commercial UX, public docs, onboarding, pricing, and operator guides.
