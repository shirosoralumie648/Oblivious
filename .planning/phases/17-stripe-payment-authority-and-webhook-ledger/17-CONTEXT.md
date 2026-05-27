# Phase 17 Context: Stripe Payment Authority and Webhook Ledger

## Milestone

v06 Billing And Marketplace Operations.

## Why This Phase Exists

The commercial-complete program requires Stripe checkout and webhook handlers to be mounted on running routes, signature-verified, idempotent, tenant-aware, and covered by integration tests before subscription lifecycle, top-up fulfillment, invoice/refund handling, or Marketplace settlement can be trusted.

The repository already contains `src/server/internal/stripe/checkout.go` and `src/server/internal/stripe/webhook.go`, but the current implementation is not sufficient for v06:

- Stripe routes are not mounted in `src/server/internal/http/router.go`.
- Checkout metadata only carries `user_id` and `plan_id`; it does not carry `organization_id`, checkout kind, or top-up intent metadata.
- Webhook idempotency uses `audit_logs` instead of a dedicated provider event ledger.
- `audit_logs.actor_id = 'system'` is unsafe against the current foreign key shape.
- `subscriptions` now require `organization_id`, but the current webhook insert path does not set it.

## Requirements

- **PAY-01:** Stripe checkout routes are mounted in the running server, require authenticated tenant context, persist payment intent metadata, and can be tested without live Stripe keys.
- **PAY-02:** Stripe webhook route verifies signatures from the raw request body, records provider events idempotently, rejects invalid signatures, and preserves processing status/errors for admin inspection.

## Boundaries

Included:

- `POST /api/v1/billing/checkout` authenticated route.
- `POST /api/v1/billing/stripe/webhook` public signature-verified route.
- A dedicated Stripe/payment event ledger table and store.
- A checkout session creator interface so tests do not call Stripe network APIs.
- DB-backed tests for mounted routes, signature rejection, signed fixture acceptance, and duplicate event idempotency.

Excluded from Phase 17:

- Full subscription lifecycle mutation.
- Top-up fulfillment and quota credit after payment.
- Invoice, refund, failed-payment, and plan-change state machines.
- Marketplace publisher settlement, platform fee, payout state, and refund impact.
- Admin UI pages for billing state.

Those remain required in Phases 18-20.

## External Contract Notes

Stripe webhook verification must use the exact raw request body and the `Stripe-Signature` header. Tests should use Stripe's webhook test signing helpers instead of hand-rolled signatures.

## Verification Targets

- RED/GREEN focused tests:
  - `cd src/server && go test ./internal/stripe -run 'Webhook|Checkout|Ledger' -count=1`
  - `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook' -count=1`
- Broader phase check:
  - `cd src/server && go test ./internal/stripe ./internal/http ./internal/config -count=1`
  - `bash scripts/check.sh docs`
  - `git diff --check`

## Residual Risk After This Phase

Phase 17 only proves payment route authority and event ingestion. The product remains non-commercial-complete until later v06 phases apply payment events to subscription/top-up/refund state, model Marketplace settlement and payouts, expose admin inspection surfaces, and close v06 evidence.
