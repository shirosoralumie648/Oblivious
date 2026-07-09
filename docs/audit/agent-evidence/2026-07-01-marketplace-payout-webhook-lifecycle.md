# Marketplace Payout Webhook Lifecycle

## Scope

- Added a dedicated marketplace payout webhook endpoint:
  - `POST /api/v1/billing/marketplace-payout/webhook`
  - Headers: `Oblivious-Payment-Timestamp`, `Oblivious-Payment-Signature`
  - Signature: HMAC-SHA256 over `timestamp + "." + raw_body`.
- Added a payout-only HTTP handler so the marketplace payout secret cannot process checkout, refund, or subscription events.
- Added OpenAPI schema/path coverage for `MarketplacePayoutWebhookEvent`.

## Code Evidence

- `src/server/internal/http/marketplace_payout_webhook_handler.go`
  - Verifies the domestic provider HMAC signature.
  - Accepts only `payout.paid` and `payout.failed`.
  - Records webhook events through the provider webhook ledger.
  - Applies `marketplace.ProviderPayoutLifecycleEvent` through `SettlementService.ApplyProviderPayoutLifecycle`.
- `src/server/internal/http/router.go`
  - Registers `/api/v1/billing/marketplace-payout/webhook`.
  - Builds the configured marketplace payout provider from `MARKETPLACE_PAYOUT_PROVIDER=webhook`.
- `src/server/internal/stripe/ledger.go`
  - Changes webhook idempotency from global `event_id` to provider-scoped `(provider, event_id)`.
- `src/server/migrations/0084_provider_scoped_webhook_events.sql`
  - Drops legacy Stripe-only provider checks for webhook/lifecycle/refund/invoice tables.
  - Adds provider non-empty checks.
  - Adds provider-scoped webhook event uniqueness.

## Tests Added

- `src/server/internal/http/marketplace_payout_webhook_handler_test.go`
  - Duplicate `payout.paid` events are recorded/applied once.
  - Signed non-payout events are rejected before ledger/lifecycle effects.
- Existing in-memory webhook ledger test fakes were updated to match provider-scoped idempotency.

## Verification

- `git diff --check` passes.
- `go test ./src/server/internal/http ./src/server/internal/stripe ./src/server/internal/marketplace` could not run because `go` is not available in this environment.
- `gofmt` could not run because `gofmt` is not available in this environment.
