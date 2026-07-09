# Marketplace Payout Provider Required Evidence

Date: 2026-07-02

## Summary

Marketplace payout dispatch now fails closed when no payout provider is configured.

Before this change, `dispatchPayout` returned provider `local` with no provider payout id when `payoutProvider` was nil. That let payable settlements move into `payout_pending` without any external payout rail.

## Code Changes

- `src/server/internal/marketplace/settlement.go`
  - Added `ErrMarketplacePayoutProviderRequired`.
  - `dispatchPayout` now returns that error when the service has no configured payout provider.
  - `CreateDuePayouts` fails before locking/selecting due settlements when no provider is configured.
- `src/server/internal/http/admin_handler.go`
  - Maps provider-required payout errors to `503 service_unavailable` for `POST /api/v1/admin/billing/payouts/create-due`.
  - Serializes empty settlement lists as `[]` instead of `null` for Admin Billing response stability.
- `src/server/internal/marketplace/settlement_test.go`
  - Replaced the local payout-state test with provider-required fail-closed coverage.
  - Added due-payout fail-closed coverage and kept configured-provider dispatch coverage.
- `scripts/verify-commercial-db-evidence.sh`
  - Replaced the old local-payout test token with provider-required fail-closed tests.

## Verification

```bash
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-money-movement
```

Result: PASS with disposable pgvector PostgreSQL and `skipped tests: none`.

Focused non-DB handler command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run 'TestAdminBilling(CreateDueMarketplacePayoutsHandler(CallsSettlementService|RequiresConfiguredProvider)|CreateDueMarketplacePayoutsDispatchesConfiguredProvider|MarketplaceSettlementsResponseContract)' -count=1 -v
```

Result: PASS; the DB-backed configured-provider route subtest skips without `TEST_DATABASE_URL`, and the full no-skip DB evidence is covered by the disposable PostgreSQL profile above.

## Remaining Gap

Commercial payout readiness still requires target-runtime evidence for:

- outbound payout dispatch through the configured provider,
- inbound provider payout webhook lifecycle,
- refund/chargeback/reconciliation behavior,
- target-runtime evidence beyond the repository-local disposable PostgreSQL profile.
