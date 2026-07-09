# Relay Usage Price Snapshot Evidence

Date: 2026-07-02

## Scope

Billable Relay usage must preserve the exact pricing interpretation used at request time so later catalog changes do not mutate billing, refund, or reconciliation evidence.

## Implementation

- Added `PricingQuote` and dimension-level catalog snapshots in `src/server/internal/relay/pricing.go`.
- Preserved the existing `CalculateCost*` APIs while routing billing preauthorization, pending streaming usage, and final usage records through the same quote path in `src/server/internal/relay/router.go`.
- Added `price_snapshot`, `price_currency`, `price_source`, and `price_effective_from` fields to Relay usage records and SQL persistence in `src/server/internal/relay/usage.go`, `src/server/internal/usage/store.go`, and `src/server/migrations/0088_relay_usage_price_snapshots.sql`.
- Exposed the persisted snapshot fields through Admin usage log reads and frontend API types in `src/server/internal/admin/usage_log_store.go` and `src/web/src/types/admin.ts`.
- Registered `/api/v1/admin/usage-logs` and `/api/v1/admin/usage-analytics` in the main router and documented them in OpenAPI/route-surface manifest so the Admin Usage Logs page reaches the backend audit surface.
- Follow-up evidence in `docs/audit/agent-evidence/2026-07-02-relay-usage-price-reconciliation.md` adds an Admin reconciliation endpoint for ledger cost versus immutable snapshot cost.
- Covered SQL catalog metadata loading, group multiplier snapshots, streaming pending/final snapshot replacement, SQL writer JSON persistence, and migration shape with focused tests.

## Verification

Commands run from `src/server`:

```bash
go test ./internal/relay -run "TestPricing|TestLoadPricingStoreFromSQL|TestRouterRouteWithBilling(AppliesTrustedUserGroupPricingMultiplier|RecordsPendingUsageBeforeStreamingProvider)" -count=1 -v
go test ./internal/usage -run "TestSQLRecorder_RecordRelayUsageWritesGatewayFields|TestRelayUsagePriceSnapshotsMigration" -count=1 -v
go test ./internal/admin -run "TestServiceListUsageLogsNormalizesFiltersAndReturnsGatewayFields|TestSQLStoreListUsageLogsFallsBackFromZeroTotalTokens" -count=1 -v
go test ./internal/http -run "TestAdminHandlerListsUsageLogsWithFilters" -count=1 -v
go test ./internal/http -run "Test(RouteSurfaceAdminRoutesRequireAdmin|RouteSurfaceAdminSubRoutes|RouteSurfaceManifestAdminRoutesDispatch|AdminHandlerListsUsageLogsWithFilters|AdminHandlerGetsUsageAnalyticsWithFilters)" -count=1 -v
python scripts/verify_openapi_contract.py
```

Both commands passed.

## Remaining Gap

Immutable per-request snapshots are now in the durable usage ledger, and a first Admin ledger-vs-snapshot reconciliation report exists. Final commercial billing still needs provider price import/sync, approval/audit history, and scheduled reconciliation across checkout, webhook, quota settlement, refunds, and target-runtime request logs.
