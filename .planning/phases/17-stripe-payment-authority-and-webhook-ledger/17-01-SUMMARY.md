# Phase 17 Summary: Stripe Payment Authority and Webhook Ledger

## Status

Complete.

## Requirements Closed

- **PAY-01:** Stripe checkout routes are mounted in the running server, require authenticated tenant context, persist payment intent metadata, and can be tested without live Stripe keys.
- **PAY-02:** Stripe webhook route verifies signatures from the raw request body, records provider events idempotently, rejects invalid signatures, and preserves processing status/errors for admin inspection.

## Delivered

- Added `src/server/migrations/0028_payment_authority.sql` with:
  - `payment_intents` for tenant-aware checkout intent state.
  - `stripe_webhook_events` for provider webhook idempotency and processing status.
- Mounted `POST /api/v1/billing/checkout` behind authenticated session and CSRF middleware.
- Mounted `POST /api/v1/billing/stripe/webhook` as a public endpoint guarded by Stripe raw-body signature verification.
- Refactored Stripe webhook handling away from `audit_logs` idempotency into a dedicated `WebhookLedger`.
- Added checkout creator injection through `NewRouterWithOptions` so route tests use a fake checkout session creator instead of live Stripe network calls.
- Checkout now pre-creates the local `payment_intents` row before calling Stripe, includes that internal ID in Stripe metadata, then backfills the provider checkout session ID. This avoids external checkout sessions without a local reconciliation record.
- Extended checkout metadata with `organization_id`, `user_id`, `payment_intent_id`, `plan_id`, and `checkout_kind`.
- Added Stripe config fields for secret key, success/cancel URLs, and webhook secret.

## Tests Added

- `src/server/internal/stripe/webhook_test.go`
  - invalid signatures return `400` and record nothing.
  - signed `checkout.session.completed` fixtures record a processed webhook event.
  - duplicate Stripe event IDs are recorded once.
- `src/server/internal/http/stripe_handler_test.go`
  - webhook route is mounted and rejects invalid signatures.
  - signed webhook route fixture records one PostgreSQL `stripe_webhook_events` row across duplicate delivery attempts.
  - checkout requires a session.
  - authenticated checkout pre-creates and then backfills tenant payment intent state with a fake checkout creator.

## Verification

- `cd src/server && go test ./internal/stripe ./internal/config -count=1`
- `cd src/server && TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook' -count=1`
- `cd src/server && TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe ./internal/http ./internal/config ./internal/quota -count=1`
- `bash scripts/check.sh docs`
- `GOPROXY=https://mirrors.aliyun.com/goproxy/,direct GOSUMDB=sum.golang.google.cn bash scripts/check.sh all`
- `git diff --check`

## Remaining v06 Work

Phase 17 does not complete v06. The following remain required:

- Phase 18: subscription lifecycle, invoices, failed-payment states, plan changes, top-up fulfillment, and refunds.
- Phase 19: Marketplace publisher settlement, platform fee, payout state, refund impact, and moderation/abuse workflows.
- Phase 20: admin billing inspection surfaces and v06 closeout evidence.
