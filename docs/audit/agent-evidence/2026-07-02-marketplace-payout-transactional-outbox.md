# Marketplace Payout Transactional Outbox - 2026-07-02

This slice closes the payout crash window where an external payout provider could be called before the local payout ledger and settlement binding committed.

## What Changed

- `MarkSettlementPayoutPending` now validates the configured payout provider name, writes `marketplace_payouts` plus the settlement `payout_id` binding in one SQL transaction, commits, and only then dispatches the provider payout.
- `CreateDuePayouts` now writes due payout rows and settlement bindings before provider dispatch. Provider calls use the committed payout id as the idempotency key.
- Existing `marketplace_payouts` rows with `status='payout_pending'` and empty `provider_payout_id` are treated as queued dispatch outbox rows. The next `CreateDuePayouts` run retries them with the same payout id instead of creating a replacement payout.
- Dispatch success updates `provider_payout_id` and `metadata.dispatch_status='dispatched'`; dispatch failure records the error in payout metadata while preserving the committed payout ledger for retry or inbound webhook reconciliation.

## Files

- `src/server/internal/marketplace/settlement.go`
- `src/server/internal/marketplace/settlement_test.go`

## Verification

```text
command: go test ./internal/marketplace -run "TestSettlement(MarkPayoutPending|CreateDuePayouts).*Provider|TestSettlement(MarkPayoutPending|CreateDuePayouts)CommitFailureDoesNotCallProvider|TestSettlementCreateDuePayoutsRetriesQueuedDispatchWithSamePayoutID|TestSettlementPayoutRequiresConfiguredProvider|TestWebhookPayoutProvider" -count=1 -v
result: pass

command: go test ./internal/marketplace -count=1
result: pass

command: go test ./internal/http -run "TestAdminBilling(CreateDueMarketplacePayouts|MarksMarketplacePayout|MarkPayout|MarketplacePayout)|TestMarketplacePayoutWebhook|TestDomesticPaymentWebhookRouteAppliesMarketplacePayout" -count=1
result: pass

command: GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-money-movement
result: pass with disposable pgvector PostgreSQL and skipped tests: none
```

Covered tests include:

- `TestSettlementMarkPayoutPendingCommitFailureDoesNotCallProvider`
- `TestSettlementCreateDuePayoutsCommitFailureDoesNotCallProvider`
- `TestSettlementCreateDuePayoutsRetriesQueuedDispatchWithSamePayoutID`
- Webhook payout provider request/signature tests
- Admin Billing and domestic payout webhook handler tests

## Remaining Boundary

This is repository-local payout dispatch durability. Final commercial payout readiness still requires target-environment outbound payout dispatch, inbound payout webhook lifecycle, settlement ledger reconciliation, refund/chargeback handling, and live Stripe/Alipay/WeChat Pay provider rail evidence.
