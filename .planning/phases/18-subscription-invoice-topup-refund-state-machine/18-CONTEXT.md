# Phase 18 Context: Subscription Invoice Top-up Refund State Machine

## Milestone

v06 Billing And Marketplace Operations.

## Why This Phase Exists

Phase 17 proved that Stripe checkout and webhook routes are mounted, tenant-aware, signature-verified, and idempotently recorded. That is necessary but not sufficient for commercial money movement: provider events are currently only ingested into `stripe_webhook_events`; they do not yet mutate subscriptions, invoices, top-ups, failed-payment state, plan changes, or refunds.

The commercial-complete program requires these payment effects to be implemented as auditable state transitions. Phase 18 consumes the Phase 17 payment authority foundation and applies provider events to local billing state exactly once.

## Requirement

- **PAY-03:** Subscription lifecycle, invoices, refunds, failed-payment states, plan changes, and top-ups are implemented as auditable state transitions.

## Current Evidence And Gaps

Existing Phase 17 assets:

- `POST /api/v1/billing/checkout` creates `payment_intents` with `organization_id`, `user_id`, `package_id`, `kind`, amount, currency, status, and metadata.
- `POST /api/v1/billing/stripe/webhook` verifies the raw Stripe signature and records one `stripe_webhook_events` row per provider event ID.
- `checkout.session.completed` metadata preserves `organization_id`, `user_id`, `payment_intent_id`, `plan_id`, and `checkout_kind`.

Current gaps:

- `checkout.session.completed` does not create or update `subscriptions`.
- `checkout.session.completed` for top-ups is not accepted by checkout and does not create or fulfill `topup_orders`.
- `invoice.paid`, `invoice.payment_failed`, `customer.subscription.updated`, `customer.subscription.deleted`, and refund events are not applied locally.
- `payment_intents.status` is not the source of an audited lifecycle; it is only an authority record for the checkout attempt.
- Existing `POST /api/v1/app/quota/topup` directly credits quota without payment evidence. That path is not acceptable as a commercial paid top-up flow.
- Admin inspection remains Phase 20, but Phase 18 must store enough structured state for Phase 20 to expose it.

## Boundaries

Included:

- Add a billing lifecycle schema for invoices, refunds, and append-only transition records.
- Extend payment intent, subscription, and top-up state where needed to connect provider IDs to local rows.
- Add a Stripe lifecycle service that applies supported webhook events exactly once after ledger insertion succeeds.
- Apply subscription checkout completion, top-up fulfillment, invoice paid/failed state, subscription update/delete state, plan changes, and refunds.
- Support payment-backed top-up checkout and prevent the legacy direct top-up endpoint from silently crediting commercial quota.
- Add DB-backed tests for lifecycle idempotency and route-level webhook application.

Excluded:

- Marketplace publisher settlement, platform fees, payout state, and refund impact. Those remain Phase 19.
- Admin billing inspection APIs/UI. Those remain Phase 20, although this phase must create queryable state for them.
- Live Stripe network tests. Automated tests must keep using signed fixtures and fake checkout creators.
- v06 closeout or final commercial-readiness claims.

## Lifecycle Design

Phase 18 should add a narrow lifecycle boundary inside `src/server/internal/stripe`:

- `LifecycleService` parses already verified Stripe events and orchestrates state changes.
- `LifecycleStore` owns SQL writes for payment intents, subscriptions, top-up orders, invoices, refunds, and transition records.
- `WebhookHandler` continues to verify signatures and record `stripe_webhook_events`; it calls the lifecycle service only when `RecordWebhookEvent` reports a newly inserted provider event.
- `billing_lifecycle_events` is the append-only audit trail for business state transitions. It avoids unsafe `audit_logs.actor_id = 'system'` writes against the current user FK shape.

Provider event application should be idempotent at two layers:

1. `stripe_webhook_events.event_id` prevents duplicate webhook delivery from being applied twice.
2. `billing_lifecycle_events.transition_key` prevents repeated local effects if a handler is retried after partial work.

## Event Coverage

Minimum supported events for PAY-03:

- `checkout.session.completed`
  - `checkout_kind=subscription`: complete the local payment intent, create or update the organization-scoped subscription, update user plan assignment, and record a transition.
  - `checkout_kind=topup`: complete the local payment intent, mark the top-up order paid, credit quota exactly once, and record a transition.
- `invoice.paid`
  - upsert invoice state, mark related subscription active/current, clear failed-payment state, apply scheduled plan change when present, and record a transition.
- `invoice.payment_failed`
  - upsert invoice state, mark related subscription `past_due`, mark related payment intent failed when known, and record a transition.
- `customer.subscription.updated`
  - update provider subscription IDs, period fields, cancel-at-period-end state, and plan-change state.
- `customer.subscription.deleted`
  - mark subscription cancelled without deleting tenant billing history.
- `charge.refunded` and `refund.created`
  - upsert refund records, mark full/partial refund state, reverse paid top-up quota only through an audited transition, and leave Marketplace refund impact for Phase 19.

## Verification Targets

Focused RED/GREEN:

- `cd src/server && go test ./internal/stripe -run 'Lifecycle|CheckoutCompleted|Topup|Invoice|Refund|Subscription' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook|Topup|Invoice|Refund|Subscription' -count=1`

Broader phase verification:

- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe ./internal/http ./internal/quota -count=1`
- `bash scripts/check.sh docs`
- `GOPROXY=https://mirrors.aliyun.com/goproxy/,direct GOSUMDB=sum.golang.google.cn bash scripts/check.sh all`
- `git diff --check`

## Residual Risk After This Phase

Phase 18 should close PAY-03 only. The product remains non-commercial-complete until Phase 19 models Marketplace settlement/governance, Phase 20 exposes admin billing evidence and closes v06, v07 proves production operations, and v08 removes remaining product placeholders.
