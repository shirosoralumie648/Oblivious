# Production Marketplace Payout Provider

Date: 2026-07-01

## Commercial release blocker

Marketplace payout creation could fall back to the settlement service's local provider when no payout provider was injected. That is acceptable for development fixtures, but not for production because a payout row can be marked `payout_pending` without dispatching funds to a real payout rail.

## Changes

- Added `MARKETPLACE_PAYOUT_PROVIDER`, `MARKETPLACE_PAYOUT_WEBHOOK_URL`, and `MARKETPLACE_PAYOUT_WEBHOOK_SECRET` config.
- Production now rejects `MARKETPLACE_PAYOUT_PROVIDER=local`.
- `MARKETPLACE_PAYOUT_PROVIDER=webhook` requires a valid HTTP(S) endpoint and webhook secret.
- Added `marketplace.WebhookPayoutProvider`, which signs the JSON payout request body with HMAC-SHA256 and requires a non-empty `providerPayoutID` response.
- Wired router construction to inject the configured marketplace payout provider unless a test already injects one.
- 2026-07-02 update: service-level payout dispatch now returns `ErrMarketplacePayoutProviderRequired` when no provider is configured, so `MarkSettlementPayoutPending` and `CreateDuePayouts` no longer create local payout rows.
- Updated `.env.example` and Kubernetes ConfigMap/Secret examples with marketplace payout settings.

## Verification

- `git diff --check` passed with only existing LF-to-CRLF warnings.
- `gofmt -w src\server\internal\config\config.go src\server\internal\config\config_test.go src\server\internal\http\router.go src\server\internal\marketplace\payout_provider.go src\server\internal\marketplace\payout_provider_test.go` could not run because `gofmt` is not installed on PATH.
- 2026-07-01: `go test ./src/server/internal/config ./src/server/internal/marketplace ./src/server/internal/http` could not run because `go` was not installed on PATH.
- 2026-07-02: `go test ./internal/marketplace -run "TestSettlement(PayoutRequiresConfiguredProvider|CreateDuePayoutsRequiresConfiguredProvider|CreateDuePayoutsAggregatesAvailableSettlementsOnce|CreateDuePayoutsDispatchesConfiguredProvider|MarkPayoutPendingDispatchesConfiguredProvider|MarkPayoutPaidUpdatesPayoutAndSettlementsOnce|PublisherStatsIncludesSettlementAmounts)" -count=1 -v` passed with DB-backed tests skipped because `TEST_DATABASE_URL` is not set.
- 2026-07-02: `go test ./internal/http -run "TestAdminBillingCreateDueMarketplacePayoutsHandler(CallsSettlementService|RequiresConfiguredProvider)|TestAdminBillingCreateDueMarketplacePayoutsDispatchesConfiguredProvider" -count=1 -v` passed with the DB-backed configured-provider route test skipped because `TEST_DATABASE_URL` is not set.

## Residual risk

- The webhook provider is intentionally generic; a real production payout processor still needs to be deployed behind `MARKETPLACE_PAYOUT_WEBHOOK_URL`.
- `go.sum` is still not refreshed for the earlier ClickHouse driver dependency because the Go toolchain is unavailable in this environment.
