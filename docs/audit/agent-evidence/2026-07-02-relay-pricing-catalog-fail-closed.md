# Relay Pricing Catalog Fail-Closed Evidence

Date: 2026-07-02

## Scope

Commercial relay billing must not depend on runtime hardcoded price estimates. Production startup must load an active SQL price catalog, and request execution must fail closed when the selected model/API/usage dimension has no configured price.

## Implementation

- Added `relay_pricing_entries` migration as the runtime price authority, including chat/responses/completions token prices, image count prices, embeddings prompt-token prices, and model-agnostic `files` `storage_bytes` pricing.
- Added `LoadPricingStoreFromSQL`, strict cost calculation, `ErrRelayPriceNotConfigured`, and model-agnostic file pricing support in `src/server/internal/relay/pricing.go`.
- Wired `src/server/cmd/relay/main.go` and `src/server/internal/http/server.go` to load SQL prices and fail production startup if catalog/settings loading fails.
- Hardened `src/server/internal/relay/relay.go` so production construction requires a configured, non-empty pricing store.
- Hardened `src/server/internal/relay/router.go` so missing prices return `relay_pricing_not_configured` and do not call upstream.
- Kept non-production defaults only as a development fallback after failed SQL catalog load.

## Verification

Commands run from `src/server`:

```bash
go test ./internal/relay -run "Test(Pricing|LoadPricing|BillingHook|NewRelay|RouteWithBilling|Router)" -count=1
go test ./cmd/relay -run Test -count=1
go test ./internal/http -run "TestCombineHandlersRelayAliasesReachProductionRelayPolicy|TestRouteSurface.*Pricing|TestAdminHandlerUpdatesRelayPricingSettings|TestRelay" -count=1
GIN_MODE=release go test ./internal/relay -count=1
```

All commands passed after the pricing catalog and router error-classification changes.

## Remaining Gap

The SQL catalog closes the runtime hardcoded-price blocker. Later 2026-07-02 slices added immutable price snapshots on billable Relay usage records, ledger-vs-snapshot reconciliation, manual provider-source catalog import/diff/reject/approval/rollback/event audit, admin-triggered LiteLLM sync-to-pending-import, repository-local maintenance run evidence, and non-nil usage billing for supported audio/moderation routes. Final commercial billing still needs target-runtime provider price freshness proof and broad cross-system reconciliation across provider exports, request logs, checkout, webhook, quota, refund, and payout.
