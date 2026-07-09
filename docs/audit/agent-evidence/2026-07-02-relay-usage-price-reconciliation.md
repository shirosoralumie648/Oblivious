# Relay Usage Price Reconciliation Evidence

Date: 2026-07-02

## Scope

Operators need a billing-grade read surface that proves durable Relay usage ledger costs still match the immutable pricing snapshots written at request time. This closes the first reconciliation step after persisted price snapshots; a later 2026-07-02 slice adds manual provider-source catalog import and approval.

## Implementation

- Added `GET /api/v1/admin/billing/reconciliation/relay-usage-prices` as an admin-only billing inspection endpoint.
- The endpoint filters Relay usage rows by organization, user, API token, request ID, API type, feature type, quota mode, model, channel, provider, status, time range, and pagination.
- `src/server/internal/admin/usage_log_store.go` compares `usage_records.cost` with `usage_records.price_snapshot->>'totalCost'`, separately counting missing snapshots and cost mismatches.
- The response includes checked, matched, missing-snapshot, and mismatched record counts, ledger/snapshot/delta totals, and sampled issue rows with `missing_snapshot` or `cost_mismatch`.
- OpenAPI, route-surface manifest, route-surface tests, and the frontend Admin API client now expose the reconciliation contract.

## Verification

Commands run:

```bash
go test ./internal/admin -count=1
go test ./internal/http -count=1
npm test -- --run src/features/admin/api.test.ts
python scripts/verify_openapi_contract.py
```

All commands passed.

## Remaining Gap

This endpoint reconciles persisted ledger cost against the immutable request-time snapshot. Later maintenance-run ledger work records scheduled reconciliation runs, but commercial completion still needs broader reconciliation across checkout, webhook, quota settlement, refunds, provider usage exports, payout, and target-runtime request logs, plus target price freshness proof.
